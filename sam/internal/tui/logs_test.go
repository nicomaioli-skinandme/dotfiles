package tui

import (
	"errors"
	"log/slog"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/nicomaioli-skinandme/dotfiles/sam/internal/config"
	"github.com/nicomaioli-skinandme/dotfiles/sam/internal/logx"
)

// seededLogsModel returns a model whose ring holds the given log calls
// (applied in order) and whose current view is the loaded logs list.
func seededLogsModel(t *testing.T, seed func(*slog.Logger)) *model {
	t.Helper()
	logger, ring, _ := logx.New(slog.LevelDebug, "")
	seed(logger)
	m := newModel("", nil, nil, ResWorkspaces, config.Tui{}, Deps{Logger: logger, LogRing: ring})
	m.width, m.height = 80, 24
	msg := m.loadLogs()().(itemsLoadedMsg)
	m.resource = ResLogs
	m.applyLoaded(msg)
	return m
}

func TestParseCommandLogs(t *testing.T) {
	if got := parseCommand(":logs"); got != (command{kind: cmdResource, resource: ResLogs}) {
		t.Errorf("parseCommand(\":logs\") = %+v, want resource ResLogs", got)
	}
}

func TestSwitchToLogsAllowedWithoutWorkspace(t *testing.T) {
	logger, ring, _ := logx.New(slog.LevelInfo, "")
	m := newModel("", nil, nil, ResWorkspaces, config.Tui{}, Deps{Logger: logger, LogRing: ring})

	cmd := m.switchResource(ResLogs)
	if m.resource != ResLogs {
		t.Fatalf("resource = %v, want ResLogs", m.resource)
	}
	if m.status == "pick a workspace first" {
		t.Error("logs view should not require a workspace")
	}
	if cmd == nil {
		t.Error("expected a load command for the logs view")
	}

	// Other resources are still gated when no workspace is active.
	m2 := newModel("", nil, nil, ResWorkspaces, config.Tui{}, Deps{Logger: logger, LogRing: ring})
	if m2.switchResource(ResIssues); m2.status != "pick a workspace first" {
		t.Errorf("issues without workspace: status = %q, want gate", m2.status)
	}
}

func TestLoadLogsBuildsRowsNewestFirst(t *testing.T) {
	m := seededLogsModel(t, func(l *slog.Logger) {
		l.Info("started")
		l.Warn("fetch branches", "err", errors.New("network down"))
		l.Error("load issues", "err", errors.New("gh: not authenticated"))
	})

	if len(m.items) != 3 {
		t.Fatalf("items = %d, want 3", len(m.items))
	}
	// Newest first: the error row leads.
	if m.items[0].Title != "load issues" {
		t.Errorf("first row = %q, want newest (load issues)", m.items[0].Title)
	}
	// Detail carries the full error for the modal and for `/` search.
	e := m.logEntries[m.items[0].ID]
	if e.Detail != "gh: not authenticated" {
		t.Errorf("entry detail = %q, want full error", e.Detail)
	}
}

func TestActivateLogOpensDetailModal(t *testing.T) {
	m := seededLogsModel(t, func(l *slog.Logger) {
		l.Error("load issues", "err", errors.New("gh: not authenticated\nrun: gh auth login"))
	})

	if _, cmd := m.activate(); cmd != nil {
		t.Errorf("activating a log row should not return a command, got %v", cmd)
	}
	if m.modal.kind != modalDetail {
		t.Fatalf("modal kind = %v, want modalDetail", m.modal.kind)
	}
	// The modal body should carry the full multi-line detail.
	if got := m.modal.viewport.View(); got == "" {
		t.Error("detail viewport rendered empty")
	}
}

func TestLoadErrorOpensModalAndLogs(t *testing.T) {
	logger, ring, _ := logx.New(slog.LevelInfo, "")
	m := newModel("ws", &config.Workspace{}, nil, ResIssues, config.Tui{}, Deps{Logger: logger, LogRing: ring})
	m.resource = ResIssues

	m.applyLoaded(itemsLoadedMsg{resource: ResIssues, err: errors.New("gh: HTTP 401")})

	// The error surfaces in the modal, not a terse status string.
	if m.modal.kind != modalError {
		t.Errorf("modal kind = %v, want modalError", m.modal.kind)
	}
	if !m.modal.confirmYes {
		t.Error("error modal should default-highlight View logs (confirmYes)")
	}
	if m.status != "" {
		t.Errorf("status = %q, want empty (no terse string)", m.status)
	}
	// The full error is still logged for the `:logs` detail view and temp file.
	entries := ring.Entries()
	if len(entries) != 1 {
		t.Fatalf("ring entries = %d, want 1 logged error", len(entries))
	}
	if entries[0].Level != slog.LevelError || entries[0].Detail != "gh: HTTP 401" {
		t.Errorf("logged entry = %+v, want ERROR with full detail", entries[0])
	}
}

func TestLogIconPersistent(t *testing.T) {
	logger, ring, _ := logx.New(slog.LevelInfo, "")
	m := newModel("ws", &config.Workspace{Trunk: "main"}, nil, ResWorktrees, config.Tui{}, Deps{Logger: logger, LogRing: ring})
	m.width, m.height = 80, 24

	// No warnings/errors: no icon, and no count (the icon is presence-only now).
	if bar := m.renderStatusBar(); strings.Contains(bar, "⚠") {
		t.Errorf("icon shown with no warnings/errors: %q", bar)
	}

	// An info entry alone does not trip the icon.
	logger.Info("loaded")
	if bar := m.renderStatusBar(); strings.Contains(bar, "⚠") {
		t.Errorf("info entry tripped the icon: %q", bar)
	}

	// A warning shows the icon — without a count (the old format was "⚠ N").
	logger.Warn("w", "err", errors.New("x"))
	bar := m.renderStatusBar()
	if !strings.Contains(bar, "⚠") {
		t.Errorf("status bar missing log icon: %q", bar)
	}
	if strings.Contains(bar, "⚠ 1") {
		t.Errorf("icon should carry no count: %q", bar)
	}

	// It persists across renders — there's no seen/reset that clears it.
	logger.Error("e", "err", errors.New("y"))
	if bar := m.renderStatusBar(); !strings.Contains(bar, "⚠") {
		t.Errorf("icon should persist while warnings/errors remain: %q", bar)
	}
}

func TestErrorModalViewLogsNavigates(t *testing.T) {
	logger, ring, _ := logx.New(slog.LevelInfo, "")
	m := newModel("ws", &config.Workspace{Trunk: "main"}, nil, ResWorktrees, config.Tui{}, Deps{Logger: logger, LogRing: ring})
	m.width, m.height = 80, 24

	// A failed operation opens the error modal with View logs highlighted.
	m.failNow("Couldn't build the session", errors.New("boom"))
	if m.modal.kind != modalError || !m.modal.confirmYes {
		t.Fatalf("modal = {%v, confirmYes=%v}, want modalError with View logs default", m.modal.kind, m.modal.confirmYes)
	}

	// Enter follows the highlighted View logs action to the logs view.
	_, cmd := m.handleKey(tea.KeyPressMsg{Code: tea.KeyEnter})
	if m.modal.kind != modalNone {
		t.Errorf("modal kind = %v, want closed", m.modal.kind)
	}
	if m.resource != ResLogs {
		t.Errorf("resource = %v, want ResLogs", m.resource)
	}
	if cmd == nil {
		t.Error("expected a load command for the logs view")
	}
}

func TestErrorModalDismiss(t *testing.T) {
	m := newModel("ws", &config.Workspace{Trunk: "main"}, nil, ResWorktrees, config.Tui{}, Deps{})
	m.failNow("Couldn't build the session", errors.New("boom"))

	// Toggle to Dismiss, then enter — closes the modal, stays put.
	m.handleKey(tea.KeyPressMsg{Code: tea.KeyLeft})
	if m.modal.confirmYes {
		t.Fatal("left should move highlight off View logs")
	}
	if _, cmd := m.handleKey(tea.KeyPressMsg{Code: tea.KeyEnter}); cmd != nil {
		t.Errorf("dismiss should not return a command, got %v", cmd)
	}
	if m.modal.kind != modalNone {
		t.Errorf("modal kind = %v, want closed", m.modal.kind)
	}
	if m.resource != ResWorktrees {
		t.Errorf("resource = %v, want unchanged ResWorktrees", m.resource)
	}

	// esc also dismisses.
	m.failNow("again", errors.New("boom"))
	m.handleKey(tea.KeyPressMsg{Code: tea.KeyEscape})
	if m.modal.kind != modalNone {
		t.Errorf("esc should dismiss: modal kind = %v", m.modal.kind)
	}
}

func TestLogIconClickNavigates(t *testing.T) {
	logger, ring, _ := logx.New(slog.LevelInfo, "")
	logger.Warn("w", "err", errors.New("x"))
	m := newModel("ws", &config.Workspace{Trunk: "main"}, nil, ResWorktrees, config.Tui{}, Deps{Logger: logger, LogRing: ring})
	m.width, m.height = 80, 24

	// Rendering the status bar records the icon's clickable bounds.
	m.renderStatusBar()
	if m.logIcon.x1 <= m.logIcon.x0 {
		t.Fatalf("icon hit region not recorded: %+v", m.logIcon)
	}

	// A left-click on the icon jumps to the logs view.
	m.Update(tea.MouseClickMsg{X: m.logIcon.x0, Y: m.height - 1, Button: tea.MouseLeft})
	if m.resource != ResLogs {
		t.Errorf("resource = %v, want ResLogs after icon click", m.resource)
	}
}

func TestLogIconClickOffTargetIgnored(t *testing.T) {
	logger, ring, _ := logx.New(slog.LevelInfo, "")
	logger.Warn("w", "err", errors.New("x"))
	m := newModel("ws", &config.Workspace{Trunk: "main"}, nil, ResWorktrees, config.Tui{}, Deps{Logger: logger, LogRing: ring})
	m.width, m.height = 80, 24
	m.renderStatusBar()

	// A click away from the icon does nothing.
	m.Update(tea.MouseClickMsg{X: 0, Y: 0, Button: tea.MouseLeft})
	if m.resource != ResWorktrees {
		t.Errorf("resource = %v, want unchanged ResWorktrees", m.resource)
	}
}

func TestLogRowSearchMatchesDetail(t *testing.T) {
	m := seededLogsModel(t, func(l *slog.Logger) {
		l.Info("started")
		l.Error("load issues", "err", errors.New("gh: not authenticated"))
	})

	m.query = "authenticated"
	m.applyFilter()
	if len(m.filtered) != 1 || m.filtered[0].Title != "load issues" {
		t.Errorf("filter on detail text = %+v, want the gh error row", m.filtered)
	}
}
