package pr

import (
	"fmt"

	"github.com/nicomaioli-skinandme/dotfiles/sam/internal/config"
	"github.com/nicomaioli-skinandme/dotfiles/sam/internal/session"
	"github.com/nicomaioli-skinandme/dotfiles/sam/internal/worktree"
)

// Controller orchestrates the from-pr review flow across the worktree and
// session entities. It imports those Services (the only cross-entity edges)
// and never another Controller.
type Controller struct {
	prs       Service
	worktrees worktree.Service
	sessions  session.Service
}

// NewController returns a pr Controller backed by the given services.
func NewController(prs Service, worktrees worktree.Service, sessions session.Service) Controller {
	return Controller{prs: prs, worktrees: worktrees, sessions: sessions}
}

// List returns the PRs awaiting the current user's review.
func (c Controller) List(ws *config.Workspace) ([]PR, error) {
	return c.prs.List(ws)
}

// Apply bootstraps the review worktree for a PR and returns the tmux
// session name to attach to — it does NOT attach (the CLI attaches after;
// the TUI attaches via its Result). It fetches so the PR's head branch is
// locally reachable, fast-forwards the trunk, creates the worktree on the
// head branch, builds the tmux session, and adds the Claude review pane —
// idempotently. It makes no changes on GitHub.
func (c Controller) Apply(ws *config.Workspace, wsName string, p PR) (string, error) {
	branch := p.HeadRefName

	if err := c.worktrees.Fetch(ws); err != nil {
		return "", err
	}
	if err := c.worktrees.FastForwardTrunk(ws); err != nil {
		return "", err
	}

	path, err := c.worktrees.Create(ws, branch, 0, wsName)
	if err != nil {
		return "", err
	}

	sess := c.sessions.Name(wsName, branch)
	if !c.sessions.Has(sess) {
		if err := c.sessions.Build(sess, ws, path); err != nil {
			return "", err
		}
		data := session.ClaudeData{
			PRNumber: p.Number,
			PRTitle:  p.Title,
			PRRepo:   p.Repository,
			PRURL:    fmt.Sprintf("https://github.com/%s/pull/%d", p.Repository, p.Number),
			PRAuthor: p.Author,
			PRBranch: branch,
		}
		if err := c.sessions.AddClaudePane(sess, ws.FromPR.RepoWindow, ws.FromPR.ClaudePrompt, ws.FromPR.ClaudePaneTitle, ws.FromPR.PermissionMode, data, path); err != nil {
			return "", err
		}
	}
	return sess, nil
}

// Review is the non-interactive CLI entry point: it resolves PR `num` (repo
// defaults to the workspace's branch repo), bootstraps the review worktree,
// and attaches.
func (c Controller) Review(ws *config.Workspace, wsName string, num int, repo string) error {
	if repo == "" {
		repo = ws.BranchRepo
	}
	p, err := c.prs.ByFlag(repo, num)
	if err != nil {
		return err
	}
	sess, err := c.Apply(ws, wsName, p)
	if err != nil {
		return err
	}
	return c.sessions.SwitchOrAttach(sess)
}
