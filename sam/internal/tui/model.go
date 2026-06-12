package tui

import (
	"log/slog"
	"strings"

	"charm.land/bubbles/v2/spinner"
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"

	"github.com/nicomaioli-skinandme/dotfiles/sam/internal/config"
	"github.com/nicomaioli-skinandme/dotfiles/sam/internal/issue"
	"github.com/nicomaioli-skinandme/dotfiles/sam/internal/logx"
	"github.com/nicomaioli-skinandme/dotfiles/sam/internal/pr"
)

// inputMode is what the top bar is doing: nothing (keys drive the list),
// filtering the list (`/`), or entering a `:` command.
type inputMode int

const (
	modeNormal inputMode = iota
	modeSearch
	modeCommand
)

// paneFocus is which pane normal-mode keys drive on a faceted view: the main
// list (default) or the filter sidebar.
type paneFocus int

const (
	focusMain paneFocus = iota
	focusSidebar
)

// sidebarWidth is the fixed column width of the filter sidebar.
const sidebarWidth = 26

// model is the root bubbletea model. It is a pointer model: Update
// mutates in place and returns the same pointer.
type model struct {
	width, height int

	workspaceName string
	workspace     *config.Workspace
	all           map[string]config.Workspace
	deps          Deps

	resource   Resource
	branchPick bool // transient: the list is showing branches for `a` (new worktree)

	items    []Item // full set for the current view
	filtered []Item // items matching query (mirrors items when query is empty)
	cursor   int    // index into filtered
	query    string
	selected map[string]bool // multi-select set, keyed by Item.ID

	issues  map[string]issue.Issue // resolved issues by Item.ID (ResIssues)
	prs     map[string]pr.PR       // resolved PRs by Item.ID (ResPRs)
	columns []string               // project columns in board order (ResIssues), seeds the sidebar filter
	pending *fromIssueState        // in-flight from-issue flow, if any

	log        *slog.Logger          // diagnostic sink (never nil; discards when unset)
	ring       *logx.Ring            // in-memory log buffer the `:logs` view reads (may be nil)
	logPath    string                // temp file the logger tees to (shown in the logs empty state)
	logEntries map[string]logx.Entry // entries backing the current logs list, keyed by Item.ID
	logIcon    hitRegion             // where renderStatusBar last drew the ⚠ icon, for click hit-testing

	mode    inputMode
	input   textinput.Model
	ac      autocomplete // `:` command popup
	sidebar sidebar      // facet filter panel (issue columns, log levels)
	focus   paneFocus    // which pane normal-mode keys drive (faceted views only)
	styles  styles       // palette-derived render styles

	loading  bool
	deleting map[string]bool // worktree IDs with an in-flight delete
	spinner  spinner.Model
	status   string // transient message line (errors, "no issues", etc.)

	modal modalState
	form  *formState // add-workspace form when open (gates keys + body rendering)

	// focusID, when set, names the Item.ID the cursor should land on after
	// the next load (e.g. a just-added workspace).
	focusID string

	stack []snapshot // one entry per pushed sub-view (branch pick)
	err   error
}

// maxStackDepth caps the navigation history: <ESC>/h walks back at most
// this many hops before the stack runs dry.
const maxStackDepth = 5

// hitRegion is a clickable span on a single row, [x0, x1). The zero value
// matches nothing, so an un-drawn target (e.g. the ⚠ icon when there are no
// warnings) is never clicked by accident.
type hitRegion struct {
	row, x0, x1 int
}

func (h hitRegion) contains(x, y int) bool {
	return y == h.row && x >= h.x0 && x < h.x1
}

// snapshot captures a list view so <ESC> can restore it.
type snapshot struct {
	resource   Resource
	branchPick bool
	items      []Item
	cursor     int
	query      string
}

func newModel(workspaceName string, workspace *config.Workspace, all map[string]config.Workspace, start Resource, tuiCfg config.Tui, deps Deps) *model {
	ti := textinput.New()
	ti.SetVirtualCursor(true)

	sp := spinner.New()
	sp.Spinner = spinner.Dot

	st := newStyles(tuiCfg.Colors)

	// A nil logger means a non-menu caller (or a test) didn't wire one;
	// discard so the model can log unconditionally.
	logger := deps.Logger
	if logger == nil {
		logger = slog.New(slog.DiscardHandler)
	}

	return &model{
		workspaceName: workspaceName,
		workspace:     workspace,
		all:           all,
		deps:          deps,
		resource:      start,
		selected:      map[string]bool{},
		deleting:      map[string]bool{},
		input:         ti,
		spinner:       sp,
		ac:            newAutocomplete(tuiCfg.Autocomplete.Max, st),
		sidebar:       newSidebar(sidebarWidth, st),
		styles:        st,
		log:           logger,
		ring:          deps.LogRing,
		logPath:       deps.LogPath,
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

	case tea.MouseClickMsg:
		// The only mouse affordance: left-clicking the ⚠ icon jumps to `:logs`.
		// Ignored while an overlay (modal/form) owns input.
		if m.modal.kind == modalNone && m.form == nil &&
			msg.Button == tea.MouseLeft && m.logIcon.contains(msg.X, msg.Y) {
			return m, m.switchResource(ResLogs)
		}
		return m, nil

	case spinner.TickMsg:
		if m.loading || len(m.deleting) > 0 || (m.form != nil && m.form.busy != "") {
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

	case attachReadyMsg:
		return m.handleAttachReady(msg)

	case attachedMsg:
		return m.handleAttached(msg)

	case worktreeAddedMsg:
		return m.handleWorktreeAdded(msg)

	case fromIssuePreparedMsg:
		return m.handleFromIssuePrepared(msg)

	case fromIssueDoneMsg:
		return m.handleFromIssueDone(msg)

	case fromPRDoneMsg:
		return m.handleFromPRDone(msg)

	case formRepoProbedMsg:
		return m.handleFormRepoProbed(msg)

	case formProjectFetchedMsg:
		return m.handleFormProjectFetched(msg)

	case formScopesCheckedMsg:
		return m.handleFormScopesChecked(msg)

	case formSavedMsg:
		return m.handleFormSaved(msg)
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

	if m.form != nil {
		return m.handleFormKey(msg)
	}

	switch m.mode {
	case modeSearch, modeCommand:
		return m.handleInputKey(msg)
	}

	// Normal mode. On faceted views the sidebar may claim the key (pane
	// switching, or navigation/toggle while it's focused) before the main-list
	// bindings run.
	if m.hasSidebar() {
		if handled, model, cmd := m.handleSidebarKey(key); handled {
			return model, cmd
		}
	}

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
	case "A":
		return m.addNew()
	case "d":
		return m.del()
	case "tab":
		m.toggleSelect()
	case "?":
		m.openHelp()
	case "esc", "h", "left":
		return m.back()
	case "R":
		return m, m.reloadCurrent()
	case "x":
		m.clearFilter()
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
		m.ac.Open(commandCandidates())
	}
	m.input.CursorEnd()
	m.input.Focus()
}

// leaveInput returns to normal mode, blurs the top bar, and dismisses the
// command popup (a no-op when it was never open).
func (m *model) leaveInput() {
	m.mode = modeNormal
	m.input.Blur()
	m.ac.Close()
}

// back handles <ESC>/h: pop the navigation history one hop, then reload the
// restored view so state that changed while we were away (a closed session, a
// new worktree) is reflected. The popped snapshot shows immediately while the
// fresh load runs. A no-op when the stack is empty.
func (m *model) back() (tea.Model, tea.Cmd) {
	if len(m.stack) == 0 {
		return m, nil
	}
	m.popView()
	m.focusMainPane()
	return m, m.reloadCurrent()
}

// reloadCurrent re-fetches the current view's data in place. It dispatches to
// the branch-pick loader when that sub-view is active; otherwise the normal
// resource loader. The cursor/query are left as-is (applyLoaded no longer
// resets the cursor), so the highlight survives the refresh.
func (m *model) reloadCurrent() tea.Cmd {
	if m.branchPick {
		m.loading = true
		return tea.Batch(m.spinner.Tick, m.loadBranches())
	}
	return m.loadResource()
}

// clearFilter drops the active search query (a no-op when none is set).
func (m *model) clearFilter() {
	if m.query != "" {
		m.query = ""
		m.applyFilter()
	}
}

// pushView saves the current list onto the navigation history so a later
// <ESC>/h can restore it. Capped at maxStackDepth — the oldest entry is
// dropped once the cap is exceeded.
func (m *model) pushView() {
	m.stack = append(m.stack, snapshot{
		resource:   m.resource,
		branchPick: m.branchPick,
		items:      m.items,
		cursor:     m.cursor,
		query:      m.query,
	})
	if len(m.stack) > maxStackDepth {
		m.stack = m.stack[len(m.stack)-maxStackDepth:]
	}
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
// Refuses to leave ResWorkspaces while no workspace is active — every
// other resource needs one.
func (m *model) switchResource(r Resource) tea.Cmd {
	if m.workspace == nil && r != ResWorkspaces && r != ResLogs {
		m.status = "pick a workspace first"
		return nil
	}
	if r != m.resource {
		m.pushView() // record current view so back() can return to it
	}
	m.resource = r
	m.branchPick = false
	m.query = ""
	m.cursor = 0
	m.status = ""
	m.items = nil // avoid showing the previous resource's rows/count mid-load
	m.filtered = nil
	// Drop the previous resource's filter facets and return focus to the main
	// list; the load re-seeds the sidebar for the new resource.
	m.columns = nil
	m.sidebar.SetSections(nil)
	m.focusMainPane()
	return m.loadResource()
}

// applyFilter recomputes filtered from items: the `/` substring query AND the
// sidebar's active facets must both pass. Cursor is clamped to the result.
func (m *model) applyFilter() {
	needle := strings.ToLower(m.query)
	out := make([]Item, 0, len(m.items))
	for _, it := range m.items {
		if needle != "" && !strings.Contains(strings.ToLower(it.Title+" "+it.Detail), needle) {
			continue
		}
		if !m.facetPass(it) {
			continue
		}
		out = append(out, it)
	}
	m.filtered = out
	if m.cursor >= len(m.filtered) {
		m.cursor = len(m.filtered) - 1
	}
	if m.cursor < 0 {
		m.cursor = 0
	}
}

// facetPass reports whether an item survives the sidebar's active facet
// toggles. A view with no facet section seeded (worktrees, non-project issues,
// logs before load) passes everything; a section present but with everything
// toggled off passes nothing.
func (m *model) facetPass(it Item) bool {
	switch m.resource {
	case ResIssues:
		if !m.sidebar.hasSection("columns") {
			return true
		}
		return m.sidebar.Selected("columns")[m.issues[it.ID].Status]
	case ResLogs:
		if !m.sidebar.hasSection("level") {
			return true
		}
		return m.sidebar.Selected("level")[levelLabel(m.logEntries[it.ID].Level)]
	}
	return true
}

// hasSidebar reports whether the current view shows the filter sidebar: the
// logs view always, the issues view once a project produced columns.
func (m *model) hasSidebar() bool {
	switch m.resource {
	case ResLogs:
		return true
	case ResIssues:
		return m.sidebar.hasSection("columns")
	}
	return false
}

// focusMainPane returns keyboard focus to the main list and blurs the sidebar.
func (m *model) focusMainPane() {
	m.focus = focusMain
	m.sidebar.SetFocused(false)
}

// handleSidebarKey gives the filter sidebar first refusal on a normal-mode key
// on faceted views. h/l switch panes (h→sidebar, l→main); while the sidebar is
// focused, j/k navigate it and enter/space toggle the highlighted row. It
// returns handled=false for keys it doesn't claim (`:`, `/`, `esc`, `?`, …) so
// they fall through to the shared bindings — notably esc still backs out.
func (m *model) handleSidebarKey(key string) (bool, tea.Model, tea.Cmd) {
	switch key {
	case "h", "left":
		m.focus = focusSidebar
		m.sidebar.SetFocused(true)
		return true, m, nil
	case "l", "right":
		m.focusMainPane()
		return true, m, nil
	}
	if m.focus != focusSidebar {
		return false, m, nil
	}
	switch key {
	case "j", "down":
		m.sidebar.Move(1)
		return true, m, nil
	case "k", "up":
		m.sidebar.Move(-1)
		return true, m, nil
	case "enter", " ", "space":
		m.sidebar.Act()
		m.applyFilter()
		return true, m, nil
	}
	return false, m, nil
}

// syncSidebar rebuilds the filter sections for the current resource from the
// freshly loaded data and config, preserving prior toggles (SetSections
// merges). Called after a load lands.
func (m *model) syncSidebar() {
	switch m.resource {
	case ResIssues:
		if len(m.columns) == 0 {
			m.sidebar.SetSections(nil)
			return
		}
		def := m.defaultColumns()
		items := make([]sidebarItem, len(m.columns))
		for i, c := range m.columns {
			items[i] = sidebarItem{label: c, on: def[c]}
		}
		m.sidebar.SetSections([]sidebarSection{{key: "columns", title: "Columns", items: items}})
	case ResLogs:
		labels := []string{"ERROR", "WARN", "INFO", "DEBUG"}
		items := make([]sidebarItem, len(labels))
		for i, l := range labels {
			items[i] = sidebarItem{label: l, on: true}
		}
		m.sidebar.SetSections([]sidebarSection{{key: "level", title: "Level", items: items}})
	default:
		m.sidebar.SetSections(nil)
	}
}

// defaultColumns is the set of columns toggled on by default: the workspace's
// configured backlog statuses, or every column when none are configured.
func (m *model) defaultColumns() map[string]bool {
	def := map[string]bool{}
	var backlog []string
	if m.workspace != nil {
		backlog = m.workspace.GhProject.BacklogStatuses
	}
	if len(backlog) == 0 {
		for _, c := range m.columns {
			def[c] = true
		}
		return def
	}
	for _, s := range backlog {
		def[s] = true
	}
	return def
}
