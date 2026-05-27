package tui

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

// Fixed chrome around the list: top bar, divider, status bar.
const chromeHeight = 3

var (
	dividerStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	hintStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("244"))
	cursorStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("213")).Bold(true)
	selectedStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("213"))
	rowStyle      = lipgloss.NewStyle()
	activeStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("42"))
	detailStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("244"))
	breadcrumb    = lipgloss.NewStyle().Foreground(lipgloss.Color("39")).Bold(true)
	statusInfo    = lipgloss.NewStyle().Foreground(lipgloss.Color("214"))
	sidebarTitle  = lipgloss.NewStyle().Foreground(lipgloss.Color("244")).Bold(true)
	sidebarActive = lipgloss.NewStyle().Foreground(lipgloss.Color("39")).Bold(true)
	modalBorder   = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("213")).
			Padding(1, 3)
	modalAffirm = lipgloss.NewStyle().Padding(0, 2)
	modalActive = lipgloss.NewStyle().Padding(0, 2).Reverse(true).Bold(true)
)

func (m *model) View() tea.View {
	if m.width == 0 || m.height == 0 {
		return tea.NewView("")
	}

	base := lipgloss.JoinVertical(
		lipgloss.Left,
		m.renderTopBar(),
		dividerStyle.Render(strings.Repeat("─", m.width)),
		m.renderBody(),
		m.renderStatusBar(),
	)

	if m.modal.kind == modalNone {
		v := tea.NewView(base)
		v.AltScreen = true
		return v
	}

	v := tea.NewView(m.overlay(base, m.renderModal()))
	v.AltScreen = true
	return v
}

// renderTopBar shows the input bar. In normal mode it's a faint hint; in
// search/command mode it's the focused text input.
func (m *model) renderTopBar() string {
	if m.mode == modeNormal {
		return hintStyle.Render("  press / to search · : for commands · ? for help")
	}
	return m.input.View()
}

func (m *model) renderBody() string {
	h := m.height - chromeHeight
	if h < 1 {
		h = 1
	}

	if m.loading {
		return pad(fmt.Sprintf("  %s loading %s…", m.spinner.View(), m.resource.Name()), m.width, h)
	}

	// Empty main list: fall back to a sidebar of resources plus a hint.
	if len(m.filtered) == 0 {
		return m.renderEmpty(h)
	}

	return m.renderList(h)
}

func (m *model) renderList(h int) string {
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

	lines := make([]string, 0, h)
	for i := start; i < end; i++ {
		lines = append(lines, m.renderRow(m.filtered[i], i == m.cursor))
	}
	for len(lines) < h {
		lines = append(lines, "")
	}
	return strings.Join(lines, "\n")
}

func (m *model) renderRow(it Item, isCursor bool) string {
	cursor := "  "
	if isCursor {
		cursor = cursorStyle.Render("▸ ")
	}

	sel := " "
	if m.selected[it.ID] {
		sel = selectedStyle.Render("✓")
	}

	bullet := "  "
	if it.Active {
		bullet = activeStyle.Render("● ")
	}

	title := it.Title
	if isCursor {
		title = cursorStyle.Render(title)
	} else {
		title = rowStyle.Render(title)
	}

	line := fmt.Sprintf("%s%s %s%s", cursor, sel, bullet, title)
	if it.Detail != "" {
		line += "  " + detailStyle.Render("("+it.Detail+")")
	}
	return truncate(line, m.width)
}

// renderEmpty is the sidebar fallback: a resource switcher on the left,
// an empty-state message on the right.
func (m *model) renderEmpty(h int) string {
	rows := []string{sidebarTitle.Render("RESOURCES"), ""}
	for _, r := range resources {
		name := "  " + r.Name()
		if r == m.resource && !m.branchPick {
			name = sidebarActive.Render("▸ " + r.Name())
		}
		rows = append(rows, name)
	}
	sidebar := lipgloss.NewStyle().Width(16).Render(strings.Join(rows, "\n"))

	msg := "no items"
	if m.branchPick {
		msg = "no branches available"
	}
	body := hintStyle.Render(msg + "\n\npress : to switch resource")
	main := lipgloss.Place(m.width-16, h, lipgloss.Center, lipgloss.Center, body)

	return pad(lipgloss.JoinHorizontal(lipgloss.Top, sidebar, main), m.width, h)
}

func (m *model) renderStatusBar() string {
	scope := m.resource.Name()
	if m.branchPick {
		scope = "new worktree · pick branch"
	}
	left := breadcrumb.Render(fmt.Sprintf(" %s › %s", m.workspaceName, scope))

	count := fmt.Sprintf("%d items", len(m.filtered))
	right := hintStyle.Render(count + "   ? help ")

	mid := ""
	if m.status != "" {
		mid = "  " + statusInfo.Render(m.status)
	}

	gap := m.width - lipgloss.Width(left) - lipgloss.Width(right) - lipgloss.Width(mid)
	if gap < 1 {
		gap = 1
	}
	return left + mid + strings.Repeat(" ", gap) + right
}

func (m *model) renderModal() string {
	switch m.modal.kind {
	case modalHelp:
		return modalBorder.Render(m.helpText())
	case modalInput:
		body := lipgloss.JoinVertical(
			lipgloss.Left,
			m.modal.title,
			"",
			m.modal.input.View(),
			"",
			hintStyle.Render("enter confirm · esc cancel"),
		)
		return modalBorder.Render(body)
	}
	// Confirm modal.
	no := modalAffirm.Render("No")
	yes := modalAffirm.Render("Yes")
	if m.modal.confirmYes {
		yes = modalActive.Render("Yes")
	} else {
		no = modalActive.Render("No")
	}
	buttons := lipgloss.JoinHorizontal(lipgloss.Top, no, "   ", yes)
	body := lipgloss.JoinVertical(lipgloss.Center, m.modal.title, "", buttons)
	return modalBorder.Render(body)
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
		"  ? / esc   dismiss this help",
		"  : q       quit",
	}
	switch {
	case m.branchPick:
		lines = append(lines, "", "  enter     create worktree from branch")
	case m.resource == ResWorktrees:
		lines = append(lines, "", "  a         new worktree", "  d         delete worktree")
	case m.resource == ResWorkspaces:
		lines = append(lines, "", "  enter     switch workspace", "  a         add workspace")
	}
	return strings.Join(lines, "\n")
}

// overlay composites a centered modal over the dimmed base using a
// lipgloss canvas.
func (m *model) overlay(base, modal string) string {
	canvas := lipgloss.NewCanvas(m.width, m.height)
	canvas.Compose(lipgloss.NewLayer(base))
	mw, mh := lipgloss.Width(modal), lipgloss.Height(modal)
	x := (m.width - mw) / 2
	y := (m.height - mh) / 2
	if x < 0 {
		x = 0
	}
	if y < 0 {
		y = 0
	}
	canvas.Compose(lipgloss.NewLayer(modal).X(x).Y(y).Z(1))
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
