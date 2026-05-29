package tui

import (
	"unicode/utf8"

	"charm.land/lipgloss/v2"

	"github.com/sahilm/fuzzy"
)

// autocomplete is a self-contained, model-decoupled popup list: callers
// feed it a candidate pool plus the current query text and read back a
// highlighted, fuzzy-ranked selection. It holds no reference to *model and
// takes only primitives, so it can back any focused field (command mode
// today; search/branch-pick later). The zero value is a closed, empty
// popup; use newAutocomplete to seed maxVisible.
type autocomplete struct {
	open       bool
	candidates []string  // caller-supplied pool
	query      string    // current fuzzy filter (typed text only)
	matches    []acMatch // recomputed from candidates+query, ranked by fuzzy score
	cursor     int       // index into matches
	maxVisible int       // hard cap on rendered rows
}

// acMatch is one surviving candidate plus the byte offsets of the runes
// that matched the query, so View can highlight them. indices are byte
// offsets into value (sahilm/fuzzy reports byte positions), which lets the
// rune-walk in highlight stay correct for multi-byte strings.
type acMatch struct {
	value   string
	indices []int
}

const defaultAutocompleteMax = 5

// newAutocomplete returns a closed popup. max <= 0 falls back to the
// default so callers (and tests) can pass an unset config value safely.
func newAutocomplete(max int) autocomplete {
	if max <= 0 {
		max = defaultAutocompleteMax
	}
	return autocomplete{maxVisible: max}
}

// Open shows the popup over the given candidate pool with an empty query
// (so every candidate is visible) and the cursor on the first row.
func (a *autocomplete) Open(candidates []string) {
	a.open = true
	a.candidates = candidates
	a.query = ""
	a.cursor = 0
	a.recompute()
}

// Close hides the popup. Candidates are retained so a reopen is cheap.
func (a *autocomplete) Close() {
	a.open = false
}

// SetQuery refilters against the typed text. Call this only on real text
// edits — never on Tab/arrow navigation — so completing or cycling the
// highlight doesn't collapse the list to the accepted value.
func (a *autocomplete) SetQuery(q string) {
	a.query = q
	a.recompute()
}

// recompute rebuilds matches from candidates and the current query. An
// empty query shows the full pool in its given order (no highlight);
// otherwise fuzzy.Find ranks by relevance score, best first.
func (a *autocomplete) recompute() {
	if a.query == "" {
		a.matches = make([]acMatch, len(a.candidates))
		for i, c := range a.candidates {
			a.matches[i] = acMatch{value: c}
		}
	} else {
		found := fuzzy.Find(a.query, a.candidates)
		a.matches = make([]acMatch, len(found))
		for i, m := range found {
			a.matches[i] = acMatch{value: m.Str, indices: m.MatchedIndexes}
		}
	}
	a.clampCursor()
}

// Move shifts the highlight by delta, clamped to the match list (no wrap).
func (a *autocomplete) Move(delta int) {
	a.cursor += delta
	a.clampCursor()
}

// Cycle shifts the highlight by delta, wrapping around the ends. Used by
// Tab / Shift+Tab so repeated presses walk the whole list.
func (a *autocomplete) Cycle(delta int) {
	n := len(a.matches)
	if n == 0 {
		a.cursor = 0
		return
	}
	a.cursor = ((a.cursor+delta)%n + n) % n
}

func (a *autocomplete) clampCursor() {
	if a.cursor >= len(a.matches) {
		a.cursor = len(a.matches) - 1
	}
	if a.cursor < 0 {
		a.cursor = 0
	}
}

// Selected returns the highlighted value, or false when there are no
// matches.
func (a *autocomplete) Selected() (string, bool) {
	if a.cursor < 0 || a.cursor >= len(a.matches) {
		return "", false
	}
	return a.matches[a.cursor].value, true
}

// Visible reports whether the popup should be drawn: open with at least
// one match.
func (a *autocomplete) Visible() bool {
	return a.open && len(a.matches) > 0
}

// anchorPos is the cell the popup hangs off: row/col of the field it
// completes (e.g. the `:` prompt at row 0).
type anchorPos struct {
	row int
	col int
}

// View renders the bordered popup content (no positioning). At most
// maxVisible rows are shown via a scroll window around the cursor; the
// highlighted row is reversed and matched characters are accented. Rows
// are clamped to maxWidth display columns so a long entry never produces a
// popup wider than the screen.
func (a *autocomplete) View(maxWidth int) string {
	if !a.Visible() {
		return ""
	}

	// Leave room for the border (1 col each side) + padding (1 col each side).
	const frame = 4
	rowWidth := maxWidth - frame
	if rowWidth < 1 {
		rowWidth = 1
	}

	start, end := a.window()
	lines := make([]string, 0, end-start)
	for i := start; i < end; i++ {
		line := highlight(a.matches[i].value, a.matches[i].indices, acMatchStyle)
		if i == a.cursor {
			line = acSelectedStyle.Render(line)
		}
		lines = append(lines, truncate(line, rowWidth))
	}

	body := lipgloss.JoinVertical(lipgloss.Left, lines...)
	return autocompleteBorder.Render(body)
}

// window returns the [start,end) slice of matches to render, a scroll
// window of at most maxVisible rows kept around the cursor (mirrors
// renderList's centering).
func (a *autocomplete) window() (int, int) {
	n := len(a.matches)
	h := a.maxVisible
	if h > n {
		h = n
	}
	start := 0
	if n > h {
		start = a.cursor - h/2
		if start < 0 {
			start = 0
		}
		if start > n-h {
			start = n - h
		}
	}
	return start, start + h
}

// Position computes the popup's top-left (x, y) given the anchor, the
// popup's measured size, and the screen size. It prefers opening below the
// anchor and flips above when that would overflow the bottom edge; if
// neither fits (popup taller than the screen) it pins to the top. X aligns
// under the anchor column, clamped on-screen. Pure math — unit-testable.
func (a *autocomplete) Position(anchor anchorPos, popupW, popupH, screenW, screenH int) (int, int) {
	x := anchor.col
	if x+popupW > screenW {
		x = screenW - popupW
	}
	if x < 0 {
		x = 0
	}

	var y int
	below := anchor.row + 1
	switch {
	case below+popupH <= screenH:
		y = below
	case anchor.row-popupH >= 0:
		y = anchor.row - popupH
	default:
		y = screenH - popupH
		if y < 0 {
			y = 0
		}
	}
	return x, y
}

// highlight renders s with the runes at the given byte offsets styled.
// indices are byte offsets (as reported by sahilm/fuzzy); walking s by
// rune while tracking the byte offset keeps the highlight correct for
// multi-byte strings.
func highlight(s string, indices []int, style lipgloss.Style) string {
	if len(indices) == 0 {
		return s
	}
	mark := make(map[int]bool, len(indices))
	for _, i := range indices {
		mark[i] = true
	}

	var b []byte
	for off := 0; off < len(s); {
		_, size := utf8.DecodeRuneInString(s[off:])
		seg := s[off : off+size]
		if mark[off] {
			b = append(b, style.Render(seg)...)
		} else {
			b = append(b, seg...)
		}
		off += size
	}
	return string(b)
}
