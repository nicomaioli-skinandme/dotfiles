package clanker

import "github.com/nicomaioli-skinandme/dotfiles/sam/internal/proc"

// Service enumerates running claude processes and their tmux context.
// Infra-only (proc); the zero value is ready to use.
type Service struct{}

// List returns the running claude processes, each correlated to its tmux
// pane when it lives in one. Active is left false for the Controller to
// annotate.
func (Service) List() ([]Clanker, error) {
	claudes, err := proc.Claudes()
	if err != nil {
		return nil, err
	}
	panes, err := proc.TmuxPanes()
	if err != nil {
		return nil, err
	}
	out := make([]Clanker, 0, len(claudes))
	for _, c := range claudes {
		cwd, _ := proc.Cwd(c.PID)
		k := Clanker{PID: c.PID, Cwd: cwd}
		if pane, ok := proc.FindTmuxPane(panes, c.PID); ok {
			k.Session = pane.Session
			k.WindowIdx = pane.WindowIdx
			k.WindowName = pane.WindowName
			k.PaneIdx = pane.PaneIdx
			k.PaneTitle = pane.PaneTitle
		}
		out = append(out, k)
	}
	return out, nil
}
