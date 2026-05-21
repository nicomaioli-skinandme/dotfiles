package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/nicomaioli-skinandme/dotfiles/sam/internal/gitx"
	"github.com/nicomaioli-skinandme/dotfiles/sam/internal/tmuxx"
	"github.com/nicomaioli-skinandme/dotfiles/sam/internal/ui"
)

const (
	menuValueFromIssue = "__from_issue__"
	menuValueNew       = "__new__"
	menuValueDelete    = "__delete__"
)

func newMenuCmd() *cobra.Command {
	return &cobra.Command{
		Use:    "menu",
		Short:  "Interactive picker (default when sam is run with no subcommand)",
		Hidden: true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			project, err := loadProject()
			if err != nil {
				return err
			}
			worktrees, err := gitx.Worktrees(project.Worktrees)
			if err != nil {
				return err
			}

			items := []ui.Item{
				{Value: "system", Label: ui.Decorate("system", "system", tmuxx.HasSession("system"))},
				{
					Value: project.MainBranch,
					Label: ui.Decorate(
						project.MainBranch,
						fmt.Sprintf("%s  (main repo)", project.MainBranch),
						tmuxx.HasSession(project.MainBranch),
					),
				},
			}
			for _, w := range worktrees {
				items = append(items, ui.Item{
					Value: w,
					Label: ui.Decorate(w, w, tmuxx.HasSession(w)),
				})
			}
			items = append(items,
				ui.Item{Value: menuValueFromIssue, Label: "+ from issue"},
				ui.Item{Value: menuValueNew, Label: "+ new worktree"},
				ui.Item{Value: menuValueDelete, Label: "- delete worktree"},
			)

			sel, err := ui.Picker("sam", items)
			if err != nil {
				if errors.Is(err, ui.ErrCancelled) {
					return nil
				}
				return err
			}

			switch sel.Value {
			case menuValueFromIssue:
				return runFromIssue(project, 0, "", true)
			case menuValueNew:
				return runNewWorktree(project, "")
			case menuValueDelete:
				return runDelete(cmd.OutOrStdout(), project, "")
			}

			name := sel.Value
			if tmuxx.HasSession(name) {
				return tmuxx.SwitchOrAttach(name)
			}
			if name == "system" {
				if err := tmuxx.EnsureSystemSession(); err != nil {
					return err
				}
				return tmuxx.SwitchOrAttach("system")
			}
			if err := tmuxx.EnsureSystemSession(); err != nil {
				return err
			}
			var baseDir string
			if name == project.MainBranch {
				baseDir = project.Repo
			} else {
				baseDir = filepath.Join(project.Worktrees, name)
			}
			if err := tmuxx.BuildSession(name, project, baseDir); err != nil {
				return err
			}
			return tmuxx.SwitchOrAttach(name)
		},
	}
}

// shouldDefaultToMenu reports whether `sam` was invoked with no
// subcommand and no top-level help request. --project (which carries a
// value, attached or detached) does not count as a subcommand.
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
		if a == "--help" || a == "-h" {
			return false
		}
		if len(a) > 0 && a[0] == '-' {
			if a == "--project" {
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
