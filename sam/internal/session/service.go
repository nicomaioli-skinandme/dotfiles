// Package session is the session entity: it wraps tmuxx to own tmux
// session lifecycle (name/has/build/kill/attach) and exposes a Controller
// for the build-if-missing-then-attach action consumed by the cli and tui.
// The Service imports only infra (tmuxx); cross-entity orchestration that
// needs a session lives in other entities' Controllers via this Service.
package session

import (
	"os/exec"
	"path/filepath"

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

// Ensure returns the session name for the named worktree within ws,
// building its tmux layout (detached, via Build) when the session doesn't
// yet exist. It never attaches — callers attach afterwards (the CLI via
// SwitchOrAttach, the TUI via AttachCmd/Switch). Because Build uses
// `new-session -d`, the session is owned by the daemonized tmux server,
// not by sam.
func (s Service) Ensure(ws *config.Workspace, wsName, name string) (string, error) {
	sess := s.Name(wsName, name)
	if !s.Has(sess) {
		if err := s.Build(sess, ws, baseDir(ws, name)); err != nil {
			return "", err
		}
	}
	return sess, nil
}

// AttachCmd returns the command that attaches the controlling terminal to
// the session. It is for callers that keep running after the client exits
// (the TUI, via tea.ExecProcess); the session must already exist.
func (Service) AttachCmd(name string) *exec.Cmd {
	return tmuxx.AttachCmd(name)
}

// Switch points the current tmux client at the session (inside-tmux path);
// it returns immediately without taking over the terminal.
func (Service) Switch(name string) error {
	return tmuxx.Switch(name)
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

// baseDir maps a worktree name to its directory: the trunk lives at the
// repo root, every other branch under the worktrees dir.
func baseDir(ws *config.Workspace, name string) string {
	if name == ws.Trunk {
		return ws.Repo
	}
	return filepath.Join(ws.Worktrees, name)
}
