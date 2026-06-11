package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"

	"github.com/nicomaioli-skinandme/dotfiles/sam/internal/config"
	"github.com/nicomaioli-skinandme/dotfiles/sam/internal/ghx"
	"github.com/nicomaioli-skinandme/dotfiles/sam/internal/wizard"
)

// The add-workspace form is a transient body state (like the branch-pick
// sub-view): a non-nil model.form gates rendering and key routing while
// m.resource stays ResWorkspaces underneath. It replays the old standalone
// wizard's flow one step at a time — answered steps collapse to summary
// lines, the active step is an editable field, and the I/O between steps
// (repo probing, GitHub Project lookup, scope check, save) runs as
// commands behind the spinner. Nothing is written until the final save.

// formStepID names each step of the add-workspace flow.
type formStepID int

const (
	stepRepo           formStepID = iota // repo path input → async probe
	stepName                             // workspace name (config key)
	stepWorktrees                        // worktrees dir input
	stepTrunk                            // trunk branch input
	stepBranchRepo                       // GitHub owner/name input
	stepProjectConfirm                   // "Configure a GitHub Project?"
	stepProjectURL                       // project URL input → async fetch
	stepInProgress                       // which status means In Progress
	stepBacklog                          // which statuses count as backlog
	stepIssueRepos                       // comma-separated issue repos
	stepHook                             // worktree_setup hook kind
	stepHookValue                        // hook command / script path
)

// formFieldKind is the input control a step renders.
type formFieldKind int

const (
	fieldInput   formFieldKind = iota // single-line text entry
	fieldConfirm                      // yes/no buttons
	fieldSelect                       // pick one option
	fieldMulti                        // toggle any number of options
)

type formOption struct{ value, label string }

// formStep is one materialized step. Steps are appended as the flow
// advances (the graph is dynamic — the project sub-steps only exist after
// a Yes), so the last element is always the active step; the rest are
// answered and render as their summary lines.
type formStep struct {
	id      formStepID
	kind    formFieldKind
	title   string
	desc    string          // faint hint under the title
	input   textinput.Model // fieldInput
	yes     bool            // fieldConfirm highlight
	options []formOption    // fieldSelect / fieldMulti
	cursor  int             // option highlight
	checked map[string]bool // fieldMulti, keyed by option value
	summary string          // display value once answered ("" = active)
	errMsg  string          // inline validation error
}

// formAnswers accumulates the typed answers across steps; composeWorkspace
// turns them into the config entry on save.
type formAnswers struct {
	repo, name, worktrees, trunk, branchRepo string

	project       bool
	projectOwner  string
	projectNumber int
	projectID     string
	field         ghx.ProjectStatusField
	inProgressID  string
	backlogNames  []string
	issueRepos    []string

	hookKind      string // "none" | "command" | "script"
	worktreeSetup string
}

type formState struct {
	steps   []formStep
	answers formAnswers

	busy       string  // non-empty: async work in flight; shown next to the spinner
	failure    string  // retryable failure banner (fetch / scopes / save)
	retry      tea.Cmd // re-issues the failed command
	retryLabel string  // busy label to restore on retry

	firstRun bool // Esc on the first step quits the TUI instead of closing the form
}

func (f *formState) push(s formStep) { f.steps = append(f.steps, s) }

func (f *formState) active() *formStep { return &f.steps[len(f.steps)-1] }

// reopenLast re-activates the last step (after Esc-back or a failed async
// submit) by clearing its answered summary and refocusing its input.
func (f *formState) reopenLast() {
	s := f.active()
	s.summary = ""
	if s.kind == fieldInput {
		s.input.Focus()
	}
}

func (f *formState) fail(err error) {
	f.busy = ""
	f.failure = err.Error()
}

// Async step results. Each command closes over deps.Setup (never the
// model), per the pattern of the other action commands.
type formRepoProbedMsg struct {
	repo, trunk, originSlug string
	err                     error
}

type formProjectFetchedMsg struct {
	projectID string
	field     ghx.ProjectStatusField
	err       error
}

type formScopesCheckedMsg struct{ err error }

type formSavedMsg struct {
	name string
	ws   config.Workspace
	path string
	err  error
}

// openAddForm enters the add-workspace form (the `a` action on the
// workspaces view).
func (m *model) openAddForm() (tea.Model, tea.Cmd) {
	m.status = ""
	m.form = newAddForm(false)
	return m, nil
}

// newAddForm builds the form with its first step (repo path) active.
// firstRun marks the no-config-yet launch, where cancelling quits the TUI.
func newAddForm(firstRun bool) *formState {
	cwd, _ := os.Getwd()
	f := &formState{firstRun: firstRun}
	f.push(inputStep(stepRepo, "Repo path", "path to the git repository", cwd))
	return f
}

func inputStep(id formStepID, title, desc, def string) formStep {
	ti := textinput.New()
	ti.SetVirtualCursor(true)
	ti.SetValue(def)
	ti.CursorEnd()
	ti.Focus()
	return formStep{id: id, kind: fieldInput, title: title, desc: desc, input: ti}
}

func confirmStep(id formStepID, title string) formStep {
	return formStep{id: id, kind: fieldConfirm, title: title}
}

func selectStep(id formStepID, title string, opts []formOption) formStep {
	return formStep{id: id, kind: fieldSelect, title: title, options: opts}
}

func multiStep(id formStepID, title string, opts []formOption, preselected []string) formStep {
	checked := make(map[string]bool, len(preselected))
	for _, v := range preselected {
		checked[v] = true
	}
	return formStep{id: id, kind: fieldMulti, title: title, options: opts, checked: checked}
}

// handleFormKey routes keys while the add-workspace form is open. Search
// and command modes are unreachable by construction — on text steps every
// key flows into the input, elsewhere unknown keys are no-ops.
func (m *model) handleFormKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	f := m.form
	key := msg.String()

	// Async work in flight: ignore everything (ctrl+c quits upstream).
	if f.busy != "" {
		return m, nil
	}

	// A retryable failure (project fetch, scope check, save) owns the keys.
	if f.failure != "" {
		switch key {
		case "r":
			f.failure = ""
			f.busy = f.retryLabel
			return m, tea.Batch(m.spinner.Tick, f.retry)
		case "esc":
			f.failure = ""
			f.retry = nil
			f.reopenLast()
		}
		return m, nil
	}

	s := f.active()
	switch s.kind {
	case fieldInput:
		switch key {
		case "enter":
			return m.formSubmit()
		case "esc":
			return m.formBack()
		}
		s.errMsg = "" // typing again clears a stale validation error
		var c tea.Cmd
		s.input, c = s.input.Update(msg)
		return m, c

	case fieldConfirm:
		switch key {
		case "left", "right", "h", "l", "tab":
			s.yes = !s.yes
		case "y", "Y":
			s.yes = true
			return m.formSubmit()
		case "n", "N":
			s.yes = false
			return m.formSubmit()
		case "enter":
			return m.formSubmit()
		case "esc":
			return m.formBack()
		}

	case fieldSelect, fieldMulti:
		switch key {
		case "j", "down":
			if s.cursor < len(s.options)-1 {
				s.cursor++
			}
		case "k", "up":
			if s.cursor > 0 {
				s.cursor--
			}
		case " ", "space", "tab":
			if s.kind == fieldMulti {
				v := s.options[s.cursor].value
				s.checked[v] = !s.checked[v]
			}
		case "enter":
			return m.formSubmit()
		case "esc":
			return m.formBack()
		}
	}
	return m, nil
}

// formBack steps back one step (Esc). On the first step it cancels the
// form: back to the workspaces list, or — on first run, when there is
// nothing to go back to — out of the TUI entirely. Nothing has been
// written in either case.
func (m *model) formBack() (tea.Model, tea.Cmd) {
	f := m.form
	if len(f.steps) <= 1 {
		if f.firstRun {
			return m, tea.Quit
		}
		m.form = nil
		return m, nil
	}
	f.steps = f.steps[:len(f.steps)-1]
	f.reopenLast()
	return m, nil
}

// formStartBusy marks the form busy with label and runs cmd behind the
// spinner, remembering it for `r` (retry) should it fail.
func (m *model) formStartBusy(label string, cmd tea.Cmd) tea.Cmd {
	m.form.busy = label
	m.form.retry = cmd
	m.form.retryLabel = label
	return tea.Batch(m.spinner.Tick, cmd)
}

// formSubmit validates and records the active step's answer, then either
// materializes the next step or fires the async work the answer gates.
func (m *model) formSubmit() (tea.Model, tea.Cmd) {
	f := m.form
	s := f.active()

	switch s.id {
	case stepRepo:
		v := strings.TrimSpace(s.input.Value())
		if v == "" {
			s.errMsg = "repo path required"
			return m, nil
		}
		s.summary = v
		return m, m.formStartBusy("inspecting repo…", m.probeRepoCmd(v))

	case stepName:
		v := strings.TrimSpace(s.input.Value())
		if v == "" {
			s.errMsg = "workspace name cannot be empty"
			return m, nil
		}
		if _, exists := m.all[v]; exists {
			s.errMsg = fmt.Sprintf("workspace %q already exists", v)
			return m, nil
		}
		f.answers.name = v
		s.summary = v
		f.push(inputStep(stepWorktrees, "Worktrees path", "where new worktrees will be created", f.answers.repo+".worktrees"))

	case stepWorktrees:
		v := strings.TrimSpace(s.input.Value())
		if v == "" {
			s.errMsg = "worktrees path required"
			return m, nil
		}
		expanded, err := wizard.ExpandAbs(v)
		if err != nil {
			s.errMsg = err.Error()
			return m, nil
		}
		f.answers.worktrees = expanded
		s.summary = expanded
		f.push(inputStep(stepTrunk, "Trunk", "detected from origin/HEAD", f.answers.trunk))

	case stepTrunk:
		v := strings.TrimSpace(s.input.Value())
		if v == "" {
			s.errMsg = "trunk cannot be empty"
			return m, nil
		}
		f.answers.trunk = v
		s.summary = v
		f.push(inputStep(stepBranchRepo, "branch_repo", "GitHub owner/name where issue branches live", f.answers.branchRepo))

	case stepBranchRepo:
		v := strings.TrimSpace(s.input.Value())
		f.answers.branchRepo = v
		s.summary = displayOr(v, "—")
		f.push(confirmStep(stepProjectConfirm, "Configure a GitHub Project?"))

	case stepProjectConfirm:
		f.answers.project = s.yes
		if s.yes {
			s.summary = "Yes"
			f.push(inputStep(stepProjectURL, "GitHub Project URL", "e.g. https://github.com/orgs/<org>/projects/<n>", ""))
		} else {
			s.summary = "No"
			f.push(hookStep())
		}

	case stepProjectURL:
		v := strings.TrimSpace(s.input.Value())
		owner, number, err := wizard.ParseProjectURL(v)
		if err != nil {
			s.errMsg = err.Error()
			return m, nil
		}
		f.answers.projectOwner = owner
		f.answers.projectNumber = number
		s.summary = v
		return m, m.formStartBusy("fetching GitHub Project…", m.fetchProjectCmd(owner, number))

	case stepInProgress:
		o := s.options[s.cursor]
		f.answers.inProgressID = o.value
		s.summary = o.label
		f.push(multiStep(stepBacklog, "Which statuses count as 'backlog'?",
			s.options, wizard.BacklogPreselect(f.answers.field.Options, o.value)))

	case stepBacklog:
		names := make([]string, 0, len(s.options))
		for _, o := range s.options {
			if s.checked[o.value] {
				names = append(names, o.label)
			}
		}
		f.answers.backlogNames = names
		s.summary = displayOr(strings.Join(names, ", "), "—")
		f.push(inputStep(stepIssueRepos, "issue_repos",
			"comma-separated GitHub owner/name list; issues from these repos appear in from-issue",
			f.answers.branchRepo))

	case stepIssueRepos:
		repos := wizard.SplitComma(s.input.Value())
		if len(repos) == 0 && f.answers.branchRepo != "" {
			repos = []string{f.answers.branchRepo}
		}
		f.answers.issueRepos = repos
		s.summary = displayOr(strings.Join(repos, ", "), "—")
		f.push(hookStep())

	case stepHook:
		o := s.options[s.cursor]
		f.answers.hookKind = o.value
		s.summary = o.label
		switch o.value {
		case "command":
			f.push(inputStep(stepHookValue, "Shell command",
				"runs in the new worktree dir; env: SAM_BRANCH, SAM_WORKTREE, SAM_REPO, SAM_WORKSPACE, SAM_ISSUE_NUMBER", ""))
		case "script":
			f.push(inputStep(stepHookValue, "Script path", "absolute or relative to the worktree dir", ""))
		default:
			return m, m.formFinish()
		}

	case stepHookValue:
		v := strings.TrimSpace(s.input.Value())
		f.answers.worktreeSetup = v
		s.summary = displayOr(v, "—")
		return m, m.formFinish()
	}
	return m, nil
}

func hookStep() formStep {
	return selectStep(stepHook, "Post-worktree setup hook", []formOption{
		{value: "none", label: "None"},
		{value: "command", label: "Shell command (run via sh -c)"},
		{value: "script", label: "Path to script (also run via sh -c)"},
	})
}

// displayOr substitutes alt for an empty value in a summary line.
func displayOr(v, alt string) string {
	if v == "" {
		return alt
	}
	return v
}

// formFinish runs the closing I/O: validate gh scopes, then (from the
// scope handler) compose and save the workspace.
func (m *model) formFinish() tea.Cmd {
	scopes := []string{"repo"}
	if m.form.answers.project {
		scopes = append(scopes, "project")
	}
	return m.formStartBusy("checking gh scopes…", m.checkScopesCmd(scopes))
}

func (m *model) probeRepoCmd(path string) tea.Cmd {
	svc := m.deps.Setup
	return func() tea.Msg {
		repo, trunk, slug, err := svc.ProbeRepo(path)
		return formRepoProbedMsg{repo: repo, trunk: trunk, originSlug: slug, err: err}
	}
}

func (m *model) fetchProjectCmd(owner string, number int) tea.Cmd {
	svc := m.deps.Setup
	return func() tea.Msg {
		id, fld, err := svc.FetchProject(owner, number)
		return formProjectFetchedMsg{projectID: id, field: fld, err: err}
	}
}

func (m *model) checkScopesCmd(scopes []string) tea.Cmd {
	svc := m.deps.Setup
	return func() tea.Msg {
		return formScopesCheckedMsg{err: svc.CheckScopes(scopes)}
	}
}

func (m *model) saveWorkspaceCmd(name string, ws config.Workspace) tea.Cmd {
	svc := m.deps.Setup
	return func() tea.Msg {
		path, err := svc.SaveWorkspace(name, ws)
		return formSavedMsg{name: name, ws: ws, path: path, err: err}
	}
}

// handleFormRepoProbed lands the repo probe: a bad path is an inline
// validation error on the (still active) repo step; success records the
// probed defaults and advances to the name step.
func (m *model) handleFormRepoProbed(msg formRepoProbedMsg) (tea.Model, tea.Cmd) {
	f := m.form
	if f == nil {
		return m, nil
	}
	f.busy = ""
	s := f.active()
	if msg.err != nil {
		s.summary = ""
		s.errMsg = msg.err.Error()
		return m, nil
	}
	f.answers.repo = msg.repo
	f.answers.trunk = msg.trunk
	f.answers.branchRepo = msg.originSlug
	s.summary = msg.repo // show the expanded path, not what was typed
	f.push(inputStep(stepName, "Workspace name", "key under [workspaces.<name>]", filepath.Base(msg.repo)))
	return m, nil
}

// handleFormProjectFetched lands the GitHub Project lookup: failure is
// retryable (network), success materializes the in-progress picker from
// the fetched Status options.
func (m *model) handleFormProjectFetched(msg formProjectFetchedMsg) (tea.Model, tea.Cmd) {
	f := m.form
	if f == nil {
		return m, nil
	}
	if msg.err != nil {
		m.log.Error("fetch gh project", "err", msg.err)
		f.fail(msg.err)
		return m, nil
	}
	f.busy = ""
	f.answers.projectID = msg.projectID
	f.answers.field = msg.field
	opts := make([]formOption, len(msg.field.Options))
	for i, o := range msg.field.Options {
		opts[i] = formOption{value: o.ID, label: o.Name}
	}
	f.push(selectStep(stepInProgress, "Which status means 'In Progress'?", opts))
	return m, nil
}

// handleFormScopesChecked lands the gh scope validation; on success it
// composes the workspace and fires the save.
func (m *model) handleFormScopesChecked(msg formScopesCheckedMsg) (tea.Model, tea.Cmd) {
	f := m.form
	if f == nil {
		return m, nil
	}
	if msg.err != nil {
		m.log.Error("check gh scopes", "err", msg.err)
		f.fail(msg.err) // ErrMissingScopes carries the `gh auth refresh` remedy
		return m, nil
	}
	f.busy = ""
	return m, m.formStartBusy("saving…", m.saveWorkspaceCmd(f.answers.name, composeWorkspace(f.answers)))
}

// handleFormSaved closes the form on success: the new workspace joins the
// in-memory set, the list reloads, and the cursor lands on the new row.
func (m *model) handleFormSaved(msg formSavedMsg) (tea.Model, tea.Cmd) {
	f := m.form
	if f == nil {
		return m, nil
	}
	if msg.err != nil {
		m.log.Error("save workspace", "err", msg.err)
		f.fail(msg.err)
		return m, nil
	}
	m.form = nil
	m.all[msg.name] = msg.ws
	m.focusID = msg.name
	m.log.Info("added workspace " + msg.name)
	m.status = "wrote " + msg.path
	return m, m.loadResource()
}

// composeWorkspace overlays the form's answers on the wizard defaults
// (tmux layout, max_branch_len, repo window).
func composeWorkspace(a formAnswers) config.Workspace {
	ws := config.Default()
	ws.Repo = a.repo
	ws.Worktrees = a.worktrees
	ws.Trunk = a.trunk
	ws.BranchRepo = a.branchRepo
	ws.WorktreeSetup = a.worktreeSetup
	if a.project {
		ws.GhProject = config.GhProject{
			Owner:           a.projectOwner,
			Number:          a.projectNumber,
			ID:              a.projectID,
			StatusFieldID:   a.field.FieldID,
			InProgressID:    a.inProgressID,
			IssueRepos:      a.issueRepos,
			BacklogStatuses: a.backlogNames,
		}
	}
	return ws
}
