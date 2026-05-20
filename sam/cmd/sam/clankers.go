package main

import (
	"encoding/json"
	"fmt"
	"io"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/nicomaioli-skinandme/dotfiles/sam/internal/proc"
)

type clankerWindow struct {
	Index int    `json:"index"`
	Name  string `json:"name"`
}

type clankerPane struct {
	Index int    `json:"index"`
	Title string `json:"title"`
}

type clankerTmux struct {
	Session string        `json:"session"`
	Window  clankerWindow `json:"window"`
	Pane    clankerPane   `json:"pane"`
}

type clankerRecord struct {
	PID  int          `json:"pid"`
	Cwd  string       `json:"cwd"`
	Tmux *clankerTmux `json:"tmux"`
}

func newClankersCmd() *cobra.Command {
	var human bool
	cmd := &cobra.Command{
		Use:   "clankers",
		Short: "List running claude processes with tmux session/cwd",
		RunE: func(cmd *cobra.Command, _ []string) error {
			claudes, err := proc.Claudes()
			if err != nil {
				return err
			}
			panes, err := proc.TmuxPanes()
			if err != nil {
				return err
			}

			records := make([]clankerRecord, 0, len(claudes))
			for _, c := range claudes {
				cwd, _ := proc.Cwd(c.PID)
				rec := clankerRecord{PID: c.PID, Cwd: cwd}
				if pane, ok := proc.FindTmuxPane(panes, c.PID); ok {
					rec.Tmux = &clankerTmux{
						Session: pane.Session,
						Window:  clankerWindow{Index: pane.WindowIdx, Name: pane.WindowName},
						Pane:    clankerPane{Index: pane.PaneIdx, Title: pane.PaneTitle},
					}
				}
				records = append(records, rec)
			}

			if human {
				return writeHumanTable(cmd.OutOrStdout(), records)
			}
			return writeJSONL(cmd.OutOrStdout(), records)
		},
	}
	cmd.Flags().BoolVar(&human, "human", false, "tab-aligned table")
	return cmd
}

func writeJSONL(w io.Writer, records []clankerRecord) error {
	enc := json.NewEncoder(w)
	for _, r := range records {
		if err := enc.Encode(r); err != nil {
			return err
		}
	}
	return nil
}

func writeHumanTable(w io.Writer, records []clankerRecord) error {
	tw := tabwriter.NewWriter(w, 1, 1, 2, ' ', 0)
	fmt.Fprintln(tw, "PID\tCWD\tSESSION\tWIN\tWINDOW\tPANE\tTITLE")
	for _, r := range records {
		session, win, winName, paneIdx, title := "-", "-", "-", "-", "-"
		if r.Tmux != nil {
			session = r.Tmux.Session
			win = fmt.Sprintf("%d", r.Tmux.Window.Index)
			winName = r.Tmux.Window.Name
			paneIdx = fmt.Sprintf("%d", r.Tmux.Pane.Index)
			title = r.Tmux.Pane.Title
		}
		cwd := r.Cwd
		if cwd == "" {
			cwd = "-"
		}
		fmt.Fprintf(tw, "%d\t%s\t%s\t%s\t%s\t%s\t%s\n",
			r.PID, cwd, session, win, winName, paneIdx, title)
	}
	return tw.Flush()
}
