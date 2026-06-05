package tui

import (
	"strings"
	"testing"

	"charm.land/lipgloss/v2"

	"github.com/nicomaioli-skinandme/dotfiles/sam/internal/config"
	"github.com/nicomaioli-skinandme/dotfiles/sam/internal/issue"
	"github.com/nicomaioli-skinandme/dotfiles/sam/internal/pr"
)

func TestParseCommand(t *testing.T) {
	cases := []struct {
		in   string
		want command
	}{
		{":q", command{kind: cmdQuit}},
		{":quit", command{kind: cmdQuit}},
		{"q", command{kind: cmdQuit}},
		{":", command{kind: cmdNone}},
		{"   ", command{kind: cmdNone}},
		{":workspaces", command{kind: cmdResource, resource: ResWorkspaces}},
		{"worktrees", command{kind: cmdResource, resource: ResWorktrees}},
		{":issues", command{kind: cmdResource, resource: ResIssues}},
		{":prs", command{kind: cmdResource, resource: ResPRs}},
		{" :clankers ", command{kind: cmdResource, resource: ResClankers}},
		{":bogus", command{kind: cmdUnknown}},
	}
	for _, c := range cases {
		got := parseCommand(c.in)
		if got != c.want {
			t.Errorf("parseCommand(%q) = %+v, want %+v", c.in, got, c.want)
		}
	}
}

// testModel builds a model with a fixed item set for state tests.
func testModel(items []Item) *model {
	m := newModel("ws", &config.Workspace{Trunk: "main"}, nil, ResWorktrees, config.Tui{}, Deps{})
	m.items = items
	m.applyFilter()
	return m
}

func sampleItems() []Item {
	return []Item{
		{ID: "release", Title: "release", Active: true},
		{ID: "main", Title: "main", Detail: "main worktree", Type: WorktreeMain},
		{ID: "feat-login", Title: "feat-login"},
		{ID: "fix-crash", Title: "fix-crash", Active: true},
	}
}

func TestApplyFilter(t *testing.T) {
	m := testModel(sampleItems())

	if len(m.filtered) != 4 {
		t.Fatalf("empty query: got %d filtered, want 4", len(m.filtered))
	}

	m.query = "fe"
	m.applyFilter()
	if len(m.filtered) != 1 || m.filtered[0].ID != "feat-login" {
		t.Fatalf("query %q: got %+v", m.query, m.filtered)
	}

	// Detail is searched too.
	m.query = "main worktree"
	m.applyFilter()
	if len(m.filtered) != 1 || m.filtered[0].ID != "main" {
		t.Fatalf("detail match: got %+v", m.filtered)
	}

	// Filtering clamps the cursor into range.
	m.cursor = 3
	m.query = "fix"
	m.applyFilter()
	if m.cursor != 0 {
		t.Fatalf("cursor not clamped: got %d", m.cursor)
	}
}

func TestMoveCursor(t *testing.T) {
	m := testModel(sampleItems())

	m.moveCursor(-1)
	if m.cursor != 0 {
		t.Errorf("up at top: got %d, want 0", m.cursor)
	}
	m.moveCursor(2)
	if m.cursor != 2 {
		t.Errorf("down by 2: got %d, want 2", m.cursor)
	}
	m.moveCursor(100)
	if m.cursor != 3 {
		t.Errorf("clamp at bottom: got %d, want 3", m.cursor)
	}

	// Empty list pins the cursor at 0.
	empty := testModel(nil)
	empty.moveCursor(1)
	if empty.cursor != 0 {
		t.Errorf("empty list cursor: got %d, want 0", empty.cursor)
	}
}

func TestToggleSelect(t *testing.T) {
	m := testModel(sampleItems())
	m.cursor = 2
	m.toggleSelect()
	if !m.selected["feat-login"] {
		t.Fatal("expected feat-login selected after toggle")
	}
	m.toggleSelect()
	if m.selected["feat-login"] {
		t.Fatal("expected feat-login deselected after second toggle")
	}
}

func TestSwitchResourceResetsState(t *testing.T) {
	m := testModel(sampleItems())
	m.cursor = 3
	m.query = "fix"
	m.applyFilter()
	m.selected["fix-crash"] = true

	_ = m.switchResource(ResWorkspaces) // returned cmd loads lazily; ignore here

	if m.resource != ResWorkspaces {
		t.Errorf("resource: got %v, want workspaces", m.resource)
	}
	if m.query != "" {
		t.Errorf("query not reset: %q", m.query)
	}
	if m.cursor != 0 {
		t.Errorf("cursor not reset: %d", m.cursor)
	}
	if m.branchPick {
		t.Error("branchPick should be false after switch")
	}
}

func TestActivateIssueStartsFlow(t *testing.T) {
	m := newModel("ws", &config.Workspace{}, nil, ResIssues, config.Tui{}, Deps{})
	m.resource = ResIssues
	m.items = []Item{{ID: "owner/repo#42", Title: "#42 thing"}}
	m.issues = map[string]issue.Issue{
		"owner/repo#42": {Number: 42, Repository: "owner/repo", Title: "thing"},
	}
	m.applyFilter()

	// Activating an issue kicks off the async prepare step (resolve user +
	// plan branch) behind the loading spinner; it must not quit yet.
	_, cmd := m.activate()
	if cmd == nil {
		t.Fatal("expected a prepare command from activating an issue")
	}
	if !m.loading {
		t.Error("expected loading to be set while preparing")
	}
	if m.result.Attach != "" {
		t.Errorf("must not set a result before the flow completes, got %q", m.result.Attach)
	}
}

func TestActivatePRStartsFlow(t *testing.T) {
	m := newModel("ws", &config.Workspace{}, nil, ResPRs, config.Tui{}, Deps{})
	m.resource = ResPRs
	m.items = []Item{{ID: "owner/repo#7", Title: "#7 thing"}}
	m.prs = map[string]pr.PR{
		"owner/repo#7": {Number: 7, Repository: "owner/repo", Title: "thing", HeadRefName: "feat-x"},
	}
	m.applyFilter()

	// Activating a PR kicks off the async bootstrap behind the spinner; it
	// must not set a result or quit yet (no modals in the PR flow).
	_, cmd := m.activate()
	if cmd == nil {
		t.Fatal("expected a bootstrap command from activating a PR")
	}
	if !m.loading {
		t.Error("expected loading to be set while bootstrapping")
	}
	if m.result.Attach != "" {
		t.Errorf("must not set a result before the flow completes, got %q", m.result.Attach)
	}
}

func TestBranchEditModalRenders(t *testing.T) {
	m := newModel("ws", &config.Workspace{MaxBranchLen: 5}, nil, ResIssues, config.Tui{}, Deps{})
	m.pending = &fromIssueState{branch: "1-really-long-branch"}

	// Branch exceeds the limit, so the edit modal opens without applying.
	m.fromIssueBranchStep(false)
	if m.modal.kind != modalInput {
		t.Fatalf("expected input modal, got kind %d", m.modal.kind)
	}
	out := m.renderModal()
	if !strings.Contains(out, "1-really-long-branch") {
		t.Errorf("input modal should show the editable branch; got:\n%s", out)
	}
}

func TestOneLine(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"single", "single"},
		{"HTTP 401: Bad credentials\nTry authenticating with: gh auth login",
			"HTTP 401: Bad credentials Try authenticating with: gh auth login"},
		{"a\tb\n\nc   d", "a b c d"},
		{"", ""},
	}
	for _, c := range cases {
		if got := oneLine(c.in); got != c.want {
			t.Errorf("oneLine(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

// A multiline status (e.g. a multiline gh 401) must not spill the status bar
// past one row or wider than the screen — that overflow is what froze the TUI.
func TestStatusBarStaysOneLine(t *testing.T) {
	m := testModel(sampleItems())
	m.width, m.height = 80, 24
	m.status = "HTTP 401: Bad credentials\nTry authenticating with: gh auth login"

	bar := m.renderStatusBar()
	if strings.Contains(bar, "\n") {
		t.Errorf("status bar must be a single line; got:\n%q", bar)
	}
	if w := lipgloss.Width(bar); w > m.width {
		t.Errorf("status bar width %d exceeds screen width %d", w, m.width)
	}
}

func TestFromIssuePreparedPromptsReassign(t *testing.T) {
	m := newModel("ws", &config.Workspace{}, nil, ResIssues, config.Tui{}, Deps{})
	m.handleFromIssuePrepared(fromIssuePreparedMsg{
		iss:    issue.Issue{Number: 1, Assignees: []string{"someone-else"}},
		me:     "me",
		branch: "1-x",
	})
	if m.modal.kind != modalConfirm {
		t.Fatalf("expected a reassign confirm modal, got kind %d", m.modal.kind)
	}
	if m.pending == nil || m.pending.branch != "1-x" {
		t.Fatalf("expected pending state retained, got %+v", m.pending)
	}
}

func TestActivateWorktreeRecordsAttach(t *testing.T) {
	ws := &config.Workspace{Trunk: "main", Repo: "/repo", Worktrees: "/wt"}
	m := newModel("ws", ws, nil, ResWorktrees, config.Tui{}, Deps{})
	m.items = []Item{{ID: "feat-x", Title: "feat-x", Type: WorktreeLinked}}
	m.applyFilter()

	m.activate()
	// Result carries the worktree name; the caller builds-if-missing and
	// attaches via the session Controller after the TUI exits.
	if m.result.Attach != "feat-x" {
		t.Fatalf("attach: got %q, want %q", m.result.Attach, "feat-x")
	}
	if m.result.AttachSession != "" {
		t.Errorf("AttachSession must be empty for a worktree activation; got %q", m.result.AttachSession)
	}
	if m.result.Workspace != ws || m.result.WorkspaceName != "ws" {
		t.Errorf("result must carry the active workspace; got %v / %q",
			m.result.Workspace, m.result.WorkspaceName)
	}
}

func TestActivateWorktreeAfterSwitchCarriesNewWorkspace(t *testing.T) {
	wsA := config.Workspace{Trunk: "sam-tui-test-a-main", Repo: "/a", Worktrees: "/a.wt"}
	wsB := config.Workspace{Trunk: "sam-tui-test-b-main", Repo: "/b", Worktrees: "/b.wt"}
	all := map[string]config.Workspace{"a": wsA, "b": wsB}
	m := newModel("a", &wsA, all, ResWorktrees, config.Tui{}, Deps{})

	// Simulate the user invoking `:workspaces` and picking "b".
	if cmd := m.switchWorkspace("b"); cmd == nil {
		t.Fatal("switchWorkspace returned nil cmd")
	}
	if m.workspaceName != "b" || m.workspace.Repo != "/b" {
		t.Fatalf("after switch: name=%q repo=%q", m.workspaceName, m.workspace.Repo)
	}

	// Now pick the main worktree entry for the *switched-to* workspace.
	m.items = []Item{{ID: wsB.Trunk, Title: wsB.Trunk, Type: WorktreeMain}}
	m.applyFilter()
	m.activate()

	if m.result.WorkspaceName != "b" {
		t.Errorf("result must carry the switched-to workspace name; got %q", m.result.WorkspaceName)
	}
	if m.result.Workspace == nil || m.result.Workspace.Repo != "/b" {
		t.Errorf("result must carry the switched-to workspace pointer; got %v", m.result.Workspace)
	}
	if m.result.Attach != wsB.Trunk {
		t.Errorf("result must attach to the switched-to worktree; got %q", m.result.Attach)
	}
}

func TestDeleteGuardsMainWorktree(t *testing.T) {
	m := newModel("ws", &config.Workspace{Trunk: "main", Worktrees: "/wt", Repo: "/repo"}, nil, ResWorktrees, config.Tui{}, Deps{})
	m.items = []Item{{ID: "main", Title: "main", Type: WorktreeMain}}
	m.applyFilter()

	m.cursor = 0
	m.del()
	if m.modal.kind != modalNone {
		t.Error("delete on main worktree should not open a modal")
	}
}
