package main

import (
	"errors"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/nicomaioli-skinandme/dotfiles/sam/internal/config"
	"github.com/nicomaioli-skinandme/dotfiles/sam/internal/ghx"
	"github.com/nicomaioli-skinandme/dotfiles/sam/internal/issueflow"
	"github.com/nicomaioli-skinandme/dotfiles/sam/internal/tmuxx"
	"github.com/nicomaioli-skinandme/dotfiles/sam/internal/tui"
)

func newFromIssueCmd() *cobra.Command {
	var issueFlag int
	var repoFlag string
	cmd := &cobra.Command{
		Use:   "from-issue",
		Short: "Pick a backlog issue and bootstrap a worktree + tmux session",
		Args:  cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			interactive := issueFlag == 0 && repoFlag == ""
			if !interactive && (issueFlag == 0 || repoFlag == "") {
				return errors.New("--issue and --repo must be set together")
			}
			if interactive {
				// Picking an issue (and its reassign/branch prompts) happens
				// in the full-screen TUI.
				return runMenu(tui.ResIssues)
			}
			workspaceName, workspace, err := loadWorkspace()
			if err != nil {
				return err
			}
			return runFromIssue(workspaceName, workspace, issueFlag, repoFlag)
		},
	}
	cmd.Flags().IntVar(&issueFlag, "issue", 0, "issue number (non-interactive)")
	cmd.Flags().StringVar(&repoFlag, "repo", "",
		"issue repo, e.g. org/name (non-interactive)")
	return cmd
}

// runFromIssue bootstraps a worktree for a specific issue non-interactively
// (the --issue/--repo path, and the TUI's deferred pick). Interactive
// reassign/branch-edit prompts live in the TUI; here, an issue assigned to
// someone else is an error rather than a prompt.
func runFromIssue(workspaceName string, workspace *config.Workspace, issueNum int, repo string) error {
	issue, err := issueflow.ByFlag(workspace, issueNum, repo)
	if err != nil {
		return err
	}
	me, err := ghx.CurrentUser()
	if err != nil {
		return err
	}
	if other, needs := issueflow.NeedsReassign(issue, me); needs {
		return fmt.Errorf("issue %s#%d assigned to %s; reassign from the interactive picker",
			repo, issueNum, other)
	}
	branch, existing, err := issueflow.Plan(workspace, issue)
	if err != nil {
		return err
	}
	session, err := issueflow.Apply(workspace, workspaceName, issue, me, false, branch, existing)
	if err != nil {
		return err
	}
	return tmuxx.SwitchOrAttach(session)
}
