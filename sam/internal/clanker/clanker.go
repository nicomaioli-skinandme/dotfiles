// Package clanker is the clanker entity: its Service enumerates running
// claude processes and correlates each to its tmux pane (via the proc
// infra); its Controller annotates whether each one's session is live (via
// the session entity). "clanker" is the project's term for a running claude
// process — see the glossary.
package clanker

// Clanker is one running claude process. The tmux fields are populated only
// when the process was found inside a tmux pane (Session == "" otherwise).
// Active reports whether that tmux session is live; the Controller fills it
// in.
type Clanker struct {
	PID        int
	Cwd        string
	Session    string
	WindowIdx  int
	WindowName string
	PaneIdx    int
	PaneTitle  string
	Active     bool
}

// InTmux reports whether the clanker was located in a tmux pane.
func (c Clanker) InTmux() bool { return c.Session != "" }
