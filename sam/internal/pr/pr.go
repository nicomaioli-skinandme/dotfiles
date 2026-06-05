// Package pr is the pr entity: its Service lists the PRs awaiting your
// review and resolves a specific one; its Controller bootstraps a worktree
// + tmux session on a PR's head branch for review. Unlike issue it makes no
// GitHub writes — it checks out the existing head branch read-only. The
// Service imports only infra (ghx); the cross-entity edges (worktree,
// session) live in the Controller.
package pr

import "github.com/nicomaioli-skinandme/dotfiles/sam/internal/ghx"

// PR is the resolved shape downstream bootstrap consumes.
type PR struct {
	Repository  string
	Number      int
	Title       string
	HeadRefName string
	Author      string
	IsDraft     bool
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
