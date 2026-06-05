// Package cli is the workspace View: the `sam workspace …` cobra commands.
// workspace has no Controller, so the View uses workspace.Service directly.
// `add` is the one interactive exception in the otherwise non-interactive
// CLI — it keeps the guided wizard (non-interactive config is tracked
// separately, see #30). Only the output-format parser is injected.
package cli

import (
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/spf13/cobra"

	"github.com/nicomaioli-skinandme/dotfiles/sam/internal/config"
	"github.com/nicomaioli-skinandme/dotfiles/sam/internal/output"
	"github.com/nicomaioli-skinandme/dotfiles/sam/internal/ui"
	"github.com/nicomaioli-skinandme/dotfiles/sam/internal/wizard"
	"github.com/nicomaioli-skinandme/dotfiles/sam/internal/workspace"
)

type Format = func() (output.Format, error)

type workspaceRecord struct {
	Name      string `json:"name"`
	Repo      string `json:"repo"`
	Worktrees string `json:"worktrees"`
	Trunk     string `json:"trunk"`
}

// NewCmd builds the `workspace` noun command (alias `workspaces`) with its
// list/add verbs.
func NewCmd(svc workspace.Service, format Format) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "workspace",
		Aliases: []string{"workspaces"},
		Short:   "Manage sam workspace configuration",
	}
	cmd.AddCommand(newListCmd(svc, format))
	cmd.AddCommand(newAddCmd())
	return cmd
}

func newListCmd(svc workspace.Service, format Format) *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List configured workspaces",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			f, err := format()
			if err != nil {
				return err
			}
			cfg, _, err := svc.LoadConfig()
			if err != nil {
				return err
			}
			active := svc.List(cfg)
			recs := make([]workspaceRecord, 0, len(active))
			for _, a := range active {
				recs = append(recs, workspaceRecord{
					Name:      a.Name,
					Repo:      a.WS.Repo,
					Worktrees: a.WS.Worktrees,
					Trunk:     a.WS.Trunk,
				})
			}
			return output.Render(cmd.OutOrStdout(), f, recs, workspaceTable(recs))
		},
	}
}

func newAddCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "add",
		Short: "Add a workspace to ~/.config/sam/config.toml via guided wizard",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return RunAddWizard(cmd.OutOrStdout())
		},
	}
}

// RunAddWizard loads any existing config, runs the guided wizard to append
// a workspace, and saves, reporting the written path to w. A cancelled
// wizard is a no-op. It is exported so the menu (the TUI's `a` on the
// workspaces view) drives the same flow as `sam workspace add`.
func RunAddWizard(w io.Writer) error {
	path, err := config.DefaultPath()
	if err != nil {
		return err
	}
	var existing *config.Config
	if _, statErr := os.Stat(path); statErr == nil {
		existing, err = config.Load(path)
		if err != nil {
			return err
		}
	} else if !errors.Is(statErr, os.ErrNotExist) {
		return statErr
	}
	updated, err := wizard.Run(existing)
	if err != nil {
		if errors.Is(err, ui.ErrCancelled) {
			return nil
		}
		return err
	}
	if err := config.Save(updated, path); err != nil {
		return err
	}
	fmt.Fprintf(w, "Wrote %s\n", path)
	return nil
}

func workspaceTable(recs []workspaceRecord) output.TableData {
	td := output.TableData{Header: []string{"NAME", "REPO", "TRUNK"}}
	for _, r := range recs {
		td.Rows = append(td.Rows, []string{r.Name, r.Repo, r.Trunk})
	}
	return td
}
