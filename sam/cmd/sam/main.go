package main

import (
	"errors"
	"fmt"
	"os"
	"runtime/debug"
	"strings"
	"syscall"

	"github.com/spf13/cobra"

	"github.com/nicomaioli-skinandme/dotfiles/sam/internal/config"
	_ "github.com/nicomaioli-skinandme/dotfiles/sam/internal/ghx"
	_ "github.com/nicomaioli-skinandme/dotfiles/sam/internal/gitx"
	_ "github.com/nicomaioli-skinandme/dotfiles/sam/internal/tmuxx"
	"github.com/nicomaioli-skinandme/dotfiles/sam/internal/ui"
	"github.com/nicomaioli-skinandme/dotfiles/sam/internal/wizard"
)

var (
	workspaceFlag string
	humanFlag     bool
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
	root.PersistentFlags().BoolVarP(&humanFlag, "human", "H", false,
		"human-readable output (table) where supported")
	root.AddCommand(newConfigPrintCmd())
	root.AddCommand(newClankersCmd())
	root.AddCommand(newListCmd())
	root.AddCommand(newNewWorktreeCmd())
	root.AddCommand(newDeleteCmd())
	root.AddCommand(newFromIssueCmd())
	root.AddCommand(newMenuCmd())
	root.AddCommand(newWorkspaceCmd())
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
//
// First-run side effect: when ~/.config/sam/config.toml is missing,
// runs the setup wizard, saves the result, and re-execs `sam` with
// no arguments so the user lands in a clean menu. This call does not
// return in that case.
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
func loadWorkspaceAndConfig() (string, *config.Workspace, *config.Config, error) {
	path, err := config.DefaultPath()
	if err != nil {
		return "", nil, nil, err
	}
	if _, err := os.Stat(path); errors.Is(err, os.ErrNotExist) {
		if err := runFirstRunWizard(path); err != nil {
			return "", nil, nil, err
		}
		// runFirstRunWizard either re-execs (no return) or returns
		// nil after the user cancelled, in which case we exit.
		os.Exit(0)
	} else if err != nil {
		return "", nil, nil, err
	}
	cfg, err := config.Load(path)
	if err != nil {
		return "", nil, nil, err
	}
	cwd, _ := os.Getwd()
	name, ws, err := config.Resolve(cfg, workspaceFlag, cwd)
	if err != nil {
		return "", nil, nil, err
	}
	return name, ws, cfg, nil
}

func runFirstRunWizard(path string) error {
	fmt.Fprintln(os.Stderr, "sam: no config at "+path+" — launching setup wizard.")
	cfg, err := wizard.Run(nil)
	if err != nil {
		if errors.Is(err, ui.ErrCancelled) {
			return nil
		}
		return err
	}
	if err := config.Save(cfg, path); err != nil {
		return err
	}
	fmt.Fprintln(os.Stderr, "Wrote "+path)
	// Re-exec `sam` with no args so the user lands in a clean menu,
	// as if they had just invoked sam fresh. Whatever they originally
	// typed (e.g. `sam from-issue`) is intentionally discarded.
	bin, err := os.Executable()
	if err != nil {
		return err
	}
	return syscall.Exec(bin, []string{"sam"}, os.Environ())
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
