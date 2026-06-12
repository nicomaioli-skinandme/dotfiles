package tui

import tea "charm.land/bubbletea/v2"

// failNow is the single place a genuine operation failure is presented. It
// logs the full (often multi-line) error to the ring and temp-file sink — so
// it stays readable in the `:logs` detail modal — and pops the error modal
// with a short, fixed headline.
//
// headline is a message we control, never err.Error(): raw gh/git output is
// multi-line and would corrupt the alt-screen render if it leaked into fixed
// chrome (see the status-bar one-row note). The full error lives only in the
// logs.
//
// Inline input validation ("branch name required") and other non-failures are
// not operation failures and must not route through here — they stay as
// lightweight status text.
func (m *model) failNow(headline string, err error) {
	m.loading = false
	m.log.Error(headline, "err", err)
	m.openErrorModal(headline)
}

// fail is failNow for the message handlers that return (tea.Model, tea.Cmd).
func (m *model) fail(headline string, err error) (tea.Model, tea.Cmd) {
	m.failNow(headline, err)
	return m, nil
}
