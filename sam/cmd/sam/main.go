package main

import (
	"errors"
	"fmt"
	"os"
	"runtime/debug"
	"strings"

	"github.com/spf13/cobra"

	"github.com/nicomaioli-skinandme/dotfiles/sam/internal/clanker"
	clankercli "github.com/nicomaioli-skinandme/dotfiles/sam/internal/clanker/cli"
	"github.com/nicomaioli-skinandme/dotfiles/sam/internal/config"
	configcli "github.com/nicomaioli-skinandme/dotfiles/sam/internal/config/cli"
	"github.com/nicomaioli-skinandme/dotfiles/sam/internal/issue"
	issuecli "github.com/nicomaioli-skinandme/dotfiles/sam/internal/issue/cli"
	"github.com/nicomaioli-skinandme/dotfiles/sam/internal/output"
	"github.com/nicomaioli-skinandme/dotfiles/sam/internal/pr"
	prcli "github.com/nicomaioli-skinandme/dotfiles/sam/internal/pr/cli"
	"github.com/nicomaioli-skinandme/dotfiles/sam/internal/session"
	sessioncli "github.com/nicomaioli-skinandme/dotfiles/sam/internal/session/cli"
	"github.com/nicomaioli-skinandme/dotfiles/sam/internal/tui"
	"github.com/nicomaioli-skinandme/dotfiles/sam/internal/wizard"
	"github.com/nicomaioli-skinandme/dotfiles/sam/internal/workspace"
	workspacecli "github.com/nicomaioli-skinandme/dotfiles/sam/internal/workspace/cli"
	"github.com/nicomaioli-skinandme/dotfiles/sam/internal/worktree"
	worktreecli "github.com/nicomaioli-skinandme/dotfiles/sam/internal/worktree/cli"
)

var (
	workspaceFlag string
	outputFlag    string
	logLevelFlag  string
)

func main() {
	root := &cobra.Command{
		Use:           "sam",
		Short:         "Slop+Me — tmux dev session manager",
		Version:       version(),
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	root.SetVersionTemplate("{{.Version}}\n")
	root.PersistentFlags().StringVar(&workspaceFlag, "workspace", "",
		"workspace name (overrides cwd-based resolution)")
	root.PersistentFlags().StringVarP(&outputFlag, "output", "o", "table",
		"output format: table (default) or json")
	root.PersistentFlags().StringVar(&logLevelFlag, "log-level", "",
		"minimum log level: debug, info, warn, or error (overrides [log] level / $SAM_LOG_LEVEL)")

	// Build the entity services + controllers once. Services wrap infra;
	// controllers wire in the cross-entity services they orchestrate.
	sessionSvc := session.Service{}
	worktreeSvc := worktree.Service{}
	issueSvc := issue.Service{}
	prSvc := pr.Service{}
	clankerSvc := clanker.Service{}

	sessionCtrl := session.NewController(sessionSvc)
	worktreeCtrl := worktree.NewController(worktreeSvc, sessionSvc)
	issueCtrl := issue.NewController(issueSvc, worktreeSvc, sessionSvc)
	prCtrl := pr.NewController(prSvc, worktreeSvc, sessionSvc)
	clankerCtrl := clanker.NewController(clankerSvc, sessionSvc)

	// Resolution + output are a single per-invocation concern, injected
	// into the Views as closures so the cli/ packages stay free of the
	// workspace resolver. mustResolve errors when cwd is ambiguous;
	// tryResolve leaves the workspace nil instead (for `config print`).
	mustResolve := func() (*config.Workspace, string, error) {
		name, ws, err := loadWorkspace()
		return ws, name, err
	}
	tryResolve := func() (*config.Workspace, string, error) {
		name, ws, _, err := loadWorkspaceAndConfig()
		return ws, name, err
	}
	loadCfg := func() (*config.Config, string, error) {
		return workspace.Service{}.LoadConfig()
	}
	parseFormat := func() (output.Format, error) {
		return output.Parse(outputFlag)
	}

	root.AddCommand(worktreecli.NewCmd(worktreeCtrl, mustResolve, parseFormat))
	root.AddCommand(issuecli.NewCmd(issueCtrl, mustResolve, parseFormat))
	root.AddCommand(prcli.NewCmd(prCtrl, mustResolve, parseFormat))
	root.AddCommand(clankercli.NewCmd(clankerCtrl, parseFormat))
	root.AddCommand(sessioncli.NewCmd(sessionCtrl, mustResolve))
	root.AddCommand(workspacecli.NewCmd(workspace.Service{}, parseFormat))
	root.AddCommand(configcli.NewCmd(loadCfg, tryResolve))

	// The TUI consumes the same controllers/services as the cli.
	deps := tui.Deps{
		Worktrees:   worktreeCtrl,
		WorktreeSvc: worktreeSvc,
		Issues:      issueCtrl,
		IssueSvc:    issueSvc,
		PRs:         prCtrl,
		Clankers:    clankerCtrl,
		SessionSvc:  sessionSvc,
		Setup:       wizard.Service{},
	}
	root.AddCommand(newMenuCmd(deps))
	root.AddCommand(&cobra.Command{
		Use:   "version",
		Short: "Print the build version (short commit hash)",
		RunE: func(cmd *cobra.Command, _ []string) error {
			fmt.Fprintln(cmd.OutOrStdout(), root.Version)
			return nil
		},
	})

	maybeDefaultToMenu(root)

	if err := root.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "sam:", err)
		os.Exit(1)
	}
}

// loadWorkspace resolves the active workspace for the current invocation.
// Returns (name, *Workspace) so callers that need the workspace name (e.g.
// the worktree-setup hook) don't have to look it up again.
//
// Non-menu callers require an active workspace; when cwd is ambiguous
// and --workspace was not given, loadWorkspace returns an error pointing
// the user at --workspace. The interactive menu uses
// loadWorkspaceAndConfig directly so it can open the workspace-select
// view in that case.
func loadWorkspace() (string, *config.Workspace, error) {
	name, ws, cfg, err := loadWorkspaceAndConfig()
	if err != nil {
		return name, ws, err
	}
	if ws == nil {
		return "", nil, fmt.Errorf("no workspace matches this directory: pass --workspace (have: %s)",
			workspaceNames(cfg))
	}
	return name, ws, nil
}

func workspaceNames(cfg *config.Config) string {
	names := make([]string, 0, len(cfg.Workspaces))
	for n := range cfg.Workspaces {
		names = append(names, n)
	}
	return strings.Join(names, ", ")
}

// loadWorkspaceAndConfig is loadWorkspace's underlying resolver: it
// returns the (possibly nil) resolved workspace alongside the full
// config. A nil ws with a nil err means "no workspace selected; ask
// the user." Non-menu callers should go through loadWorkspace
// instead, which surfaces an error in that case.
//
// A missing config file is an error here (the CLI is non-interactive);
// the menu handles first-run setup itself before calling this, so it
// never hits that path.
func loadWorkspaceAndConfig() (string, *config.Workspace, *config.Config, error) {
	path, err := config.DefaultPath()
	if err != nil {
		return "", nil, nil, err
	}
	if _, err := os.Stat(path); errors.Is(err, os.ErrNotExist) {
		return "", nil, nil, fmt.Errorf(
			"no config at %s — run `sam` to set up a workspace, or hand-write the file (see `sam config doctor`)",
			path)
	} else if err != nil {
		return "", nil, nil, err
	}
	cfg, err := config.Load(path)
	if err != nil {
		return "", nil, nil, err
	}
	cwd, _ := os.Getwd()
	active, err := workspace.Service{}.Resolve(cfg, workspaceFlag, cwd)
	if err != nil {
		return "", nil, nil, err
	}
	if active == nil {
		return "", nil, cfg, nil
	}
	return active.Name, active.WS, cfg, nil
}

func version() string {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return "unknown"
	}
	for _, s := range info.Settings {
		if s.Key != "vcs.revision" {
			continue
		}
		if len(s.Value) >= 7 {
			return s.Value[:7]
		}
		return s.Value
	}
	return "unknown"
}
