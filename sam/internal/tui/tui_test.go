package tui

import (
	"os/exec"
	"strings"
	"testing"

	"charm.land/lipgloss/v2"

	"github.com/nicomaioli-skinandme/dotfiles/sam/internal/config"
	"github.com/nicomaioli-skinandme/dotfiles/sam/internal/issue"
	"github.com/nicomaioli-skinandme/dotfiles/sam/internal/pr"
)

// fakeSession is a SessionService stand-in that records calls instead of
// shelling out to tmux, so activation tests don't touch a real server.
type fakeSession struct {
	inTmux      bool
	ensureCalls []ensureCall
	switched    []string
}

type ensureCall struct {
	ws     *config.Workspace
	wsName string
	name   string
}

func (f *fakeSession) Name(wsName, branch string) string { return wsName + "-" + branch }
func (f *fakeSession) Has(string) bool                   { return true }
func (f *fakeSession) Current() (string, error)          { return "", nil }
func (f *fakeSession) InTmux() bool                      { return f.inTmux }

func (f *fakeSession) Ensure(ws *config.Workspace, wsName, name string) (string, error) {
	f.ensureCalls = append(f.ensureCalls, ensureCall{ws: ws, wsName: wsName, name: name})
	return f.Name(wsName, name), nil
}

func (f *fakeSession) AttachCmd(name string) *exec.Cmd {
	return exec.Command("tmux", "attach-session", "-t", name)
}

func (f *fakeSession) Switch(name string) error {
	f.switched = append(f.switched, name)
	return nil
}

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

func TestSwitchResourcePushesHistory(t *testing.T) {
	m := testModel(sampleItems()) // starts on ResWorktrees

	_ = m.switchResource(ResIssues)

	if len(m.stack) != 1 {
		t.Fatalf("stack depth: got %d, want 1", len(m.stack))
	}
	if got := m.stack[len(m.stack)-1].resource; got != ResWorktrees {
		t.Errorf("snapshot resource: got %v, want worktrees", got)
	}
}

func TestSwitchResourceSelfSwitchDoesNotPush(t *testing.T) {
	m := testModel(sampleItems()) // starts on ResWorktrees

	_ = m.switchResource(ResWorktrees)

	if len(m.stack) != 0 {
		t.Errorf("self-switch pushed history: stack depth %d, want 0", len(m.stack))
	}
}

func TestNavigationHistoryCapsAtFive(t *testing.T) {
	m := testModel(sampleItems())

	// Alternate resources so every switch is a real change and pushes.
	seq := []Resource{ResIssues, ResWorktrees, ResIssues, ResWorktrees, ResIssues, ResWorktrees, ResIssues}
	for _, r := range seq {
		_ = m.switchResource(r)
	}

	if len(m.stack) != maxStackDepth {
		t.Fatalf("stack depth: got %d, want %d", len(m.stack), maxStackDepth)
	}
}

func TestBackPopsAcrossResources(t *testing.T) {
	m := testModel(sampleItems())
	m.query = "fix"
	m.applyFilter()

	_ = m.switchResource(ResIssues)
	_ = m.switchResource(ResPRs)

	_, _ = m.back() // PRs -> Issues
	if m.resource != ResIssues {
		t.Fatalf("after first back: got %v, want issues", m.resource)
	}

	_, _ = m.back() // Issues -> Worktrees
	if m.resource != ResWorktrees {
		t.Fatalf("after second back: got %v, want worktrees", m.resource)
	}
	if m.query != "fix" {
		t.Errorf("query not restored: got %q, want %q", m.query, "fix")
	}
	if len(m.stack) != 0 {
		t.Errorf("stack not empty after walking back: %d", len(m.stack))
	}
}

func TestBackIsPopOnlyWithActiveFilter(t *testing.T) {
	m := testModel(sampleItems())
	_ = m.switchResource(ResIssues) // stack: [worktrees]
	m.query = "x"
	m.applyFilter()

	_, _ = m.back()

	// back pops to the previous resource; it does not merely clear the
	// filter and stay put.
	if m.resource != ResWorktrees {
		t.Errorf("back did not pop: resource %v, want worktrees", m.resource)
	}
	if len(m.stack) != 0 {
		t.Errorf("stack depth after back: %d, want 0", len(m.stack))
	}
}

func TestClearFilterKeyClearsWithoutNavigating(t *testing.T) {
	m := testModel(sampleItems())
	_ = m.switchResource(ResIssues) // stack: [worktrees], resource issues
	m.query = "foo"
	m.applyFilter()

	m.handleKey(runeKey('x'))

	if m.query != "" {
		t.Errorf("query not cleared: %q", m.query)
	}
	if m.resource != ResIssues {
		t.Errorf("clear filter navigated: resource %v, want issues", m.resource)
	}
	if len(m.stack) != 1 {
		t.Errorf("clear filter touched stack: depth %d, want 1", len(m.stack))
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
	if m.result != (Result{}) {
		t.Errorf("must not set a result before the flow completes, got %+v", m.result)
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
	if m.result != (Result{}) {
		t.Errorf("must not set a result before the flow completes, got %+v", m.result)
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

func TestActivateWorktreeEnsuresAndAttaches(t *testing.T) {
	ws := &config.Workspace{Trunk: "main", Repo: "/repo", Worktrees: "/wt"}
	fs := &fakeSession{}
	m := newModel("ws", ws, nil, ResWorktrees, config.Tui{}, Deps{SessionSvc: fs})
	m.items = []Item{{ID: "feat-x", Title: "feat-x", Type: WorktreeLinked}}
	m.applyFilter()

	_, cmd := m.activate()
	if !m.loading {
		t.Error("expected loading to be set while ensuring the session")
	}
	if cmd == nil {
		t.Fatal("expected a command from activating a worktree")
	}
	// The TUI attaches in place now — it must not record a result or quit.
	if m.result != (Result{}) {
		t.Errorf("worktree activation must not set a result; got %+v", m.result)
	}

	// Running the ensure command builds-if-missing and reports the session.
	msg := m.ensureSessionCmd(m.workspace, m.workspaceName, "feat-x")()
	ready, ok := msg.(attachReadyMsg)
	if !ok {
		t.Fatalf("expected attachReadyMsg, got %T", msg)
	}
	if ready.err != nil || ready.session != "ws-feat-x" {
		t.Fatalf("ensure: got session=%q err=%v", ready.session, ready.err)
	}
	if len(fs.ensureCalls) == 0 || fs.ensureCalls[len(fs.ensureCalls)-1].name != "feat-x" {
		t.Fatalf("Ensure not called for feat-x; calls=%+v", fs.ensureCalls)
	}
}

func TestActivateWorktreeAfterSwitchEnsuresNewWorkspace(t *testing.T) {
	wsA := config.Workspace{Trunk: "sam-tui-test-a-main", Repo: "/a", Worktrees: "/a.wt"}
	wsB := config.Workspace{Trunk: "sam-tui-test-b-main", Repo: "/b", Worktrees: "/b.wt"}
	all := map[string]config.Workspace{"a": wsA, "b": wsB}
	fs := &fakeSession{}
	m := newModel("a", &wsA, all, ResWorktrees, config.Tui{}, Deps{SessionSvc: fs})

	// Simulate the user invoking `:workspaces` and picking "b".
	if cmd := m.switchWorkspace("b"); cmd == nil {
		t.Fatal("switchWorkspace returned nil cmd")
	}
	if m.workspaceName != "b" || m.workspace.Repo != "/b" {
		t.Fatalf("after switch: name=%q repo=%q", m.workspaceName, m.workspace.Repo)
	}

	// Now pick the main worktree entry for the *switched-to* workspace and
	// run the ensure command it dispatches.
	m.items = []Item{{ID: wsB.Trunk, Title: wsB.Trunk, Type: WorktreeMain}}
	m.applyFilter()
	if _, cmd := m.activate(); cmd == nil {
		t.Fatal("expected a command from activating a worktree")
	}
	_ = m.ensureSessionCmd(m.workspace, m.workspaceName, wsB.Trunk)()

	last := fs.ensureCalls[len(fs.ensureCalls)-1]
	if last.wsName != "b" || last.ws == nil || last.ws.Repo != "/b" {
		t.Errorf("Ensure must use the switched-to workspace; got wsName=%q ws=%+v", last.wsName, last.ws)
	}
	if last.name != wsB.Trunk {
		t.Errorf("Ensure must target the switched-to worktree; got %q", last.name)
	}
}

func TestAttachInTmuxUsesSwitch(t *testing.T) {
	fs := &fakeSession{inTmux: true}
	m := newModel("ws", &config.Workspace{}, nil, ResWorktrees, config.Tui{}, Deps{SessionSvc: fs})
	m.loading = true

	_, cmd := m.attachToSession("ws-feat-x")
	if m.loading {
		t.Error("attachToSession must clear loading")
	}
	if cmd == nil {
		t.Fatal("expected a switch command inside tmux")
	}
	msg := cmd()
	if _, ok := msg.(attachedMsg); !ok {
		t.Fatalf("expected attachedMsg, got %T", msg)
	}
	if len(fs.switched) != 1 || fs.switched[0] != "ws-feat-x" {
		t.Fatalf("expected switch-client to ws-feat-x; got %+v", fs.switched)
	}
}

func TestAttachedReloadsAndKeepsCursor(t *testing.T) {
	fs := &fakeSession{}
	m := newModel("ws", &config.Workspace{Trunk: "main"}, nil, ResWorktrees, config.Tui{}, Deps{SessionSvc: fs})
	m.items = sampleItems()
	m.applyFilter()
	m.cursor = 2
	m.loading = true

	_, cmd := m.handleAttached(attachedMsg{})
	if m.loading {
		t.Error("handleAttached must clear loading")
	}
	if m.cursor != 2 {
		t.Errorf("cursor must be preserved across an attach; got %d", m.cursor)
	}
	if cmd == nil {
		t.Error("handleAttached must reload the current resource")
	}
}

func TestBackReloads(t *testing.T) {
	m := testModel(sampleItems()) // starts on ResWorktrees
	m.cursor = 2

	_ = m.switchResource(ResIssues) // stack: [worktrees]

	_, cmd := m.back()
	if m.resource != ResWorktrees {
		t.Fatalf("back did not pop: resource %v, want worktrees", m.resource)
	}
	if m.cursor != 2 {
		t.Errorf("back did not restore cursor from snapshot: got %d, want 2", m.cursor)
	}
	if cmd == nil {
		t.Error("back must reload the restored view")
	}
}

func TestReloadKeyReloads(t *testing.T) {
	m := testModel(sampleItems())
	m.cursor = 2

	_, cmd := m.handleKey(runeKey('R'))
	if cmd == nil {
		t.Error("R must issue a reload command")
	}
	if m.resource != ResWorktrees {
		t.Errorf("R changed resource: got %v, want worktrees", m.resource)
	}
	if len(m.stack) != 0 {
		t.Errorf("R touched the navigation stack: depth %d, want 0", len(m.stack))
	}
	if m.cursor != 2 {
		t.Errorf("R moved the cursor: got %d, want 2", m.cursor)
	}
}

func TestApplyLoadedPreservesCursor(t *testing.T) {
	m := testModel(sampleItems())
	m.cursor = 2

	// A reload returning a list still long enough keeps the highlight put.
	m.applyLoaded(itemsLoadedMsg{resource: ResWorktrees, items: sampleItems()})
	if m.cursor != 2 {
		t.Errorf("reload did not preserve cursor: got %d, want 2", m.cursor)
	}

	// A shorter list clamps the cursor into range.
	m.applyLoaded(itemsLoadedMsg{resource: ResWorktrees, items: []Item{{ID: "only", Title: "only"}}})
	if m.cursor != 0 {
		t.Errorf("reload did not clamp cursor to shorter list: got %d, want 0", m.cursor)
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

func TestAddNewOpensInputModal(t *testing.T) {
	m := testModel(sampleItems()) // ResWorktrees

	if _, cmd := m.addNew(); cmd != nil {
		t.Error("addNew should not return a command; it only opens the modal")
	}
	if m.modal.kind != modalInput {
		t.Fatalf("expected input modal, got kind %d", m.modal.kind)
	}
	if m.modal.title != "New branch name" {
		t.Errorf("modal title: got %q", m.modal.title)
	}

	// Not available from the branch picker (where `a`'s selection is in flight).
	m2 := testModel(sampleItems())
	m2.branchPick = true
	m2.addNew()
	if m2.modal.kind != modalNone {
		t.Error("addNew should be a no-op in the branch picker")
	}
}

func TestAddNewKeyRoutes(t *testing.T) {
	m := testModel(sampleItems())
	m.handleKey(runeKey('A'))
	if m.modal.kind != modalInput {
		t.Fatalf("`A` should open the new-branch input modal, got kind %d", m.modal.kind)
	}
}

func TestCreateBranchValidation(t *testing.T) {
	ws := &config.Workspace{Trunk: "main", MaxBranchLen: 8}
	cases := []struct {
		name     string
		in       string
		wantCmd  bool   // valid input returns a command and sets loading
		wantStat string // expected status substring on rejection
	}{
		{"empty", "  ", false, "required"},
		{"slash", "feat/x", false, "'/'"},
		{"space", "feat x", false, "spaces"},
		{"too long", "way-too-long-name", false, "too long"},
		{"valid", "scratch", true, ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			m := newModel("ws", ws, nil, ResWorktrees, config.Tui{}, Deps{})
			cmd := m.createBranchCmd(c.in)
			if c.wantCmd {
				if cmd == nil {
					t.Fatal("valid name should return a command")
				}
				if !m.loading {
					t.Error("valid name should set loading")
				}
				return
			}
			if cmd != nil {
				t.Errorf("invalid name should return nil command")
			}
			if m.loading {
				t.Error("invalid name should not set loading")
			}
			if !strings.Contains(m.status, c.wantStat) {
				t.Errorf("status %q should mention %q", m.status, c.wantStat)
			}
		})
	}
}

func TestWorktreeAddedPopGuard(t *testing.T) {
	// The branch-picker (`a`) flow pushed a view, so a successful add pops
	// back to the worktrees list.
	picker := testModel(sampleItems())
	picker.pushView()
	picker.branchPick = true
	picker.handleWorktreeAdded(worktreeAddedMsg{branch: "feat-x"})
	if len(picker.stack) != 0 {
		t.Errorf("branch-pick add should pop the pushed view; stack len %d", len(picker.stack))
	}

	// The modal-driven (`A`) flow never pushed a view, so it must not pop
	// some unrelated stacked view.
	modal := testModel(sampleItems())
	modal.pushView() // simulate an unrelated prior navigation
	modal.branchPick = false
	modal.handleWorktreeAdded(worktreeAddedMsg{branch: "feat-x"})
	if len(modal.stack) != 1 {
		t.Errorf("modal-driven add must not pop; stack len %d, want 1", len(modal.stack))
	}
}
