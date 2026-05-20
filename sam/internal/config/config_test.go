package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const fullAndbegin = `
default_project = "andbegin"

[projects.andbegin]
repo            = "~/Code/andbegin-monorepo"
worktrees       = "~/Code/andbegin-monorepo.worktrees"
main_branch     = "master"
branch_repo     = "skinandme/andbegin-monorepo"
max_branch_len  = 20

[projects.andbegin.gh_project]
owner             = "skinandmeprojects"
number            = 45
id                = "PVT_kwDOBLqSu84AVoEL"
status_field_id   = "PVTSSF_lADOBLqSu84AVoELzgN0d5A"
in_progress_id    = "15c99605"
issue_repos       = ["skinandmeprojects/andbegin", "skinandmeprojects/projects"]
backlog_statuses  = ["Backlog", "Platform Backlog"]

[projects.andbegin.from_issue]
claude_prompt     = "/plan pull the context from {{ .IssueURL }}"
claude_pane_title = "IMPL {{ .IssueTitle }}"
repo_window       = "repo"

[[projects.andbegin.tmux.windows]]
name = "repo"
cwd  = "."

[[projects.andbegin.tmux.windows]]
name = "local"
cwd  = "backend"
  [[projects.andbegin.tmux.windows.panes]]
  split = "h"
  cwd   = "store-ui"

[[projects.andbegin.tmux.windows]]
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
	if cfg.DefaultProject != "andbegin" {
		t.Errorf("default_project: got %q", cfg.DefaultProject)
	}
	p, ok := cfg.Projects["andbegin"]
	if !ok {
		t.Fatal("project andbegin missing")
	}
	home, _ := os.UserHomeDir()
	wantRepo := filepath.Join(home, "Code/andbegin-monorepo")
	if p.Repo != wantRepo {
		t.Errorf("repo: got %q want %q", p.Repo, wantRepo)
	}
	wantWt := filepath.Join(home, "Code/andbegin-monorepo.worktrees")
	if p.Worktrees != wantWt {
		t.Errorf("worktrees: got %q want %q", p.Worktrees, wantWt)
	}
	if p.MaxBranchLen != 20 {
		t.Errorf("max_branch_len: got %d", p.MaxBranchLen)
	}
	if p.GhProject.Number != 45 {
		t.Errorf("gh_project.number: got %d", p.GhProject.Number)
	}
	if len(p.GhProject.IssueRepos) != 2 {
		t.Errorf("issue_repos: got %v", p.GhProject.IssueRepos)
	}
	if p.FromIssue.RepoWindow != "repo" {
		t.Errorf("repo_window: got %q", p.FromIssue.RepoWindow)
	}
	if got, want := len(p.Tmux.Windows), 3; got != want {
		t.Fatalf("windows: got %d want %d", got, want)
	}
	if p.Tmux.Windows[1].Name != "local" || len(p.Tmux.Windows[1].Panes) != 1 {
		t.Errorf("local window malformed: %+v", p.Tmux.Windows[1])
	}
	if p.Tmux.Windows[1].Panes[0].Split != "h" {
		t.Errorf("local pane split: got %q", p.Tmux.Windows[1].Panes[0].Split)
	}
}

func TestLoad_MissingRequiredField(t *testing.T) {
	body := `
[projects.andbegin]
worktrees   = "~/wt"
main_branch = "master"
`
	path := writeConfig(t, body)
	_, err := Load(path)
	if err == nil {
		t.Fatal("expected error for missing repo")
	}
	if !strings.Contains(err.Error(), "andbegin") || !strings.Contains(err.Error(), "repo") {
		t.Errorf("error should mention project and field: %v", err)
	}
}

func TestResolve_SingleProjectNoDefault(t *testing.T) {
	body := `
[projects.solo]
repo        = "/x"
worktrees   = "/y"
main_branch = "main"
`
	path := writeConfig(t, body)
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	name, p, err := Resolve(cfg, "")
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if name != "solo" || p == nil {
		t.Errorf("Resolve: got %q %v", name, p)
	}
}

func TestLoad_UndefinedDefault(t *testing.T) {
	body := `
default_project = "ghost"

[projects.andbegin]
repo        = "/x"
worktrees   = "/y"
main_branch = "main"
`
	path := writeConfig(t, body)
	_, err := Load(path)
	if err == nil {
		t.Fatal("expected error for undefined default_project")
	}
	if !strings.Contains(err.Error(), "ghost") {
		t.Errorf("error should mention undefined default: %v", err)
	}
}

func TestLoad_RepoWindowMismatch(t *testing.T) {
	body := `
[projects.andbegin]
repo        = "/x"
worktrees   = "/y"
main_branch = "main"

[projects.andbegin.from_issue]
repo_window = "nope"

[[projects.andbegin.tmux.windows]]
name = "repo"
cwd  = "."

[[projects.andbegin.tmux.windows]]
name = "local"
cwd  = "backend"

[[projects.andbegin.tmux.windows]]
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
default_project = "andbegin"

[projects.andbegin]
repo        = "/a"
worktrees   = "/wa"
main_branch = "main"

[projects.other]
repo        = "/b"
worktrees   = "/wb"
main_branch = "main"
`
	path := writeConfig(t, body)
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	name, _, err := Resolve(cfg, "other")
	if err != nil {
		t.Fatalf("Resolve(other): %v", err)
	}
	if name != "other" {
		t.Errorf("explicit flag should win: got %q", name)
	}
	if _, _, err := Resolve(cfg, "ghost"); err == nil {
		t.Error("expected error for undefined --project")
	}
}

func TestLoad_MultiProjectNoDefault(t *testing.T) {
	body := `
[projects.a]
repo        = "/a"
worktrees   = "/wa"
main_branch = "main"

[projects.b]
repo        = "/b"
worktrees   = "/wb"
main_branch = "main"
`
	path := writeConfig(t, body)
	_, err := Load(path)
	if err == nil {
		t.Fatal("expected error: multiple projects without default")
	}
	if !strings.Contains(err.Error(), "default_project") {
		t.Errorf("error should mention default_project: %v", err)
	}
}
