package main

import (
	"os"

	"github.com/spf13/cobra"

	"github.com/nicomaioli-skinandme/dotfiles/sam/internal/config"
	"github.com/nicomaioli-skinandme/dotfiles/sam/internal/tmuxx"
	"github.com/nicomaioli-skinandme/dotfiles/sam/internal/tui"
)

func newMenuCmd() *cobra.Command {
	return &cobra.Command{
		Use:    "menu",
		Short:  "Interactive picker (default when sam is run with no subcommand)",
		Hidden: true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runMenu(tui.ResWorktrees)
		},
	}
}

// runMenu launches the full-screen TUI on the given starting resource,
// then performs the action it returned. The TUI never grabs tmux itself
// (attaching replaces the process image); it hands back a tui.Result and
// we act on it here, after the program has released the terminal.
func runMenu(start tui.Resource) error {
	name, workspace, cfg, err := loadWorkspaceAndConfig()
	if err != nil {
		return err
	}

	// When cwd doesn't resolve to a workspace and --workspace wasn't
	// given, open on the workspace-select view so the user picks one
	// explicitly. The TUI will switch m.workspace once they pick.
	if workspace == nil {
		start = tui.ResWorkspaces
	}

	res, err := tui.Run(name, workspace, cfg.Workspaces, start, cfg.Tui)
	if err != nil {
		return err
	}

	// Post-TUI actions must use the workspace the user ended on, not
	// the one we loaded at startup. The TUI carries the active
	// workspace back through Result for exactly this reason.
	ws := workspace
	wsName := name
	if res.Workspace != nil {
		ws = res.Workspace
		wsName = res.WorkspaceName
	}

	switch {
	case res.RunWizard:
		// The wizard owns the terminal; run it, then drop back into the menu.
		if err := runWorkspaceAdd(os.Stdout); err != nil {
			return err
		}
		return runMenu(tui.ResWorkspaces)

	case res.NewWorktreeBranch != "":
		return runNewWorktree(wsName, ws, res.NewWorktreeBranch)

	case res.Attach != "":
		if res.Build != nil {
			if err := buildForAttach(res.Attach, ws, res.Build); err != nil {
				return err
			}
		}
		return tmuxx.SwitchOrAttach(res.Attach)
	}

	return nil // user quit
}

// buildForAttach creates the tmux session named by attach before the
// caller switches to it.
func buildForAttach(attach string, workspace *config.Workspace, spec *tui.BuildSpec) error {
	return tmuxx.BuildSession(attach, workspace, spec.BaseDir)
}

// shouldDefaultToMenu reports whether `sam` was invoked with no
// subcommand and no top-level help request. --workspace (which carries
// a value, attached or detached) does not count as a subcommand.
func shouldDefaultToMenu(args []string) bool {
	skipNext := false
	for _, a := range args {
		if skipNext {
			skipNext = false
			continue
		}
		if a == "--" {
			return true
		}
		if a == "--help" || a == "-h" || a == "--version" {
			return false
		}
		if len(a) > 0 && a[0] == '-' {
			// Flags that take a detached value consume the next arg, so it
			// isn't mistaken for a subcommand. Attached forms (--flag=v,
			// -ov) are single args and fall through the continue below.
			if a == "--workspace" || a == "--output" || a == "-o" {
				skipNext = true
			}
			continue
		}
		return false
	}
	return true
}

// maybeDefaultToMenu wires `sam` (no subcommand) to invoke `sam menu`.
func maybeDefaultToMenu(root *cobra.Command) {
	if !shouldDefaultToMenu(os.Args[1:]) {
		return
	}
	newArgs := append([]string{"menu"}, os.Args[1:]...)
	root.SetArgs(newArgs)
}
