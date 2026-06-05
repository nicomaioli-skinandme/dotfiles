// Package prflow holds the source-agnostic core of `sam from-pr`:
// listing the open PRs that request you as a reviewer, and bootstrapping
// a worktree + tmux session on a PR's head branch for review. Unlike
// issueflow it creates no branch and changes no GitHub state — it checks
// out the PR's existing head branch read-only. The interactive decision
// (which PR) is made by callers (the CLI flag path and the TUI), so this
// package contains no prompting.
package prflow

import (
	"fmt"

	"github.com/nicomaioli-skinandme/dotfiles/sam/internal/config"
	"github.com/nicomaioli-skinandme/dotfiles/sam/internal/ghx"
	"github.com/nicomaioli-skinandme/dotfiles/sam/internal/gitx"
	"github.com/nicomaioli-skinandme/dotfiles/sam/internal/setup"
	"github.com/nicomaioli-skinandme/dotfiles/sam/internal/tmuxx"
)

// PR is the resolved shape downstream bootstrap consumes.
type PR struct {
	Repository  string
	Number      int
	Title       string
	HeadRefName string
	Author      string
	IsDraft     bool
}

// List returns the open PRs in the workspace's branch repo that request
// the current gh user as a reviewer.
func List(ws *config.Workspace) ([]PR, error) {
	prs, err := ghx.PRsForReview(ws.BranchRepo)
	if err != nil {
		return nil, err
	}
	out := make([]PR, len(prs))
	for i, p := range prs {
		out[i] = fromGh(p)
	}
	return out, nil
}

// ByFlag resolves a specific PR for the non-interactive CLI path.
func ByFlag(ws *config.Workspace, num int, repo string) (PR, error) {
	pr, err := ghx.PRView(repo, num)
	if err != nil {
		return PR{}, err
	}
	return fromGh(pr), nil
}

// Apply bootstraps the review worktree for a PR and returns the tmux
// session name to attach to. It fetches so the PR's head branch is
// locally reachable, fast-forwards the trunk, creates the worktree on the
// head branch, builds the tmux session, and adds the Claude review pane —
// idempotently. It makes no changes on GitHub.
func Apply(ws *config.Workspace, workspaceName string, pr PR) (string, error) {
	branch := pr.HeadRefName

	// Fetch so origin/<branch> is reachable for the worktree checkout — the
	// branch may exist only on the remote (the PR author just pushed it).
	if err := gitx.Fetch(ws.Repo); err != nil {
		return "", err
	}
	if err := gitx.FastForwardTrunk(ws.Repo, ws.Trunk); err != nil {
		return "", err
	}

	path, err := setup.CreateWorktree(ws, branch, 0, workspaceName)
	if err != nil {
		return "", err
	}

	session := tmuxx.SessionName(workspaceName, branch)
	if !tmuxx.HasSession(session) {
		if err := tmuxx.BuildSession(session, ws, path); err != nil {
			return "", err
		}
		data := tmuxx.ClaudeData{
			PRNumber: pr.Number,
			PRTitle:  pr.Title,
			PRRepo:   pr.Repository,
			PRURL:    fmt.Sprintf("https://github.com/%s/pull/%d", pr.Repository, pr.Number),
			PRAuthor: pr.Author,
			PRBranch: branch,
		}
		if err := tmuxx.AddClaudePane(session, ws.FromPR.RepoWindow, ws.FromPR.ClaudePrompt, ws.FromPR.ClaudePaneTitle, ws.FromPR.PermissionMode, data, path); err != nil {
			return "", err
		}
	}
	return session, nil
}

func fromGh(p ghx.PR) PR {
	return PR{
		Repository:  p.Repository,
		Number:      p.Number,
		Title:       p.Title,
		HeadRefName: p.HeadRefName,
		Author:      p.Author,
		IsDraft:     p.IsDraft,
	}
}
