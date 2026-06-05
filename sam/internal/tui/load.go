package tui

import (
	"fmt"
	"sort"

	tea "charm.land/bubbletea/v2"

	"github.com/nicomaioli-skinandme/dotfiles/sam/internal/gitx"
	"github.com/nicomaioli-skinandme/dotfiles/sam/internal/issueflow"
	"github.com/nicomaioli-skinandme/dotfiles/sam/internal/prflow"
	"github.com/nicomaioli-skinandme/dotfiles/sam/internal/proc"
	"github.com/nicomaioli-skinandme/dotfiles/sam/internal/tmuxx"
)

// itemsLoadedMsg carries the result of loading a resource's rows. It
// records which view it was loading for so a stale load (the user
// switched resources before it returned) can be ignored.
type itemsLoadedMsg struct {
	resource   Resource
	branchPick bool
	items      []Item
	issues     map[string]issueflow.Issue // resolved issues, keyed by Item.ID (ResIssues only)
	prs        map[string]prflow.PR       // resolved PRs, keyed by Item.ID (ResPRs only)
	status     string                     // non-fatal note shown in the status line (e.g. "no issues")
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
		// Surface load failures in the status line and keep the TUI usable
		// (switch to another resource, quit) rather than aborting. The error
		// is kept generic on purpose — raw gh/git output is multiline and
		// would overflow the status bar; richer error surfacing comes later.
		if m.resource == ResIssues || m.resource == ResPRs {
			m.status = "gh errored"
		} else {
			m.status = "couldn't load " + m.resource.Name()
		}
		m.items = nil
	} else {
		m.items = msg.items
	}
	m.issues = msg.issues
	m.prs = msg.prs
	m.cursor = 0
	m.applyFilter()
}

func (m *model) loadWorktrees() tea.Cmd {
	ws := m.workspace
	wsName := m.workspaceName
	return func() tea.Msg {
		worktrees, err := gitx.Worktrees(ws.Worktrees)
		if err != nil {
			return itemsLoadedMsg{resource: ResWorktrees, err: err}
		}
		items := []Item{
			{
				ID:     ws.Trunk,
				Title:  ws.Trunk,
				Detail: "main worktree",
				Active: tmuxx.HasSession(tmuxx.SessionName(wsName, ws.Trunk)),
				Type:   WorktreeMain,
			},
		}
		for _, w := range worktrees {
			items = append(items, Item{ID: w, Title: w, Active: tmuxx.HasSession(tmuxx.SessionName(wsName, w)), Type: WorktreeLinked})
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
	return func() tea.Msg {
		issues, err := issueflow.List(ws)
		if err != nil {
			return itemsLoadedMsg{resource: ResIssues, err: err}
		}
		items := make([]Item, 0, len(issues))
		byID := make(map[string]issueflow.Issue, len(issues))
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
			if issueflow.HasGhProject(ws) {
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
	return func() tea.Msg {
		prs, err := prflow.List(ws)
		if err != nil {
			return itemsLoadedMsg{resource: ResPRs, err: err}
		}
		items := make([]Item, 0, len(prs))
		byID := make(map[string]prflow.PR, len(prs))
		for _, pr := range prs {
			id := fmt.Sprintf("%s#%d", pr.Repository, pr.Number)
			detail := pr.Author
			if pr.IsDraft {
				detail += " · draft"
			}
			items = append(items, Item{
				ID:     id,
				Title:  fmt.Sprintf("#%d  %s", pr.Number, pr.Title),
				Detail: detail,
			})
			byID[id] = pr
		}
		status := ""
		if len(items) == 0 {
			status = "no PRs awaiting your review in " + ws.BranchRepo
		}
		return itemsLoadedMsg{resource: ResPRs, items: items, prs: byID, status: status}
	}
}

func (m *model) loadClankers() tea.Cmd {
	return func() tea.Msg {
		claudes, err := proc.Claudes()
		if err != nil {
			return itemsLoadedMsg{resource: ResClankers, err: err}
		}
		panes, err := proc.TmuxPanes()
		if err != nil {
			return itemsLoadedMsg{resource: ResClankers, err: err}
		}
		items := make([]Item, 0, len(claudes))
		for _, c := range claudes {
			cwd, _ := proc.Cwd(c.PID)
			it := Item{ID: fmt.Sprintf("pid-%d", c.PID), Title: fmt.Sprintf("claude (%d)", c.PID), Detail: cwd}
			if pane, ok := proc.FindTmuxPane(panes, c.PID); ok {
				it.ID = pane.Session // activatable: jump to this session
				it.Title = pane.Session
				it.Detail = fmt.Sprintf("%s  ·  %s", pane.PaneTitle, cwd)
				it.Active = tmuxx.HasSession(pane.Session)
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

// loadBranches builds the branch-pick list for `a` on the worktrees
// view: branches by recency, excluding the trunk and any branch
// that already has a worktree. It fetches first so branches that exist
// only on the remote (a teammate's just-pushed branch, common when
// checking out for review) gain an origin/<branch> ref and show up. A
// fetch failure is non-fatal — offline, we still list cached branches.
func (m *model) loadBranches() tea.Cmd {
	ws := m.workspace
	return func() tea.Msg {
		fetchNote := ""
		if err := gitx.Fetch(ws.Repo); err != nil {
			fetchNote = "fetch failed; showing cached branches"
		}
		all, err := gitx.BranchesByRecency(ws.Repo)
		if err != nil {
			return itemsLoadedMsg{branchPick: true, err: err}
		}
		existing, err := gitx.Worktrees(ws.Worktrees)
		if err != nil {
			return itemsLoadedMsg{branchPick: true, err: err}
		}
		exclude := map[string]bool{ws.Trunk: true}
		for _, w := range existing {
			exclude[w] = true
		}
		items := make([]Item, 0, len(all))
		for _, b := range all {
			if exclude[b] {
				continue
			}
			items = append(items, Item{ID: b, Title: b})
		}
		status := fetchNote
		if len(items) == 0 && status == "" {
			status = "no branches available for a new worktree"
		}
		return itemsLoadedMsg{branchPick: true, items: items, status: status}
	}
}
