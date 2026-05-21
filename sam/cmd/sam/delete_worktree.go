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
	"github.com/nicomaioli-skinandme/dotfiles/sam/internal/ui"
)

func newDeleteCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "delete [NAME]",
		Short: "Delete a worktree (and its tmux session)",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			project, err := loadProject()
			if err != nil {
				return err
			}
			nameArg := ""
			if len(args) == 1 {
				nameArg = args[0]
			}
			return runDelete(cmd.OutOrStdout(), project, nameArg)
		},
	}
}

func runDelete(out io.Writer, project *config.Project, nameArg string) error {
	candidates, err := gitx.Worktrees(project.Worktrees)
	if err != nil {
		return err
	}
	if len(candidates) == 0 {
		return fmt.Errorf("no worktrees to delete")
	}

	target := nameArg
	if target != "" {
		found := false
		for _, c := range candidates {
			if c == target {
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("worktree %q not found under %s", target, project.Worktrees)
		}
	} else {
		items := make([]ui.Item, 0, len(candidates))
		for _, c := range candidates {
			items = append(items, ui.Item{Value: c, Label: c})
		}
		sel, err := ui.Picker("Select worktree to delete", items)
		if err != nil {
			if errors.Is(err, ui.ErrCancelled) {
				return nil
			}
			return err
		}
		target = sel.Value
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

	if err := gitx.WorktreeRemoveForce(project.Repo, filepath.Join(project.Worktrees, target)); err != nil {
		return err
	}

	fmt.Fprintf(out, "Deleted worktree: %s\n", target)
	return nil
}
