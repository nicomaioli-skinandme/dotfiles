package tui

import (
	"errors"
	"log/slog"
	"strings"
	"testing"

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
	// Opening the view marks everything seen.
	if m.logsSeenSeq != m.ring.MaxSeq() {
		t.Errorf("logsSeenSeq = %d, want MaxSeq %d", m.logsSeenSeq, m.ring.MaxSeq())
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

func TestLoadErrorIsLogged(t *testing.T) {
	logger, ring, _ := logx.New(slog.LevelInfo, "")
	m := newModel("ws", &config.Workspace{}, nil, ResIssues, config.Tui{}, Deps{Logger: logger, LogRing: ring})
	m.resource = ResIssues

	m.applyLoaded(itemsLoadedMsg{resource: ResIssues, err: errors.New("gh: HTTP 401")})

	if m.status != "gh errored" {
		t.Errorf("status = %q, want generic %q", m.status, "gh errored")
	}
	entries := ring.Entries()
	if len(entries) != 1 {
		t.Fatalf("ring entries = %d, want 1 logged error", len(entries))
	}
	if entries[0].Level != slog.LevelError || entries[0].Detail != "gh: HTTP 401" {
		t.Errorf("logged entry = %+v, want ERROR with full detail", entries[0])
	}
}

func TestUnseenBadge(t *testing.T) {
	logger, ring, _ := logx.New(slog.LevelInfo, "")
	logger.Warn("w", "err", errors.New("x"))
	logger.Error("e", "err", errors.New("y"))
	m := newModel("ws", &config.Workspace{Trunk: "main"}, nil, ResWorktrees, config.Tui{}, Deps{Logger: logger, LogRing: ring})
	m.width, m.height = 80, 24

	if bar := m.renderStatusBar(); !strings.Contains(bar, "⚠ 2") {
		t.Errorf("status bar missing unseen badge: %q", bar)
	}

	// Marking everything seen (as opening :logs does) clears the badge.
	m.logsSeenSeq = ring.MaxSeq()
	if bar := m.renderStatusBar(); strings.Contains(bar, "⚠") {
		t.Errorf("badge should clear once seen: %q", bar)
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
