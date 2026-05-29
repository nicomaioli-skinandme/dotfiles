package main

import (
	"github.com/spf13/cobra"

	"github.com/nicomaioli-skinandme/dotfiles/sam/internal/config"
	"github.com/nicomaioli-skinandme/dotfiles/sam/internal/gitx"
	"github.com/nicomaioli-skinandme/dotfiles/sam/internal/setup"
	"github.com/nicomaioli-skinandme/dotfiles/sam/internal/tmuxx"
	"github.com/nicomaioli-skinandme/dotfiles/sam/internal/tui"
)

func newNewWorktreeCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "new-worktree [BRANCH]",
		Short: "Create a worktree (and tmux session) for an existing branch",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				// Branch selection happens in the full-screen TUI (press `a`).
				return runMenu(tui.ResWorktrees)
			}
			workspaceName, workspace, err := loadWorkspace()
			if err != nil {
				return err
			}
			return runNewWorktree(workspaceName, workspace, args[0])
		},
	}
}

// runNewWorktree creates a worktree + tmux session for an existing branch
// and attaches. The branch is always provided (by the CLI arg or the
// TUI's branch picker); selection itself lives in the TUI.
func runNewWorktree(workspaceName string, workspace *config.Workspace, branch string) error {
	if err := gitx.FastForwardMain(workspace.Repo, workspace.MainBranch); err != nil {
		return err
	}
	path, err := setup.CreateWorktree(workspace, branch, 0, workspaceName)
	if err != nil {
		return err
	}
	session := tmuxx.SessionName(workspaceName, branch)
	if err := tmuxx.BuildSession(session, workspace, path); err != nil {
		return err
	}
	return tmuxx.SwitchOrAttach(session)
}
