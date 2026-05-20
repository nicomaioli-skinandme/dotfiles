// Package config defines sam's typed configuration schema and loads it
// from `~/.config/sam/config.toml` (or any caller-supplied path).
package config

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/spf13/viper"
)

type Config struct {
	DefaultProject string             `mapstructure:"default_project"`
	Projects       map[string]Project `mapstructure:"projects"`
}

type Project struct {
	Repo         string     `mapstructure:"repo"`
	Worktrees    string     `mapstructure:"worktrees"`
	MainBranch   string     `mapstructure:"main_branch"`
	BranchRepo   string     `mapstructure:"branch_repo"`
	MaxBranchLen int        `mapstructure:"max_branch_len"`
	GhProject    GhProject  `mapstructure:"gh_project"`
	FromIssue    FromIssue  `mapstructure:"from_issue"`
	Tmux         TmuxLayout `mapstructure:"tmux"`
}

type GhProject struct {
	Owner           string   `mapstructure:"owner"`
	Number          int      `mapstructure:"number"`
	ID              string   `mapstructure:"id"`
	StatusFieldID   string   `mapstructure:"status_field_id"`
	InProgressID    string   `mapstructure:"in_progress_id"`
	IssueRepos      []string `mapstructure:"issue_repos"`
	BacklogStatuses []string `mapstructure:"backlog_statuses"`
}

type FromIssue struct {
	ClaudePrompt    string `mapstructure:"claude_prompt"`
	ClaudePaneTitle string `mapstructure:"claude_pane_title"`
	RepoWindow      string `mapstructure:"repo_window"`
}

type TmuxLayout struct {
	Windows []Window `mapstructure:"windows"`
}

type Window struct {
	Name  string `mapstructure:"name"`
	Cwd   string `mapstructure:"cwd"`
	Panes []Pane `mapstructure:"panes"`
}

type Pane struct {
	Split string `mapstructure:"split"`
	Cwd   string `mapstructure:"cwd"`
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

// Resolve picks which project to use given an optional explicit name (from
// the --project flag). Returns the project name and a pointer into cfg.Projects.
func Resolve(cfg *Config, explicit string) (string, *Project, error) {
	if explicit != "" {
		p, ok := cfg.Projects[explicit]
		if !ok {
			return "", nil, fmt.Errorf("project %q not found in config (have: %s)",
				explicit, projectNames(cfg))
		}
		return explicit, &p, nil
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
	return "", nil, fmt.Errorf("no project selected: pass --project or set default_project (have: %s)",
		projectNames(cfg))
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
	} else if len(cfg.Projects) > 1 {
		return fmt.Errorf("default_project is unset and multiple projects configured: %s",
			projectNames(cfg))
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
