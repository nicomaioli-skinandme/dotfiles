package tui

import (
	"fmt"
	"sort"

	tea "charm.land/bubbletea/v2"

	"github.com/nicomaioli-skinandme/dotfiles/sam/internal/gitx"
	"github.com/nicomaioli-skinandme/dotfiles/sam/internal/issueflow"
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
		// (switch to another resource, quit) rather than aborting.
		m.status = "error: " + msg.err.Error()
		m.items = nil
	} else {
		m.items = msg.items
	}
	m.issues = msg.issues
	m.cursor = 0
	m.applyFilter()
}

func (m *model) loadWorktrees() tea.Cmd {
	ws := m.workspace
	return func() tea.Msg {
		worktrees, err := gitx.Worktrees(ws.Worktrees)
		if err != nil {
			return itemsLoadedMsg{resource: ResWorktrees, err: err}
		}
		items := []Item{
			{ID: "system", Title: "system", Active: tmuxx.HasSession("system")},
			{
				ID:     ws.MainBranch,
				Title:  ws.MainBranch,
				Detail: "main repo",
				Active: tmuxx.HasSession(ws.MainBranch),
			},
		}
		for _, w := range worktrees {
			items = append(items, Item{ID: w, Title: w, Active: tmuxx.HasSession(w)})
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
// view: branches by recency, excluding the main branch and any branch
// that already has a worktree.
func (m *model) loadBranches() tea.Cmd {
	ws := m.workspace
	return func() tea.Msg {
		all, err := gitx.BranchesByRecency(ws.Repo)
		if err != nil {
			return itemsLoadedMsg{branchPick: true, err: err}
		}
		existing, err := gitx.Worktrees(ws.Worktrees)
		if err != nil {
			return itemsLoadedMsg{branchPick: true, err: err}
		}
		exclude := map[string]bool{ws.MainBranch: true}
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
		status := ""
		if len(items) == 0 {
			status = "no branches available for a new worktree"
		}
		return itemsLoadedMsg{branchPick: true, items: items, status: status}
	}
}
