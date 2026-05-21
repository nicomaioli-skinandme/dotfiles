package main

import (
	"errors"
	"fmt"
	"slices"

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
			projectName, project, err := loadProject()
			if err != nil {
				return err
			}
			interactive := issueFlag == 0 && repoFlag == ""
			if !interactive && (issueFlag == 0 || repoFlag == "") {
				return errors.New("--issue and --repo must be set together")
			}
			return runFromIssue(projectName, project, issueFlag, repoFlag, interactive)
		},
	}
	cmd.Flags().IntVar(&issueFlag, "issue", 0, "issue number (non-interactive)")
	cmd.Flags().StringVar(&repoFlag, "repo", "",
		"issue repo, e.g. org/name (non-interactive)")
	return cmd
}

func runFromIssue(projectName string, project *config.Project, issueFlag int, repoFlag string, interactive bool) error {
	if project.GhProject.Owner == "" || project.GhProject.Number == 0 {
		return fmt.Errorf("project %q has no [gh_project] configured; add one to %s or run `sam project add`",
			projectName, "~/.config/sam/config.toml")
	}
	items, err := ghx.ProjectItems(project.GhProject)
	if err != nil {
		return err
	}

	var item ghx.ProjectItem
	if interactive {
		backlog := filterBacklog(items, project.GhProject.IssueRepos, project.GhProject.BacklogStatuses)
		if len(backlog) == 0 {
			return errors.New("no backlog issues found")
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
				return nil
			}
			return err
		}
		for _, it := range backlog {
			if it.ID == sel.Value {
				item = it
				break
			}
		}
	} else {
		found, ok := findItem(items, issueFlag, repoFlag)
		if !ok {
			return fmt.Errorf("issue %s#%d is not on project %s/#%d",
				repoFlag, issueFlag, project.GhProject.Owner, project.GhProject.Number)
		}
		item = found
	}

	me, err := ghx.CurrentUser()
	if err != nil {
		return err
	}

	issueRepo := item.Content.Repository
	issueNum := item.Content.Number

	switch {
	case len(item.Assignees) == 0:
		if err := ghx.IssueAddAssignee(issueRepo, issueNum, me); err != nil {
			return err
		}
	case slices.Contains(item.Assignees, me):
		// already mine
	default:
		other := item.Assignees[0]
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

	if item.Status != statusInProgress {
		if err := ghx.ProjectItemSetStatus(project.GhProject, item.ID, project.GhProject.InProgressID); err != nil {
			return err
		}
	}

	existing, _ := ghx.IssueDevelopList(issueRepo, issueNum)
	branch := existing
	if branch == "" {
		branch = fmt.Sprintf("%d-%s", issueNum, gitx.Slugify(item.Content.Title))
	}

	if project.MaxBranchLen > 0 && len(branch) > project.MaxBranchLen && interactive {
		choice, err := ui.Picker(
			fmt.Sprintf("Branch name is %d chars (limit %d)", len(branch), project.MaxBranchLen),
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
					if err := gitx.PushRefspec(project.Repo, "origin/"+existing, newBranch); err != nil {
						return err
					}
					if err := gitx.PushDelete(project.Repo, existing); err != nil {
						return err
					}
				}
				branch = newBranch
			}
		}
	}

	if existing == "" {
		if err := ghx.IssueDevelop(issueRepo, project.BranchRepo, issueNum, branch); err != nil {
			return err
		}
	}

	// Fetch so the branch gh just created on the remote is locally reachable.
	if err := gitx.Fetch(project.Repo); err != nil {
		return err
	}

	if err := gitx.FastForwardMain(project.Repo, project.MainBranch); err != nil {
		return err
	}

	path, err := setup.CreateWorktree(project, branch, issueNum, projectName)
	if err != nil {
		return err
	}

	if err := tmuxx.EnsureSystemSession(); err != nil {
		return err
	}
	if !tmuxx.HasSession(branch) {
		if err := tmuxx.BuildSession(branch, project, path); err != nil {
			return err
		}
		data := tmuxx.ClaudeData{
			IssueNumber: issueNum,
			IssueTitle:  item.Content.Title,
			IssueRepo:   issueRepo,
			IssueURL:    fmt.Sprintf("https://github.com/%s/issues/%d", issueRepo, issueNum),
		}
		if err := tmuxx.AddClaudePane(branch, project, data); err != nil {
			return err
		}
	}
	return tmuxx.SwitchOrAttach(branch)
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
