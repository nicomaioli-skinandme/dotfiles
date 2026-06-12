package tui

import (
	"strings"

	"charm.land/lipgloss/v2"
)

// sidebar is a reusable, model-decoupled left-hand filter panel: a stack of
// collapsible sections, each holding toggleable items (facet values). It owns
// its section / collapse / toggle state and the highlight, holds no reference
// to *model, and takes only primitives + palette styles — so it can back any
// faceted view (issue columns today, log levels, assignees/repos later).
//
// Navigation works over a flattened row list (each section header followed by
// its items, items hidden while the section is collapsed); Move walks it and
// Act operates on the highlighted row (collapse a header, toggle an item). The
// model seeds sections with SetSections and reads back the on-items with
// Selected to drive its filter.
type sidebar struct {
	sections []sidebarSection
	row      int  // highlight into the flattened visible-row list
	focused  bool // whether the model is routing keys here (vs the main list)
	width    int

	// Palette styles copied from styles{} so the panel renders in theme.
	header   lipgloss.Style
	item     lipgloss.Style
	selected lipgloss.Style
	cursor   lipgloss.Style
	hint     lipgloss.Style
}

type sidebarSection struct {
	key       string // stable id the model filters on, e.g. "columns", "level"
	title     string
	collapsed bool
	items     []sidebarItem
}

type sidebarItem struct {
	label string
	on    bool
}

// sbRow names one flattened row: a section header (item == -1) or an item.
type sbRow struct {
	section int
	item    int
}

func newSidebar(width int, st styles) sidebar {
	return sidebar{
		width:    width,
		header:   st.row,
		item:     st.row,
		selected: st.selected,
		cursor:   st.cursor,
		hint:     st.hint,
	}
}

// rows flattens sections to the navigable/rendered row list, skipping the
// items of collapsed sections.
func (s *sidebar) rows() []sbRow {
	rows := make([]sbRow, 0, len(s.sections))
	for si, sec := range s.sections {
		rows = append(rows, sbRow{section: si, item: -1})
		if sec.collapsed {
			continue
		}
		for ii := range sec.items {
			rows = append(rows, sbRow{section: si, item: ii})
		}
	}
	return rows
}

func (s *sidebar) Empty() bool { return len(s.sections) == 0 }

// hasSection reports whether a section with the given key is present. The
// model uses it to tell "no facet for this view" (filter passes everything)
// from "facet present but all toggled off" (filter passes nothing).
func (s *sidebar) hasSection(key string) bool {
	for _, sec := range s.sections {
		if sec.key == key {
			return true
		}
	}
	return false
}

func (s *sidebar) Width() int { return s.width }
func (s *sidebar) Focused() bool {
	return s.focused
}
func (s *sidebar) SetFocused(f bool) { s.focused = f }

// Move shifts the highlight by delta, clamped (no wrap).
func (s *sidebar) Move(delta int) {
	s.row += delta
	s.clampRow()
}

// Act toggles the highlighted row: collapse/expand a section header, or flip
// an item on/off. Called by the model for both Enter and space.
func (s *sidebar) Act() {
	rows := s.rows()
	if s.row < 0 || s.row >= len(rows) {
		return
	}
	r := rows[s.row]
	if r.item < 0 {
		s.sections[r.section].collapsed = !s.sections[r.section].collapsed
		s.clampRow() // collapsing shrinks the row list
		return
	}
	it := &s.sections[r.section].items[r.item]
	it.on = !it.on
}

// Selected returns the set of on-item labels for the section with the given
// key (empty when the section is absent).
func (s *sidebar) Selected(key string) map[string]bool {
	out := map[string]bool{}
	for _, sec := range s.sections {
		if sec.key != key {
			continue
		}
		for _, it := range sec.items {
			if it.on {
				out[it.label] = true
			}
		}
	}
	return out
}

// SetSections installs a fresh section set, preserving the prior collapse and
// toggle state for any section/item that still exists (matched by key/label).
// This keeps a reload (R, back-nav) or a re-seed from non-stale to keep the
// user's filter rather than snapping back to defaults; genuinely new items
// keep their seeded default.
func (s *sidebar) SetSections(next []sidebarSection) {
	prevCollapsed := map[string]bool{}
	prevOn := map[string]bool{}
	for _, sec := range s.sections {
		prevCollapsed[sec.key] = sec.collapsed
		for _, it := range sec.items {
			prevOn[sec.key+"\x00"+it.label] = it.on
		}
	}
	for si := range next {
		if c, ok := prevCollapsed[next[si].key]; ok {
			next[si].collapsed = c
		}
		for ii := range next[si].items {
			if on, ok := prevOn[next[si].key+"\x00"+next[si].items[ii].label]; ok {
				next[si].items[ii].on = on
			}
		}
	}
	s.sections = next
	s.clampRow()
}

func (s *sidebar) clampRow() {
	n := len(s.rows())
	if s.row >= n {
		s.row = n - 1
	}
	if s.row < 0 {
		s.row = 0
	}
}

// View renders the panel to exactly height lines at the configured width.
// Section headers show ▾/▸ for expanded/collapsed; items show [✓]/[ ] + label.
// The highlighted row is drawn in the cursor style only while focused, so an
// unfocused panel doesn't compete with the main-list cursor.
func (s *sidebar) View(height int) string {
	rows := s.rows()
	lines := make([]string, 0, height)
	for ri, r := range rows {
		highlighted := s.focused && ri == s.row
		if r.item < 0 {
			sec := s.sections[r.section]
			arrow := "▾"
			if sec.collapsed {
				arrow = "▸"
			}
			text := arrow + " " + sec.title
			if highlighted {
				text = s.cursor.Render(text)
			} else {
				text = s.header.Render(text)
			}
			lines = append(lines, truncate(text, s.width))
			continue
		}
		it := s.sections[r.section].items[r.item]
		box := "[ ]"
		if it.on {
			box = s.selected.Render("[✓]")
		}
		ptr := "  "
		label := it.label
		switch {
		case highlighted:
			ptr = s.cursor.Render("▸ ")
			label = s.cursor.Render(label)
		case it.on:
			label = s.item.Render(label)
		default:
			label = s.hint.Render(label)
		}
		lines = append(lines, truncate(ptr+box+" "+label, s.width))
	}
	for len(lines) < height {
		lines = append(lines, "")
	}
	if len(lines) > height {
		lines = lines[:height]
	}
	return strings.Join(lines, "\n")
}
