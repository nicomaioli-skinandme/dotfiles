// Package cli is the workspace View: the `sam workspace …` cobra commands.
// workspace has no Controller, so the View uses workspace.Service directly.
// It is fully non-interactive (just `list`): adding a workspace is a
// menu-only flow (the guided wizard), and the programmatic path is
// hand-writing config.toml — see #30. Only the output-format parser is
// injected.
package cli

import (
	"github.com/spf13/cobra"

	"github.com/nicomaioli-skinandme/dotfiles/sam/internal/output"
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

func workspaceTable(recs []workspaceRecord) output.TableData {
	td := output.TableData{Header: []string{"NAME", "REPO", "TRUNK"}}
	for _, r := range recs {
		td.Rows = append(td.Rows, []string{r.Name, r.Repo, r.Trunk})
	}
	return td
}
