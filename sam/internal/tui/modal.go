package tui

import (
	"charm.land/bubbles/v2/textinput"
	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
)

// modalKind is the overlay currently shown over the list, if any.
type modalKind int

const (
	modalNone    modalKind = iota
	modalConfirm           // yes/no, defaults to No
	modalInput             // single-line text entry
	modalHelp              // contextual shortcut reference
	modalDetail            // scrollable read-only text (a full log entry)
	modalError             // a failed operation: Dismiss / View logs
)

// modalState holds the active overlay. For a confirm modal, onConfirm is
// run when the user answers Yes; for an input modal, onSubmit is run with
// the entered value.
type modalState struct {
	kind        modalKind
	title       string
	confirmYes  bool // current right-button highlight: a confirm's Yes (default No), or the error modal's View logs (default true)
	destructive bool // a confirm whose Yes is destructive (renders in the destroy palette)
	onConfirm   func() tea.Cmd
	input       textinput.Model
	onSubmit    func(string) tea.Cmd
	viewport    viewport.Model // scrollable body for modalDetail
}

func (m *model) closeModal() {
	m.modal = modalState{}
}

func (m *model) openHelp() {
	m.modal = modalState{kind: modalHelp}
}

// openErrorModal surfaces a failed operation. confirmYes starts true so the
// View logs button is highlighted by default (the issue's preference) —
// pressing enter takes the user straight to the full error in `:logs`.
func (m *model) openErrorModal(headline string) {
	m.modal = modalState{kind: modalError, title: headline, confirmYes: true}
}

// handleModalKey routes keys while an overlay is shown.
func (m *model) handleModalKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch m.modal.kind {
	case modalHelp:
		switch msg.String() {
		case "?", "esc", "q", "enter":
			m.closeModal()
		}
		return m, nil

	case modalDetail:
		switch msg.String() {
		case "esc", "q", "enter":
			m.closeModal()
			return m, nil
		}
		var c tea.Cmd
		m.modal.viewport, c = m.modal.viewport.Update(msg)
		return m, c

	case modalError:
		switch msg.String() {
		case "left", "right", "h", "l", "tab":
			m.modal.confirmYes = !m.modal.confirmYes
		case "esc", "q":
			m.closeModal()
		case "enter":
			viewLogs := m.modal.confirmYes
			m.closeModal()
			if viewLogs {
				return m, m.switchResource(ResLogs)
			}
		}
		return m, nil

	case modalConfirm:
		switch msg.String() {
		case "left", "right", "h", "l", "tab":
			m.modal.confirmYes = !m.modal.confirmYes
		case "y", "Y":
			return m.confirmYes()
		case "n", "N", "esc":
			m.cancelModal()
		case "enter":
			if m.modal.confirmYes {
				return m.confirmYes()
			}
			m.cancelModal()
		}
		return m, nil

	case modalInput:
		switch msg.String() {
		case "esc":
			m.cancelModal()
			return m, nil
		case "enter":
			fn := m.modal.onSubmit
			v := m.modal.input.Value()
			m.closeModal()
			if fn != nil {
				return m, fn(v)
			}
			return m, nil
		}
		var c tea.Cmd
		m.modal.input, c = m.modal.input.Update(msg)
		return m, c
	}
	return m, nil
}

// confirmYes dismisses the modal, then runs its action. Closing first
// lets the action open a follow-up modal (e.g. the branch-edit input).
func (m *model) confirmYes() (tea.Model, tea.Cmd) {
	fn := m.modal.onConfirm
	m.closeModal()
	if fn != nil {
		return m, fn()
	}
	return m, nil
}

// cancelModal dismisses the modal and abandons any in-flight from-issue
// flow it was gating.
func (m *model) cancelModal() {
	m.closeModal()
	m.pending = nil
}
