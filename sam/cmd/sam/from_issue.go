package main

import (
	"errors"
	"fmt"
	"slices"
	"strconv"

	"github.com/spf13/cobra"

	"github.com/nicomaioli-skinandme/dotfiles/sam/internal/config"
	"github.com/nicomaioli-skinandme/dotfiles/sam/internal/ghx"
	"github.com/nicomaioli-skinandme/dotfiles/sam/internal/gitx"
	"github.com/nicomaioli-skinandme/dotfiles/sam/internal/setup"
	"github.com/nicomaioli-skinandme/dotfiles/sam/internal/tmuxx"
	"github.com/nicomaioli-skinandme/dotfiles/sam/internal/ui"
)

// statusInProgress mirrors the literal label bash compared against. The
// project field ID + option ID for the actual write live in config.
const statusInProgress = "🏗 In Progress"

func newFromIssueCmd() *cobra.Command {
	var issueFlag int
	var repoFlag string
	cmd := &cobra.Command{
		Use:   "from-issue",
		Short: "Pick a backlog issue and bootstrap a worktree + tmux session",
		Args:  cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			workspaceName, workspace, err := loadWorkspace()
			if err != nil {
				return err
			}
			interactive := issueFlag == 0 && repoFlag == ""
			if !interactive && (issueFlag == 0 || repoFlag == "") {
				return errors.New("--issue and --repo must be set together")
			}
			return runFromIssue(workspaceName, workspace, issueFlag, repoFlag, interactive)
		},
	}
	cmd.Flags().IntVar(&issueFlag, "issue", 0, "issue number (non-interactive)")
	cmd.Flags().StringVar(&repoFlag, "repo", "",
		"issue repo, e.g. org/name (non-interactive)")
	return cmd
}

// resolvedIssue is the single shape downstream bootstrap consumes,
// produced by both project-board and plain-issue sources. ItemID/Status
// stay empty when no [gh_project] is configured — the conditional
// status write below treats that as "skip".
type resolvedIssue struct {
	ItemID     string
	Status     string
	Assignees  []string
	Repository string
	Number     int
	Title      string
}

func runFromIssue(workspaceName string, workspace *config.Workspace, issueFlag int, repoFlag string, interactive bool) error {
	hasGhProject := workspace.GhProject.Owner != "" && workspace.GhProject.Number != 0

	resolved, err := resolveIssue(workspace, issueFlag, repoFlag, interactive, hasGhProject)
	if err != nil {
		return err
	}
	if resolved == nil {
		// user cancelled the picker
		return nil
	}

	me, err := ghx.CurrentUser()
	if err != nil {
		return err
	}

	issueRepo := resolved.Repository
	issueNum := resolved.Number

	switch {
	case len(resolved.Assignees) == 0:
		if err := ghx.IssueAddAssignee(issueRepo, issueNum, me); err != nil {
			return err
		}
	case slices.Contains(resolved.Assignees, me):
		// already mine
	default:
		other := resolved.Assignees[0]
		if !interactive {
			return fmt.Errorf("issue %s#%d assigned to %s; rerun interactively to reassign",
				issueRepo, issueNum, other)
		}
		ok, err := ui.Confirm(fmt.Sprintf("Issue is assigned to %s. Reassign to you?", other))
		if err != nil {
			if errors.Is(err, ui.ErrCancelled) {
				return nil
			}
			return err
		}
		if !ok {
			return nil
		}
		if err := ghx.IssueSwapAssignee(issueRepo, issueNum, other, me); err != nil {
			return err
		}
	}

	if hasGhProject && resolved.ItemID != "" && resolved.Status != statusInProgress {
		if err := ghx.ProjectItemSetStatus(workspace.GhProject, resolved.ItemID, workspace.GhProject.InProgressID); err != nil {
			return err
		}
	}

	existing, _ := ghx.IssueDevelopList(issueRepo, issueNum)
	branch := existing
	if branch == "" {
		branch = fmt.Sprintf("%d-%s", issueNum, gitx.Slugify(resolved.Title))
	}

	if workspace.MaxBranchLen > 0 && len(branch) > workspace.MaxBranchLen && interactive {
		choice, err := ui.Picker(
			fmt.Sprintf("Branch name is %d chars (limit %d)", len(branch), workspace.MaxBranchLen),
			[]ui.Item{
				{Value: "keep", Label: "Keep as is"},
				{Value: "edit", Label: "Manually edit"},
			},
		)
		if err != nil {
			if errors.Is(err, ui.ErrCancelled) {
				return nil
			}
			return err
		}
		if choice.Value == "edit" {
			newBranch, err := ui.Input("Branch name", branch, fmt.Sprintf("%d-", issueNum))
			if err != nil {
				if errors.Is(err, ui.ErrCancelled) {
					return nil
				}
				return err
			}
			if newBranch != "" && newBranch != branch {
				if existing != "" {
					if err := gitx.PushRefspec(workspace.Repo, "origin/"+existing, newBranch); err != nil {
						return err
					}
					if err := gitx.PushDelete(workspace.Repo, existing); err != nil {
						return err
					}
				}
				branch = newBranch
			}
		}
	}

	if existing == "" {
		if err := ghx.IssueDevelop(issueRepo, workspace.BranchRepo, issueNum, branch); err != nil {
			return err
		}
	}

	// Fetch so the branch gh just created on the remote is locally reachable.
	if err := gitx.Fetch(workspace.Repo); err != nil {
		return err
	}

	if err := gitx.FastForwardMain(workspace.Repo, workspace.MainBranch); err != nil {
		return err
	}

	path, err := setup.CreateWorktree(workspace, branch, issueNum, workspaceName)
	if err != nil {
		return err
	}

	if err := tmuxx.EnsureSystemSession(); err != nil {
		return err
	}
	if !tmuxx.HasSession(branch) {
		if err := tmuxx.BuildSession(branch, workspace, path); err != nil {
			return err
		}
		data := tmuxx.ClaudeData{
			IssueNumber: issueNum,
			IssueTitle:  resolved.Title,
			IssueRepo:   issueRepo,
			IssueURL:    fmt.Sprintf("https://github.com/%s/issues/%d", issueRepo, issueNum),
		}
		if err := tmuxx.AddClaudePane(branch, workspace, data); err != nil {
			return err
		}
	}
	return tmuxx.SwitchOrAttach(branch)
}

// resolveIssue picks the source for the issue depending on whether a
// GitHub Project (v2) board is configured and whether the user passed
// --issue/--repo. Returns (nil, nil) when the interactive picker is
// cancelled.
func resolveIssue(workspace *config.Workspace, issueFlag int, repoFlag string, interactive, hasGhProject bool) (*resolvedIssue, error) {
	switch {
	case hasGhProject && interactive:
		return resolveFromProjectBacklog(workspace)
	case hasGhProject && !interactive:
		return resolveFromProjectByFlag(workspace, issueFlag, repoFlag)
	case !hasGhProject && interactive:
		return resolveFromIssueList(workspace)
	default: // !hasGhProject && !interactive
		return resolveFromIssueView(issueFlag, repoFlag)
	}
}

func resolveFromProjectBacklog(workspace *config.Workspace) (*resolvedIssue, error) {
	items, err := ghx.ProjectItems(workspace.GhProject)
	if err != nil {
		return nil, err
	}
	backlog := filterBacklog(items, workspace.GhProject.IssueRepos, workspace.GhProject.BacklogStatuses)
	if len(backlog) == 0 {
		return nil, errors.New("no backlog issues found")
	}
	picks := make([]ui.Item, len(backlog))
	for i, it := range backlog {
		picks[i] = ui.Item{
			Value: it.ID,
			Label: fmt.Sprintf("#%d  %s  (%s)", it.Content.Number, it.Content.Title, it.Content.Repository),
		}
	}
	sel, err := ui.Picker("Select backlog issue", picks)
	if err != nil {
		if errors.Is(err, ui.ErrCancelled) {
			return nil, nil
		}
		return nil, err
	}
	for _, it := range backlog {
		if it.ID == sel.Value {
			return projectItemToResolved(it), nil
		}
	}
	return nil, fmt.Errorf("picker returned unknown id %q", sel.Value)
}

func resolveFromProjectByFlag(workspace *config.Workspace, issueFlag int, repoFlag string) (*resolvedIssue, error) {
	items, err := ghx.ProjectItems(workspace.GhProject)
	if err != nil {
		return nil, err
	}
	found, ok := findItem(items, issueFlag, repoFlag)
	if !ok {
		return nil, fmt.Errorf("issue %s#%d is not on project %s/#%d",
			repoFlag, issueFlag, workspace.GhProject.Owner, workspace.GhProject.Number)
	}
	return projectItemToResolved(found), nil
}

func resolveFromIssueList(workspace *config.Workspace) (*resolvedIssue, error) {
	issues, err := ghx.IssueList(workspace.BranchRepo)
	if err != nil {
		return nil, err
	}
	if len(issues) == 0 {
		return nil, fmt.Errorf("no open issues in %s", workspace.BranchRepo)
	}
	picks := make([]ui.Item, len(issues))
	for i, it := range issues {
		picks[i] = ui.Item{
			Value: strconv.Itoa(it.Number),
			Label: fmt.Sprintf("#%d  %s  (%s)", it.Number, it.Title, it.Repository),
		}
	}
	sel, err := ui.Picker("Select open issue", picks)
	if err != nil {
		if errors.Is(err, ui.ErrCancelled) {
			return nil, nil
		}
		return nil, err
	}
	for _, it := range issues {
		if strconv.Itoa(it.Number) == sel.Value {
			return issueToResolved(it), nil
		}
	}
	return nil, fmt.Errorf("picker returned unknown number %q", sel.Value)
}

func resolveFromIssueView(issueFlag int, repoFlag string) (*resolvedIssue, error) {
	issue, err := ghx.IssueView(repoFlag, issueFlag)
	if err != nil {
		return nil, err
	}
	return issueToResolved(issue), nil
}

func projectItemToResolved(it ghx.ProjectItem) *resolvedIssue {
	return &resolvedIssue{
		ItemID:     it.ID,
		Status:     it.Status,
		Assignees:  it.Assignees,
		Repository: it.Content.Repository,
		Number:     it.Content.Number,
		Title:      it.Content.Title,
	}
}

func issueToResolved(it ghx.Issue) *resolvedIssue {
	return &resolvedIssue{
		Assignees:  it.Assignees,
		Repository: it.Repository,
		Number:     it.Number,
		Title:      it.Title,
	}
}

// filterBacklog returns items whose repo is in repos AND status is in
// statuses. Pure for testability.
func filterBacklog(items []ghx.ProjectItem, repos, statuses []string) []ghx.ProjectItem {
	out := make([]ghx.ProjectItem, 0, len(items))
	for _, it := range items {
		if !slices.Contains(repos, it.Content.Repository) {
			continue
		}
		if !slices.Contains(statuses, it.Status) {
			continue
		}
		out = append(out, it)
	}
	return out
}

// findItem returns the item matching (num, repo). Pure for testability.
func findItem(items []ghx.ProjectItem, num int, repo string) (ghx.ProjectItem, bool) {
	for _, it := range items {
		if it.Content.Number == num && it.Content.Repository == repo {
			return it, true
		}
	}
	return ghx.ProjectItem{}, false
}
