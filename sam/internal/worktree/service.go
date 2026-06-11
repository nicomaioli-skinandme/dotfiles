package worktree

import (
	"path/filepath"

	"github.com/nicomaioli-skinandme/dotfiles/sam/internal/config"
	"github.com/nicomaioli-skinandme/dotfiles/sam/internal/gitx"
	"github.com/nicomaioli-skinandme/dotfiles/sam/internal/setup"
)

// Service wraps git/worktree primitives. Infra-only (gitx + setup); the
// zero value is ready to use. It is tmux-free so it stays at the Service
// layer — session activity is annotated by the Controller.
type Service struct{}

// List returns the main worktree (the trunk at the repo root) followed by
// the linked worktrees under ws.Worktrees. SessionActive is left false for
// the Controller to fill in.
func (Service) List(ws *config.Workspace) ([]Worktree, error) {
	linked, err := gitx.Worktrees(ws.Worktrees)
	if err != nil {
		return nil, err
	}
	out := make([]Worktree, 0, 1+len(linked))
	out = append(out, Worktree{Name: ws.Trunk, Path: ws.Repo, Type: Main})
	for _, w := range linked {
		out = append(out, Worktree{
			Name: w,
			Path: filepath.Join(ws.Worktrees, w),
			Type: Linked,
		})
	}
	return out, nil
}

// Create creates a worktree for branch and runs the workspace's
// worktree_setup hook, returning the worktree path. Idempotent. issueNum is
// exposed to the hook (0 when there's no associated issue).
func (Service) Create(ws *config.Workspace, branch string, issueNum int, wsName string) (string, error) {
	return setup.CreateWorktree(ws, branch, issueNum, wsName)
}

// CreateNew creates a worktree for a brand-new branch rooted at `start`
// (e.g. origin/<trunk>) and runs the worktree_setup hook, returning the
// worktree path. Idempotent like Create.
func (Service) CreateNew(ws *config.Workspace, branch, start string, issueNum int, wsName string) (string, error) {
	return setup.CreateWorktreeNewBranch(ws, branch, start, issueNum, wsName)
}

// Remove force-removes the named linked worktree.
func (Service) Remove(ws *config.Workspace, name string) error {
	return gitx.WorktreeRemoveForce(ws.Repo, filepath.Join(ws.Worktrees, name))
}

// Fetch fetches the workspace's repo remote.
func (Service) Fetch(ws *config.Workspace) error {
	return gitx.Fetch(ws.Repo)
}

// FastForwardTrunk fast-forwards the trunk branch when checked out.
func (Service) FastForwardTrunk(ws *config.Workspace) error {
	return gitx.FastForwardTrunk(ws.Repo, ws.Trunk)
}

// Branches returns branch names by recency, excluding the trunk and any
// branch that already has a worktree — the candidates for a new worktree.
// It does not fetch; callers that want freshly-pushed remote branches to
// appear should Fetch first.
func (Service) Branches(ws *config.Workspace) ([]string, error) {
	all, err := gitx.BranchesByRecency(ws.Repo)
	if err != nil {
		return nil, err
	}
	existing, err := gitx.Worktrees(ws.Worktrees)
	if err != nil {
		return nil, err
	}
	exclude := map[string]bool{ws.Trunk: true}
	for _, w := range existing {
		exclude[w] = true
	}
	out := make([]string, 0, len(all))
	for _, b := range all {
		if exclude[b] {
			continue
		}
		out = append(out, b)
	}
	return out, nil
}
