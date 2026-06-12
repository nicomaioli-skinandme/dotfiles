package tui

import (
	"fmt"
	"log/slog"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

// Fixed chrome around the list: top bar, divider, status bar.
const chromeHeight = 3

func (m *model) View() tea.View {
	if m.width == 0 || m.height == 0 {
		return tea.NewView("")
	}

	base := lipgloss.JoinVertical(
		lipgloss.Left,
		m.renderTopBar(),
		m.styles.divider.Render(strings.Repeat("─", m.width)),
		m.renderBody(),
		m.renderStatusBar(),
	)

	if m.modal.kind != modalNone {
		v := tea.NewView(m.overlay(base, m.renderModal()))
		v.AltScreen = true
		v.MouseMode = tea.MouseModeCellMotion
		return v
	}

	// The autocomplete popup floats under the focused input. It never
	// coexists with a modal (no modal is open during `:`), so the modal
	// branch above takes precedence.
	if m.ac.Visible() {
		popup := m.ac.View(m.width)
		anchor := anchorPos{row: 0, col: lipgloss.Width(m.input.Prompt)}
		x, y := m.ac.Position(anchor, lipgloss.Width(popup), lipgloss.Height(popup), m.width, m.height)
		v := tea.NewView(m.overlayAt(base, popup, x, y, 1))
		v.AltScreen = true
		v.MouseMode = tea.MouseModeCellMotion
		return v
	}

	v := tea.NewView(base)
	v.AltScreen = true
	// Cell-motion mouse reporting drives the clickable ⚠ icon. The only mouse
	// affordance is that click; this trades the terminal's native drag-select
	// (now behind the terminal's modifier key) for it.
	v.MouseMode = tea.MouseModeCellMotion
	return v
}

// renderTopBar shows the input bar. In normal mode it's a faint hint; in
// search/command mode it's the focused text input.
func (m *model) renderTopBar() string {
	if m.mode == modeNormal {
		return m.styles.hint.Render("  press / to search · : for commands · ? for help")
	}
	return m.input.View()
}

func (m *model) renderBody() string {
	h := m.height - chromeHeight
	if h < 1 {
		h = 1
	}

	if m.form != nil {
		return m.renderForm(h)
	}

	if m.loading {
		what := m.resource.Name()
		if m.branchPick {
			what = "branches"
		}
		return pad(fmt.Sprintf("  %s loading %s…", m.spinner.View(), what), m.width, h)
	}

	// Faceted views show the filter sidebar beside the list.
	if m.hasSidebar() {
		return m.renderWithSidebar(h)
	}

	// Empty main list: show a centered empty-state hint.
	if len(m.filtered) == 0 {
		return m.renderEmpty(h, m.width)
	}

	return m.renderList(h, m.width)
}

// renderWithSidebar lays the filter sidebar (fixed width) to the left of the
// main list, separated by a faint vertical rule. The list (or its empty state)
// fills the remaining width.
func (m *model) renderWithSidebar(h int) string {
	leftBlock := lipgloss.NewStyle().Width(m.sidebar.Width()).Render(m.sidebar.View(h))

	sepLines := make([]string, h)
	for i := range sepLines {
		sepLines[i] = "│"
	}
	sep := m.styles.divider.Render(strings.Join(sepLines, "\n"))

	rightW := m.width - m.sidebar.Width() - 3 // sidebar + " │ "
	if rightW < 1 {
		rightW = 1
	}
	var right string
	if len(m.filtered) == 0 {
		right = m.renderEmpty(h, rightW)
	} else {
		right = m.renderList(h, rightW)
	}
	return lipgloss.JoinHorizontal(lipgloss.Top, leftBlock, " ", sep, " ", right)
}

func (m *model) renderList(h, width int) string {
	start := 0
	if len(m.filtered) > h {
		start = m.cursor - h/2
		if start < 0 {
			start = 0
		}
		if start > len(m.filtered)-h {
			start = len(m.filtered) - h
		}
	}
	end := start + h
	if end > len(m.filtered) {
		end = len(m.filtered)
	}

	// On faceted views the main list's cursor is "active" (highlighted) only
	// while the main pane holds focus; with the sidebar focused it stays
	// visible but muted so the two panes don't both look selected.
	active := !m.hasSidebar() || m.focus == focusMain

	lines := make([]string, 0, h)
	for i := start; i < end; i++ {
		lines = append(lines, m.renderRow(m.filtered[i], i == m.cursor, active, width))
	}
	for len(lines) < h {
		lines = append(lines, "")
	}
	return strings.Join(lines, "\n")
}

func (m *model) renderRow(it Item, isCursor, active bool, width int) string {
	if m.resource == ResLogs {
		return m.renderLogRow(it, isCursor, active, width)
	}

	cursor := "  "
	if isCursor {
		if active {
			cursor = m.styles.cursor.Render("▸ ")
		} else {
			cursor = m.styles.hint.Render("▸ ")
		}
	}

	sel := " "
	if m.selected[it.ID] {
		sel = m.styles.selected.Render("✓")
	}

	bullet := "  "
	if it.Active {
		bullet = m.styles.active.Render("● ")
	}

	title := it.Title
	if isCursor && active {
		title = m.styles.cursor.Render(title)
	} else {
		title = m.styles.row.Render(title)
	}

	line := fmt.Sprintf("%s%s %s%s", cursor, sel, bullet, title)
	switch {
	case m.deleting[it.ID]:
		line += "  " + m.spinner.View() + " " + m.styles.deleting.Render("deleting…")
	case it.Detail != "":
		line += "  " + m.styles.detail.Render("("+it.Detail+")")
	}
	return truncate(line, width)
}

// renderLogRow draws a `:logs` row: a faint timestamp, a severity-coloured
// level, and the message. The full detail is shown in the detail modal on
// activate. The entry is looked up by Item.ID (filtering reorders rows, so
// the slice index is unreliable).
func (m *model) renderLogRow(it Item, isCursor, active bool, width int) string {
	cursor := "  "
	if isCursor {
		if active {
			cursor = m.styles.cursor.Render("▸ ")
		} else {
			cursor = m.styles.hint.Render("▸ ")
		}
	}

	e := m.logEntries[it.ID]
	ts := m.styles.detail.Render(e.Time.Format("15:04:05"))
	level := m.logLevelStyle(e.Level).Render(fmt.Sprintf("%-5s", e.Level.String()))

	msg := it.Title
	if isCursor && active {
		msg = m.styles.cursor.Render(msg)
	} else {
		msg = m.styles.row.Render(msg)
	}

	return truncate(fmt.Sprintf("%s%s %s  %s", cursor, ts, level, msg), width)
}

// levelLabel buckets a log level into one of the four filterable names,
// mirroring logLevelStyle's thresholds, so a non-standard level (e.g.
// ERROR+2) still maps to a sidebar toggle.
func levelLabel(l slog.Level) string {
	switch {
	case l >= slog.LevelError:
		return "ERROR"
	case l >= slog.LevelWarn:
		return "WARN"
	case l >= slog.LevelInfo:
		return "INFO"
	default:
		return "DEBUG"
	}
}

// logLevelStyle maps a log level to a palette style: ERROR→destroy,
// WARN→primary, INFO→body, DEBUG→faint.
func (m *model) logLevelStyle(l slog.Level) lipgloss.Style {
	switch {
	case l >= slog.LevelError:
		return m.styles.deleting
	case l >= slog.LevelWarn:
		return m.styles.active
	case l >= slog.LevelInfo:
		return m.styles.row
	default:
		return m.styles.hint
	}
}

// renderEmpty centers an empty-state hint when the current list has no
// rows: a short message plus a navigation hint (or, for logs, the path
// the log is being written to).
func (m *model) renderEmpty(h, width int) string {
	msg := "no items"
	hint := "press : to switch resource"
	switch {
	case m.branchPick:
		msg = "no branches available"
	case m.resource == ResLogs:
		msg = "no log entries match"
		if m.logPath != "" {
			hint = "writing to " + m.logPath
		}
	case m.resource == ResIssues && m.hasSidebar():
		msg = "no issues in the selected columns"
		hint = "h to focus filters · space to toggle"
	}
	body := m.styles.hint.Render(msg + "\n\n" + hint)
	return pad(lipgloss.Place(width, h, lipgloss.Center, lipgloss.Center, body), width, h)
}

func (m *model) renderStatusBar() string {
	scope := m.resource.Name()
	if m.branchPick {
		scope = "new worktree · pick branch"
	}
	if m.form != nil {
		scope = "workspaces · add"
	}
	var crumb string
	if m.workspaceName == "" {
		crumb = fmt.Sprintf(" %s", scope)
	} else {
		crumb = fmt.Sprintf(" %s › %s", m.workspaceName, scope)
	}
	left := m.styles.breadcrumb.Render(crumb)

	// A persistent ⚠ whenever the log holds any warning or error, so failures
	// stay noticeable after the error modal is dismissed. No count (it read as
	// an editor diagnostics tally and reset confusingly); just presence.
	// Destroy palette when any are errors, primary for warnings only. The icon
	// is clickable — see logIcon below and the MouseClickMsg handler.
	badge := ""
	if m.ring.CountSince(slog.LevelWarn, 0) > 0 {
		style := m.styles.active
		if m.ring.CountSince(slog.LevelError, 0) > 0 {
			style = m.styles.deleting
		}
		badge = style.Render("⚠") + "   "
	}

	count := fmt.Sprintf("%d items", len(m.filtered))
	right := badge + m.styles.hint.Render(count+"   ? help ")

	// Record the icon's clickable bounds for the mouse handler. The status bar
	// is the last row; the icon leads the right-aligned cluster. Cleared (zero
	// value, matches nothing) when no icon is drawn.
	m.logIcon = hitRegion{}
	if badge != "" {
		x0 := m.width - lipgloss.Width(right)
		m.logIcon = hitRegion{row: m.height - 1, x0: x0, x1: x0 + lipgloss.Width(badge)}
	}

	// The status bar must stay exactly one row: renderBody reserves only
	// chromeHeight rows, so a multiline status (e.g. a multiline gh error)
	// would make the frame taller than the screen and corrupt the alt-screen
	// render. Flatten to one line and truncate to the space available between
	// the breadcrumb and the counter.
	mid := ""
	if m.status != "" {
		avail := m.width - lipgloss.Width(left) - lipgloss.Width(right) - 3 // 2-space pad + 1-space min gap
		if avail > 0 {
			mid = "  " + m.styles.statusInfo.Render(truncate(oneLine(m.status), avail))
		}
	}

	gap := m.width - lipgloss.Width(left) - lipgloss.Width(right) - lipgloss.Width(mid)
	if gap < 1 {
		gap = 1
	}
	return left + mid + strings.Repeat(" ", gap) + right
}

// oneLine collapses any run of whitespace (including newlines) to a single
// space so a status never spills past the one-row status bar.
func oneLine(s string) string {
	return strings.Join(strings.Fields(s), " ")
}

func (m *model) renderModal() string {
	switch m.modal.kind {
	case modalHelp:
		return m.styles.modalBorder.Render(m.helpText())
	case modalDetail:
		body := lipgloss.JoinVertical(
			lipgloss.Left,
			m.modal.title,
			"",
			m.modal.viewport.View(),
			"",
			m.styles.hint.Render("↑/↓ scroll · esc close"),
		)
		return m.styles.modalBorder.Render(body)
	case modalInput:
		body := lipgloss.JoinVertical(
			lipgloss.Left,
			m.modal.title,
			"",
			m.modal.input.View(),
			"",
			m.styles.hint.Render("enter confirm · esc cancel"),
		)
		return m.styles.modalBorder.Render(body)
	case modalError:
		// The headline is a short fixed message (never the raw error, which is
		// multi-line); the full error lives in `:logs`. Rendered in the destroy
		// palette so it reads as a failure. View logs is the highlighted default.
		dismiss := m.styles.modalAffirm.Render("Dismiss")
		viewLogs := m.styles.modalAffirm.Render("View logs")
		if m.modal.confirmYes {
			viewLogs = m.styles.modalActive.Render("View logs")
		} else {
			dismiss = m.styles.modalActive.Render("Dismiss")
		}
		buttons := lipgloss.JoinHorizontal(lipgloss.Top, dismiss, "   ", viewLogs)
		body := lipgloss.JoinVertical(
			lipgloss.Center,
			m.styles.deleting.Render(m.modal.title),
			m.styles.hint.Render("see :logs for the full error"),
			"",
			buttons,
		)
		return m.styles.modalBorder.Render(body)
	}
	// Confirm modal. The highlighted button uses the destroy palette for a
	// destructive action (e.g. deleting a worktree), otherwise the neutral
	// active style.
	active := m.styles.modalActive
	if m.modal.destructive {
		active = m.styles.modalDestroy
	}
	no := m.styles.modalAffirm.Render("No")
	yes := m.styles.modalAffirm.Render("Yes")
	if m.modal.confirmYes {
		yes = active.Render("Yes")
	} else {
		no = active.Render("No")
	}
	buttons := lipgloss.JoinHorizontal(lipgloss.Top, no, "   ", yes)
	body := lipgloss.JoinVertical(lipgloss.Center, m.modal.title, "", buttons)
	return m.styles.modalBorder.Render(body)
}

// sidebarHelp is the extra help block shown on faceted views (issues, logs),
// where h/l switch panes and the sidebar filters the list.
var sidebarHelp = []string{
	"",
	"  Filter sidebar",
	"  h / l     focus sidebar / main list",
	"  j / k     move within the focused pane",
	"  space / ⏎ toggle column-level / collapse section",
}

// helpText lists the bindings available in the current context.
func (m *model) helpText() string {
	lines := []string{
		"Shortcuts",
		"",
		"  j / ↓     down",
		"  k / ↑     up",
		"  enter / l activate",
		"  / search   : command",
		"  tab       multi-select",
		"  esc / h   back",
		"  R         reload",
		"  x         clear filter",
		"  ? / esc   dismiss this help",
		"  : q       quit",
	}
	switch {
	case m.branchPick:
		lines = append(lines, "", "  enter     create worktree from branch")
	case m.resource == ResWorktrees:
		lines = append(lines, "",
			"  a         worktree from branch",
			"  A         new branch + worktree",
			"  d         delete worktree")
	case m.resource == ResWorkspaces:
		lines = append(lines, "", "  enter     switch workspace", "  a         add workspace")
	case m.resource == ResPRs:
		lines = append(lines, "", "  enter     create worktree from PR")
	case m.resource == ResIssues:
		lines = append(lines, "", "  enter     develop issue (main list)")
		lines = append(lines, sidebarHelp...)
	case m.resource == ResLogs:
		lines = append(lines, "", "  enter     view full entry (main list)")
		lines = append(lines, sidebarHelp...)
	}
	return strings.Join(lines, "\n")
}

// overlay composites a centered modal over the base using a lipgloss
// canvas.
func (m *model) overlay(base, modal string) string {
	mw, mh := lipgloss.Width(modal), lipgloss.Height(modal)
	x := (m.width - mw) / 2
	y := (m.height - mh) / 2
	if x < 0 {
		x = 0
	}
	if y < 0 {
		y = 0
	}
	return m.overlayAt(base, modal, x, y, 1)
}

// overlayAt composites layer over base at (x, y) with the given z-index.
// It is the single compositing path shared by the centered modal (overlay)
// and the anchored autocomplete popup.
//
// Positioning must go through a Compositor: a Layer drawn directly onto a
// Canvas ignores its own X/Y and fills the whole canvas area, so composing
// base and layer as separate Canvas.Compose calls would place the layer at
// the origin and clobber the base. The Compositor flattens the layers to
// their absolute bounds and draws each in z order.
func (m *model) overlayAt(base, layer string, x, y, z int) string {
	canvas := lipgloss.NewCanvas(m.width, m.height)
	canvas.Compose(lipgloss.NewCompositor(
		lipgloss.NewLayer(base),
		lipgloss.NewLayer(layer).X(x).Y(y).Z(z),
	))
	return canvas.Render()
}

// pad ensures s occupies exactly h lines of width w (best-effort).
func pad(s string, w, h int) string {
	lines := strings.Split(s, "\n")
	for len(lines) < h {
		lines = append(lines, "")
	}
	if len(lines) > h {
		lines = lines[:h]
	}
	return strings.Join(lines, "\n")
}

// truncate clamps a (possibly styled) line to w display columns.
func truncate(s string, w int) string {
	if lipgloss.Width(s) <= w {
		return s
	}
	return lipgloss.NewStyle().MaxWidth(w).Render(s)
}
