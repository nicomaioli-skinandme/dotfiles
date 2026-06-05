// Package cli is the pr View: the `sam pr …` cobra commands. It imports
// only its own entity's Controller plus infra; the active-workspace
// resolver and output-format parser are injected by cmd/sam.
package cli

import (
	"strconv"

	"github.com/spf13/cobra"

	"github.com/nicomaioli-skinandme/dotfiles/sam/internal/config"
	"github.com/nicomaioli-skinandme/dotfiles/sam/internal/output"
	"github.com/nicomaioli-skinandme/dotfiles/sam/internal/pr"
)

type (
	Resolve = func() (*config.Workspace, string, error)
	Format  = func() (output.Format, error)
)

type prRecord struct {
	Number      int    `json:"number"`
	Title       string `json:"title"`
	Repository  string `json:"repository"`
	HeadRefName string `json:"head_ref_name"`
	Author      string `json:"author"`
	IsDraft     bool   `json:"is_draft"`
}

// NewCmd builds the `pr` noun command (alias `prs`) with its list/review
// verbs.
func NewCmd(ctrl pr.Controller, resolve Resolve, format Format) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "pr",
		Aliases: []string{"prs"},
		Short:   "List PRs awaiting your review and check them out for review",
	}
	cmd.AddCommand(newListCmd(ctrl, resolve, format))
	cmd.AddCommand(newReviewCmd(ctrl, resolve))
	return cmd
}

func newListCmd(ctrl pr.Controller, resolve Resolve, format Format) *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List the PRs awaiting your review",
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
			prs, err := ctrl.List(ws)
			if err != nil {
				return err
			}
			recs := make([]prRecord, 0, len(prs))
			for _, p := range prs {
				recs = append(recs, prRecord{
					Number:      p.Number,
					Title:       p.Title,
					Repository:  p.Repository,
					HeadRefName: p.HeadRefName,
					Author:      p.Author,
					IsDraft:     p.IsDraft,
				})
			}
			return output.Render(cmd.OutOrStdout(), f, recs, prTable(recs))
		},
	}
}

func newReviewCmd(ctrl pr.Controller, resolve Resolve) *cobra.Command {
	var repo string
	cmd := &cobra.Command{
		Use:   "review <number>",
		Short: "Bootstrap a review worktree + tmux session for a PR, then attach",
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
			return ctrl.Review(ws, name, num, repo)
		},
	}
	cmd.Flags().StringVar(&repo, "repo", "", "PR repo as org/name (defaults to the workspace branch repo)")
	return cmd
}

func prTable(recs []prRecord) output.TableData {
	td := output.TableData{Header: []string{"NUMBER", "TITLE", "AUTHOR", "BRANCH", "DRAFT"}}
	for _, r := range recs {
		draft := "no"
		if r.IsDraft {
			draft = "yes"
		}
		td.Rows = append(td.Rows, []string{strconv.Itoa(r.Number), r.Title, r.Author, r.HeadRefName, draft})
	}
	return td
}
