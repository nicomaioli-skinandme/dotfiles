// Package cli is the issue View: the `sam issue …` cobra commands. It
// imports only its own entity's Controller plus infra; the active-workspace
// resolver and output-format parser are injected by cmd/sam.
package cli

import (
	"strconv"

	"github.com/spf13/cobra"

	"github.com/nicomaioli-skinandme/dotfiles/sam/internal/config"
	"github.com/nicomaioli-skinandme/dotfiles/sam/internal/issue"
	"github.com/nicomaioli-skinandme/dotfiles/sam/internal/output"
)

type (
	Resolve = func() (*config.Workspace, string, error)
	Format  = func() (output.Format, error)
)

type issueRecord struct {
	Number     int      `json:"number"`
	Title      string   `json:"title"`
	Repository string   `json:"repository"`
	Status     string   `json:"status,omitempty"`
	Assignees  []string `json:"assignees,omitempty"`
}

// NewCmd builds the `issue` noun command (alias `issues`) with its
// list/develop verbs.
func NewCmd(ctrl issue.Controller, resolve Resolve, format Format) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "issue",
		Aliases: []string{"issues"},
		Short:   "List backlog issues and develop them into worktrees",
	}
	cmd.AddCommand(newListCmd(ctrl, resolve, format))
	cmd.AddCommand(newDevelopCmd(ctrl, resolve))
	return cmd
}

func newListCmd(ctrl issue.Controller, resolve Resolve, format Format) *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List the workspace's backlog (or open) issues",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			ws, _, err := resolve()
			if err != nil {
				return err
			}
			f, err := format()
			if err != nil {
				return err
			}
			// Default to the configured backlog columns (ignored when no
			// GitHub Project). Flag-supplied column filters would populate
			// issue.Filter here.
			issues, err := ctrl.List(ws, issue.Filter{Columns: ws.GhProject.BacklogStatuses})
			if err != nil {
				return err
			}
			recs := make([]issueRecord, 0, len(issues))
			for _, it := range issues {
				recs = append(recs, issueRecord{
					Number:     it.Number,
					Title:      it.Title,
					Repository: it.Repository,
					Status:     it.Status,
					Assignees:  it.Assignees,
				})
			}
			return output.Render(cmd.OutOrStdout(), f, recs, issueTable(recs))
		},
	}
}

func newDevelopCmd(ctrl issue.Controller, resolve Resolve) *cobra.Command {
	var (
		branch   string
		repo     string
		reassign bool
	)
	cmd := &cobra.Command{
		Use:   "develop <number>",
		Short: "Bootstrap a worktree + tmux session for an issue, then attach",
		Args:  cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			num, err := strconv.Atoi(args[0])
			if err != nil {
				return err
			}
			ws, name, err := resolve()
			if err != nil {
				return err
			}
			return ctrl.Develop(ws, name, num, repo, branch, reassign)
		},
	}
	cmd.Flags().StringVar(&branch, "branch", "", "branch name to use (overrides the derived name)")
	cmd.Flags().StringVar(&repo, "repo", "", "issue repo as org/name (defaults to the workspace branch repo)")
	cmd.Flags().BoolVar(&reassign, "reassign", false, "take the issue even when it is assigned to someone else")
	return cmd
}

func issueTable(recs []issueRecord) output.TableData {
	td := output.TableData{Header: []string{"NUMBER", "TITLE", "REPO"}}
	for _, r := range recs {
		td.Rows = append(td.Rows, []string{strconv.Itoa(r.Number), r.Title, r.Repository})
	}
	return td
}
