// Package doctor inspects an active sam config and reports every problem it
// finds without mutating anything — the engine behind `sam config doctor`.
// It layers filesystem and gh/network checks on top of config.SchemaIssues
// and config.Decode's unknown-key detection, so it lives outside the config
// package (which stays free of gitx/ghx and does no I/O).
package doctor

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/nicomaioli-skinandme/dotfiles/sam/internal/config"
	"github.com/nicomaioli-skinandme/dotfiles/sam/internal/ghx"
	"github.com/nicomaioli-skinandme/dotfiles/sam/internal/gitx"
)

// Report is the outcome of a check run: the config path inspected and every
// issue found (empty when the config is healthy).
type Report struct {
	Path   string
	Issues []string
}

// OK reports whether the config is free of issues.
func (r Report) OK() bool { return len(r.Issues) == 0 }

// Run inspects the config at path and returns all problems found, without
// mutating anything. Local (offline) checks run first; when the file parses,
// gh/network checks follow. Network failures — including being offline — are
// reported as issues rather than aborting the run.
func Run(path string) Report {
	cfg, issues := localIssues(path)
	if cfg != nil {
		issues = append(issues, networkIssues(cfg)...)
	}
	return Report{Path: path, Issues: issues}
}

// localIssues performs the offline checks: file presence, TOML parse,
// unknown keys, schema validity, and filesystem reachability. It returns the
// decoded config (nil when the file is absent or unparseable) so the caller
// can decide whether to run the network checks.
func localIssues(path string) (*config.Config, []string) {
	if _, err := os.Stat(path); err != nil {
		if os.IsNotExist(err) {
			return nil, []string{fmt.Sprintf("no config file at %s — run `sam` to create one, or hand-write it", path)}
		}
		return nil, []string{fmt.Sprintf("cannot read %s: %v", path, err)}
	}

	cfg, unknown, err := config.Decode(path)
	if err != nil {
		// Read/parse failure: nothing further can be checked.
		return nil, []string{err.Error()}
	}

	var issues []string
	keys := append([]string(nil), unknown...)
	sort.Strings(keys)
	for _, k := range keys {
		issues = append(issues, fmt.Sprintf("unknown key %q", k))
	}
	issues = append(issues, config.SchemaIssues(cfg)...)
	issues = append(issues, filesystemIssues(cfg)...)
	return cfg, issues
}

// filesystemIssues checks that each workspace points at things that exist on
// disk: the repo is a git repository, the worktrees parent directory exists
// (sam creates the worktrees dir itself, but its parent must be there), and a
// path-shaped worktree_setup hook resolves to a real file.
func filesystemIssues(cfg *config.Config) []string {
	var issues []string
	for _, name := range workspaceNames(cfg) {
		w := cfg.Workspaces[name]
		if w.Repo != "" && !gitx.IsRepo(w.Repo) {
			issues = append(issues, fmt.Sprintf("workspace %q: repo %s is not a git repository", name, w.Repo))
		}
		if w.Worktrees != "" {
			parent := filepath.Dir(w.Worktrees)
			if fi, err := os.Stat(parent); err != nil || !fi.IsDir() {
				issues = append(issues, fmt.Sprintf(
					"workspace %q: worktrees parent %s does not exist", name, parent))
			}
		}
		if p := scriptPath(w.WorktreeSetup); p != "" {
			if _, err := os.Stat(p); err != nil {
				issues = append(issues, fmt.Sprintf(
					"workspace %q: worktree_setup script %s does not exist", name, p))
			}
		}
	}
	return issues
}

// networkIssues performs the gh-backed checks: the gh CLI has the scopes the
// config needs (always `repo`; `project` when any workspace configures a
// GitHub Project), and each configured gh_project resolves.
func networkIssues(cfg *config.Config) []string {
	var issues []string

	need := []string{"repo"}
	for _, w := range cfg.Workspaces {
		if hasGhProject(w) {
			need = append(need, "project")
			break
		}
	}
	if scopes, err := ghx.AuthScopes(); err != nil {
		issues = append(issues, fmt.Sprintf("gh auth: %v", err))
	} else {
		have := make(map[string]bool, len(scopes))
		for _, s := range scopes {
			have[s] = true
		}
		for _, s := range need {
			if !have[s] {
				issues = append(issues, fmt.Sprintf(
					"gh is missing the %q scope (run: gh auth refresh -s %s)", s, s))
			}
		}
	}

	for _, name := range workspaceNames(cfg) {
		w := cfg.Workspaces[name]
		if !hasGhProject(w) {
			continue
		}
		if w.GhProject.Owner == "" || w.GhProject.Number == 0 {
			issues = append(issues, fmt.Sprintf(
				"workspace %q: gh_project needs both owner and number", name))
			continue
		}
		if _, err := ghx.ProjectMeta(w.GhProject.Owner, w.GhProject.Number); err != nil {
			issues = append(issues, fmt.Sprintf("workspace %q: gh_project %s/#%d does not resolve: %v",
				name, w.GhProject.Owner, w.GhProject.Number, err))
		}
	}
	return issues
}

func hasGhProject(w config.Workspace) bool {
	return w.GhProject.Owner != "" || w.GhProject.Number != 0
}

// scriptPath returns the (~-expanded) path when s looks like a filesystem
// path rather than an inline shell command, else "". worktree_setup may be
// either; only path-shaped values (no spaces, with a path prefix) are worth
// a file-existence check.
func scriptPath(s string) string {
	s = strings.TrimSpace(s)
	if s == "" || strings.ContainsAny(s, " \t") {
		return ""
	}
	switch {
	case strings.HasPrefix(s, "/"), strings.HasPrefix(s, "./"), strings.HasPrefix(s, "../"):
		return s
	case strings.HasPrefix(s, "~/"):
		if home, err := os.UserHomeDir(); err == nil {
			return filepath.Join(home, s[2:])
		}
	}
	return ""
}

func workspaceNames(cfg *config.Config) []string {
	names := make([]string, 0, len(cfg.Workspaces))
	for name := range cfg.Workspaces {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}
