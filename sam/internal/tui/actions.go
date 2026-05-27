package tui

import (
	"fmt"
	"path/filepath"
	"strings"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"

	"github.com/nicomaioli-skinandme/dotfiles/sam/internal/config"
	"github.com/nicomaioli-skinandme/dotfiles/sam/internal/ghx"
	"github.com/nicomaioli-skinandme/dotfiles/sam/internal/gitx"
	"github.com/nicomaioli-skinandme/dotfiles/sam/internal/issueflow"
	"github.com/nicomaioli-skinandme/dotfiles/sam/internal/tmuxx"
)

// actionDoneMsg reports the result of an in-TUI mutation (e.g. delete)
// so the model can refresh and surface a status line.
type actionDoneMsg struct {
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
		m.result = Result{NewWorktreeBranch: it.ID}
		return m, tea.Quit
	}
	switch m.resource {
	case ResWorktrees:
		return m.activateWorktree(it)
	case ResWorkspaces:
		return m, m.switchWorkspace(it.ID)
	case ResIssues:
		return m.activateIssue(it)
	case ResClankers:
		return m.activateClanker(it)
	}
	return m, nil
}

// activateWorktree records an attach (building the session first when it
// doesn't already exist), mirroring the old menu's selection logic.
func (m *model) activateWorktree(it Item) (tea.Model, tea.Cmd) {
	name := it.ID
	switch {
	case tmuxx.HasSession(name):
		m.result = Result{Attach: name}
	case name == "system":
		m.result = Result{Attach: "system", Build: &BuildSpec{EnsureSystem: true}}
	case name == m.workspace.MainBranch:
		m.result = Result{Attach: name, Build: &BuildSpec{BaseDir: m.workspace.Repo}}
	default:
		baseDir := filepath.Join(m.workspace.Worktrees, name)
		m.result = Result{Attach: name, Build: &BuildSpec{BaseDir: baseDir}}
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
	issue    issueflow.Issue
	me       string
	branch   string
	existing string
	reassign bool
}

// fromIssuePreparedMsg reports the result of resolving the current user
// and planning the issue's branch.
type fromIssuePreparedMsg struct {
	issue    issueflow.Issue
	me       string
	branch   string
	existing string
	err      error
}

// fromIssueDoneMsg reports the result of the bootstrap (issueflow.Apply).
type fromIssueDoneMsg struct {
	session string
	err     error
}

// activateIssue starts the from-issue flow for the picked issue: resolve
// the current user and plan the branch, then (via modals) confirm any
// reassignment and branch edit before bootstrapping.
func (m *model) activateIssue(it Item) (tea.Model, tea.Cmd) {
	issue, ok := m.issues[it.ID]
	if !ok {
		m.status = "no issue data for " + it.ID
		return m, nil
	}
	m.loading = true
	m.status = ""
	ws := m.workspace
	return m, tea.Batch(m.spinner.Tick, func() tea.Msg {
		me, err := ghx.CurrentUser()
		if err != nil {
			return fromIssuePreparedMsg{err: err}
		}
		branch, existing, err := issueflow.Plan(ws, issue)
		if err != nil {
			return fromIssuePreparedMsg{err: err}
		}
		return fromIssuePreparedMsg{issue: issue, me: me, branch: branch, existing: existing}
	})
}

// handleFromIssuePrepared continues the flow once the user/branch are
// known: prompt to reassign if the issue belongs to someone else, else
// move on to the branch step.
func (m *model) handleFromIssuePrepared(msg fromIssuePreparedMsg) (tea.Model, tea.Cmd) {
	m.loading = false
	if msg.err != nil {
		m.status = "error: " + msg.err.Error()
		return m, nil
	}
	m.pending = &fromIssueState{issue: msg.issue, me: msg.me, branch: msg.branch, existing: msg.existing}

	if other, needs := issueflow.NeedsReassign(msg.issue, msg.me); needs {
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
	if issueflow.NeedsBranchEdit(m.workspace, m.pending.branch) {
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
	return tea.Batch(m.spinner.Tick, func() tea.Msg {
		session, err := issueflow.Apply(ws, wsName, st.issue, st.me, st.reassign, st.branch, st.existing)
		return fromIssueDoneMsg{session: session, err: err}
	})
}

// handleFromIssueDone records the session to attach to and quits, or
// surfaces an error and stays.
func (m *model) handleFromIssueDone(msg fromIssueDoneMsg) (tea.Model, tea.Cmd) {
	m.loading = false
	m.pending = nil
	if msg.err != nil {
		m.status = "error: " + msg.err.Error()
		return m, nil
	}
	m.result = Result{Attach: msg.session}
	return m, tea.Quit
}

// activateClanker attaches to the clanker's tmux session when it has one.
func (m *model) activateClanker(it Item) (tea.Model, tea.Cmd) {
	if !tmuxx.HasSession(it.ID) {
		m.status = "no tmux session for this clanker"
		return m, nil
	}
	m.result = Result{Attach: it.ID}
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
		m.pushView()
		m.branchPick = true
		m.query = ""
		m.cursor = 0
		m.items = nil
		return m, m.loadBranches()
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
	switch it.ID {
	case "system":
		m.status = "cannot delete the system session"
		return m, nil
	case m.workspace.MainBranch:
		m.status = "cannot delete the main repo"
		return m, nil
	}
	if cur, _ := tmuxx.CurrentSession(); cur == it.ID {
		m.status = "cannot delete the session you're attached to"
		return m, nil
	}
	target := it.ID
	ws := m.workspace
	m.modal = modalState{
		kind:  modalConfirm,
		title: fmt.Sprintf("Delete worktree %q?", target),
		onConfirm: func() tea.Cmd {
			return deleteWorktreeCmd(ws, target)
		},
	}
	return m, nil
}

// deleteWorktreeCmd kills the session (if any) and force-removes the
// worktree, off the UI goroutine.
func deleteWorktreeCmd(ws *config.Workspace, target string) tea.Cmd {
	return func() tea.Msg {
		if tmuxx.HasSession(target) {
			if err := tmuxx.KillSession(target); err != nil {
				return actionDoneMsg{err: err}
			}
		}
		if err := gitx.WorktreeRemoveForce(ws.Repo, filepath.Join(ws.Worktrees, target)); err != nil {
			return actionDoneMsg{err: err}
		}
		return actionDoneMsg{reload: true, info: "deleted " + target}
	}
}

// handleActionDone refreshes the list after a mutation and reports the
// outcome in the status line.
func (m *model) handleActionDone(msg actionDoneMsg) (tea.Model, tea.Cmd) {
	if msg.err != nil {
		m.status = "error: " + msg.err.Error()
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
		raw := m.input.Value()
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
	return m, c
}
