package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const fullAndbegin = `
[workspaces.andbegin]
repo            = "~/Code/andbegin-monorepo"
worktrees       = "~/Code/andbegin-monorepo.worktrees"
trunk     = "master"
branch_repo     = "skinandme/andbegin-monorepo"
max_branch_len  = 20

[workspaces.andbegin.gh_project]
owner             = "skinandmeprojects"
number            = 45
id                = "PVT_kwDOBLqSu84AVoEL"
status_field_id   = "PVTSSF_lADOBLqSu84AVoELzgN0d5A"
in_progress_id    = "15c99605"
issue_repos       = ["skinandmeprojects/andbegin", "skinandmeprojects/projects"]
backlog_statuses  = ["Backlog", "Platform Backlog"]

[workspaces.andbegin.from_issue]
claude_prompt     = "/plan pull the context from {{ .IssueURL }}"
claude_pane_title = "IMPL {{ .IssueTitle }}"
repo_window       = "repo"

[[workspaces.andbegin.tmux.windows]]
name = "repo"
cwd  = "."

[[workspaces.andbegin.tmux.windows]]
name = "local"
cwd  = "backend"
  [[workspaces.andbegin.tmux.windows.panes]]
  split = "h"
  cwd   = "store-ui"

[[workspaces.andbegin.tmux.windows]]
name = "uat"
cwd  = "deployment/uat"
`

func writeConfig(t *testing.T, body string) string {
	t.Helper()
	t.Setenv("HOME", t.TempDir())
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatalf("write tmp config: %v", err)
	}
	return path
}

func TestLoad_HappyPath(t *testing.T) {
	path := writeConfig(t, fullAndbegin)
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	w, ok := cfg.Workspaces["andbegin"]
	if !ok {
		t.Fatal("workspace andbegin missing")
	}
	home, _ := os.UserHomeDir()
	wantRepo := filepath.Join(home, "Code/andbegin-monorepo")
	if w.Repo != wantRepo {
		t.Errorf("repo: got %q want %q", w.Repo, wantRepo)
	}
	wantWt := filepath.Join(home, "Code/andbegin-monorepo.worktrees")
	if w.Worktrees != wantWt {
		t.Errorf("worktrees: got %q want %q", w.Worktrees, wantWt)
	}
	if w.MaxBranchLen != 20 {
		t.Errorf("max_branch_len: got %d", w.MaxBranchLen)
	}
	if w.GhProject.Number != 45 {
		t.Errorf("gh_project.number: got %d", w.GhProject.Number)
	}
	if len(w.GhProject.IssueRepos) != 2 {
		t.Errorf("issue_repos: got %v", w.GhProject.IssueRepos)
	}
	if w.FromIssue.RepoWindow != "repo" {
		t.Errorf("repo_window: got %q", w.FromIssue.RepoWindow)
	}
	if got, want := len(w.Tmux.Windows), 3; got != want {
		t.Fatalf("windows: got %d want %d", got, want)
	}
	if w.Tmux.Windows[1].Name != "local" || len(w.Tmux.Windows[1].Panes) != 1 {
		t.Errorf("local window malformed: %+v", w.Tmux.Windows[1])
	}
	if w.Tmux.Windows[1].Panes[0].Split != "h" {
		t.Errorf("local pane split: got %q", w.Tmux.Windows[1].Panes[0].Split)
	}
}

func TestLoad_MissingRequiredField(t *testing.T) {
	body := `
[workspaces.andbegin]
worktrees   = "~/wt"
trunk = "master"
`
	path := writeConfig(t, body)
	_, err := Load(path)
	if err == nil {
		t.Fatal("expected error for missing repo")
	}
	if !strings.Contains(err.Error(), "andbegin") || !strings.Contains(err.Error(), "repo") {
		t.Errorf("error should mention workspace and field: %v", err)
	}
}

const soloWorkspace = `
[workspaces.solo]
repo        = "/x"
worktrees   = "/y"
trunk = "main"
`

func TestLoad_AutocompleteMaxDefault(t *testing.T) {
	// Absent [tui] section defaults the cap.
	path := writeConfig(t, soloWorkspace)
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Tui.Autocomplete.Max != DefaultAutocompleteMax {
		t.Errorf("default max: got %d want %d", cfg.Tui.Autocomplete.Max, DefaultAutocompleteMax)
	}

	// Explicit value is honored.
	path = writeConfig(t, soloWorkspace+"\n[tui.autocomplete]\nmax = 2\n")
	cfg, err = Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Tui.Autocomplete.Max != 2 {
		t.Errorf("explicit max: got %d want 2", cfg.Tui.Autocomplete.Max)
	}

	// Negative value is rejected.
	path = writeConfig(t, soloWorkspace+"\n[tui.autocomplete]\nmax = -1\n")
	if _, err := Load(path); err == nil {
		t.Fatal("expected error for negative max")
	}
}

func TestLoad_RepoWindowMismatch(t *testing.T) {
	body := `
[workspaces.andbegin]
repo        = "/x"
worktrees   = "/y"
trunk = "main"

[workspaces.andbegin.from_issue]
repo_window = "nope"

[[workspaces.andbegin.tmux.windows]]
name = "repo"
cwd  = "."

[[workspaces.andbegin.tmux.windows]]
name = "local"
cwd  = "backend"

[[workspaces.andbegin.tmux.windows]]
name = "uat"
cwd  = "deployment/uat"
`
	path := writeConfig(t, body)
	_, err := Load(path)
	if err == nil {
		t.Fatal("expected error for repo_window mismatch")
	}
	if !strings.Contains(err.Error(), "repo_window") || !strings.Contains(err.Error(), "nope") {
		t.Errorf("error should mention field and value: %v", err)
	}
}

func TestLoad_FromPRReadsConfig(t *testing.T) {
	// No [from_pr] → nothing injected; the flow simply won't launch Claude.
	path := writeConfig(t, fullAndbegin)
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	w := cfg.Workspaces["andbegin"]
	if w.FromPR.ClaudePrompt != "" || w.FromPR.RepoWindow != "" {
		t.Errorf("from_pr should be empty when unconfigured; got %+v", w.FromPR)
	}

	// An explicit [from_pr] is read verbatim.
	body := fullAndbegin + `
[workspaces.andbegin.from_pr]
claude_prompt     = "/review only"
claude_pane_title = "RV {{ .PRTitle }}"
repo_window       = "uat"
`
	path = writeConfig(t, body)
	cfg, err = Load(path)
	if err != nil {
		t.Fatalf("Load explicit from_pr: %v", err)
	}
	w = cfg.Workspaces["andbegin"]
	if w.FromPR.ClaudePrompt != "/review only" || w.FromPR.RepoWindow != "uat" {
		t.Errorf("explicit from_pr not honored: %+v", w.FromPR)
	}
}

func TestLoad_FromPRRepoWindowMismatch(t *testing.T) {
	body := `
[workspaces.andbegin]
repo        = "/x"
worktrees   = "/y"
trunk = "main"

[workspaces.andbegin.from_pr]
repo_window = "nope"

[[workspaces.andbegin.tmux.windows]]
name = "repo"
cwd  = "."
`
	path := writeConfig(t, body)
	_, err := Load(path)
	if err == nil {
		t.Fatal("expected error for from_pr.repo_window mismatch")
	}
	if !strings.Contains(err.Error(), "from_pr.repo_window") || !strings.Contains(err.Error(), "nope") {
		t.Errorf("error should mention field and value: %v", err)
	}
}

