package worktree

import (
	"fmt"

	"github.com/nicomaioli-skinandme/dotfiles/sam/internal/config"
	"github.com/nicomaioli-skinandme/dotfiles/sam/internal/session"
)

// Controller exposes the user-facing worktree actions. It owns the only
// cross-entity edge in this slice: worktree → session (annotating active
// sessions, attaching after add, killing on delete).
type Controller struct {
	worktrees Service
	sessions  session.Service
}

// NewController returns a worktree Controller backed by the given services.
func NewController(worktrees Service, sessions session.Service) Controller {
	return Controller{worktrees: worktrees, sessions: sessions}
}

// List returns the workspace's worktrees with SessionActive annotated from
// the session entity.
func (c Controller) List(ws *config.Workspace, wsName string) ([]Worktree, error) {
	wts, err := c.worktrees.List(ws)
	if err != nil {
		return nil, err
	}
	for i := range wts {
		wts[i].SessionActive = c.sessions.Has(c.sessions.Name(wsName, wts[i].Name))
	}
	return wts, nil
}

// Add creates a worktree for an existing branch, builds its tmux session
// (if absent), and attaches. The branch is always supplied by the caller
// (a CLI arg or the TUI's branch picker); selection itself is not done
// here. SwitchOrAttach replaces the process image, so callers must invoke
// Add only after any full-screen TUI has released the terminal.
func (c Controller) Add(ws *config.Workspace, wsName, branch string) error {
	if err := c.worktrees.FastForwardTrunk(ws); err != nil {
		return err
	}
	path, err := c.worktrees.Create(ws, branch, 0, wsName)
	if err != nil {
		return err
	}
	sess := c.sessions.Name(wsName, branch)
	if !c.sessions.Has(sess) {
		if err := c.sessions.Build(sess, ws, path); err != nil {
			return err
		}
	}
	return c.sessions.SwitchOrAttach(sess)
}

// Delete removes a named linked worktree and kills its tmux session. It is
// non-interactive: it errors rather than prompting when the worktree is the
// main one, doesn't exist, or is the session the caller is currently
// attached to.
func (c Controller) Delete(ws *config.Workspace, wsName, name string) error {
	if name == ws.Trunk {
		return fmt.Errorf("cannot delete the main worktree")
	}
	wts, err := c.worktrees.List(ws)
	if err != nil {
		return err
	}
	found := false
	for _, w := range wts {
		if w.Type == Linked && w.Name == name {
			found = true
			break
		}
	}
	if !found {
		return fmt.Errorf("worktree %q not found under %s", name, ws.Worktrees)
	}

	sess := c.sessions.Name(wsName, name)
	if cur, _ := c.sessions.Current(); cur == sess {
		return fmt.Errorf("cannot delete %q: you are attached to its session", name)
	}
	if c.sessions.Has(sess) {
		if err := c.sessions.Kill(sess); err != nil {
			return err
		}
	}
	return c.worktrees.Remove(ws, name)
}
