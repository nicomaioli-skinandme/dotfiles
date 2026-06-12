package issue

import (
	"testing"

	"github.com/nicomaioli-skinandme/dotfiles/sam/internal/config"
	"github.com/nicomaioli-skinandme/dotfiles/sam/internal/ghx"
)

func mkItem(id string, num int, repo, title, status string, assignees ...string) ghx.ProjectItem {
	it := ghx.ProjectItem{ID: id, Status: status, Assignees: assignees}
	it.Content.Number = num
	it.Content.Repository = repo
	it.Content.Title = title
	return it
}

func TestFilterByColumns(t *testing.T) {
	issues := []Issue{
		{Number: 1, Status: "📋 Backlog"},
		{Number: 2, Status: "🏗 In Progress"},
		{Number: 3, Status: "Platform Backlog"},
		{Number: 4, Status: "✅ Done"},
	}
	columns := []string{"📋 Backlog", "Platform Backlog"}

	got := filterByColumns(issues, columns)
	if len(got) != 2 || got[0].Number != 1 || got[1].Number != 3 {
		t.Fatalf("filterByColumns = %v, want #1 and #3", got)
	}

	// Empty columns is a no-op: every issue passes (the TUI's "all columns").
	if r := filterByColumns(issues, nil); len(r) != len(issues) {
		t.Errorf("nil columns: got len=%d, want %d", len(r), len(issues))
	}
}

func TestFilterOpenBoard(t *testing.T) {
	items := []ghx.ProjectItem{
		mkItem("a", 1, "org/api", "open backlog", "📋 Backlog"),
		mkItem("b", 2, "org/api", "open in-progress", "🏗 In Progress"),
		mkItem("c", 3, "org/api", "closed/absent", "✅ Done"),
		mkItem("d", 4, "org/web", "repo not tracked", "📋 Backlog"),
	}
	// open holds only the issues `gh issue list --state open` returned for the
	// tracked repos: #1 and #2 in org/api. #3 is closed (absent), org/web isn't
	// tracked at all.
	open := map[string]bool{
		issueKey("org/api", 1): true,
		issueKey("org/api", 2): true,
	}

	got := filterOpenBoard(items, open)
	if len(got) != 2 {
		t.Fatalf("len=%d, want 2 (%v)", len(got), got)
	}
	// Order preserved from items; all columns kept (no status filter).
	if got[0].Number != 1 || got[0].Status != "📋 Backlog" {
		t.Errorf("got[0]=%+v, want #1 Backlog", got[0])
	}
	if got[1].Number != 2 || got[1].Status != "🏗 In Progress" {
		t.Errorf("got[1]=%+v, want #2 In Progress", got[1])
	}

	if r := filterOpenBoard(items, nil); len(r) != 0 {
		t.Errorf("nothing open: got len=%d", len(r))
	}
}

func TestFindItem(t *testing.T) {
	items := []ghx.ProjectItem{
		mkItem("a", 1, "org/api", "x", "📋 Backlog"),
		mkItem("b", 2, "org/api", "y", "📋 Backlog"),
		mkItem("c", 1, "org/web", "z", "📋 Backlog"),
	}

	got, ok := findItem(items, 2, "org/api")
	if !ok || got.ID != "b" {
		t.Errorf("want b/true, got %s/%v", got.ID, ok)
	}
	got, ok = findItem(items, 1, "org/web")
	if !ok || got.ID != "c" {
		t.Errorf("want c/true, got %s/%v", got.ID, ok)
	}
	if _, ok := findItem(items, 99, "org/api"); ok {
		t.Error("want false for missing num")
	}
	if _, ok := findItem(items, 1, "org/missing"); ok {
		t.Error("want false for missing repo")
	}
}

func TestNeedsReassign(t *testing.T) {
	cases := []struct {
		name      string
		assignees []string
		me        string
		other     string
		needs     bool
	}{
		{"unassigned", nil, "me", "", false},
		{"mine", []string{"me"}, "me", "", false},
		{"someone else", []string{"alice"}, "me", "alice", true},
		{"mixed includes me", []string{"alice", "me"}, "me", "", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			other, needs := Service{}.NeedsReassign(Issue{Assignees: c.assignees}, c.me)
			if other != c.other || needs != c.needs {
				t.Errorf("got (%q,%v), want (%q,%v)", other, needs, c.other, c.needs)
			}
		})
	}
}

func TestNeedsBranchEdit(t *testing.T) {
	cases := []struct {
		limit  int
		branch string
		want   bool
	}{
		{0, "anything-at-all-very-long", false}, // no limit
		{10, "short", false},
		{10, "this-is-too-long", true},
		{10, "exactly-10", false}, // len 10, not > 10
	}
	for _, c := range cases {
		ws := &config.Workspace{MaxBranchLen: c.limit}
		if got := (Service{}).NeedsBranchEdit(ws, c.branch); got != c.want {
			t.Errorf("NeedsBranchEdit(limit=%d, %q)=%v, want %v", c.limit, c.branch, got, c.want)
		}
	}
}
