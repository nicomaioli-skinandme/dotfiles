// Package config defines sam's typed configuration schema and loads it
// from `~/.config/sam/config.toml` (or any caller-supplied path).
package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/spf13/viper"
)

type Config struct {
	DefaultProject string             `mapstructure:"default_project" toml:"default_project,omitempty"`
	Projects       map[string]Project `mapstructure:"projects" toml:"projects"`
}

type Project struct {
	Repo          string     `mapstructure:"repo" toml:"repo"`
	Worktrees     string     `mapstructure:"worktrees" toml:"worktrees"`
	MainBranch    string     `mapstructure:"main_branch" toml:"main_branch"`
	BranchRepo    string     `mapstructure:"branch_repo" toml:"branch_repo,omitempty"`
	MaxBranchLen  int        `mapstructure:"max_branch_len" toml:"max_branch_len,omitempty"`
	WorktreeSetup string     `mapstructure:"worktree_setup" toml:"worktree_setup,omitempty"`
	GhProject     GhProject  `mapstructure:"gh_project" toml:"gh_project,omitempty"`
	FromIssue     FromIssue  `mapstructure:"from_issue" toml:"from_issue,omitempty"`
	Tmux          TmuxLayout `mapstructure:"tmux" toml:"tmux,omitempty"`
}

type GhProject struct {
	Owner           string   `mapstructure:"owner" toml:"owner,omitempty"`
	Number          int      `mapstructure:"number" toml:"number,omitempty"`
	ID              string   `mapstructure:"id" toml:"id,omitempty"`
	StatusFieldID   string   `mapstructure:"status_field_id" toml:"status_field_id,omitempty"`
	InProgressID    string   `mapstructure:"in_progress_id" toml:"in_progress_id,omitempty"`
	IssueRepos      []string `mapstructure:"issue_repos" toml:"issue_repos,omitempty"`
	BacklogStatuses []string `mapstructure:"backlog_statuses" toml:"backlog_statuses,omitempty"`
}

type FromIssue struct {
	ClaudePrompt    string `mapstructure:"claude_prompt" toml:"claude_prompt,omitempty"`
	ClaudePaneTitle string `mapstructure:"claude_pane_title" toml:"claude_pane_title,omitempty"`
	RepoWindow      string `mapstructure:"repo_window" toml:"repo_window,omitempty"`
}

type TmuxLayout struct {
	Windows []Window `mapstructure:"windows" toml:"windows,omitempty"`
}

type Window struct {
	Name  string `mapstructure:"name" toml:"name"`
	Cwd   string `mapstructure:"cwd" toml:"cwd"`
	Panes []Pane `mapstructure:"panes" toml:"panes,omitempty"`
}

type Pane struct {
	Split string `mapstructure:"split" toml:"split"`
	Cwd   string `mapstructure:"cwd" toml:"cwd"`
}

// ErrInsideRepo is returned by Resolve when cwd is a descendant of a
// project's repo (but not the repo root) and no other resolution
// mechanism applied. The caller surfaces a guidance message and exits.
type ErrInsideRepo struct {
	Project string
	Repo    string
}

func (e *ErrInsideRepo) Error() string {
	return fmt.Sprintf("invoked inside %s repo (%s); cd to the repo root or a worktree",
		e.Project, e.Repo)
}

// DefaultPath returns the default config location: $HOME/.config/sam/config.toml.
func DefaultPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config", "sam", "config.toml"), nil
}

// Load reads, unmarshals, expands `~`, and validates the config at path.
func Load(path string) (*Config, error) {
	v := viper.New()
	v.SetConfigFile(path)
	v.SetConfigType("toml")
	if err := v.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("read config %s: %w", path, err)
	}

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("parse config %s: %w", path, err)
	}

	if err := expandPaths(&cfg); err != nil {
		return nil, err
	}

	if err := validate(&cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}

// Resolve picks which project to use given an optional explicit name
// (from --project) and the caller's cwd. Resolution order:
//  1. explicit
//  2. cwd matches a project's repo or sits inside its worktrees dir
//  3. cwd is inside a project's repo (but not at root) → ErrInsideRepo
//  4. default_project
//  5. single-project shortcut
func Resolve(cfg *Config, explicit, cwd string) (string, *Project, error) {
	if explicit != "" {
		p, ok := cfg.Projects[explicit]
		if !ok {
			return "", nil, fmt.Errorf("project %q not found in config (have: %s)",
				explicit, projectNames(cfg))
		}
		return explicit, &p, nil
	}

	if cwd != "" {
		cwd = filepath.Clean(cwd)
		// Pass 1: exact repo match or descendant of worktrees.
		for name, p := range cfg.Projects {
			repo := filepath.Clean(p.Repo)
			wt := filepath.Clean(p.Worktrees)
			if cwd == repo {
				return name, &p, nil
			}
			if isDescendant(cwd, wt) {
				return name, &p, nil
			}
		}
		// Pass 2: descendant of repo (but not at root) → guidance error.
		for name, p := range cfg.Projects {
			repo := filepath.Clean(p.Repo)
			if isDescendant(cwd, repo) {
				return "", nil, &ErrInsideRepo{Project: name, Repo: repo}
			}
		}
	}

	if cfg.DefaultProject != "" {
		p := cfg.Projects[cfg.DefaultProject]
		return cfg.DefaultProject, &p, nil
	}
	if len(cfg.Projects) == 1 {
		for name, p := range cfg.Projects {
			return name, &p, nil
		}
	}
	return "", nil, fmt.Errorf("no project matches this directory: pass --project, set default_project, or run `sam project add` (have: %s)",
		projectNames(cfg))
}

// IsInsideRepo reports whether err is the in-repo-subdir guidance error.
func IsInsideRepo(err error) bool {
	var e *ErrInsideRepo
	return errors.As(err, &e)
}

// isDescendant reports whether child is a strict descendant of parent.
// Both arguments are expected to be Clean'd absolute paths.
func isDescendant(child, parent string) bool {
	if parent == "" || child == parent {
		return false
	}
	sep := string(os.PathSeparator)
	return strings.HasPrefix(child, parent+sep)
}

// Default returns a Project with sensible defaults for the fields the
// wizard fills silently (from_issue, tmux, max_branch_len). Callers
// overlay user-supplied values on top.
func Default() Project {
	return Project{
		MaxBranchLen: 20,
		FromIssue: FromIssue{
			ClaudePrompt:    "/plan pull the context from {{ .IssueURL }}, including comments. Plan to implement, ask any relevant questions.",
			ClaudePaneTitle: "IMPL {{ .IssueTitle }}",
			RepoWindow:      "repo",
		},
		Tmux: TmuxLayout{
			Windows: []Window{{Name: "repo", Cwd: "."}},
		},
	}
}

func expandPaths(cfg *Config) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	for name, p := range cfg.Projects {
		p.Repo = expandHome(p.Repo, home)
		p.Worktrees = expandHome(p.Worktrees, home)
		cfg.Projects[name] = p
	}
	return nil
}

func expandHome(path, home string) string {
	if path == "~" {
		return home
	}
	if strings.HasPrefix(path, "~/") {
		return filepath.Join(home, path[2:])
	}
	return path
}

func validate(cfg *Config) error {
	if len(cfg.Projects) == 0 {
		return fmt.Errorf("no projects configured")
	}
	if cfg.DefaultProject != "" {
		if _, ok := cfg.Projects[cfg.DefaultProject]; !ok {
			return fmt.Errorf("default_project %q is not defined (have: %s)",
				cfg.DefaultProject, projectNames(cfg))
		}
	}

	for name, p := range cfg.Projects {
		if p.Repo == "" {
			return fmt.Errorf("project %q: repo is required", name)
		}
		if p.Worktrees == "" {
			return fmt.Errorf("project %q: worktrees is required", name)
		}
		if p.MainBranch == "" {
			return fmt.Errorf("project %q: main_branch is required", name)
		}
		if p.FromIssue.RepoWindow != "" {
			found := false
			for _, w := range p.Tmux.Windows {
				if w.Name == p.FromIssue.RepoWindow {
					found = true
					break
				}
			}
			if !found {
				return fmt.Errorf("project %q: from_issue.repo_window %q does not match any tmux window",
					name, p.FromIssue.RepoWindow)
			}
		}
	}
	return nil
}

func projectNames(cfg *Config) string {
	names := make([]string, 0, len(cfg.Projects))
	for n := range cfg.Projects {
		names = append(names, n)
	}
	sort.Strings(names)
	return strings.Join(names, ", ")
}
