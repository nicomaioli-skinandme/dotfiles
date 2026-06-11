package tui

import (
	"errors"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/nicomaioli-skinandme/dotfiles/sam/internal/config"
	"github.com/nicomaioli-skinandme/dotfiles/sam/internal/ghx"
)

// fakeSetup is a SetupService stand-in that records the save and returns
// canned probe/fetch results, so form tests never touch git, gh, or disk.
type fakeSetup struct {
	trunk, slug string
	probeErr    error

	field    ghx.ProjectStatusField
	fetchErr error

	scopesErr     error
	checkedScopes []string

	saveErr   error
	savedName string
	savedWS   config.Workspace
}

func (f *fakeSetup) ProbeRepo(p string) (string, string, string, error) {
	if f.probeErr != nil {
		return "", "", "", f.probeErr
	}
	return p, f.trunk, f.slug, nil
}

func (f *fakeSetup) FetchProject(string, int) (string, ghx.ProjectStatusField, error) {
	if f.fetchErr != nil {
		return "", ghx.ProjectStatusField{}, f.fetchErr
	}
	return "PVT_1", f.field, nil
}

func (f *fakeSetup) CheckScopes(required []string) error {
	f.checkedScopes = required
	return f.scopesErr
}

func (f *fakeSetup) SaveWorkspace(name string, ws config.Workspace) (string, error) {
	if f.saveErr != nil {
		return "", f.saveErr
	}
	f.savedName = name
	f.savedWS = ws
	return "/tmp/config.toml", nil
}

func formModel(setup *fakeSetup) *model {
	m := newModel("", nil, map[string]config.Workspace{}, ResWorkspaces, config.Tui{}, Deps{Setup: setup})
	m.form = newAddForm(false)
	return m
}

func enterKey() tea.KeyPressMsg { return tea.KeyPressMsg{Code: tea.KeyEnter} }
func escKey() tea.KeyPressMsg   { return tea.KeyPressMsg{Code: tea.KeyEscape} }

// typeInto replaces the active input step's text.
func typeInto(m *model, s string) {
	m.form.active().input.SetValue(s)
	m.form.active().input.CursorEnd()
}

// drain executes a command tree (flattening batches) and feeds every
// non-tick message back into Update, mimicking the program loop enough to
// drive the form's async steps in tests.
func drain(t *testing.T, m *model, cmd tea.Cmd) {
	t.Helper()
	if cmd == nil {
		return
	}
	switch msg := cmd().(type) {
	case tea.BatchMsg:
		for _, c := range msg {
			drain(t, m, c)
		}
	case formRepoProbedMsg, formProjectFetchedMsg, formScopesCheckedMsg, formSavedMsg:
		_, next := m.Update(msg)
		drain(t, m, next)
	}
}

func TestFormHappyPathNoProject(t *testing.T) {
	setup := &fakeSetup{trunk: "main", slug: "acme/widgets"}
	m := formModel(setup)

	// Repo step: submit a path; the probe runs async and lands its result.
	typeInto(m, "/repo")
	_, cmd := m.handleKey(enterKey())
	if m.form.busy == "" {
		t.Fatal("expected busy while probing repo")
	}
	drain(t, m, cmd)
	if got := m.form.active().id; got != stepName {
		t.Fatalf("after probe expected name step, got %v", got)
	}
	if v := m.form.active().input.Value(); v != "repo" {
		t.Errorf("name default = %q, want base of repo path", v)
	}

	// Name → worktrees → trunk → branch_repo, accepting every default.
	m.handleKey(enterKey())
	if got := m.form.active(); got.id != stepWorktrees || got.input.Value() != "/repo.worktrees" {
		t.Fatalf("expected worktrees step with derived default, got %v %q", got.id, got.input.Value())
	}
	m.handleKey(enterKey())
	if got := m.form.active(); got.id != stepTrunk || got.input.Value() != "main" {
		t.Fatalf("expected trunk step defaulting to probed trunk, got %v %q", got.id, got.input.Value())
	}
	m.handleKey(enterKey())
	if got := m.form.active(); got.id != stepBranchRepo || got.input.Value() != "acme/widgets" {
		t.Fatalf("expected branch_repo step defaulting to origin slug, got %v %q", got.id, got.input.Value())
	}
	m.handleKey(enterKey())

	// Decline the GitHub Project, take no hook; the scope check fires.
	if m.form.active().id != stepProjectConfirm {
		t.Fatalf("expected project confirm, got %v", m.form.active().id)
	}
	_, _ = m.handleKey(tea.KeyPressMsg{Text: "n", Code: 'n'})
	if m.form.active().id != stepHook {
		t.Fatalf("expected hook step after No, got %v", m.form.active().id)
	}
	_, cmd = m.handleKey(enterKey()) // None
	if m.form.busy == "" {
		t.Fatal("expected busy while checking scopes")
	}
	drain(t, m, cmd)

	// Scopes ok → save ran through the fake → form closed, model updated.
	if m.form != nil {
		t.Fatalf("expected form closed after save, still on %v", m.form.active().id)
	}
	if len(setup.checkedScopes) != 1 || setup.checkedScopes[0] != "repo" {
		t.Errorf("checked scopes = %v, want [repo] without a project", setup.checkedScopes)
	}
	if setup.savedName != "repo" {
		t.Errorf("saved name = %q, want repo", setup.savedName)
	}
	if setup.savedWS.Repo != "/repo" || setup.savedWS.Trunk != "main" || setup.savedWS.Worktrees != "/repo.worktrees" {
		t.Errorf("saved workspace = %+v", setup.savedWS)
	}
	if setup.savedWS.MaxBranchLen == 0 {
		t.Error("saved workspace missing config.Default() overlay")
	}
	if _, ok := m.all["repo"]; !ok {
		t.Error("new workspace not added to the in-memory set")
	}
	if m.focusID != "repo" {
		t.Errorf("focusID = %q, want repo (cursor should land on the new row)", m.focusID)
	}
	if !strings.Contains(m.status, "wrote ") {
		t.Errorf("status = %q, want a wrote-path message", m.status)
	}
}

func TestFormProjectFlowWithFetchRetry(t *testing.T) {
	setup := &fakeSetup{
		trunk: "main", slug: "acme/widgets",
		fetchErr: errors.New("gh exploded"),
		field: ghx.ProjectStatusField{
			FieldID: "F1",
			Options: []ghx.ProjectStatusOption{
				{ID: "1", Name: "Todo"},
				{ID: "2", Name: "In Progress"},
				{ID: "3", Name: "Done"},
			},
		},
	}
	m := formModel(setup)

	typeInto(m, "/repo")
	_, cmd := m.handleKey(enterKey())
	drain(t, m, cmd)
	for _, step := range []formStepID{stepName, stepWorktrees, stepTrunk, stepBranchRepo} {
		if m.form.active().id != step {
			t.Fatalf("expected %v active, got %v", step, m.form.active().id)
		}
		m.handleKey(enterKey())
	}
	_, _ = m.handleKey(tea.KeyPressMsg{Text: "y", Code: 'y'}) // configure a project

	if m.form.active().id != stepProjectURL {
		t.Fatalf("expected project URL step, got %v", m.form.active().id)
	}
	typeInto(m, "https://github.com/orgs/acme/projects/7")
	_, cmd = m.handleKey(enterKey())
	drain(t, m, cmd)

	// Fetch failed: retryable banner, not a step error.
	if m.form.failure == "" {
		t.Fatal("expected a failure banner after the fetch error")
	}

	// r retries; this time the fake succeeds.
	setup.fetchErr = nil
	_, cmd = m.handleKey(tea.KeyPressMsg{Text: "r", Code: 'r'})
	drain(t, m, cmd)
	if m.form.failure != "" || m.form.active().id != stepInProgress {
		t.Fatalf("expected in-progress picker after retry, got failure=%q step=%v", m.form.failure, m.form.active().id)
	}

	// Pick "In Progress" (cursor 1) → backlog multi preselects Todo only.
	m.handleKey(tea.KeyPressMsg{Text: "j", Code: 'j'})
	m.handleKey(enterKey())
	s := m.form.active()
	if s.id != stepBacklog {
		t.Fatalf("expected backlog step, got %v", s.id)
	}
	if !s.checked["1"] || s.checked["2"] || s.checked["3"] {
		t.Errorf("backlog preselect = %v, want only Todo", s.checked)
	}
	m.handleKey(enterKey())

	if m.form.active().id != stepIssueRepos {
		t.Fatalf("expected issue_repos step, got %v", m.form.active().id)
	}
	m.handleKey(enterKey()) // default branch_repo
	_, cmd = m.handleKey(enterKey()) // hook: None → finish
	drain(t, m, cmd)

	if m.form != nil {
		t.Fatal("expected form closed after save")
	}
	if len(setup.checkedScopes) != 2 || setup.checkedScopes[1] != "project" {
		t.Errorf("checked scopes = %v, want [repo project]", setup.checkedScopes)
	}
	gp := setup.savedWS.GhProject
	if gp.Owner != "acme" || gp.Number != 7 || gp.ID != "PVT_1" || gp.StatusFieldID != "F1" || gp.InProgressID != "2" {
		t.Errorf("saved gh_project = %+v", gp)
	}
	if len(gp.BacklogStatuses) != 1 || gp.BacklogStatuses[0] != "Todo" {
		t.Errorf("backlog statuses = %v, want [Todo]", gp.BacklogStatuses)
	}
	if len(gp.IssueRepos) != 1 || gp.IssueRepos[0] != "acme/widgets" {
		t.Errorf("issue repos = %v, want [acme/widgets]", gp.IssueRepos)
	}
}

func TestFormDuplicateNameInlineError(t *testing.T) {
	setup := &fakeSetup{trunk: "main"}
	m := formModel(setup)
	m.all["taken"] = config.Workspace{}

	typeInto(m, "/repo")
	_, cmd := m.handleKey(enterKey())
	drain(t, m, cmd)

	typeInto(m, "taken")
	m.handleKey(enterKey())
	s := m.form.active()
	if s.id != stepName {
		t.Fatalf("expected to stay on the name step, got %v", s.id)
	}
	if s.errMsg == "" {
		t.Fatal("expected an inline duplicate-name error")
	}
}

func TestFormBadRepoInlineError(t *testing.T) {
	setup := &fakeSetup{probeErr: errors.New("/nope is not a git repository")}
	m := formModel(setup)

	typeInto(m, "/nope")
	_, cmd := m.handleKey(enterKey())
	drain(t, m, cmd)

	s := m.form.active()
	if s.id != stepRepo || s.summary != "" {
		t.Fatalf("expected repo step re-active, got %v summary=%q", s.id, s.summary)
	}
	if s.errMsg == "" {
		t.Fatal("expected an inline probe error")
	}
	if m.form.busy != "" {
		t.Errorf("busy = %q, want cleared", m.form.busy)
	}
}

func TestFormEscStepsBackThenCancels(t *testing.T) {
	setup := &fakeSetup{trunk: "main"}
	m := formModel(setup)

	typeInto(m, "/repo")
	_, cmd := m.handleKey(enterKey())
	drain(t, m, cmd)
	if m.form.active().id != stepName {
		t.Fatal("expected name step")
	}

	// Esc re-activates the repo step (summary cleared, value kept).
	m.handleKey(escKey())
	s := m.form.active()
	if s.id != stepRepo || s.summary != "" {
		t.Fatalf("expected repo step re-active, got %v summary=%q", s.id, s.summary)
	}
	if s.input.Value() != "/repo" {
		t.Errorf("repo input lost its value: %q", s.input.Value())
	}

	// Esc on the first step closes the form without quitting.
	_, quitCmd := m.handleKey(escKey())
	if m.form != nil {
		t.Fatal("expected form closed")
	}
	if quitCmd != nil {
		t.Fatal("non-first-run cancel must not quit the TUI")
	}
}

func TestFormFirstRunEscQuits(t *testing.T) {
	m := formModel(&fakeSetup{})
	m.form = newAddForm(true)

	_, cmd := m.handleKey(escKey())
	if cmd == nil {
		t.Fatal("expected a quit command")
	}
	if _, ok := cmd().(tea.QuitMsg); !ok {
		t.Fatalf("expected tea.QuitMsg, got %T", cmd())
	}
}

func TestFormRenders(t *testing.T) {
	setup := &fakeSetup{trunk: "main"}
	m := formModel(setup)
	m.width, m.height = 80, 24

	typeInto(m, "/repo")
	_, cmd := m.handleKey(enterKey())
	drain(t, m, cmd)

	frame := m.View().Content
	if !strings.Contains(frame, "Add workspace") {
		t.Error("frame missing the form header")
	}
	if !strings.Contains(frame, "Repo path") || !strings.Contains(frame, "/repo") {
		t.Error("frame missing the answered repo step summary")
	}
	if !strings.Contains(frame, "Workspace name") {
		t.Error("frame missing the active step title")
	}
	if !strings.Contains(frame, "workspaces · add") {
		t.Error("status bar missing the form scope")
	}
}

func TestFormSaveFailureIsRetryable(t *testing.T) {
	setup := &fakeSetup{trunk: "main", saveErr: errors.New("disk full")}
	m := formModel(setup)

	typeInto(m, "/repo")
	_, cmd := m.handleKey(enterKey())
	drain(t, m, cmd)
	for m.form.active().id != stepProjectConfirm {
		m.handleKey(enterKey())
	}
	m.handleKey(tea.KeyPressMsg{Text: "n", Code: 'n'})
	_, cmd = m.handleKey(enterKey()) // hook: None → scopes → save (fails)
	drain(t, m, cmd)

	if m.form == nil || m.form.failure == "" {
		t.Fatal("expected a retryable save failure banner")
	}
	setup.saveErr = nil
	_, cmd = m.handleKey(tea.KeyPressMsg{Text: "r", Code: 'r'})
	drain(t, m, cmd)
	if m.form != nil {
		t.Fatal("expected form closed after successful retry")
	}
	if setup.savedName != "repo" {
		t.Errorf("saved name = %q, want repo", setup.savedName)
	}
}
