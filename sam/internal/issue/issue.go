// Package issue is the issue entity: its Service resolves issues (from a
// GitHub Project backlog or `gh issue list`), plans branch names, and owns
// the GitHub writes (assign, set-in-progress, develop, branch rename); its
// Controller orchestrates the develop flow across the worktree and session
// entities. The Service imports only infra (ghx, gitx); the cross-entity
// edges (worktree, session) live in the Controller.
package issue

import (
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

// filterBacklog returns items whose repo is in repos AND status is in
// statuses. Pure for testability.
func filterBacklog(items []ghx.ProjectItem, repos, statuses []string) []ghx.ProjectItem {
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
