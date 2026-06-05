// Package session is the session entity: it wraps tmuxx to own tmux
// session lifecycle (name/has/build/kill/attach) and exposes a Controller
// for the build-if-missing-then-attach action consumed by the cli and tui.
// The Service imports only infra (tmuxx); cross-entity orchestration that
// needs a session lives in other entities' Controllers via this Service.
package session

import (
	"github.com/nicomaioli-skinandme/dotfiles/sam/internal/config"
	"github.com/nicomaioli-skinandme/dotfiles/sam/internal/tmuxx"
)

// ClaudeData is the data passed to the Claude pane templates. Aliased from
// tmuxx so the issue/pr controllers populate it through the session entity
// without importing tmuxx directly.
type ClaudeData = tmuxx.ClaudeData

// Service wraps tmux session primitives. Infra-only; the zero value is
// ready to use.
type Service struct{}

// Name returns the tmux session name for a branch within a workspace.
func (Service) Name(wsName, branch string) string {
	return tmuxx.SessionName(wsName, branch)
}

// Has reports whether a tmux session with the given name exists.
func (Service) Has(name string) bool {
	return tmuxx.HasSession(name)
}

// Build creates session `name` and applies the workspace's tmux layout,
// rooted at baseDir. It does not launch a Claude pane.
func (Service) Build(name string, ws *config.Workspace, baseDir string) error {
	return tmuxx.BuildSession(name, ws, baseDir)
}

// AddClaudePane splits the configured repo window and launches Claude with
// the rendered prompt/title. An empty prompt is a no-op.
func (Service) AddClaudePane(name, repoWindow, prompt, title, permMode string, data ClaudeData, cwd string) error {
	return tmuxx.AddClaudePane(name, repoWindow, prompt, title, permMode, data, cwd)
}

// Kill tears down a tmux session.
func (Service) Kill(name string) error {
	return tmuxx.KillSession(name)
}

// SwitchOrAttach switches the current tmux client to the session, or
// attaches the controlling terminal when not already inside tmux. Outside
// tmux it replaces the process image, so callers must invoke it after any
// full-screen TUI has released the terminal — never from inside it.
func (Service) SwitchOrAttach(name string) error {
	return tmuxx.SwitchOrAttach(name)
}

// Current returns the current tmux session name, or "" when not in tmux.
func (Service) Current() (string, error) {
	return tmuxx.CurrentSession()
}

// InTmux reports whether sam is running inside a tmux client.
func (Service) InTmux() bool {
	return tmuxx.InTmux()
}
