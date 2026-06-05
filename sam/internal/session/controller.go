package session

import (
	"fmt"
	"path/filepath"

	"github.com/nicomaioli-skinandme/dotfiles/sam/internal/config"
)

// Controller orchestrates session actions on top of the Service. It is the
// single owner of the build-if-missing-then-attach action that the CLI
// (`sam session attach`), the worktree-activate path, and the issue/pr
// flows all funnel through.
type Controller struct {
	svc Service
}

// NewController returns a session Controller backed by svc.
func NewController(svc Service) Controller {
	return Controller{svc: svc}
}

// Attach attaches to the session for the named worktree within ws,
// building the tmux layout first when the session doesn't yet exist. name
// is a worktree/branch name: the trunk name resolves to the main worktree
// at ws.Repo, any other name to ws.Worktrees/<name>. It builds the layout
// only — no Claude pane (that's the issue/pr bootstrap's job).
func (c Controller) Attach(ws *config.Workspace, wsName, name string) error {
	sess := c.svc.Name(wsName, name)
	if !c.svc.Has(sess) {
		if err := c.svc.Build(sess, ws, baseDir(ws, name)); err != nil {
			return err
		}
	}
	return c.svc.SwitchOrAttach(sess)
}

// AttachExisting attaches to an already-named session (e.g. the tmux
// session a clanker is running in), never building. It errors when the
// session no longer exists.
func (c Controller) AttachExisting(name string) error {
	if !c.svc.Has(name) {
		return fmt.Errorf("no tmux session %q", name)
	}
	return c.svc.SwitchOrAttach(name)
}

// baseDir maps a worktree name to its directory: the trunk lives at the
// repo root, every other branch under the worktrees dir.
func baseDir(ws *config.Workspace, name string) string {
	if name == ws.Trunk {
		return ws.Repo
	}
	return filepath.Join(ws.Worktrees, name)
}
