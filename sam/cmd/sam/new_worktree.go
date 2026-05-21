package main

import (
	"errors"
	"fmt"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/nicomaioli-skinandme/dotfiles/sam/internal/config"
	"github.com/nicomaioli-skinandme/dotfiles/sam/internal/gitx"
	"github.com/nicomaioli-skinandme/dotfiles/sam/internal/tmuxx"
	"github.com/nicomaioli-skinandme/dotfiles/sam/internal/ui"
)

func newNewWorktreeCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "new-worktree [BRANCH]",
		Short: "Create a worktree (and tmux session) for an existing branch",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			project, err := loadProject()
			if err != nil {
				return err
			}
			branchArg := ""
			if len(args) == 1 {
				branchArg = args[0]
			}
			return runNewWorktree(project, branchArg)
		},
	}
}

func runNewWorktree(project *config.Project, branchArg string) error {
	if err := gitx.FastForwardMain(project.Repo, project.MainBranch); err != nil {
		return err
	}

	branch := branchArg
	if branch == "" {
		all, err := gitx.BranchesByRecency(project.Repo)
		if err != nil {
			return err
		}
		existing, err := gitx.Worktrees(project.Worktrees)
		if err != nil {
			return err
		}
		exclude := map[string]bool{project.MainBranch: true}
		for _, w := range existing {
			exclude[w] = true
		}
		items := make([]ui.Item, 0, len(all))
		for _, b := range all {
			if exclude[b] {
				continue
			}
			items = append(items, ui.Item{Value: b, Label: b})
		}
		if len(items) == 0 {
			return fmt.Errorf("no branches available to create a worktree from")
		}
		sel, err := ui.Picker("Select branch for new worktree", items)
		if err != nil {
			if errors.Is(err, ui.ErrCancelled) {
				return nil
			}
			return err
		}
		branch = sel.Value
	}

	path := filepath.Join(project.Worktrees, branch)
	if err := gitx.WorktreeAdd(project.Repo, path, branch); err != nil {
		return err
	}
	if err := tmuxx.EnsureSystemSession(); err != nil {
		return err
	}
	if err := tmuxx.BuildSession(branch, project, path); err != nil {
		return err
	}
	return tmuxx.SwitchOrAttach(branch)
}
