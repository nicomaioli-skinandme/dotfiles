package issue

import (
	"fmt"

	"github.com/nicomaioli-skinandme/dotfiles/sam/internal/config"
	"github.com/nicomaioli-skinandme/dotfiles/sam/internal/session"
	"github.com/nicomaioli-skinandme/dotfiles/sam/internal/worktree"
)

// Controller orchestrates the from-issue develop flow: it sequences the
// issue Service's GitHub writes with the worktree and session entities. It
// imports those entities' Services (the only cross-entity edges) and never
// another Controller.
type Controller struct {
	issues    Service
	worktrees worktree.Service
	sessions  session.Service
}

// NewController returns an issue Controller backed by the given services.
func NewController(issues Service, worktrees worktree.Service, sessions session.Service) Controller {
	return Controller{issues: issues, worktrees: worktrees, sessions: sessions}
}

// List returns the candidate issues for the workspace.
func (c Controller) List(ws *config.Workspace) ([]Issue, error) {
	return c.issues.List(ws)
}

// HasGhProject reports whether the workspace links a GitHub Project.
func (c Controller) HasGhProject(ws *config.Workspace) bool {
	return c.issues.HasGhProject(ws)
}

// Prepare resolves the data the interactive caller (the TUI) needs before
// it shows its reassign / branch-edit modals: the current user and the
// planned branch (with any pre-existing linked branch).
func (c Controller) Prepare(ws *config.Workspace, iss Issue) (me, branch, existing string, err error) {
	me, err = c.issues.CurrentUser()
	if err != nil {
		return "", "", "", err
	}
	branch, existing, err = c.issues.Plan(ws, iss)
	if err != nil {
		return "", "", "", err
	}
	return me, branch, existing, nil
}

// Apply runs the from-issue bootstrap given the caller's decisions and
// returns the tmux session name to attach to — it does NOT attach (the CLI
// attaches after; the TUI attaches via its Result after releasing the
// terminal). reassign must be true to move an issue off another assignee;
// branch/existing come from Plan (branch may have been edited). It assigns
// the issue, moves its project status to In Progress, creates the
// branch/worktree, builds the tmux session, and adds the Claude pane —
// idempotently.
func (c Controller) Apply(ws *config.Workspace, wsName string, iss Issue, me string, reassign bool, branch, existing string) (string, error) {
	if err := c.issues.Assign(iss, me, reassign); err != nil {
		return "", err
	}
	if err := c.issues.SetInProgress(ws, iss); err != nil {
		return "", err
	}

	// A branch edit (branch != existing) renames the remote branch gh
	// already created before we check it out locally.
	if existing != "" && branch != existing {
		if err := c.issues.RenameRemoteBranch(ws, existing, branch); err != nil {
			return "", err
		}
	}
	if existing == "" {
		if err := c.issues.Develop(iss.Repository, ws.BranchRepo, iss.Number, branch); err != nil {
			return "", err
		}
	}

	// Fetch so the branch gh just created on the remote is locally
	// reachable, then fast-forward the trunk before branching the worktree.
	if err := c.worktrees.Fetch(ws); err != nil {
		return "", err
	}
	if err := c.worktrees.FastForwardTrunk(ws); err != nil {
		return "", err
	}

	path, err := c.worktrees.Create(ws, branch, iss.Number, wsName)
	if err != nil {
		return "", err
	}

	sess := c.sessions.Name(wsName, branch)
	if !c.sessions.Has(sess) {
		if err := c.sessions.Build(sess, ws, path); err != nil {
			return "", err
		}
		data := session.ClaudeData{
			IssueNumber: iss.Number,
			IssueTitle:  iss.Title,
			IssueRepo:   iss.Repository,
			IssueURL:    fmt.Sprintf("https://github.com/%s/issues/%d", iss.Repository, iss.Number),
		}
		if err := c.sessions.AddClaudePane(sess, ws.FromIssue.RepoWindow, ws.FromIssue.ClaudePrompt, ws.FromIssue.ClaudePaneTitle, "", data, path); err != nil {
			return "", err
		}
	}
	return sess, nil
}

// Develop is the fully non-interactive CLI entry point: it resolves issue
// `num` (repo defaults to the workspace's branch repo), enforces the
// reassign and branch-length gates by erroring (never prompting) when a
// decision it can't derive is required, bootstraps, and attaches.
func (c Controller) Develop(ws *config.Workspace, wsName string, num int, repo, branchOverride string, reassign bool) error {
	if repo == "" {
		repo = ws.BranchRepo
	}
	iss, err := c.issues.ByFlag(ws, num, repo)
	if err != nil {
		return err
	}
	me, err := c.issues.CurrentUser()
	if err != nil {
		return err
	}
	if other, needs := c.issues.NeedsReassign(iss, me); needs && !reassign {
		return fmt.Errorf("issue %s#%d is assigned to %s; pass --reassign to take it", repo, num, other)
	}
	branch, existing, err := c.issues.Plan(ws, iss)
	if err != nil {
		return err
	}
	if branchOverride != "" {
		branch = branchOverride
	} else if c.issues.NeedsBranchEdit(ws, branch) {
		return fmt.Errorf("planned branch %q exceeds max length %d; pass --branch to set one",
			branch, ws.MaxBranchLen)
	}

	sess, err := c.Apply(ws, wsName, iss, me, reassign, branch, existing)
	if err != nil {
		return err
	}
	return c.sessions.SwitchOrAttach(sess)
}
