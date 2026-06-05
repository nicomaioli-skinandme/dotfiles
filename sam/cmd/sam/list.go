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
	Type          string `json:"type"` // "main" (repo root) or "linked"
	SessionActive bool   `json:"session_active"`
}

func newListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List worktrees (the main worktree plus linked worktrees)",
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
				worktreeRecord{Name: workspace.Trunk, Path: workspace.Repo, Type: "main", SessionActive: tmuxx.HasSession(tmuxx.SessionName(name, workspace.Trunk))},
			)
			for _, w := range worktrees {
				records = append(records, worktreeRecord{
					Name:          w,
					Path:          filepath.Join(workspace.Worktrees, w),
					Type:          "linked",
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
	fmt.Fprintln(tw, "NAME\tPATH\tTYPE\tACTIVE")
	for _, r := range records {
		active := "no"
		if r.SessionActive {
			active = "yes"
		}
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\n", r.Name, r.Path, r.Type, active)
	}
	return tw.Flush()
}
