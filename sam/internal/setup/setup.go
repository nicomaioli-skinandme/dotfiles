// Package setup centralises worktree creation plus the optional
// per-workspace `worktree_setup` hook so from-issue and new-worktree
// flows share one seam.
package setup

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"

	"github.com/nicomaioli-skinandme/dotfiles/sam/internal/config"
	"github.com/nicomaioli-skinandme/dotfiles/sam/internal/gitx"
)

// CreateWorktree creates a worktree for `branch` under
// `workspace.Worktrees`, then runs the workspace's `worktree_setup` hook
// (if any) inside the new worktree directory. Returns the worktree
// path on success.
//
// Idempotent: if the worktree dir already exists, returns its path
// without re-creating or re-running the hook.
//
// `issueNumber` is exposed to the hook via SAM_ISSUE_NUMBER. Pass 0
// when there's no associated issue (e.g. from `new-worktree`).
func CreateWorktree(workspace *config.Workspace, branch string, issueNumber int, workspaceName string) (string, error) {
	path := filepath.Join(workspace.Worktrees, branch)
	if _, err := os.Stat(path); err == nil {
		return path, nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return "", err
	}
	if err := gitx.WorktreeAdd(workspace.Repo, path, branch); err != nil {
		return "", err
	}
	if workspace.WorktreeSetup == "" {
		return path, nil
	}
	if err := runHook(workspace, branch, path, issueNumber, workspaceName); err != nil {
		return "", fmt.Errorf("worktree_setup hook failed for %s: %w", branch, err)
	}
	return path, nil
}

func runHook(workspace *config.Workspace, branch, worktreePath string, issueNumber int, workspaceName string) error {
	issueStr := ""
	if issueNumber > 0 {
		issueStr = strconv.Itoa(issueNumber)
	}
	cmd := exec.Command("sh", "-c", workspace.WorktreeSetup)
	cmd.Dir = worktreePath
	cmd.Env = append(os.Environ(),
		"SAM_BRANCH="+branch,
		"SAM_WORKTREE="+worktreePath,
		"SAM_REPO="+workspace.Repo,
		"SAM_WORKSPACE="+workspaceName,
		"SAM_ISSUE_NUMBER="+issueStr,
	)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
