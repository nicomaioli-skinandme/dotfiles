package main

import (
	"os"

	"github.com/spf13/cobra"

	"github.com/nicomaioli-skinandme/dotfiles/sam/internal/session"
	"github.com/nicomaioli-skinandme/dotfiles/sam/internal/tui"
	workspacecli "github.com/nicomaioli-skinandme/dotfiles/sam/internal/workspace/cli"
	"github.com/nicomaioli-skinandme/dotfiles/sam/internal/worktree"
)

func newMenuCmd(deps tui.Deps, sessionCtrl session.Controller, worktreeCtrl worktree.Controller) *cobra.Command {
	return &cobra.Command{
		Use:    "menu",
		Short:  "Interactive picker (default when sam is run with no subcommand)",
		Hidden: true,
		RunE: func(_ *cobra.Command, _ []string) error {
			return runMenu(deps, sessionCtrl, worktreeCtrl, tui.ResWorktrees)
		},
	}
}

// runMenu launches the full-screen TUI on the given starting resource,
// then performs the action it returned. The TUI never grabs tmux itself
// (attaching replaces the process image); it hands back a tui.Result and
// we act on it here, after the program has released the terminal.
func runMenu(deps tui.Deps, sessionCtrl session.Controller, worktreeCtrl worktree.Controller, start tui.Resource) error {
	name, ws, cfg, err := loadWorkspaceAndConfig()
	if err != nil {
		return err
	}

	// When cwd doesn't resolve to a workspace and --workspace wasn't
	// given, open on the workspace-select view so the user picks one
	// explicitly. The TUI will switch m.workspace once they pick.
	if ws == nil {
		start = tui.ResWorkspaces
	}

	res, err := tui.Run(name, ws, cfg.Workspaces, start, cfg.Tui, deps)
	if err != nil {
		return err
	}

	// Post-TUI actions must use the workspace the user ended on, not
	// the one we loaded at startup. The TUI carries the active
	// workspace back through Result for exactly this reason.
	activeWS := ws
	wsName := name
	if res.Workspace != nil {
		activeWS = res.Workspace
		wsName = res.WorkspaceName
	}

	switch {
	case res.RunWizard:
		// The wizard owns the terminal; run it, then drop back into the menu.
		if err := workspacecli.RunAddWizard(os.Stdout); err != nil {
			return err
		}
		return runMenu(deps, sessionCtrl, worktreeCtrl, tui.ResWorkspaces)

	case res.NewWorktreeBranch != "":
		return worktreeCtrl.Add(activeWS, wsName, res.NewWorktreeBranch)

	case res.Attach != "":
		// A worktree name: build its session if absent, then attach.
		return sessionCtrl.Attach(activeWS, wsName, res.Attach)

	case res.AttachSession != "":
		// An already-built session (issue/pr bootstrap, or a clanker's).
		return sessionCtrl.AttachExisting(res.AttachSession)
	}

	return nil // user quit
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
