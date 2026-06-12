package tui

import "testing"

func newTestSidebar() sidebar {
	sb := newSidebar(26, styles{})
	sb.SetSections([]sidebarSection{
		{key: "columns", title: "Columns", items: []sidebarItem{
			{label: "Backlog", on: true},
			{label: "In Progress", on: false},
			{label: "Done", on: false},
		}},
		{key: "level", title: "Level", items: []sidebarItem{
			{label: "ERROR", on: true},
			{label: "INFO", on: true},
		}},
	})
	return sb
}

func TestSidebarRowsAndNav(t *testing.T) {
	sb := newTestSidebar()
	// 2 headers + 3 + 2 items = 7 rows when nothing collapsed.
	if got := len(sb.rows()); got != 7 {
		t.Fatalf("rows=%d, want 7", got)
	}
	sb.SetFocused(true)

	// Move clamps at the top and bottom.
	sb.Move(-5)
	if sb.row != 0 {
		t.Errorf("row after Move(-5)=%d, want 0", sb.row)
	}
	sb.Move(100)
	if sb.row != 6 {
		t.Errorf("row after Move(100)=%d, want 6 (last)", sb.row)
	}
}

func TestSidebarToggleItem(t *testing.T) {
	sb := newTestSidebar()
	// Row 2 is the "In Progress" item (row0 header, row1 Backlog, row2 In Progress).
	sb.row = 2
	sb.Act()
	if !sb.Selected("columns")["In Progress"] {
		t.Errorf("In Progress should be on after Act; selected=%v", sb.Selected("columns"))
	}
	sb.Act()
	if sb.Selected("columns")["In Progress"] {
		t.Error("In Progress should toggle back off")
	}
}

func TestSidebarCollapseHidesItems(t *testing.T) {
	sb := newTestSidebar()
	// Row 0 is the Columns header.
	sb.row = 0
	sb.Act() // collapse Columns
	// 1 header (Columns, collapsed) + 1 header (Level) + 2 items = 4 rows.
	if got := len(sb.rows()); got != 4 {
		t.Fatalf("rows after collapse=%d, want 4", got)
	}
	sb.Act() // expand again
	if got := len(sb.rows()); got != 7 {
		t.Fatalf("rows after expand=%d, want 7", got)
	}
}

func TestSidebarSelected(t *testing.T) {
	sb := newTestSidebar()
	cols := sb.Selected("columns")
	if len(cols) != 1 || !cols["Backlog"] {
		t.Errorf("columns selected=%v, want {Backlog}", cols)
	}
	if got := sb.Selected("missing"); len(got) != 0 {
		t.Errorf("missing section selected=%v, want empty", got)
	}
}

func TestSidebarSetSectionsMergesState(t *testing.T) {
	sb := newTestSidebar()
	// User turns In Progress on and collapses Columns.
	sb.row = 2
	sb.Act()
	sb.row = 0
	sb.Act()

	// A reload re-seeds with defaults (Backlog on, others off, expanded) plus a
	// brand-new column.
	sb.SetSections([]sidebarSection{
		{key: "columns", title: "Columns", items: []sidebarItem{
			{label: "Backlog", on: true},
			{label: "In Progress", on: false},
			{label: "Done", on: false},
			{label: "Review", on: false}, // new
		}},
	})

	got := sb.Selected("columns")
	if !got["In Progress"] {
		t.Error("merge should preserve the user's In Progress toggle")
	}
	if !got["Backlog"] {
		t.Error("Backlog should stay on")
	}
	if got["Review"] {
		t.Error("new column should keep its seeded default (off)")
	}
	if !sb.sections[0].collapsed {
		t.Error("merge should preserve the collapsed state")
	}
}
