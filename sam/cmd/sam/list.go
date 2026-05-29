package main

import (
	"encoding/json"
	"fmt"
	"io"
	"path/filepath"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/nicomaioli-skinandme/dotfiles/sam/internal/gitx"
	"github.com/nicomaioli-skinandme/dotfiles/sam/internal/tmuxx"
)

type worktreeRecord struct {
	Name          string `json:"name"`
	Path          string `json:"path"`
	SessionActive bool   `json:"session_active"`
}

func newListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List worktrees (plus the synthetic main-repo entry)",
		RunE: func(cmd *cobra.Command, _ []string) error {
			name, workspace, err := loadWorkspace()
			if err != nil {
				return err
			}
			worktrees, err := gitx.Worktrees(workspace.Worktrees)
			if err != nil {
				return err
			}

			records := make([]worktreeRecord, 0, 1+len(worktrees))
			records = append(records,
				worktreeRecord{Name: workspace.MainBranch, Path: workspace.Repo, SessionActive: tmuxx.HasSession(tmuxx.SessionName(name, workspace.MainBranch))},
			)
			for _, w := range worktrees {
				records = append(records, worktreeRecord{
					Name:          w,
					Path:          filepath.Join(workspace.Worktrees, w),
					SessionActive: tmuxx.HasSession(tmuxx.SessionName(name, w)),
				})
			}

			if humanFlag {
				return writeWorktreeTable(cmd.OutOrStdout(), records)
			}
			return json.NewEncoder(cmd.OutOrStdout()).Encode(records)
		},
	}
}

func writeWorktreeTable(w io.Writer, records []worktreeRecord) error {
	tw := tabwriter.NewWriter(w, 1, 1, 2, ' ', 0)
	fmt.Fprintln(tw, "NAME\tPATH\tACTIVE")
	for _, r := range records {
		active := "no"
		if r.SessionActive {
			active = "yes"
		}
		fmt.Fprintf(tw, "%s\t%s\t%s\n", r.Name, r.Path, active)
	}
	return tw.Flush()
}
