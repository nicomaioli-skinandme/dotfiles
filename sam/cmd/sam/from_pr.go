package main

import (
	"errors"

	"github.com/spf13/cobra"

	"github.com/nicomaioli-skinandme/dotfiles/sam/internal/config"
	"github.com/nicomaioli-skinandme/dotfiles/sam/internal/prflow"
	"github.com/nicomaioli-skinandme/dotfiles/sam/internal/tmuxx"
	"github.com/nicomaioli-skinandme/dotfiles/sam/internal/tui"
)

func newFromPRCmd() *cobra.Command {
	var prFlag int
	var repoFlag string
	cmd := &cobra.Command{
		Use:   "from-pr",
		Short: "Pick a PR awaiting your review and bootstrap a worktree + tmux session",
		Args:  cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			interactive := prFlag == 0 && repoFlag == ""
			if !interactive && (prFlag == 0 || repoFlag == "") {
				return errors.New("--pr and --repo must be set together")
			}
			if interactive {
				// Picking a PR happens in the full-screen TUI.
				return runMenu(tui.ResPRs)
			}
			workspaceName, workspace, err := loadWorkspace()
			if err != nil {
				return err
			}
			return runFromPR(workspaceName, workspace, prFlag, repoFlag)
		},
	}
	cmd.Flags().IntVar(&prFlag, "pr", 0, "PR number (non-interactive)")
	cmd.Flags().StringVar(&repoFlag, "repo", "",
		"PR repo, e.g. org/name (non-interactive)")
	return cmd
}

// runFromPR bootstraps a review worktree for a specific PR
// non-interactively (the --pr/--repo path, and the TUI's deferred pick).
func runFromPR(workspaceName string, workspace *config.Workspace, prNum int, repo string) error {
	pr, err := prflow.ByFlag(workspace, prNum, repo)
	if err != nil {
		return err
	}
	session, err := prflow.Apply(workspace, workspaceName, pr)
	if err != nil {
		return err
	}
	return tmuxx.SwitchOrAttach(session)
}
