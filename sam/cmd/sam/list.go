package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
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
		Short: "List worktrees (plus synthetic system and main entries)",
		RunE: func(cmd *cobra.Command, _ []string) error {
			_, project, err := loadProject()
			if err != nil {
				return err
			}
			home, err := os.UserHomeDir()
			if err != nil {
				return err
			}
			worktrees, err := gitx.Worktrees(project.Worktrees)
			if err != nil {
				return err
			}

			records := make([]worktreeRecord, 0, 2+len(worktrees))
			records = append(records,
				worktreeRecord{Name: "system", Path: home, SessionActive: tmuxx.HasSession("system")},
				worktreeRecord{Name: project.MainBranch, Path: project.Repo, SessionActive: tmuxx.HasSession(project.MainBranch)},
			)
			for _, w := range worktrees {
				records = append(records, worktreeRecord{
					Name:          w,
					Path:          filepath.Join(project.Worktrees, w),
					SessionActive: tmuxx.HasSession(w),
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
