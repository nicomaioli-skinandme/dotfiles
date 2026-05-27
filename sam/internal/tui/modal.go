package tui

import (
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
)

// modalKind is the overlay currently shown over the list, if any.
type modalKind int

const (
	modalNone    modalKind = iota
	modalConfirm           // yes/no, defaults to No
	modalInput             // single-line text entry
	modalHelp              // contextual shortcut reference
)

// modalState holds the active overlay. For a confirm modal, onConfirm is
// run when the user answers Yes; for an input modal, onSubmit is run with
// the entered value.
type modalState struct {
	kind       modalKind
	title      string
	confirmYes bool // current Yes/No highlight (false = No, the default)
	onConfirm  func() tea.Cmd
	input      textinput.Model
	onSubmit   func(string) tea.Cmd
}

func (m *model) closeModal() {
	m.modal = modalState{}
}

func (m *model) openHelp() {
	m.modal = modalState{kind: modalHelp}
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
