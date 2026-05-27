package tui

import (
	"strings"

	"charm.land/bubbles/v2/spinner"
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"

	"github.com/nicomaioli-skinandme/dotfiles/sam/internal/config"
	"github.com/nicomaioli-skinandme/dotfiles/sam/internal/issueflow"
)

// inputMode is what the top bar is doing: nothing (keys drive the list),
// filtering the list (`/`), or entering a `:` command.
type inputMode int

const (
	modeNormal inputMode = iota
	modeSearch
	modeCommand
)

// model is the root bubbletea model. It is a pointer model: Update
// mutates in place and returns the same pointer.
type model struct {
	width, height int

	workspaceName string
	workspace     *config.Workspace
	all           map[string]config.Workspace

	resource   Resource
	branchPick bool // transient: the list is showing branches for `a` (new worktree)

	items    []Item // full set for the current view
	filtered []Item // items matching query (mirrors items when query is empty)
	cursor   int    // index into filtered
	query    string
	selected map[string]bool // multi-select set, keyed by Item.ID

	issues  map[string]issueflow.Issue // resolved issues by Item.ID (ResIssues)
	pending *fromIssueState            // in-flight from-issue flow, if any

	mode  inputMode
	input textinput.Model

	loading bool
	spinner spinner.Model
	status  string // transient message line (errors, "no issues", etc.)

	modal modalState

	stack  []snapshot // one entry per pushed sub-view (branch pick)
	result Result
	err    error
}

// snapshot captures a list view so <ESC> can restore it.
type snapshot struct {
	resource   Resource
	branchPick bool
	items      []Item
	cursor     int
	query      string
}

func newModel(workspaceName string, workspace *config.Workspace, all map[string]config.Workspace, start Resource) *model {
	ti := textinput.New()
	ti.SetVirtualCursor(true)

	sp := spinner.New()
	sp.Spinner = spinner.Dot

	return &model{
		workspaceName: workspaceName,
		workspace:     workspace,
		all:           all,
		resource:      start,
		selected:      map[string]bool{},
		input:         ti,
		spinner:       sp,
	}
}

func (m *model) Init() tea.Cmd {
	return m.loadResource()
}

func (m *model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		return m, nil

	case tea.KeyPressMsg:
		return m.handleKey(msg)

	case spinner.TickMsg:
		if m.loading {
			var cmd tea.Cmd
			m.spinner, cmd = m.spinner.Update(msg)
			return m, cmd
		}
		return m, nil

	case itemsLoadedMsg:
		m.applyLoaded(msg)
		return m, nil

	case actionDoneMsg:
		return m.handleActionDone(msg)

	case fromIssuePreparedMsg:
		return m.handleFromIssuePrepared(msg)

	case fromIssueDoneMsg:
		return m.handleFromIssueDone(msg)
	}
	return m, nil
}

// handleKey routes a key press based on the current mode/modal. Modal
// input takes precedence, then the `/` and `:` input modes, then normal
// navigation.
func (m *model) handleKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	key := msg.String()

	// Ctrl-C always quits with no result.
	if key == "ctrl+c" {
		return m, tea.Quit
	}

	if m.modal.kind != modalNone {
		return m.handleModalKey(msg)
	}

	switch m.mode {
	case modeSearch, modeCommand:
		return m.handleInputKey(msg)
	}

	// Normal mode.
	switch key {
	case "j", "down":
		m.moveCursor(1)
	case "k", "up":
		m.moveCursor(-1)
	case "g":
		m.cursor = 0
	case "G":
		if len(m.filtered) > 0 {
			m.cursor = len(m.filtered) - 1
		}
	case "/":
		m.enterMode(modeSearch)
	case ":":
		m.enterMode(modeCommand)
	case "enter", "l", "right":
		return m.activate()
	case "a":
		return m.add()
	case "d":
		return m.del()
	case "tab":
		m.toggleSelect()
	case "?":
		m.openHelp()
	case "esc", "h", "left":
		return m.back()
	case "q":
		return m, tea.Quit
	}
	return m, nil
}

// moveCursor moves the highlight by delta, clamped to the list bounds.
func (m *model) moveCursor(delta int) {
	if len(m.filtered) == 0 {
		m.cursor = 0
		return
	}
	m.cursor += delta
	if m.cursor < 0 {
		m.cursor = 0
	}
	if m.cursor >= len(m.filtered) {
		m.cursor = len(m.filtered) - 1
	}
}

// current returns the highlighted item, or false when the list is empty.
func (m *model) current() (Item, bool) {
	if m.cursor < 0 || m.cursor >= len(m.filtered) {
		return Item{}, false
	}
	return m.filtered[m.cursor], true
}

// toggleSelect flips the multi-select state of the highlighted row.
func (m *model) toggleSelect() {
	if it, ok := m.current(); ok {
		if m.selected[it.ID] {
			delete(m.selected, it.ID)
		} else {
			m.selected[it.ID] = true
		}
	}
}

// enterMode focuses the top bar for search or command entry.
func (m *model) enterMode(mode inputMode) {
	m.mode = mode
	m.status = ""
	m.input.Reset()
	switch mode {
	case modeSearch:
		m.input.Prompt = "/"
		m.input.SetValue(m.query)
	case modeCommand:
		m.input.Prompt = ":"
	}
	m.input.CursorEnd()
	m.input.Focus()
}

// leaveInput returns to normal mode and blurs the top bar.
func (m *model) leaveInput() {
	m.mode = modeNormal
	m.input.Blur()
}

// back handles <ESC>/h: dismiss help, pop a pushed view, or clear the
// active search filter, in that order.
func (m *model) back() (tea.Model, tea.Cmd) {
	if len(m.stack) > 0 {
		m.popView()
		return m, nil
	}
	if m.query != "" {
		m.query = ""
		m.applyFilter()
	}
	return m, nil
}

// pushView saves the current list so a sub-view (branch pick) can be
// shown and later restored.
func (m *model) pushView() {
	m.stack = append(m.stack, snapshot{
		resource:   m.resource,
		branchPick: m.branchPick,
		items:      m.items,
		cursor:     m.cursor,
		query:      m.query,
	})
}

func (m *model) popView() {
	if len(m.stack) == 0 {
		return
	}
	s := m.stack[len(m.stack)-1]
	m.stack = m.stack[:len(m.stack)-1]
	m.resource = s.resource
	m.branchPick = s.branchPick
	m.items = s.items
	m.cursor = s.cursor
	m.query = s.query
	m.applyFilter()
}

// switchResource changes the active resource and reloads its items.
func (m *model) switchResource(r Resource) tea.Cmd {
	m.resource = r
	m.branchPick = false
	m.stack = nil
	m.query = ""
	m.cursor = 0
	m.status = ""
	m.items = nil // avoid showing the previous resource's rows/count mid-load
	m.filtered = nil
	return m.loadResource()
}

// applyFilter recomputes filtered from items and the current query, and
// clamps the cursor.
func (m *model) applyFilter() {
	if m.query == "" {
		m.filtered = m.items
	} else {
		needle := strings.ToLower(m.query)
		out := make([]Item, 0, len(m.items))
		for _, it := range m.items {
			if strings.Contains(strings.ToLower(it.Title+" "+it.Detail), needle) {
				out = append(out, it)
			}
		}
		m.filtered = out
	}
	if m.cursor >= len(m.filtered) {
		m.cursor = len(m.filtered) - 1
	}
	if m.cursor < 0 {
		m.cursor = 0
	}
}
