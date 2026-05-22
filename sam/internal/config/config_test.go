package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const fullAndbegin = `
default_workspace = "andbegin"

[workspaces.andbegin]
repo            = "~/Code/andbegin-monorepo"
worktrees       = "~/Code/andbegin-monorepo.worktrees"
main_branch     = "master"
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
	if cfg.DefaultWorkspace != "andbegin" {
		t.Errorf("default_workspace: got %q", cfg.DefaultWorkspace)
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
main_branch = "master"
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

func TestResolve_SingleWorkspaceNoDefault(t *testing.T) {
	body := `
[workspaces.solo]
repo        = "/x"
worktrees   = "/y"
main_branch = "main"
`
	path := writeConfig(t, body)
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	name, w, err := Resolve(cfg, "", "")
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if name != "solo" || w == nil {
		t.Errorf("Resolve: got %q %v", name, w)
	}
}

func TestLoad_UndefinedDefault(t *testing.T) {
	body := `
default_workspace = "ghost"

[workspaces.andbegin]
repo        = "/x"
worktrees   = "/y"
main_branch = "main"
`
	path := writeConfig(t, body)
	_, err := Load(path)
	if err == nil {
		t.Fatal("expected error for undefined default_workspace")
	}
	if !strings.Contains(err.Error(), "ghost") {
		t.Errorf("error should mention undefined default: %v", err)
	}
}

func TestLoad_RepoWindowMismatch(t *testing.T) {
	body := `
[workspaces.andbegin]
repo        = "/x"
worktrees   = "/y"
main_branch = "main"

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

func TestResolve_ExplicitFlag(t *testing.T) {
	body := `
default_workspace = "andbegin"

[workspaces.andbegin]
repo        = "/a"
worktrees   = "/wa"
main_branch = "main"

[workspaces.other]
repo        = "/b"
worktrees   = "/wb"
main_branch = "main"
`
	path := writeConfig(t, body)
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	name, _, err := Resolve(cfg, "other", "")
	if err != nil {
		t.Fatalf("Resolve(other): %v", err)
	}
	if name != "other" {
		t.Errorf("explicit flag should win: got %q", name)
	}
	if _, _, err := Resolve(cfg, "ghost", ""); err == nil {
		t.Error("expected error for undefined --workspace")
	}
}

func TestResolve_MultiWorkspaceNoDefaultNoCwd(t *testing.T) {
	body := `
[workspaces.a]
repo        = "/a"
worktrees   = "/wa"
main_branch = "main"

[workspaces.b]
repo        = "/b"
worktrees   = "/wb"
main_branch = "main"
`
	path := writeConfig(t, body)
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if _, _, err := Resolve(cfg, "", ""); err == nil {
		t.Fatal("expected resolve error when no flag, no default, no cwd match")
	}
}
