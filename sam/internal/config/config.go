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
	Workspaces map[string]Workspace `mapstructure:"workspaces" toml:"workspaces"`
}

type Workspace struct {
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
// workspace's repo (but not the repo root) and no other resolution
// mechanism applied. The caller surfaces a guidance message and exits.
type ErrInsideRepo struct {
	Workspace string
	Repo      string
}

func (e *ErrInsideRepo) Error() string {
	return fmt.Sprintf("invoked inside %s repo (%s); cd to the repo root or a worktree",
		e.Workspace, e.Repo)
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

// Resolve picks which workspace to use given an optional explicit name
// (from --workspace) and the caller's cwd. Resolution order:
//  1. explicit
//  2. cwd matches a workspace's repo or sits inside its worktrees dir
//  3. cwd is inside a workspace's repo (but not at root) → ErrInsideRepo
//  4. single-workspace shortcut
//
// When none of the above apply, Resolve returns ("", nil, nil) — a
// success result meaning "no workspace selected; the caller should
// prompt." The interactive menu treats this as "open the
// workspace-select view"; non-interactive commands surface their own
// error pointing the user at --workspace.
func Resolve(cfg *Config, explicit, cwd string) (string, *Workspace, error) {
	if explicit != "" {
		w, ok := cfg.Workspaces[explicit]
		if !ok {
			return "", nil, fmt.Errorf("workspace %q not found in config (have: %s)",
				explicit, workspaceNames(cfg))
		}
		return explicit, &w, nil
	}

	if cwd != "" {
		cwd = filepath.Clean(cwd)
		// Pass 1: exact repo match or descendant of worktrees.
		for name, w := range cfg.Workspaces {
			repo := filepath.Clean(w.Repo)
			wt := filepath.Clean(w.Worktrees)
			if cwd == repo {
				return name, &w, nil
			}
			if isDescendant(cwd, wt) {
				return name, &w, nil
			}
		}
		// Pass 2: descendant of repo (but not at root) → guidance error.
		for name, w := range cfg.Workspaces {
			repo := filepath.Clean(w.Repo)
			if isDescendant(cwd, repo) {
				return "", nil, &ErrInsideRepo{Workspace: name, Repo: repo}
			}
		}
	}

	if len(cfg.Workspaces) == 1 {
		for name, w := range cfg.Workspaces {
			return name, &w, nil
		}
	}
	return "", nil, nil
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

// Default returns a Workspace with sensible defaults for the fields the
// wizard fills silently (from_issue, tmux, max_branch_len). Callers
// overlay user-supplied values on top.
func Default() Workspace {
	return Workspace{
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
	for name, w := range cfg.Workspaces {
		w.Repo = expandHome(w.Repo, home)
		w.Worktrees = expandHome(w.Worktrees, home)
		cfg.Workspaces[name] = w
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
	if len(cfg.Workspaces) == 0 {
		return fmt.Errorf("no workspaces configured")
	}

	for name, w := range cfg.Workspaces {
		if w.Repo == "" {
			return fmt.Errorf("workspace %q: repo is required", name)
		}
		if w.Worktrees == "" {
			return fmt.Errorf("workspace %q: worktrees is required", name)
		}
		if w.MainBranch == "" {
			return fmt.Errorf("workspace %q: main_branch is required", name)
		}
		if w.FromIssue.RepoWindow != "" {
			found := false
			for _, win := range w.Tmux.Windows {
				if win.Name == w.FromIssue.RepoWindow {
					found = true
					break
				}
			}
			if !found {
				return fmt.Errorf("workspace %q: from_issue.repo_window %q does not match any tmux window",
					name, w.FromIssue.RepoWindow)
			}
		}
	}
	return nil
}

func workspaceNames(cfg *Config) string {
	names := make([]string, 0, len(cfg.Workspaces))
	for n := range cfg.Workspaces {
		names = append(names, n)
	}
	sort.Strings(names)
	return strings.Join(names, ", ")
}
