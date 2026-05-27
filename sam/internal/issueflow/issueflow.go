// Package issueflow holds the source-agnostic core of `sam from-issue`:
// resolving an issue (from a GitHub Project backlog or `gh issue list`),
// planning its branch name, and bootstrapping the worktree + tmux
// session. The interactive decisions (which issue, whether to reassign,
// editing the branch name) are made by callers — the CLI flag path and
// the TUI — so this package contains no prompting.
package issueflow

import (
	"fmt"
	"slices"

	"github.com/nicomaioli-skinandme/dotfiles/sam/internal/config"
	"github.com/nicomaioli-skinandme/dotfiles/sam/internal/ghx"
	"github.com/nicomaioli-skinandme/dotfiles/sam/internal/gitx"
	"github.com/nicomaioli-skinandme/dotfiles/sam/internal/setup"
	"github.com/nicomaioli-skinandme/dotfiles/sam/internal/tmuxx"
)

// StatusInProgress mirrors the literal label the legacy bash flow
// compared against. The project field ID + option ID for the actual
// write live in config.
const StatusInProgress = "🏗 In Progress"

// Issue is the single resolved shape downstream bootstrap consumes,
// produced by both project-board and plain-issue sources. ItemID/Status
// stay empty when no GitHub Project is configured.
type Issue struct {
	ItemID     string
	Status     string
	Assignees  []string
	Repository string
	Number     int
	Title      string
}

// HasGhProject reports whether the workspace links a GitHub Project.
func HasGhProject(ws *config.Workspace) bool {
	return ws.GhProject.Owner != "" && ws.GhProject.Number != 0
}

// List returns the candidate issues for the workspace: the project
// backlog when a GitHub Project is configured, otherwise open issues
// from the branch repo.
func List(ws *config.Workspace) ([]Issue, error) {
	if HasGhProject(ws) {
		return Backlog(ws)
	}
	return OpenIssues(ws)
}

// Backlog returns project-board items whose repo and status mark them as
// backlog (see config.GhProject.IssueRepos / BacklogStatuses).
func Backlog(ws *config.Workspace) ([]Issue, error) {
	items, err := ghx.ProjectItems(ws.GhProject)
	if err != nil {
		return nil, err
	}
	backlog := FilterBacklog(items, ws.GhProject.IssueRepos, ws.GhProject.BacklogStatuses)
	out := make([]Issue, len(backlog))
	for i, it := range backlog {
		out[i] = fromProjectItem(it)
	}
	return out, nil
}

// OpenIssues returns open issues from the branch repo.
func OpenIssues(ws *config.Workspace) ([]Issue, error) {
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
func ByFlag(ws *config.Workspace, num int, repo string) (Issue, error) {
	if HasGhProject(ws) {
		items, err := ghx.ProjectItems(ws.GhProject)
		if err != nil {
			return Issue{}, err
		}
		found, ok := FindItem(items, num, repo)
		if !ok {
			return Issue{}, fmt.Errorf("issue %s#%d is not on project %s/#%d",
				repo, num, ws.GhProject.Owner, ws.GhProject.Number)
		}
		return fromProjectItem(found), nil
	}
	issue, err := ghx.IssueView(repo, num)
	if err != nil {
		return Issue{}, err
	}
	return fromIssue(issue), nil
}

// Plan computes the branch name to use for an issue and reports whether a
// branch already exists on the remote (created by an earlier `gh issue
// develop`). When none exists, the name is derived from the issue number
// and a slug of its title.
func Plan(ws *config.Workspace, issue Issue) (branch, existing string, err error) {
	existing, _ = ghx.IssueDevelopList(issue.Repository, issue.Number)
	branch = existing
	if branch == "" {
		branch = fmt.Sprintf("%d-%s", issue.Number, gitx.Slugify(issue.Title))
	}
	return branch, existing, nil
}

// NeedsReassign reports the current other assignee when the issue is
// assigned to someone other than me (and not to me at all).
func NeedsReassign(issue Issue, me string) (other string, needs bool) {
	if len(issue.Assignees) == 0 || slices.Contains(issue.Assignees, me) {
		return "", false
	}
	return issue.Assignees[0], true
}

// NeedsBranchEdit reports whether branch exceeds the workspace's
// configured maximum length (0 = no limit).
func NeedsBranchEdit(ws *config.Workspace, branch string) bool {
	return ws.MaxBranchLen > 0 && len(branch) > ws.MaxBranchLen
}

// Apply runs the from-issue bootstrap given the caller's decisions and
// returns the tmux session name to attach to. reassign must be true to
// move an issue off another assignee; branch/existing come from Plan
// (branch may have been edited by the caller). It assigns the issue,
// moves its project status to In Progress, creates the branch/worktree,
// builds the tmux session, and adds the Claude pane — idempotently.
func Apply(ws *config.Workspace, workspaceName string, issue Issue, me string, reassign bool, branch, existing string) (string, error) {
	issueRepo := issue.Repository
	issueNum := issue.Number

	switch {
	case len(issue.Assignees) == 0:
		if err := ghx.IssueAddAssignee(issueRepo, issueNum, me); err != nil {
			return "", err
		}
	case slices.Contains(issue.Assignees, me):
		// already mine
	default:
		if !reassign {
			return "", fmt.Errorf("issue %s#%d assigned to %s; reassign not approved",
				issueRepo, issueNum, issue.Assignees[0])
		}
		if err := ghx.IssueSwapAssignee(issueRepo, issueNum, issue.Assignees[0], me); err != nil {
			return "", err
		}
	}

	if HasGhProject(ws) && issue.ItemID != "" && issue.Status != StatusInProgress {
		if err := ghx.ProjectItemSetStatus(ws.GhProject, issue.ItemID, ws.GhProject.InProgressID); err != nil {
			return "", err
		}
	}

	// A branch edit (branch != existing) renames the remote branch gh
	// already created before we check it out locally.
	if existing != "" && branch != existing {
		if err := gitx.PushRefspec(ws.Repo, "origin/"+existing, branch); err != nil {
			return "", err
		}
		if err := gitx.PushDelete(ws.Repo, existing); err != nil {
			return "", err
		}
	}
	if existing == "" {
		if err := ghx.IssueDevelop(issueRepo, ws.BranchRepo, issueNum, branch); err != nil {
			return "", err
		}
	}

	// Fetch so the branch gh just created on the remote is locally reachable.
	if err := gitx.Fetch(ws.Repo); err != nil {
		return "", err
	}
	if err := gitx.FastForwardMain(ws.Repo, ws.MainBranch); err != nil {
		return "", err
	}

	path, err := setup.CreateWorktree(ws, branch, issueNum, workspaceName)
	if err != nil {
		return "", err
	}

	if err := tmuxx.EnsureSystemSession(); err != nil {
		return "", err
	}
	if !tmuxx.HasSession(branch) {
		if err := tmuxx.BuildSession(branch, ws, path); err != nil {
			return "", err
		}
		data := tmuxx.ClaudeData{
			IssueNumber: issueNum,
			IssueTitle:  issue.Title,
			IssueRepo:   issueRepo,
			IssueURL:    fmt.Sprintf("https://github.com/%s/issues/%d", issueRepo, issueNum),
		}
		if err := tmuxx.AddClaudePane(branch, ws, data, path); err != nil {
			return "", err
		}
	}
	return branch, nil
}

// FilterBacklog returns items whose repo is in repos AND status is in
// statuses. Pure for testability.
func FilterBacklog(items []ghx.ProjectItem, repos, statuses []string) []ghx.ProjectItem {
	out := make([]ghx.ProjectItem, 0, len(items))
	for _, it := range items {
		if !slices.Contains(repos, it.Content.Repository) {
			continue
		}
		if !slices.Contains(statuses, it.Status) {
			continue
		}
		out = append(out, it)
	}
	return out
}

// FindItem returns the item matching (num, repo). Pure for testability.
func FindItem(items []ghx.ProjectItem, num int, repo string) (ghx.ProjectItem, bool) {
	for _, it := range items {
		if it.Content.Number == num && it.Content.Repository == repo {
			return it, true
		}
	}
	return ghx.ProjectItem{}, false
}

func fromProjectItem(it ghx.ProjectItem) Issue {
	return Issue{
		ItemID:     it.ID,
		Status:     it.Status,
		Assignees:  it.Assignees,
		Repository: it.Content.Repository,
		Number:     it.Content.Number,
		Title:      it.Content.Title,
	}
}

func fromIssue(it ghx.Issue) Issue {
	return Issue{
		Assignees:  it.Assignees,
		Repository: it.Repository,
		Number:     it.Number,
		Title:      it.Title,
	}
}
