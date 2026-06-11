package tui

import (
	"fmt"
	"sort"
	"strconv"

	tea "charm.land/bubbletea/v2"

	"github.com/nicomaioli-skinandme/dotfiles/sam/internal/issue"
	"github.com/nicomaioli-skinandme/dotfiles/sam/internal/logx"
	"github.com/nicomaioli-skinandme/dotfiles/sam/internal/pr"
	"github.com/nicomaioli-skinandme/dotfiles/sam/internal/worktree"
)

// itemsLoadedMsg carries the result of loading a resource's rows. It
// records which view it was loading for so a stale load (the user
// switched resources before it returned) can be ignored.
type itemsLoadedMsg struct {
	resource   Resource
	branchPick bool
	items      []Item
	issues     map[string]issue.Issue // resolved issues, keyed by Item.ID (ResIssues only)
	prs        map[string]pr.PR       // resolved PRs, keyed by Item.ID (ResPRs only)
	logs       map[string]logx.Entry  // log entries, keyed by Item.ID (ResLogs only)
	status     string                 // non-fatal note shown in the status line (e.g. "no issues")
	err        error
}

// loadResource returns a command that loads the current resource. Local
// resources (worktrees, workspaces) resolve immediately; remote ones
// (issues, clankers) run off the UI goroutine behind a spinner.
func (m *model) loadResource() tea.Cmd {
	switch m.resource {
	case ResWorktrees:
		return m.loadWorktrees()
	case ResWorkspaces:
		return m.loadWorkspaces()
	case ResIssues:
		m.loading = true
		return tea.Batch(m.spinner.Tick, m.loadIssues())
	case ResPRs:
		m.loading = true
		return tea.Batch(m.spinner.Tick, m.loadPRs())
	case ResClankers:
		m.loading = true
		return tea.Batch(m.spinner.Tick, m.loadClankers())
	case ResLogs:
		return m.loadLogs()
	}
	return nil
}

// applyLoaded installs a load result, ignoring stale loads.
func (m *model) applyLoaded(msg itemsLoadedMsg) {
	if msg.resource != m.resource || msg.branchPick != m.branchPick {
		return
	}
	m.loading = false
	m.status = msg.status
	if msg.err != nil {
		// Keep the status line generic — raw gh/git output is multiline and
		// would overflow the one-row bar — but log the full error so it's
		// visible in the `:logs` view. The TUI stays usable (switch resource,
		// quit) rather than aborting.
		label := "load " + m.resource.Name()
		if m.branchPick {
			label = "load branches"
		}
		m.log.Error(label, "err", msg.err)
		if m.resource == ResIssues || m.resource == ResPRs {
			m.status = "gh errored"
		} else {
			m.status = "couldn't load " + m.resource.Name()
		}
		m.items = nil
	} else {
		m.items = msg.items
		// Skip the breadcrumb for the logs view itself, so opening `:logs`
		// doesn't append a self-referential entry.
		if m.resource != ResLogs {
			m.log.Debug("loaded", "resource", m.resource.Name(), "n", len(msg.items))
		}
	}
	m.issues = msg.issues
	m.prs = msg.prs
	m.logEntries = msg.logs
	// Opening the logs view marks everything currently in the ring as seen,
	// clearing the unseen-entry badge.
	if m.resource == ResLogs {
		m.logsSeenSeq = m.ring.MaxSeq()
	}
	// Leave the cursor where it is — a reload-in-place (back nav, R, attach,
	// delete) keeps the highlight on its row, clamped to the new list by
	// applyFilter. Fresh views (switchResource, add) pre-zero the cursor
	// themselves, so this doesn't pin them to the top mid-load.
	m.applyFilter()
	// A pending focus request (e.g. a just-added workspace) wins over the
	// kept position.
	if m.focusID != "" {
		for i, it := range m.filtered {
			if it.ID == m.focusID {
				m.cursor = i
				break
			}
		}
		m.focusID = ""
	}
}

func (m *model) loadWorktrees() tea.Cmd {
	ws := m.workspace
	wsName := m.workspaceName
	ctrl := m.deps.Worktrees
	return func() tea.Msg {
		wts, err := ctrl.List(ws, wsName)
		if err != nil {
			return itemsLoadedMsg{resource: ResWorktrees, err: err}
		}
		items := make([]Item, 0, len(wts))
		for _, w := range wts {
			it := Item{ID: w.Name, Title: w.Name, Active: w.SessionActive, Type: worktreeType(w.Type)}
			if w.Type == worktree.Main {
				it.Detail = "main worktree"
			}
			items = append(items, it)
		}
		return itemsLoadedMsg{resource: ResWorktrees, items: items}
	}
}

func (m *model) loadWorkspaces() tea.Cmd {
	all := m.all
	currentName := m.workspaceName
	return func() tea.Msg {
		names := make([]string, 0, len(all))
		for name := range all {
			names = append(names, name)
		}
		sort.Strings(names)
		items := make([]Item, 0, len(names))
		for _, name := range names {
			ws := all[name]
			items = append(items, Item{
				ID:     name,
				Title:  name,
				Detail: ws.Repo,
				Active: name == currentName, // bullet marks the active workspace
			})
		}
		return itemsLoadedMsg{resource: ResWorkspaces, items: items}
	}
}

func (m *model) loadIssues() tea.Cmd {
	ws := m.workspace
	ctrl := m.deps.Issues
	return func() tea.Msg {
		issues, err := ctrl.List(ws)
		if err != nil {
			return itemsLoadedMsg{resource: ResIssues, err: err}
		}
		items := make([]Item, 0, len(issues))
		byID := make(map[string]issue.Issue, len(issues))
		for _, it := range issues {
			id := fmt.Sprintf("%s#%d", it.Repository, it.Number)
			items = append(items, Item{
				ID:     id,
				Title:  fmt.Sprintf("#%d  %s", it.Number, it.Title),
				Detail: it.Repository,
			})
			byID[id] = it
		}
		status := ""
		if len(items) == 0 {
			if ctrl.HasGhProject(ws) {
				status = "no backlog issues"
			} else {
				status = "no open issues in " + ws.BranchRepo
			}
		}
		return itemsLoadedMsg{resource: ResIssues, items: items, issues: byID, status: status}
	}
}

func (m *model) loadPRs() tea.Cmd {
	ws := m.workspace
	ctrl := m.deps.PRs
	return func() tea.Msg {
		prs, err := ctrl.List(ws)
		if err != nil {
			return itemsLoadedMsg{resource: ResPRs, err: err}
		}
		items := make([]Item, 0, len(prs))
		byID := make(map[string]pr.PR, len(prs))
		for _, p := range prs {
			id := fmt.Sprintf("%s#%d", p.Repository, p.Number)
			detail := p.Author
			if p.IsDraft {
				detail += " · draft"
			}
			items = append(items, Item{
				ID:     id,
				Title:  fmt.Sprintf("#%d  %s", p.Number, p.Title),
				Detail: detail,
			})
			byID[id] = p
		}
		status := ""
		if len(items) == 0 {
			status = "no PRs awaiting your review in " + ws.BranchRepo
		}
		return itemsLoadedMsg{resource: ResPRs, items: items, prs: byID, status: status}
	}
}

func (m *model) loadClankers() tea.Cmd {
	ctrl := m.deps.Clankers
	return func() tea.Msg {
		clankers, err := ctrl.List()
		if err != nil {
			return itemsLoadedMsg{resource: ResClankers, err: err}
		}
		items := make([]Item, 0, len(clankers))
		for _, c := range clankers {
			it := Item{ID: fmt.Sprintf("pid-%d", c.PID), Title: fmt.Sprintf("claude (%d)", c.PID), Detail: c.Cwd}
			if c.InTmux() {
				it.ID = c.Session // activatable: jump to this session
				it.Title = c.Session
				it.Detail = fmt.Sprintf("%s  ·  %s", c.PaneTitle, c.Cwd)
				it.Active = c.Active
			}
			items = append(items, it)
		}
		status := ""
		if len(items) == 0 {
			status = "no running claude processes"
		}
		return itemsLoadedMsg{resource: ResClankers, items: items, status: status}
	}
}

// loadLogs builds the logs list from the in-memory ring (newest first).
// It is local — no spinner — and tolerates a nil ring (yielding an empty
// list). Each entry's message and detail seed the row so the `/` filter
// matches both; renderLogRow draws the time and severity from the entry
// looked up by Item.ID.
func (m *model) loadLogs() tea.Cmd {
	entries := m.ring.Entries()
	return func() tea.Msg {
		items := make([]Item, 0, len(entries))
		byID := make(map[string]logx.Entry, len(entries))
		for _, e := range entries {
			id := strconv.Itoa(e.Seq)
			items = append(items, Item{ID: id, Title: e.Msg, Detail: e.Detail})
			byID[id] = e
		}
		return itemsLoadedMsg{resource: ResLogs, items: items, logs: byID}
	}
}

// loadBranches builds the branch-pick list for `a` on the worktrees
// view: branches by recency, excluding the trunk and any branch
// that already has a worktree. It fetches first so branches that exist
// only on the remote (a teammate's just-pushed branch, common when
// checking out for review) gain an origin/<branch> ref and show up. A
// fetch failure is non-fatal — offline, we still list cached branches.
func (m *model) loadBranches() tea.Cmd {
	ws := m.workspace
	svc := m.deps.WorktreeSvc
	logger := m.log
	return func() tea.Msg {
		fetchNote := ""
		if err := svc.Fetch(ws); err != nil {
			fetchNote = "fetch failed; showing cached branches"
			logger.Warn("fetch branches", "err", err)
		}
		branches, err := svc.Branches(ws)
		if err != nil {
			return itemsLoadedMsg{branchPick: true, err: err}
		}
		items := make([]Item, 0, len(branches))
		for _, b := range branches {
			items = append(items, Item{ID: b, Title: b})
		}
		status := fetchNote
		if len(items) == 0 && status == "" {
			status = "no branches available for a new worktree"
		}
		return itemsLoadedMsg{branchPick: true, items: items, status: status}
	}
}

// worktreeType maps the worktree entity's type to the TUI's row tag.
func worktreeType(t worktree.Type) WorktreeType {
	if t == worktree.Main {
		return WorktreeMain
	}
	return WorktreeLinked
}
