package main

import (
	"errors"
	"fmt"
	"io"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/nicomaioli-skinandme/dotfiles/sam/internal/config"
	"github.com/nicomaioli-skinandme/dotfiles/sam/internal/gitx"
	"github.com/nicomaioli-skinandme/dotfiles/sam/internal/tmuxx"
	"github.com/nicomaioli-skinandme/dotfiles/sam/internal/tui"
	"github.com/nicomaioli-skinandme/dotfiles/sam/internal/ui"
)

func newDeleteCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "delete [NAME]",
		Short: "Delete a worktree (and its tmux session)",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				// Worktree selection + delete confirmation happen in the TUI.
				return runMenu(tui.ResWorktrees)
			}
			_, workspace, err := loadWorkspace()
			if err != nil {
				return err
			}
			return runDelete(cmd.OutOrStdout(), workspace, args[0])
		},
	}
}

// runDelete removes a named worktree and kills its tmux session. The name
// is always provided (the CLI arg); interactive selection lives in the
// TUI. When the caller is currently inside the target session it confirms
// first and hops to the system session.
func runDelete(out io.Writer, workspace *config.Workspace, target string) error {
	candidates, err := gitx.Worktrees(workspace.Worktrees)
	if err != nil {
		return err
	}
	found := false
	for _, c := range candidates {
		if c == target {
			found = true
			break
		}
	}
	if !found {
		return fmt.Errorf("worktree %q not found under %s", target, workspace.Worktrees)
	}

	current, _ := tmuxx.CurrentSession()
	if current == target {
		ok, err := ui.Confirm(fmt.Sprintf("Currently in '%s'. Delete anyway?", target))
		if err != nil {
			if errors.Is(err, ui.ErrCancelled) {
				return nil
			}
			return err
		}
		if !ok {
			return nil
		}
		if err := tmuxx.EnsureSystemSession(); err != nil {
			return err
		}
		if err := tmuxx.SwitchOrAttach("system"); err != nil {
			return err
		}
	}

	if tmuxx.HasSession(target) {
		if err := tmuxx.KillSession(target); err != nil {
			return err
		}
	}

	if err := gitx.WorktreeRemoveForce(workspace.Repo, filepath.Join(workspace.Worktrees, target)); err != nil {
		return err
	}

	fmt.Fprintf(out, "Deleted worktree: %s\n", target)
	return nil
}
