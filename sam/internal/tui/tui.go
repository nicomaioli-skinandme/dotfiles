// Package tui is sam's full-screen interactive front end: an input bar
// on top, a single central navigation list in the middle, and a status
// bar with a breadcrumb at the bottom. It replaces the sequence of huh
// prompts that drove the old menu.
//
// Navigation is modal and vim-like. `/` filters the on-screen list, `:`
// switches between resources (`:worktrees`, `:workspaces`, `:issues`,
// `:clankers`) or quits (`:q`). `<CR>` activates the highlighted row, and
// `a`/`d` add/delete where the current resource supports it.
//
// Activating a row attaches to its tmux session without ending the
// program: the model suspends via tea.ExecProcess (outside tmux) or
// switch-client (inside tmux), runs the tmux client as a child, and
// resumes the same model when the user detaches — so sam stays put and
// you land back exactly where you left it. The tmux *session* is always
// built detached (`new-session -d`) and owned by the daemonized tmux
// server, never by sam; only the transient attach *client* is sam's
// child. The TUI quits (records a [Result]) only to run the workspace-add
// wizard, which is an in-process form that can't ride tea.ExecProcess.
package tui

import (
	"log/slog"
	"os/exec"
	"sort"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/nicomaioli-skinandme/dotfiles/sam/internal/clanker"
	"github.com/nicomaioli-skinandme/dotfiles/sam/internal/config"
	"github.com/nicomaioli-skinandme/dotfiles/sam/internal/issue"
	"github.com/nicomaioli-skinandme/dotfiles/sam/internal/logx"
	"github.com/nicomaioli-skinandme/dotfiles/sam/internal/pr"
	"github.com/nicomaioli-skinandme/dotfiles/sam/internal/worktree"
)

// Deps are the entity Controllers and Services the TUI consumes. The TUI is
// just another consumer of the same backend the cli uses: it holds the
// Controllers for the orchestrated actions (list/develop/review/delete) and
// a few Services for the primitive lookups its Elm flows need (branch
// candidates, session name/existence, the pure reassign/branch checks).
type Deps struct {
	Worktrees   worktree.Controller
	WorktreeSvc worktree.Service
	Issues      issue.Controller
	IssueSvc    issue.Service
	PRs         pr.Controller
	Clankers    clanker.Controller
	SessionSvc  SessionService

	// Logger and LogRing are the cross-cutting diagnostic sink and its
	// in-memory view (not an entity): the TUI logs through Logger and the
	// `:logs` view reads LogRing. Both may be nil — the model substitutes a
	// discard logger and treats a nil ring as empty — so tests need not wire
	// them. LogPath is the temp file the logger tees to, shown in the logs
	// view's empty state ("" when there is none).
	Logger  *slog.Logger
	LogRing *logx.Ring
	LogPath string
}

// SessionService is the slice of the session entity the TUI consumes. It
// is declared here (consumer-side) so tests can substitute a fake instead
// of shelling out to tmux; session.Service satisfies it. Ensure builds a
// worktree's session if absent and returns its name (never attaching);
// AttachCmd yields the `tmux attach-session` command the model runs as a
// child via tea.ExecProcess (outside tmux); Switch is the inside-tmux
// switch-client equivalent.
type SessionService interface {
	Name(wsName, branch string) string
	Has(name string) bool
	Current() (string, error)
	InTmux() bool
	Ensure(ws *config.Workspace, wsName, name string) (string, error)
	AttachCmd(name string) *exec.Cmd
	Switch(name string) error
}

// Resource is one navigable category, switched between with `:`.
type Resource int

const (
	ResWorktrees  Resource = iota // main worktree and linked worktrees
	ResWorkspaces                 // configured workspaces; activate switches the active one
	ResIssues                     // GitHub Project backlog / open issues (async)
	ResPRs                        // open PRs requesting you as reviewer (async)
	ResClankers                   // running claude processes and their tmux context
	ResLogs                       // this session's diagnostic log (errors, warnings, activity)
)

// resources lists the kinds in display/cycle order.
var resources = []Resource{ResWorktrees, ResWorkspaces, ResIssues, ResPRs, ResClankers, ResLogs}

func (r Resource) Name() string {
	switch r {
	case ResWorktrees:
		return "worktrees"
	case ResWorkspaces:
		return "workspaces"
	case ResIssues:
		return "issues"
	case ResPRs:
		return "prs"
	case ResClankers:
		return "clankers"
	case ResLogs:
		return "logs"
	}
	return "?"
}

// resourceByName maps a `:command` word to a resource.
func resourceByName(s string) (Resource, bool) {
	for _, r := range resources {
		if r.Name() == s {
			return r, true
		}
	}
	return 0, false
}

// commandCandidates lists the words the `:` autocomplete can complete to:
// every resource name plus "quit" (the long form of :q). Built from the
// resources slice so a new resource shows up automatically. Sorted only
// for a stable empty-query display order; fuzzy.Find re-ranks once the
// user types.
func commandCandidates() []string {
	out := make([]string, 0, len(resources)+1)
	out = append(out, "quit")
	for _, r := range resources {
		out = append(out, r.Name())
	}
	sort.Strings(out)
	return out
}

// WorktreeType tags a worktrees-view row as git's main worktree (the
// repo-root checkout) or a linked worktree (one sam created under the
// workspace's worktrees dir). Empty on rows of other resources.
type WorktreeType string

const (
	WorktreeMain   WorktreeType = "main"
	WorktreeLinked WorktreeType = "linked"
)

// Item is one row in the central list. ID is the row's stable identity
// (used for multi-select and to drive the activation/delete actions);
// Title is the display text; Detail is optional trailing context;
// Active marks rows whose tmux session is currently running; Type tags
// worktrees-view rows as the main or a linked worktree (empty elsewhere).
type Item struct {
	ID     string
	Title  string
	Detail string
	Active bool
	Type   WorktreeType
}

// Result is the value the TUI hands back on exit. Attaching to a session
// no longer goes through here — the model does that in place via
// tea.ExecProcess and keeps running. The only thing left that must happen
// after the program releases the terminal is the workspace-add wizard (an
// in-process huh form that can't ride tea.ExecProcess), so RunWizard is
// the sole field; an all-zero Result means "user quit, do nothing".
type Result struct {
	RunWizard bool // run `workspace add` wizard after exit, then re-enter the menu
}

// Run launches the full-screen TUI against the given workspace and
// returns the action the user chose. all is the set of configured
// workspaces (for `:workspaces`); start is the resource to open on;
// tuiCfg carries menu-level settings (e.g. autocomplete sizing).
func Run(workspaceName string, workspace *config.Workspace, all map[string]config.Workspace, start Resource, tuiCfg config.Tui, deps Deps) (Result, error) {
	m := newModel(workspaceName, workspace, all, start, tuiCfg, deps)
	final, err := tea.NewProgram(m).Run()
	if err != nil {
		return Result{}, err
	}
	fm := final.(*model)
	if fm.err != nil {
		return Result{}, fm.err
	}
	return fm.result, nil
}

// commandKind classifies a parsed `:` command.
type commandKind int

const (
	cmdNone     commandKind = iota // empty input
	cmdQuit                        // :q / :quit
	cmdResource                    // :<resource-name>
	cmdUnknown                     // anything else
)

type command struct {
	kind     commandKind
	resource Resource
}

// parseCommand interprets the text of the `:` input bar. The leading
// colon is optional and surrounding whitespace is ignored. It is pure so
// it can be unit-tested without a running program.
func parseCommand(raw string) command {
	s := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(raw), ":"))
	switch s {
	case "":
		return command{kind: cmdNone}
	case "q", "quit":
		return command{kind: cmdQuit}
	}
	if r, ok := resourceByName(s); ok {
		return command{kind: cmdResource, resource: r}
	}
	return command{kind: cmdUnknown}
}
