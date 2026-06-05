// Package cli is the clanker View: the `sam clanker …` cobra commands.
// Listing clankers enumerates processes globally, so it needs no
// workspace; only the output-format parser is injected by cmd/sam.
package cli

import (
	"strconv"

	"github.com/spf13/cobra"

	"github.com/nicomaioli-skinandme/dotfiles/sam/internal/clanker"
	"github.com/nicomaioli-skinandme/dotfiles/sam/internal/output"
)

type Format = func() (output.Format, error)

// The json shape mirrors the legacy `clankers` command for compatibility.
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

// NewCmd builds the `clanker` noun command (alias `clankers`) with its list
// verb.
func NewCmd(ctrl clanker.Controller, format Format) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "clanker",
		Aliases: []string{"clankers"},
		Short:   "List running claude processes with their tmux session/cwd",
	}
	cmd.AddCommand(newListCmd(ctrl, format))
	return cmd
}

func newListCmd(ctrl clanker.Controller, format Format) *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List running claude processes with their tmux session/cwd",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			f, err := format()
			if err != nil {
				return err
			}
			clankers, err := ctrl.List()
			if err != nil {
				return err
			}
			recs := make([]clankerRecord, 0, len(clankers))
			for _, k := range clankers {
				rec := clankerRecord{PID: k.PID, Cwd: k.Cwd}
				if k.InTmux() {
					rec.Tmux = &clankerTmux{
						Session: k.Session,
						Window:  clankerWindow{Index: k.WindowIdx, Name: k.WindowName},
						Pane:    clankerPane{Index: k.PaneIdx, Title: k.PaneTitle},
					}
				}
				recs = append(recs, rec)
			}
			return output.Render(cmd.OutOrStdout(), f, recs, clankerTable(recs))
		},
	}
}

func clankerTable(recs []clankerRecord) output.TableData {
	td := output.TableData{Header: []string{"PID", "CWD", "SESSION", "WIN", "WINDOW", "PANE", "TITLE"}}
	for _, r := range recs {
		session, win, winName, paneIdx, title := "-", "-", "-", "-", "-"
		if r.Tmux != nil {
			session = r.Tmux.Session
			win = strconv.Itoa(r.Tmux.Window.Index)
			winName = r.Tmux.Window.Name
			paneIdx = strconv.Itoa(r.Tmux.Pane.Index)
			title = r.Tmux.Pane.Title
		}
		cwd := r.Cwd
		if cwd == "" {
			cwd = "-"
		}
		td.Rows = append(td.Rows, []string{strconv.Itoa(r.PID), cwd, session, win, winName, paneIdx, title})
	}
	return td
}
