package tui

import (
	"fmt"
	"strings"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"

	"github.com/nicomaioli-skinandme/dotfiles/sam/internal/config"
	"github.com/nicomaioli-skinandme/dotfiles/sam/internal/issue"
	"github.com/nicomaioli-skinandme/dotfiles/sam/internal/worktree"
)

// actionDoneMsg reports the result of an in-TUI mutation (e.g. delete)
// so the model can refresh and surface a status line.
type actionDoneMsg struct {
	target string // worktree ID the mutation acted on, so the model can clear its in-flight state
	reload bool
	info   string
	err    error
}

// activate handles <CR>/l on the highlighted row, dispatching on the
// current view.
func (m *model) activate() (tea.Model, tea.Cmd) {
	it, ok := m.current()
	if !ok {
		return m, nil
	}
	if m.branchPick {
		// Picked a branch for a new worktree: defer creation to the caller.
		m.result = Result{
			NewWorktreeBranch: it.ID,
			Workspace:         m.workspace,
			WorkspaceName:     m.workspaceName,
		}
		return m, tea.Quit
	}
	switch m.resource {
	case ResWorktrees:
		return m.activateWorktree(it)
	case ResWorkspaces:
		return m, m.switchWorkspace(it.ID)
	case ResIssues:
		return m.activateIssue(it)
	case ResPRs:
		return m.activatePR(it)
	case ResClankers:
		return m.activateClanker(it)
	}
	return m, nil
}

// activateWorktree records the worktree to attach to and quits; the caller
// (after the TUI releases the terminal) builds its session if absent and
// attaches, via the session Controller.
func (m *model) activateWorktree(it Item) (tea.Model, tea.Cmd) {
	m.result = Result{
		Attach:        it.ID,
		Workspace:     m.workspace,
		WorkspaceName: m.workspaceName,
	}
	return m, tea.Quit
}

// switchWorkspace changes the active workspace in place and reloads the
// worktrees view against it.
func (m *model) switchWorkspace(name string) tea.Cmd {
	ws, ok := m.all[name]
	if !ok {
		m.status = "unknown workspace: " + name
		return nil
	}
	m.workspace = &ws
	m.workspaceName = name
	return m.switchResource(ResWorktrees)
}

// fromIssueState tracks an in-flight from-issue flow across its async
// steps and modal prompts.
type fromIssueState struct {
	iss      issue.Issue
	me       string
	branch   string
	existing string
	reassign bool
}

// fromIssuePreparedMsg reports the result of resolving the current user
// and planning the issue's branch.
type fromIssuePreparedMsg struct {
	iss      issue.Issue
	me       string
	branch   string
	existing string
	err      error
}

// fromIssueDoneMsg reports the result of the bootstrap (issue Apply).
type fromIssueDoneMsg struct {
	session string
	err     error
}

// activateIssue starts the from-issue flow for the picked issue: resolve
// the current user and plan the branch, then (via modals) confirm any
// reassignment and branch edit before bootstrapping.
func (m *model) activateIssue(it Item) (tea.Model, tea.Cmd) {
	iss, ok := m.issues[it.ID]
	if !ok {
		m.status = "no issue data for " + it.ID
		return m, nil
	}
	m.loading = true
	m.status = ""
	ws := m.workspace
	ctrl := m.deps.Issues
	return m, tea.Batch(m.spinner.Tick, func() tea.Msg {
		me, branch, existing, err := ctrl.Prepare(ws, iss)
		if err != nil {
			return fromIssuePreparedMsg{err: err}
		}
		return fromIssuePreparedMsg{iss: iss, me: me, branch: branch, existing: existing}
	})
}

// handleFromIssuePrepared continues the flow once the user/branch are
// known: prompt to reassign if the issue belongs to someone else, else
// move on to the branch step.
func (m *model) handleFromIssuePrepared(msg fromIssuePreparedMsg) (tea.Model, tea.Cmd) {
	m.loading = false
	if msg.err != nil {
		m.status = "gh errored"
		return m, nil
	}
	m.pending = &fromIssueState{iss: msg.iss, me: msg.me, branch: msg.branch, existing: msg.existing}

	if other, needs := m.deps.IssueSvc.NeedsReassign(msg.iss, msg.me); needs {
		m.modal = modalState{
			kind:  modalConfirm,
			title: fmt.Sprintf("Issue assigned to %s. Reassign to you?", other),
			onConfirm: func() tea.Cmd {
				_, c := m.fromIssueBranchStep(true)
				return c
			},
		}
		return m, nil
	}
	return m.fromIssueBranchStep(false)
}

// fromIssueBranchStep prompts to edit the branch name when it exceeds the
// workspace limit, then applies the bootstrap.
func (m *model) fromIssueBranchStep(reassign bool) (tea.Model, tea.Cmd) {
	m.pending.reassign = reassign
	if m.deps.IssueSvc.NeedsBranchEdit(m.workspace, m.pending.branch) {
		ti := textinput.New()
		ti.SetVirtualCursor(true)
		ti.SetValue(m.pending.branch)
		ti.CursorEnd()
		ti.Focus()
		m.modal = modalState{
			kind:  modalInput,
			title: fmt.Sprintf("Branch name (limit %d chars)", m.workspace.MaxBranchLen),
			input: ti,
			onSubmit: func(v string) tea.Cmd {
				if v != "" {
					m.pending.branch = v
				}
				return m.fromIssueApplyCmd()
			},
		}
		return m, nil
	}
	return m, m.fromIssueApplyCmd()
}

// fromIssueApplyCmd runs the bootstrap off the UI goroutine behind a
// spinner.
func (m *model) fromIssueApplyCmd() tea.Cmd {
	m.loading = true
	st := *m.pending
	ws := m.workspace
	wsName := m.workspaceName
	ctrl := m.deps.Issues
	return tea.Batch(m.spinner.Tick, func() tea.Msg {
		session, err := ctrl.Apply(ws, wsName, st.iss, st.me, st.reassign, st.branch, st.existing)
		return fromIssueDoneMsg{session: session, err: err}
	})
}

// handleFromIssueDone records the session to attach to and quits, or
// surfaces an error and stays.
func (m *model) handleFromIssueDone(msg fromIssueDoneMsg) (tea.Model, tea.Cmd) {
	m.loading = false
	m.pending = nil
	if msg.err != nil {
		m.status = "gh errored"
		return m, nil
	}
	// The issue Apply already built the session; attach to it directly.
	m.result = Result{
		AttachSession: msg.session,
		Workspace:     m.workspace,
		WorkspaceName: m.workspaceName,
	}
	return m, tea.Quit
}

// fromPRDoneMsg reports the result of the PR review bootstrap (pr Apply).
type fromPRDoneMsg struct {
	session string
	err     error
}

// activatePR bootstraps a review worktree for the picked PR. Unlike the
// issue flow there are no modals (no reassign/branch-edit): we check out
// the PR's existing head branch, so this just runs the pr Apply behind a
// spinner and attaches.
func (m *model) activatePR(it Item) (tea.Model, tea.Cmd) {
	p, ok := m.prs[it.ID]
	if !ok {
		m.status = "no PR data for " + it.ID
		return m, nil
	}
	m.loading = true
	m.status = ""
	ws := m.workspace
	wsName := m.workspaceName
	ctrl := m.deps.PRs
	return m, tea.Batch(m.spinner.Tick, func() tea.Msg {
		session, err := ctrl.Apply(ws, wsName, p)
		return fromPRDoneMsg{session: session, err: err}
	})
}

// handleFromPRDone records the session to attach to and quits, or
// surfaces an error and stays.
func (m *model) handleFromPRDone(msg fromPRDoneMsg) (tea.Model, tea.Cmd) {
	m.loading = false
	if msg.err != nil {
		m.status = "gh errored"
		return m, nil
	}
	// The pr Apply already built the session; attach to it directly.
	m.result = Result{
		AttachSession: msg.session,
		Workspace:     m.workspace,
		WorkspaceName: m.workspaceName,
	}
	return m, tea.Quit
}

// activateClanker attaches to the clanker's tmux session when it has one.
func (m *model) activateClanker(it Item) (tea.Model, tea.Cmd) {
	if !m.deps.SessionSvc.Has(it.ID) {
		m.status = "no tmux session for this clanker"
		return m, nil
	}
	m.result = Result{
		AttachSession: it.ID,
		Workspace:     m.workspace,
		WorkspaceName: m.workspaceName,
	}
	return m, tea.Quit
}

// add handles `a`: only the views where adding is meaningful respond.
func (m *model) add() (tea.Model, tea.Cmd) {
	switch m.resource {
	case ResWorktrees:
		if m.branchPick {
			return m, nil
		}
		// Enter the branch-pick sub-view to create a worktree from a branch.
		// loadBranches fetches first (a network call), so show the spinner.
		m.pushView()
		m.branchPick = true
		m.query = ""
		m.cursor = 0
		m.items = nil
		m.loading = true
		return m, tea.Batch(m.spinner.Tick, m.loadBranches())
	case ResWorkspaces:
		// Adding a workspace runs the huh wizard, which needs the terminal.
		m.result = Result{RunWizard: true}
		return m, tea.Quit
	}
	return m, nil
}

// del handles `d`: delete the highlighted worktree after confirmation.
func (m *model) del() (tea.Model, tea.Cmd) {
	if m.resource != ResWorktrees || m.branchPick {
		return m, nil
	}
	it, ok := m.current()
	if !ok {
		return m, nil
	}
	if it.Type == WorktreeMain {
		m.status = "cannot delete the main worktree"
		return m, nil
	}
	if cur, _ := m.deps.SessionSvc.Current(); cur == m.deps.SessionSvc.Name(m.workspaceName, it.ID) {
		m.status = "cannot delete the session you're attached to"
		return m, nil
	}
	target := it.ID
	ws := m.workspace
	wsName := m.workspaceName
	ctrl := m.deps.Worktrees
	m.modal = modalState{
		kind:        modalConfirm,
		title:       fmt.Sprintf("Delete worktree %q?", target),
		destructive: true,
		onConfirm: func() tea.Cmd {
			m.deleting[target] = true
			m.status = ""
			return tea.Batch(m.spinner.Tick, deleteWorktreeCmd(ctrl, ws, wsName, target))
		},
	}
	return m, nil
}

// deleteWorktreeCmd kills the session (if any) and force-removes the
// worktree, off the UI goroutine, via the worktree Controller.
func deleteWorktreeCmd(ctrl worktree.Controller, ws *config.Workspace, wsName, target string) tea.Cmd {
	return func() tea.Msg {
		if err := ctrl.Delete(ws, wsName, target); err != nil {
			return actionDoneMsg{target: target, err: err}
		}
		return actionDoneMsg{target: target, reload: true, info: "deleted " + target}
	}
}

// handleActionDone refreshes the list after a mutation and reports the
// outcome in the status line.
func (m *model) handleActionDone(msg actionDoneMsg) (tea.Model, tea.Cmd) {
	delete(m.deleting, msg.target)
	if msg.err != nil {
		m.status = "action failed"
		return m, nil
	}
	m.status = msg.info
	if msg.reload {
		return m, m.loadResource()
	}
	return m, nil
}

// handleInputKey drives the top bar while in search or command mode.
func (m *model) handleInputKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	// Command-mode popup navigation. These keys steer the autocomplete
	// rather than editing the input, so they're handled before the
	// textinput sees them. Tab/Shift+Tab "complete" by filling the input
	// with the highlighted entry without re-filtering (so the list keeps
	// its full shape and repeated presses cycle through it).
	if m.mode == modeCommand && m.ac.Visible() {
		switch msg.String() {
		case "up":
			m.ac.Move(-1)
			return m, nil
		case "down":
			m.ac.Move(1)
			return m, nil
		case "tab":
			m.completeFromPopup(1)
			return m, nil
		case "shift+tab":
			m.completeFromPopup(-1)
			return m, nil
		}
	}

	switch msg.String() {
	case "esc":
		if m.mode == modeSearch {
			m.query = ""
			m.applyFilter()
		}
		m.leaveInput()
		return m, nil
	case "enter":
		if m.mode == modeSearch {
			m.query = m.input.Value()
			m.applyFilter()
			m.leaveInput()
			return m, nil
		}
		// Prefer the highlighted entry over the raw text so Enter on a
		// partial query runs the selection. Only when the user has
		// actually typed something, though: a bare `:` then Enter stays a
		// no-op rather than firing whatever sits first in the popup.
		raw := m.input.Value()
		if raw != "" {
			if sel, ok := m.ac.Selected(); ok {
				raw = sel
			}
		}
		cmd := parseCommand(raw)
		m.leaveInput()
		switch cmd.kind {
		case cmdQuit:
			return m, tea.Quit
		case cmdResource:
			return m, m.switchResource(cmd.resource)
		case cmdUnknown:
			m.status = "unknown command: :" + strings.TrimSpace(strings.TrimPrefix(raw, ":"))
		}
		return m, nil
	}
	var c tea.Cmd
	m.input, c = m.input.Update(msg)
	if m.mode == modeSearch {
		m.query = m.input.Value()
		m.applyFilter()
	}
	if m.mode == modeCommand {
		m.ac.SetQuery(m.input.Value())
	}
	return m, c
}

// completeFromPopup cycles the popup highlight by delta (wrapping) and
// fills the input with the now-highlighted entry. It deliberately does not
// call SetQuery, so the candidate list keeps its shape and successive
// Tab/Shift+Tab presses walk through every entry.
func (m *model) completeFromPopup(delta int) {
	m.ac.Cycle(delta)
	if sel, ok := m.ac.Selected(); ok {
		m.input.SetValue(sel)
		m.input.CursorEnd()
	}
}
