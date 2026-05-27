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
// The program never attaches to tmux itself: attaching replaces the
// process image, which it cannot do while it owns the terminal. Instead
// it records a [Result] and quits; the caller performs the attach (and
// any deferred flow, like new-worktree or from-issue) after the program
// has exited and released the terminal.
package tui

import (
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/nicomaioli-skinandme/dotfiles/sam/internal/config"
)

// Resource is one navigable category, switched between with `:`.
type Resource int

const (
	ResWorktrees  Resource = iota // system, main repo, and per-branch worktrees
	ResWorkspaces                 // configured workspaces; activate switches the active one
	ResIssues                     // GitHub Project backlog / open issues (async)
	ResClankers                   // running claude processes and their tmux context
)

// resources lists the kinds in display/cycle order.
var resources = []Resource{ResWorktrees, ResWorkspaces, ResIssues, ResClankers}

func (r Resource) Name() string {
	switch r {
	case ResWorktrees:
		return "worktrees"
	case ResWorkspaces:
		return "workspaces"
	case ResIssues:
		return "issues"
	case ResClankers:
		return "clankers"
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

// Item is one row in the central list. ID is the row's stable identity
// (used for multi-select and to drive the activation/delete actions);
// Title is the display text; Detail is optional trailing context;
// Active marks rows whose tmux session is currently running.
type Item struct {
	ID     string
	Title  string
	Detail string
	Active bool
}

// BuildSpec tells the caller to create a tmux session before attaching.
type BuildSpec struct {
	BaseDir      string // working directory for the session's first window
	EnsureSystem bool   // ensure the always-on `system` session instead of building
}

// Result is the single value the TUI hands back on exit. At most one of
// its actions is set; an all-zero Result means "user quit, do nothing".
type Result struct {
	Attach            string     // session to switch/attach to after exit
	Build             *BuildSpec // create this session (named by Attach) first
	RunWizard         bool       // run `workspace add` wizard after exit
	NewWorktreeBranch string     // run new-worktree for this branch after exit
}

// Run launches the full-screen TUI against the given workspace and
// returns the action the user chose. all is the set of configured
// workspaces (for `:workspaces`); start is the resource to open on.
func Run(workspaceName string, workspace *config.Workspace, all map[string]config.Workspace, start Resource) (Result, error) {
	m := newModel(workspaceName, workspace, all, start)
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
