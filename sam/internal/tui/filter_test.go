package tui

import (
	"log/slog"
	"testing"

	"github.com/nicomaioli-skinandme/dotfiles/sam/internal/config"
	"github.com/nicomaioli-skinandme/dotfiles/sam/internal/issue"
	"github.com/nicomaioli-skinandme/dotfiles/sam/internal/logx"
)

func ids(items []Item) []string {
	out := make([]string, len(items))
	for i, it := range items {
		out[i] = it.ID
	}
	return out
}

// Issues: the column sidebar filters the list, defaulting to the configured
// backlog columns and updating live as columns toggle on.
func TestIssuesColumnFilter(t *testing.T) {
	ws := &config.Workspace{Trunk: "main"}
	ws.GhProject.BacklogStatuses = []string{"Backlog"}
	m := newModel("ws", ws, nil, ResIssues, config.Tui{}, Deps{})

	m.resource = ResIssues
	m.items = []Item{{ID: "r#1"}, {ID: "r#2"}, {ID: "r#3"}}
	m.issues = map[string]issue.Issue{
		"r#1": {Status: "Backlog"},
		"r#2": {Status: "In Progress"},
		"r#3": {Status: "Done"},
	}
	m.columns = []string{"Backlog", "In Progress", "Done"}
	m.syncSidebar()
	m.applyFilter()

	// Default: only the backlog column is on.
	if got := ids(m.filtered); len(got) != 1 || got[0] != "r#1" {
		t.Fatalf("default filter = %v, want [r#1]", got)
	}

	// Turn "In Progress" on (sidebar rows: 0 header, 1 Backlog, 2 In Progress).
	m.focus = focusSidebar
	m.sidebar.SetFocused(true)
	m.sidebar.row = 2
	m.sidebar.Act()
	m.applyFilter()
	if got := ids(m.filtered); len(got) != 2 || got[0] != "r#1" || got[1] != "r#2" {
		t.Fatalf("after enabling In Progress = %v, want [r#1 r#2]", got)
	}
}

// Logs: the level sidebar starts all-on (everything shown) and can be narrowed
// to a single level.
func TestLogsLevelFilter(t *testing.T) {
	m := newModel("ws", &config.Workspace{Trunk: "main"}, nil, ResLogs, config.Tui{}, Deps{})
	m.resource = ResLogs
	m.items = []Item{{ID: "1"}, {ID: "2"}, {ID: "3"}}
	m.logEntries = map[string]logx.Entry{
		"1": {Level: slog.LevelError},
		"2": {Level: slog.LevelInfo},
		"3": {Level: slog.LevelWarn},
	}
	m.syncSidebar()
	m.applyFilter()
	if got := ids(m.filtered); len(got) != 3 {
		t.Fatalf("default level filter = %v, want all 3", got)
	}

	// Toggle WARN/INFO/DEBUG off the way the UI does (rows: 0 header, 1 ERROR,
	// 2 WARN, 3 INFO, 4 DEBUG), leaving only ERROR on.
	m.focus = focusSidebar
	m.sidebar.SetFocused(true)
	for _, r := range []int{2, 3, 4} {
		m.sidebar.row = r
		m.sidebar.Act()
	}
	m.applyFilter()
	if got := ids(m.filtered); len(got) != 1 || got[0] != "1" {
		t.Fatalf("ERROR-only filter = %v, want [1]", got)
	}
}

// A view with no facet section (e.g. a non-project issues list) filters on the
// `/` query alone — facetPass must not drop everything.
func TestNoFacetPassesAll(t *testing.T) {
	m := newModel("ws", &config.Workspace{Trunk: "main"}, nil, ResIssues, config.Tui{}, Deps{})
	m.resource = ResIssues
	m.items = []Item{{ID: "r#1", Title: "alpha"}, {ID: "r#2", Title: "beta"}}
	m.issues = map[string]issue.Issue{"r#1": {}, "r#2": {}}
	// No columns → no sidebar section.
	m.syncSidebar()
	if m.hasSidebar() {
		t.Fatal("non-project issues should have no sidebar")
	}
	m.applyFilter()
	if got := ids(m.filtered); len(got) != 2 {
		t.Fatalf("no-facet filter = %v, want both", got)
	}
	m.query = "alpha"
	m.applyFilter()
	if got := ids(m.filtered); len(got) != 1 || got[0] != "r#1" {
		t.Fatalf("query filter = %v, want [r#1]", got)
	}
}
