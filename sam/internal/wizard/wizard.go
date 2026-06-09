// Package wizard implements the interactive workspace-setup flow that
// produces a validated entry in `~/.config/sam/config.toml`. It owns
// only the interaction model; the caller saves the resulting config.
package wizard

import (
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/nicomaioli-skinandme/dotfiles/sam/internal/config"
	"github.com/nicomaioli-skinandme/dotfiles/sam/internal/ghx"
	"github.com/nicomaioli-skinandme/dotfiles/sam/internal/gitx"
	"github.com/nicomaioli-skinandme/dotfiles/sam/internal/ui"
)

// ErrMissingScopes is returned when `gh` is missing scopes the user's
// selections require. The wizard exits cleanly without writing config;
// the caller prints err and exits non-zero.
type ErrMissingScopes struct {
	Missing []string
}

func (e *ErrMissingScopes) Error() string {
	return fmt.Sprintf(
		"gh is missing required scope(s) %s — run: gh auth refresh -s %s",
		strings.Join(quoted(e.Missing), ", "),
		strings.Join(e.Missing, ","),
	)
}

func quoted(in []string) []string {
	out := make([]string, len(in))
	for i, s := range in {
		out[i] = "'" + s + "'"
	}
	return out
}

// Run drives the interactive flow and returns the updated config with
// the new workspace appended. `existing` may be nil (first-run case) or
// the already-loaded config.
//
// On ui.ErrCancelled (user pressed Esc/Ctrl-C) the wizard returns
// (nil, ui.ErrCancelled) for the caller to short-circuit cleanly.
func Run(existing *config.Config) (*config.Config, error) {
	cfg := existing
	if cfg == nil {
		cfg = &config.Config{Workspaces: map[string]config.Workspace{}}
	}
	if cfg.Workspaces == nil {
		cfg.Workspaces = map[string]config.Workspace{}
	}

	fmt.Println("sam workspace setup")
	if len(cfg.Workspaces) == 0 {
		fmt.Println("First-time setup — let's configure a workspace.")
	}
	fmt.Println()

	// 1. Repo path.
	cwd, _ := os.Getwd()
	repo, err := ui.Input("Repo path", "absolute path; defaults to cwd", cwd)
	if err != nil {
		return nil, err
	}
	repo, err = expandAbs(repo)
	if err != nil {
		return nil, err
	}
	if !gitx.IsRepo(repo) {
		return nil, fmt.Errorf("%s is not a git repository", repo)
	}

	// 2. Workspace name.
	name, err := ui.Input("Workspace name", "key under [workspaces.<name>]", filepath.Base(repo))
	if err != nil {
		return nil, err
	}
	if name == "" {
		return nil, errors.New("workspace name cannot be empty")
	}
	if _, exists := cfg.Workspaces[name]; exists {
		return nil, fmt.Errorf("workspace %q already exists; pick a different name or edit %s by hand",
			name, mustDefaultPath())
	}

	// 3. Worktrees path.
	worktrees, err := ui.Input("Worktrees path", "where new worktrees will be created", repo+".worktrees")
	if err != nil {
		return nil, err
	}
	worktrees, err = expandAbs(worktrees)
	if err != nil {
		return nil, err
	}

	// 4. Trunk.
	detected, _ := gitx.DefaultBranch(repo)
	if detected == "" {
		detected = "main"
	}
	trunk, err := ui.Input("Trunk", "detected from origin/HEAD", detected)
	if err != nil {
		return nil, err
	}
	if trunk == "" {
		return nil, errors.New("trunk cannot be empty")
	}

	// 5. branch_repo (owner/name on GitHub).
	originSlug, _ := gitx.OriginRepo(repo)
	branchRepo, err := ui.Input("branch_repo", "GitHub owner/name where issue branches live", originSlug)
	if err != nil {
		return nil, err
	}

	// 6. GitHub Project (optional).
	configureGhProject, err := ui.Confirm("Configure a GitHub Project?")
	if err != nil {
		return nil, err
	}
	var ghProj config.GhProject
	scopesNeeded := []string{"repo"}
	if configureGhProject {
		ghProj, err = collectGhProject(branchRepo)
		if err != nil {
			return nil, err
		}
		scopesNeeded = append(scopesNeeded, "project")
	}

	// 7. worktree_setup hook (optional).
	hookKind, err := ui.Picker("Post-worktree setup hook", []ui.Item{
		{Value: "none", Label: "None"},
		{Value: "command", Label: "Shell command (run via sh -c)"},
		{Value: "script", Label: "Path to script (also run via sh -c)"},
	})
	if err != nil {
		return nil, err
	}
	var worktreeSetup string
	switch hookKind.Value {
	case "command":
		worktreeSetup, err = ui.Input("Shell command", "runs in the new worktree dir; env: SAM_BRANCH, SAM_WORKTREE, SAM_REPO, SAM_WORKSPACE, SAM_ISSUE_NUMBER", "")
		if err != nil {
			return nil, err
		}
	case "script":
		worktreeSetup, err = ui.Input("Script path", "absolute or relative to the worktree dir", "")
		if err != nil {
			return nil, err
		}
	}

	// 8. gh scope validation. Last step before commit.
	if err := checkScopes(scopesNeeded); err != nil {
		return nil, err
	}

	// Compose the workspace. Defaults supply from_issue + tmux + max_branch_len.
	ws := config.Default()
	ws.Repo = repo
	ws.Worktrees = worktrees
	ws.Trunk = trunk
	ws.BranchRepo = branchRepo
	ws.GhProject = ghProj
	ws.WorktreeSetup = worktreeSetup

	cfg.Workspaces[name] = ws
	return cfg, nil
}

// AddWorkspace loads any existing config, runs the guided wizard to append a
// workspace, and saves, reporting the written path to w. A cancelled wizard
// is a no-op (returns nil, leaving the config untouched). It is the sole
// entry point for the interactive setup flow: the menu drives it both on
// first run (no config file yet) and from the workspaces view (`a`).
func AddWorkspace(w io.Writer) error {
	path, err := config.DefaultPath()
	if err != nil {
		return err
	}
	var existing *config.Config
	if _, statErr := os.Stat(path); statErr == nil {
		existing, err = config.Load(path)
		if err != nil {
			return err
		}
	} else if !errors.Is(statErr, os.ErrNotExist) {
		return statErr
	}
	updated, err := Run(existing)
	if err != nil {
		if errors.Is(err, ui.ErrCancelled) {
			return nil
		}
		return err
	}
	if err := config.Save(updated, path); err != nil {
		return err
	}
	fmt.Fprintf(w, "Wrote %s\n", path)
	return nil
}

func collectGhProject(branchRepo string) (config.GhProject, error) {
	urlStr, err := ui.Input("GitHub Project URL", "e.g. https://github.com/orgs/<org>/projects/<n>", "")
	if err != nil {
		return config.GhProject{}, err
	}
	owner, number, err := parseProjectURL(urlStr)
	if err != nil {
		return config.GhProject{}, err
	}

	projectID, err := ghx.ProjectMeta(owner, number)
	if err != nil {
		return config.GhProject{}, err
	}

	fld, err := ghx.StatusField(owner, number)
	if err != nil {
		return config.GhProject{}, err
	}
	if len(fld.Options) == 0 {
		return config.GhProject{}, fmt.Errorf("Status field on project %s/#%d has no options", owner, number)
	}

	items := make([]ui.Item, len(fld.Options))
	for i, o := range fld.Options {
		items[i] = ui.Item{Value: o.ID, Label: o.Name}
	}
	inProgress, err := ui.Picker("Which status means 'In Progress'?", items)
	if err != nil {
		return config.GhProject{}, err
	}

	// Backlog defaults: every option except in-progress and obvious "done" shapes.
	pre := make([]string, 0, len(fld.Options))
	for _, o := range fld.Options {
		if o.ID == inProgress.Value {
			continue
		}
		if isDoneLike(o.Name) {
			continue
		}
		pre = append(pre, o.ID)
	}
	chosenIDs, err := ui.MultiPicker("Which statuses count as 'backlog'?", items, pre)
	if err != nil {
		return config.GhProject{}, err
	}
	chosenNames := make([]string, 0, len(chosenIDs))
	for _, id := range chosenIDs {
		for _, o := range fld.Options {
			if o.ID == id {
				chosenNames = append(chosenNames, o.Name)
				break
			}
		}
	}

	issueReposIn, err := ui.Input(
		"issue_repos",
		"comma-separated GitHub owner/name list; issues from these repos appear in from-issue",
		branchRepo,
	)
	if err != nil {
		return config.GhProject{}, err
	}
	issueRepos := splitComma(issueReposIn)
	if len(issueRepos) == 0 && branchRepo != "" {
		issueRepos = []string{branchRepo}
	}

	return config.GhProject{
		Owner:           owner,
		Number:          number,
		ID:              projectID,
		StatusFieldID:   fld.FieldID,
		InProgressID:    inProgress.Value,
		IssueRepos:      issueRepos,
		BacklogStatuses: chosenNames,
	}, nil
}

// parseProjectURL extracts (owner, number) from URLs like
// https://github.com/orgs/<owner>/projects/<n> or
// https://github.com/users/<owner>/projects/<n>.
func parseProjectURL(s string) (string, int, error) {
	s = strings.TrimSpace(s)
	u, err := url.Parse(s)
	if err != nil {
		return "", 0, fmt.Errorf("parse url: %w", err)
	}
	parts := strings.Split(strings.Trim(u.Path, "/"), "/")
	// Expect: ["orgs"|"users", owner, "projects", number]
	if len(parts) != 4 || (parts[0] != "orgs" && parts[0] != "users") || parts[2] != "projects" {
		return "", 0, fmt.Errorf("not a GitHub Project URL: %s", s)
	}
	owner := parts[1]
	num, err := strconv.Atoi(parts[3])
	if err != nil {
		return "", 0, fmt.Errorf("project number: %w", err)
	}
	return owner, num, nil
}

var doneLike = regexp.MustCompile(`(?i)\b(done|complete|completed|shipped|closed|cancelled|canceled)\b`)

func isDoneLike(name string) bool {
	return doneLike.MatchString(name)
}

func checkScopes(required []string) error {
	have, err := ghx.AuthScopes()
	if err != nil {
		return err
	}
	got := map[string]bool{}
	for _, s := range have {
		got[s] = true
	}
	var missing []string
	for _, r := range required {
		if !got[r] {
			missing = append(missing, r)
		}
	}
	if len(missing) == 0 {
		return nil
	}
	return &ErrMissingScopes{Missing: missing}
}

func splitComma(s string) []string {
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

func expandAbs(p string) (string, error) {
	if strings.HasPrefix(p, "~") {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		if p == "~" {
			p = home
		} else if strings.HasPrefix(p, "~/") {
			p = filepath.Join(home, p[2:])
		}
	}
	abs, err := filepath.Abs(p)
	if err != nil {
		return "", err
	}
	return filepath.Clean(abs), nil
}

func mustDefaultPath() string {
	p, err := config.DefaultPath()
	if err != nil {
		return "~/.config/sam/config.toml"
	}
	return p
}
