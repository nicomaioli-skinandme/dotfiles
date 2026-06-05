package tui

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/nicomaioli-skinandme/dotfiles/sam/internal/config"
)

func runeKey(r rune) tea.KeyPressMsg { return tea.KeyPressMsg{Text: string(r), Code: r} }

func TestCommandCandidates(t *testing.T) {
	got := commandCandidates()

	// Every resource name is present, plus quit.
	want := map[string]bool{"quit": false}
	for _, r := range resources {
		want[r.Name()] = false
	}
	if len(got) != len(want) {
		t.Fatalf("candidates: got %v, want %d entries", got, len(want))
	}
	for _, c := range got {
		if _, ok := want[c]; !ok {
			t.Errorf("unexpected candidate %q", c)
		}
		want[c] = true
	}
	for c, seen := range want {
		if !seen {
			t.Errorf("missing candidate %q", c)
		}
	}

	// Sorted for a stable empty-query display.
	if !sortedAsc(got) {
		t.Errorf("candidates not sorted: %v", got)
	}
}

func sortedAsc(s []string) bool {
	for i := 1; i < len(s); i++ {
		if s[i-1] > s[i] {
			return false
		}
	}
	return true
}

func TestAutocompleteRanking(t *testing.T) {
	a := newAutocomplete(5)
	a.Open([]string{"workspaces", "worktrees", "issues", "clankers", "quit"})

	// Empty query shows the whole pool, in given order, no highlight.
	if len(a.matches) != 5 {
		t.Fatalf("empty query: got %d matches, want 5", len(a.matches))
	}
	if a.matches[0].indices != nil {
		t.Errorf("empty query should not highlight: %v", a.matches[0].indices)
	}

	// A fuzzy query keeps only subsequence matches.
	a.SetQuery("iss")
	if len(a.matches) != 1 || a.matches[0].value != "issues" {
		t.Fatalf("query iss: got %+v", a.matches)
	}

	// A query matching several ranks them by relevance (best first); both
	// worktrees and workspaces contain "wo".
	a.SetQuery("wo")
	if len(a.matches) != 2 {
		t.Fatalf("query wo: got %d matches, want 2 (%+v)", len(a.matches), a.matches)
	}

	// A non-matching query empties the list and hides the popup.
	a.SetQuery("zzz")
	if len(a.matches) != 0 {
		t.Errorf("query zzz: got %+v, want none", a.matches)
	}
	if a.Visible() {
		t.Error("popup must not be visible with zero matches")
	}
}

func TestAutocompleteMaxVisibleCap(t *testing.T) {
	a := newAutocomplete(3)
	pool := []string{"aa", "ab", "ac", "ad", "ae", "af"}
	a.Open(pool) // empty query -> all 6 match

	out := a.View(40)
	// Count content rows between the top and bottom border lines.
	rows := contentRows(out)
	if rows != 3 {
		t.Fatalf("rendered rows: got %d, want 3 (cap)\n%s", rows, out)
	}

	// The cursor can still reach entries beyond the visible window.
	a.Move(5)
	if sel, _ := a.Selected(); sel != "af" {
		t.Errorf("cursor past window: got %q, want af", sel)
	}
}

// contentRows counts the inner rows of a bordered popup (total height minus
// the top and bottom border lines).
func contentRows(s string) int {
	h := lipgloss.Height(s)
	if h <= 2 {
		return 0
	}
	return h - 2
}

func TestAutocompleteMoveAndCycle(t *testing.T) {
	a := newAutocomplete(5)
	a.Open([]string{"a", "b", "c"})

	// Move clamps at the ends.
	a.Move(-1)
	if a.cursor != 0 {
		t.Errorf("move up at top: got %d, want 0", a.cursor)
	}
	a.Move(10)
	if a.cursor != 2 {
		t.Errorf("move down past end: got %d, want 2", a.cursor)
	}

	// Cycle wraps around.
	a.Cycle(1)
	if a.cursor != 0 {
		t.Errorf("cycle forward off the end: got %d, want 0", a.cursor)
	}
	a.Cycle(-1)
	if a.cursor != 2 {
		t.Errorf("cycle back off the start: got %d, want 2", a.cursor)
	}
}

func TestAutocompletePosition(t *testing.T) {
	a := newAutocomplete(5)
	cases := []struct {
		name                 string
		anchorRow, anchorCol int
		popupW, popupH       int
		screenW, screenH     int
		wantX, wantY         int
	}{
		{"below when it fits", 0, 1, 10, 4, 80, 24, 1, 1},
		{"flip above near bottom", 22, 0, 10, 4, 80, 24, 0, 18},
		{"pin top when taller than screen", 2, 0, 10, 30, 80, 24, 0, 0},
		{"clamp x at right edge", 0, 75, 10, 4, 80, 24, 70, 1},
	}
	for _, c := range cases {
		x, y := a.Position(anchorPos{row: c.anchorRow, col: c.anchorCol}, c.popupW, c.popupH, c.screenW, c.screenH)
		if x != c.wantX || y != c.wantY {
			t.Errorf("%s: got (%d,%d), want (%d,%d)", c.name, x, y, c.wantX, c.wantY)
		}
	}
}

func TestHighlight(t *testing.T) {
	style := lipgloss.NewStyle().Bold(true)

	// No indices -> string is returned verbatim.
	if got := highlight("issues", nil, style); got != "issues" {
		t.Errorf("no indices: got %q", got)
	}

	// Matched runes are wrapped in escapes; the plain text is preserved
	// when escapes are stripped, and the result differs from the input.
	got := highlight("issues", []int{0, 1, 2}, style)
	if got == "issues" {
		t.Error("expected matched runes to be styled")
	}
	if stripped := stripANSI(got); stripped != "issues" {
		t.Errorf("stripped highlight: got %q, want issues", stripped)
	}
}

// stripANSI removes CSI escape sequences so styled output can be compared
// against its plain text.
func stripANSI(s string) string {
	var b strings.Builder
	for i := 0; i < len(s); {
		if s[i] == 0x1b {
			// Skip until the terminating letter of the CSI sequence.
			for i < len(s) && !((s[i] >= 'a' && s[i] <= 'z') || (s[i] >= 'A' && s[i] <= 'Z')) {
				i++
			}
			if i < len(s) {
				i++ // skip the terminator
			}
			continue
		}
		b.WriteByte(s[i])
		i++
	}
	return b.String()
}

// Wiring: entering command mode opens the popup; typing narrows it; Enter
// runs the highlighted entry through the unchanged parseCommand flow.
func TestCommandModeAutocompleteWiring(t *testing.T) {
	m := newModel("ws", &config.Workspace{Trunk: "main"}, nil, ResWorktrees, config.Tui{})

	m.enterMode(modeCommand)
	if !m.ac.Visible() {
		t.Fatal("popup should be visible on entering command mode")
	}

	// Type "work" -> narrows to worktrees/workspaces.
	m.input.SetValue("work")
	m.ac.SetQuery("work")
	for _, mt := range m.ac.matches {
		if !strings.HasPrefix(mt.value, "work") {
			t.Errorf("unexpected match after query work: %q", mt.value)
		}
	}
	if len(m.ac.matches) != 2 {
		t.Fatalf("query work: got %d matches, want 2", len(m.ac.matches))
	}

	// Refine to a single match and run it via parseCommand using the
	// highlighted selection.
	m.ac.SetQuery("workt")
	sel, ok := m.ac.Selected()
	if !ok || sel != "worktrees" {
		t.Fatalf("selected: got %q ok=%v", sel, ok)
	}
	if cmd := parseCommand(sel); cmd.kind != cmdResource || cmd.resource != ResWorktrees {
		t.Fatalf("parseCommand(%q) = %+v", sel, cmd)
	}
}

// Driving real key presses through handleInputKey: typing filters the
// popup live, and Enter runs the highlighted entry.
func TestHandleInputKeyTypeAndEnter(t *testing.T) {
	m := newModel("ws", &config.Workspace{Trunk: "main"}, nil, ResWorktrees, config.Tui{})
	m.enterMode(modeCommand)

	for _, r := range "iss" {
		m.handleInputKey(runeKey(r))
	}
	if len(m.ac.matches) != 1 || m.ac.matches[0].value != "issues" {
		t.Fatalf("after typing iss: got %+v", m.ac.matches)
	}

	// Enter executes the highlighted entry through parseCommand.
	m.handleInputKey(tea.KeyPressMsg{Code: tea.KeyEnter})
	if m.resource != ResIssues {
		t.Errorf("Enter should switch to issues; got %v", m.resource)
	}
	if m.mode != modeNormal || m.ac.open {
		t.Errorf("command mode should close after Enter: mode=%v open=%v", m.mode, m.ac.open)
	}
}

// A bare `:` followed by Enter stays a no-op (it must not fire whatever
// candidate happens to sit first in the popup).
func TestHandleInputKeyBareColonEnterIsNoop(t *testing.T) {
	m := newModel("ws", &config.Workspace{Trunk: "main"}, nil, ResWorktrees, config.Tui{})
	m.enterMode(modeCommand)

	m.handleInputKey(tea.KeyPressMsg{Code: tea.KeyEnter})
	if m.resource != ResWorktrees {
		t.Errorf("bare colon + Enter must not switch resource; got %v", m.resource)
	}
	if m.status != "" {
		t.Errorf("bare colon + Enter must not set a status; got %q", m.status)
	}
	if m.mode != modeNormal {
		t.Errorf("should return to normal mode; got %v", m.mode)
	}
}

func TestCompleteFromPopupFillsInput(t *testing.T) {
	m := newModel("ws", &config.Workspace{Trunk: "main"}, nil, ResWorktrees, config.Tui{})
	m.enterMode(modeCommand)

	// Tab cycles the highlight and fills the input without re-filtering, so
	// the candidate list keeps its full size.
	before := len(m.ac.matches)
	m.completeFromPopup(1)
	sel, _ := m.ac.Selected()
	if m.input.Value() != sel {
		t.Errorf("input not filled: got %q, want %q", m.input.Value(), sel)
	}
	if len(m.ac.matches) != before {
		t.Errorf("completing should not refilter: got %d matches, want %d", len(m.ac.matches), before)
	}
}
