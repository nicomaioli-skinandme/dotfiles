// Package issue is the issue entity: its Service resolves issues (from a
// GitHub Project backlog or `gh issue list`), plans branch names, and owns
// the GitHub writes (assign, set-in-progress, develop, branch rename); its
// Controller orchestrates the develop flow across the worktree and session
// entities. The Service imports only infra (ghx, gitx); the cross-entity
// edges (worktree, session) live in the Controller.
package issue

import (
	"fmt"
	"slices"

	"github.com/nicomaioli-skinandme/dotfiles/sam/internal/ghx"
)

// StatusInProgress mirrors the literal label the legacy bash flow compared
// against. The project field ID + option ID for the actual write live in
// config.
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

// Filter narrows the issues List returns. The zero value means "every open
// issue" (all columns). Fields are AND-combined; an empty field matches any.
// New facets (assignee, repo, …) become new fields here, so both the TUI and
// the CLI keep calling the same List with a richer Filter rather than growing
// parallel methods.
type Filter struct {
	Columns []string // status columns to include; empty = all columns
}

// filterByColumns keeps issues whose Status is in columns. An empty columns
// slice is a no-op (all pass). Pure for testability.
func filterByColumns(issues []Issue, columns []string) []Issue {
	if len(columns) == 0 {
		return issues
	}
	out := make([]Issue, 0, len(issues))
	for _, iss := range issues {
		if slices.Contains(columns, iss.Status) {
			out = append(out, iss)
		}
	}
	return out
}

// issueKey is the stable repo#number identity used to intersect board items
// with a repo's open-issue list (and matches the TUI's row IDs).
func issueKey(repo string, num int) string {
	return fmt.Sprintf("%s#%d", repo, num)
}

// filterOpenBoard keeps the board items whose (repo, number) is in open and
// maps them to Issues, preserving each item's column (Status). Items in repos
// absent from open (i.e. not in IssueRepos) and non-open items (closed issues,
// PRs, draft items — none of which appear in `gh issue list`) are dropped.
// Pure for testability.
func filterOpenBoard(items []ghx.ProjectItem, open map[string]bool) []Issue {
	out := make([]Issue, 0, len(items))
	for _, it := range items {
		if open[issueKey(it.Content.Repository, it.Content.Number)] {
			out = append(out, fromProjectItem(it))
		}
	}
	return out
}

// findItem returns the item matching (num, repo). Pure for testability.
func findItem(items []ghx.ProjectItem, num int, repo string) (ghx.ProjectItem, bool) {
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
