// Package cli is the worktree View: the `sam worktree …` cobra commands. It
// imports only its own entity's Controller plus infra (config, output); the
// active-workspace resolver and output-format parser are injected by
// cmd/sam as closures, so resolution stays a single per-invocation concern.
package cli

import (
	"github.com/spf13/cobra"

	"github.com/nicomaioli-skinandme/dotfiles/sam/internal/config"
	"github.com/nicomaioli-skinandme/dotfiles/sam/internal/output"
	"github.com/nicomaioli-skinandme/dotfiles/sam/internal/worktree"
)

// Resolve returns the active workspace, erroring when cwd is ambiguous and
// --workspace was not given. Format returns the parsed --output value.
type (
	Resolve = func() (*config.Workspace, string, error)
	Format  = func() (output.Format, error)
)

// NewCmd builds the `worktree` noun command (alias `worktrees`) with its
// list/add/delete verbs.
func NewCmd(ctrl worktree.Controller, resolve Resolve, format Format) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "worktree",
		Aliases: []string{"worktrees"},
		Short:   "Manage git worktrees and their tmux sessions",
	}
	cmd.AddCommand(newListCmd(ctrl, resolve, format))
	cmd.AddCommand(newAddCmd(ctrl, resolve))
	cmd.AddCommand(newDeleteCmd(ctrl, resolve))
	return cmd
}

func newListCmd(ctrl worktree.Controller, resolve Resolve, format Format) *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List worktrees (the main worktree plus linked worktrees)",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			ws, name, err := resolve()
			if err != nil {
				return err
			}
			f, err := format()
			if err != nil {
				return err
			}
			wts, err := ctrl.List(ws, name)
			if err != nil {
				return err
			}
			return output.Render(cmd.OutOrStdout(), f, wts, worktreeTable(wts))
		},
	}
}

func newAddCmd(ctrl worktree.Controller, resolve Resolve) *cobra.Command {
	return &cobra.Command{
		Use:   "add <branch>",
		Short: "Create a worktree (and tmux session) for an existing branch, then attach",
		Args:  cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			ws, name, err := resolve()
			if err != nil {
				return err
			}
			return ctrl.Add(ws, name, args[0])
		},
	}
}

func newDeleteCmd(ctrl worktree.Controller, resolve Resolve) *cobra.Command {
	return &cobra.Command{
		Use:   "delete <name>",
		Short: "Delete a worktree and its tmux session",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ws, name, err := resolve()
			if err != nil {
				return err
			}
			if err := ctrl.Delete(ws, name, args[0]); err != nil {
				return err
			}
			cmd.Printf("Deleted worktree: %s\n", args[0])
			return nil
		},
	}
}

func worktreeTable(wts []worktree.Worktree) output.TableData {
	td := output.TableData{Header: []string{"NAME", "PATH", "TYPE", "ACTIVE"}}
	for _, w := range wts {
		active := "no"
		if w.SessionActive {
			active = "yes"
		}
		td.Rows = append(td.Rows, []string{w.Name, w.Path, string(w.Type), active})
	}
	return td
}
