// Package config defines sam's typed configuration schema and loads it
// from `~/.config/sam/config.toml` (or any caller-supplied path).
package config

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/go-viper/mapstructure/v2"
	"github.com/spf13/viper"
)

type Config struct {
	Workspaces map[string]Workspace `mapstructure:"workspaces" toml:"workspaces"`
	Tui        Tui                  `mapstructure:"tui" toml:"tui,omitempty"`
	Log        Log                  `mapstructure:"log" toml:"log,omitempty"`
}

// Log configures sam's diagnostic logging (the `:logs` view and the
// temp-file tee). Level is the minimum level emitted: debug, info, warn,
// or error (case-insensitive); empty falls back to DefaultLogLevel.
type Log struct {
	Level string `mapstructure:"level" toml:"level,omitempty"`
}

// DefaultLogLevel is applied when [log] level is unset.
const DefaultLogLevel = "info"

// Tui holds settings for the interactive menu (the `sam menu` TUI), as
// opposed to per-workspace configuration.
type Tui struct {
	Autocomplete Autocomplete `mapstructure:"autocomplete" toml:"autocomplete,omitempty"`
	Colors       Colors       `mapstructure:"colors" toml:"colors,omitempty"`
}

// Autocomplete configures the `:` command popup. Max is the most entries
// shown at once; 0 means "use the default".
type Autocomplete struct {
	Max int `mapstructure:"max" toml:"max,omitempty"`
}

// DefaultAutocompleteMax is applied when [tui.autocomplete] max is unset.
const DefaultAutocompleteMax = 5

// Colors is the menu TUI's semantic palette. Each value is an ANSI index
// ("0".."255"), a hex string ("#RGB" / "#RRGGBB"), or "" for the terminal
// default. Primary/Secondary/Destroy fall back to the Default* constants
// when unset; Foreground/Background left empty mean the terminal default.
type Colors struct {
	Primary    string `mapstructure:"primary" toml:"primary,omitempty"`
	Secondary  string `mapstructure:"secondary" toml:"secondary,omitempty"`
	Destroy    string `mapstructure:"destroy" toml:"destroy,omitempty"`
	Foreground string `mapstructure:"foreground" toml:"foreground,omitempty"`
	Background string `mapstructure:"background" toml:"background,omitempty"`
}

// Default palette, applied when [tui.colors] entries are unset. ANSI
// indices (not hex) so the palette tracks the user's terminal theme.
const (
	DefaultColorPrimary   = "3" // ANSI yellow
	DefaultColorSecondary = "8" // ANSI bright-black / grey
	DefaultColorDestroy   = "1" // ANSI red
)

type Workspace struct {
	Repo          string     `mapstructure:"repo" toml:"repo"`
	Worktrees     string     `mapstructure:"worktrees" toml:"worktrees"`
	Trunk         string     `mapstructure:"trunk" toml:"trunk"`
	BranchRepo    string     `mapstructure:"branch_repo" toml:"branch_repo,omitempty"`
	MaxBranchLen  int        `mapstructure:"max_branch_len" toml:"max_branch_len,omitempty"`
	WorktreeSetup string     `mapstructure:"worktree_setup" toml:"worktree_setup,omitempty"`
	GhProject     GhProject  `mapstructure:"gh_project" toml:"gh_project,omitempty"`
	FromIssue     FromIssue  `mapstructure:"from_issue" toml:"from_issue,omitempty"`
	FromPR        FromPR     `mapstructure:"from_pr" toml:"from_pr,omitempty"`
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

// FromIssue configures the Claude pane opened in an issue worktree.
// There are no default prompts: an empty ClaudePrompt means no pane is
// launched (the tmux layout is still built). RepoWindow names the tmux
// window the pane is split into.
type FromIssue struct {
	ClaudePrompt    string `mapstructure:"claude_prompt" toml:"claude_prompt,omitempty"`
	ClaudePaneTitle string `mapstructure:"claude_pane_title" toml:"claude_pane_title,omitempty"`
	RepoWindow      string `mapstructure:"repo_window" toml:"repo_window,omitempty"`
}

// FromPR mirrors FromIssue for the `prs` review flow: the Claude pane
// opened in a PR-review worktree. Same rule — an empty ClaudePrompt means
// no pane is launched. PermissionMode, when set, is passed to claude as
// `--permission-mode <mode>` (e.g. "auto"); empty means the flag is
// omitted and claude starts in its usual mode.
type FromPR struct {
	ClaudePrompt    string `mapstructure:"claude_prompt" toml:"claude_prompt,omitempty"`
	ClaudePaneTitle string `mapstructure:"claude_pane_title" toml:"claude_pane_title,omitempty"`
	RepoWindow      string `mapstructure:"repo_window" toml:"repo_window,omitempty"`
	PermissionMode  string `mapstructure:"permission_mode" toml:"permission_mode,omitempty"`
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

// DefaultPath returns the default config location: $HOME/.config/sam/config.toml.
func DefaultPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config", "sam", "config.toml"), nil
}

// Decode reads, unmarshals, and expands `~` in the config at path, WITHOUT
// validating or applying defaults. unknown lists any config keys that don't
// map to the schema (a strict-decode check viper skips by default). It is
// the raw-parse seam `sam config doctor` builds on; normal callers use Load.
func Decode(path string) (cfg *Config, unknown []string, err error) {
	v := viper.New()
	v.SetConfigFile(path)
	v.SetConfigType("toml")
	if err := v.ReadInConfig(); err != nil {
		return nil, nil, fmt.Errorf("read config %s: %w", path, err)
	}

	var c Config
	var md mapstructure.Metadata
	if err := v.Unmarshal(&c, func(dc *mapstructure.DecoderConfig) {
		dc.Metadata = &md
	}); err != nil {
		return nil, nil, fmt.Errorf("parse config %s: %w", path, err)
	}

	if err := expandPaths(&c); err != nil {
		return nil, nil, err
	}
	return &c, md.Unused, nil
}

// Load reads, unmarshals, expands `~`, and validates the config at path.
// Unknown keys are tolerated (use Decode / `sam config doctor` to surface
// them); only schema violations are fatal.
func Load(path string) (*Config, error) {
	cfg, _, err := Decode(path)
	if err != nil {
		return nil, err
	}

	if err := validate(cfg); err != nil {
		return nil, err
	}

	if cfg.Tui.Autocomplete.Max == 0 {
		cfg.Tui.Autocomplete.Max = DefaultAutocompleteMax
	}
	if cfg.Tui.Colors.Primary == "" {
		cfg.Tui.Colors.Primary = DefaultColorPrimary
	}
	if cfg.Tui.Colors.Secondary == "" {
		cfg.Tui.Colors.Secondary = DefaultColorSecondary
	}
	if cfg.Tui.Colors.Destroy == "" {
		cfg.Tui.Colors.Destroy = DefaultColorDestroy
	}
	if cfg.Log.Level == "" {
		cfg.Log.Level = DefaultLogLevel
	}
	return cfg, nil
}

// ParseLogLevel maps a config/flag level name to a slog.Level. It accepts
// the slog vocabulary (debug/info/warn/error) case-insensitively.
func ParseLogLevel(s string) (slog.Level, error) {
	var l slog.Level
	if err := l.UnmarshalText([]byte(strings.ToUpper(strings.TrimSpace(s)))); err != nil {
		return 0, fmt.Errorf("invalid log level %q: want debug, info, warn, or error", s)
	}
	return l, nil
}

// Default returns a Workspace with sensible defaults for the fields the
// wizard fills silently (tmux, max_branch_len, and the repo window the
// Claude panes target). Callers overlay user-supplied values on top.
// No default Claude prompts are set: until the user configures a
// from_issue / from_pr claude_prompt, those flows launch no Claude pane.
func Default() Workspace {
	return Workspace{
		MaxBranchLen: 20,
		FromIssue:    FromIssue{RepoWindow: "repo"},
		FromPR:       FromPR{RepoWindow: "repo"},
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

// validColor reports whether s is a value lipgloss can render: empty (the
// terminal default), an ANSI palette index 0-255, or a #RGB / #RRGGBB hex
// string.
func validColor(s string) bool {
	if s == "" {
		return true
	}
	if strings.HasPrefix(s, "#") {
		hex := s[1:]
		if len(hex) != 3 && len(hex) != 6 {
			return false
		}
		for _, r := range hex {
			if !((r >= '0' && r <= '9') || (r >= 'a' && r <= 'f') || (r >= 'A' && r <= 'F')) {
				return false
			}
		}
		return true
	}
	n, err := strconv.Atoi(s)
	return err == nil && n >= 0 && n <= 255
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
	if iss := SchemaIssues(cfg); len(iss) > 0 {
		return errors.New(iss[0])
	}
	return nil
}

// SchemaIssues reports every schema-level problem with cfg as a slice of
// human-readable messages, in a stable order (workspaces and color fields
// sorted by name). It is the collect-all counterpart of the fail-fast
// validate that Load runs; `sam config doctor` uses it to surface all
// problems at once. It performs no I/O — filesystem and network checks live
// in the doctor package.
func SchemaIssues(cfg *Config) []string {
	var issues []string

	if len(cfg.Workspaces) == 0 {
		issues = append(issues, "no workspaces configured")
	}

	if cfg.Tui.Autocomplete.Max < 0 {
		issues = append(issues, "tui.autocomplete.max must be >= 0")
	}

	if cfg.Log.Level != "" {
		if _, err := ParseLogLevel(cfg.Log.Level); err != nil {
			issues = append(issues, fmt.Sprintf("log.level: %v", err))
		}
	}

	for _, c := range []struct{ field, val string }{
		{"primary", cfg.Tui.Colors.Primary},
		{"secondary", cfg.Tui.Colors.Secondary},
		{"destroy", cfg.Tui.Colors.Destroy},
		{"foreground", cfg.Tui.Colors.Foreground},
		{"background", cfg.Tui.Colors.Background},
	} {
		if !validColor(c.val) {
			issues = append(issues, fmt.Sprintf(
				"tui.colors.%s %q: must be an ANSI index 0-255, a #hex color, or empty", c.field, c.val))
		}
	}

	names := make([]string, 0, len(cfg.Workspaces))
	for name := range cfg.Workspaces {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		w := cfg.Workspaces[name]
		if w.Repo == "" {
			issues = append(issues, fmt.Sprintf("workspace %q: repo is required", name))
		}
		if w.Worktrees == "" {
			issues = append(issues, fmt.Sprintf("workspace %q: worktrees is required", name))
		}
		if w.Trunk == "" {
			issues = append(issues, fmt.Sprintf("workspace %q: trunk is required", name))
		}
		if w.FromIssue.RepoWindow != "" && !hasWindow(w, w.FromIssue.RepoWindow) {
			issues = append(issues, fmt.Sprintf(
				"workspace %q: from_issue.repo_window %q does not match any tmux window",
				name, w.FromIssue.RepoWindow))
		}
		if w.FromPR.RepoWindow != "" && !hasWindow(w, w.FromPR.RepoWindow) {
			issues = append(issues, fmt.Sprintf(
				"workspace %q: from_pr.repo_window %q does not match any tmux window",
				name, w.FromPR.RepoWindow))
		}
	}
	return issues
}

func hasWindow(w Workspace, name string) bool {
	for _, win := range w.Tmux.Windows {
		if win.Name == name {
			return true
		}
	}
	return false
}
