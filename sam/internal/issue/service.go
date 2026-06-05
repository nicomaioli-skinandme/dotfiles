package issue

import (
	"fmt"
	"slices"

	"github.com/nicomaioli-skinandme/dotfiles/sam/internal/config"
	"github.com/nicomaioli-skinandme/dotfiles/sam/internal/ghx"
	"github.com/nicomaioli-skinandme/dotfiles/sam/internal/gitx"
)

// Service holds the issue primitives: resolving candidate issues, planning
// branch names, the reassign/branch checks, and the GitHub writes. Infra-
// only (ghx, gitx, config); the zero value is ready to use.
type Service struct{}

// CurrentUser returns the logged-in gh user (for assignment checks).
func (Service) CurrentUser() (string, error) {
	return ghx.CurrentUser()
}

// HasGhProject reports whether the workspace links a GitHub Project.
func (Service) HasGhProject(ws *config.Workspace) bool {
	return ws.GhProject.Owner != "" && ws.GhProject.Number != 0
}

// List returns the candidate issues for the workspace: the project backlog
// when a GitHub Project is configured, otherwise open issues from the
// branch repo.
func (s Service) List(ws *config.Workspace) ([]Issue, error) {
	if s.HasGhProject(ws) {
		return s.Backlog(ws)
	}
	return s.OpenIssues(ws)
}

// Backlog returns project-board items whose repo and status mark them as
// backlog (see config.GhProject.IssueRepos / BacklogStatuses).
func (Service) Backlog(ws *config.Workspace) ([]Issue, error) {
	items, err := ghx.ProjectItems(ws.GhProject)
	if err != nil {
		return nil, err
	}
	backlog := filterBacklog(items, ws.GhProject.IssueRepos, ws.GhProject.BacklogStatuses)
	out := make([]Issue, len(backlog))
	for i, it := range backlog {
		out[i] = fromProjectItem(it)
	}
	return out, nil
}

// OpenIssues returns open issues from the branch repo.
func (Service) OpenIssues(ws *config.Workspace) ([]Issue, error) {
	issues, err := ghx.IssueList(ws.BranchRepo)
	if err != nil {
		return nil, err
	}
	out := make([]Issue, len(issues))
	for i, it := range issues {
		out[i] = fromIssue(it)
	}
	return out, nil
}

// ByFlag resolves a specific issue for the non-interactive CLI path. It
// uses the project board when configured so status/assignee metadata is
// populated; otherwise it views the issue directly.
func (s Service) ByFlag(ws *config.Workspace, num int, repo string) (Issue, error) {
	if s.HasGhProject(ws) {
		items, err := ghx.ProjectItems(ws.GhProject)
		if err != nil {
			return Issue{}, err
		}
		found, ok := findItem(items, num, repo)
		if !ok {
			return Issue{}, fmt.Errorf("issue %s#%d is not on project %s/#%d",
				repo, num, ws.GhProject.Owner, ws.GhProject.Number)
		}
		return fromProjectItem(found), nil
	}
	iss, err := ghx.IssueView(repo, num)
	if err != nil {
		return Issue{}, err
	}
	return fromIssue(iss), nil
}

// Plan computes the branch name to use for an issue and reports whether a
// branch already exists on the remote (created by an earlier `gh issue
// develop`). When none exists, the name is derived from the issue number
// and a slug of its title.
func (Service) Plan(ws *config.Workspace, iss Issue) (branch, existing string, err error) {
	existing, _ = ghx.IssueDevelopList(iss.Repository, iss.Number)
	branch = existing
	if branch == "" {
		branch = fmt.Sprintf("%d-%s", iss.Number, gitx.Slugify(iss.Title))
	}
	return branch, existing, nil
}

// NeedsReassign reports the current other assignee when the issue is
// assigned to someone other than me (and not to me at all).
func (Service) NeedsReassign(iss Issue, me string) (other string, needs bool) {
	if len(iss.Assignees) == 0 || slices.Contains(iss.Assignees, me) {
		return "", false
	}
	return iss.Assignees[0], true
}

// NeedsBranchEdit reports whether branch exceeds the workspace's configured
// maximum length (0 = no limit).
func (Service) NeedsBranchEdit(ws *config.Workspace, branch string) bool {
	return ws.MaxBranchLen > 0 && len(branch) > ws.MaxBranchLen
}

// Assign makes the issue mine: it adds me when unassigned, no-ops when
// already mine, and swaps off another assignee only when reassign is true
// (otherwise it errors — the CLI requires an explicit --reassign).
func (Service) Assign(iss Issue, me string, reassign bool) error {
	switch {
	case len(iss.Assignees) == 0:
		return ghx.IssueAddAssignee(iss.Repository, iss.Number, me)
	case slices.Contains(iss.Assignees, me):
		return nil
	default:
		if !reassign {
			return fmt.Errorf("issue %s#%d assigned to %s; reassign not approved",
				iss.Repository, iss.Number, iss.Assignees[0])
		}
		return ghx.IssueSwapAssignee(iss.Repository, iss.Number, iss.Assignees[0], me)
	}
}

// SetInProgress moves the issue's project status to In Progress when a
// GitHub Project is configured and it isn't already there. No-op otherwise.
func (s Service) SetInProgress(ws *config.Workspace, iss Issue) error {
	if s.HasGhProject(ws) && iss.ItemID != "" && iss.Status != StatusInProgress {
		return ghx.ProjectItemSetStatus(ws.GhProject, iss.ItemID, ws.GhProject.InProgressID)
	}
	return nil
}

// RenameRemoteBranch renames the remote branch gh created (existing) to the
// caller's chosen branch before checkout. It fetches first so
// origin/<existing> is locally resolvable as the push source — existing
// came from the GitHub API, not local refs, so the remote-tracking ref may
// not exist yet.
func (Service) RenameRemoteBranch(ws *config.Workspace, existing, branch string) error {
	if err := gitx.Fetch(ws.Repo); err != nil {
		return err
	}
	if err := gitx.PushRefspec(ws.Repo, "origin/"+existing, branch); err != nil {
		return err
	}
	return gitx.PushDelete(ws.Repo, existing)
}

// Develop runs `gh issue develop`, creating the linked branch.
func (Service) Develop(issueRepo, branchRepo string, num int, branch string) error {
	return ghx.IssueDevelop(issueRepo, branchRepo, num, branch)
}
