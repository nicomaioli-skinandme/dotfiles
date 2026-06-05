// Package cli is the session View: the `sam session …` cobra commands. It
// imports only its own entity's Controller plus infra; the active-workspace
// resolver is injected by cmd/sam.
package cli

import (
	"github.com/spf13/cobra"

	"github.com/nicomaioli-skinandme/dotfiles/sam/internal/config"
	"github.com/nicomaioli-skinandme/dotfiles/sam/internal/session"
)

type Resolve = func() (*config.Workspace, string, error)

// NewCmd builds the `session` noun command (alias `sessions`) with its
// attach verb.
func NewCmd(ctrl session.Controller, resolve Resolve) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "session",
		Aliases: []string{"sessions"},
		Short:   "Attach to a worktree's tmux session",
	}
	cmd.AddCommand(newAttachCmd(ctrl, resolve))
	return cmd
}

func newAttachCmd(ctrl session.Controller, resolve Resolve) *cobra.Command {
	return &cobra.Command{
		Use:   "attach <name>",
		Short: "Attach to the tmux session for a worktree, building it if needed",
		Args:  cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			ws, name, err := resolve()
			if err != nil {
				return err
			}
			return ctrl.Attach(ws, name, args[0])
		},
	}
}
