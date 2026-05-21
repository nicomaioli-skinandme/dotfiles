package config

import (
	"path/filepath"
	"testing"
)

func twoProjectCfg(t *testing.T, repoA, repoB string) *Config {
	t.Helper()
	return &Config{
		DefaultProject: "a",
		Projects: map[string]Project{
			"a": {
				Repo:       repoA,
				Worktrees:  repoA + ".worktrees",
				MainBranch: "main",
			},
			"b": {
				Repo:       repoB,
				Worktrees:  repoB + ".worktrees",
				MainBranch: "main",
			},
		},
	}
}

func TestResolve_CwdMatchesRepo(t *testing.T) {
	dir := t.TempDir()
	repoA := filepath.Join(dir, "a")
	repoB := filepath.Join(dir, "b")
	cfg := twoProjectCfg(t, repoA, repoB)

	name, _, err := Resolve(cfg, "", repoB)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if name != "b" {
		t.Errorf("cwd in repo b should pick b; got %q", name)
	}
}

func TestResolve_CwdInsideWorktrees(t *testing.T) {
	dir := t.TempDir()
	repoA := filepath.Join(dir, "a")
	repoB := filepath.Join(dir, "b")
	cfg := twoProjectCfg(t, repoA, repoB)

	cwd := filepath.Join(repoB+".worktrees", "some-branch", "nested")
	name, _, err := Resolve(cfg, "", cwd)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if name != "b" {
		t.Errorf("cwd in b's worktrees should pick b; got %q", name)
	}
}

func TestResolve_CwdInsideRepoSubdir(t *testing.T) {
	dir := t.TempDir()
	repoA := filepath.Join(dir, "a")
	repoB := filepath.Join(dir, "b")
	cfg := twoProjectCfg(t, repoA, repoB)

	cwd := filepath.Join(repoA, "internal", "sub")
	_, _, err := Resolve(cfg, "", cwd)
	if err == nil {
		t.Fatal("expected ErrInsideRepo")
	}
	if !IsInsideRepo(err) {
		t.Fatalf("want ErrInsideRepo, got %T: %v", err, err)
	}
}

func TestResolve_FlagOverridesCwd(t *testing.T) {
	dir := t.TempDir()
	repoA := filepath.Join(dir, "a")
	repoB := filepath.Join(dir, "b")
	cfg := twoProjectCfg(t, repoA, repoB)

	name, _, err := Resolve(cfg, "a", repoB)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if name != "a" {
		t.Errorf("flag should override cwd; got %q", name)
	}
}

func TestResolve_DefaultFallbackWhenNoCwdMatch(t *testing.T) {
	dir := t.TempDir()
	repoA := filepath.Join(dir, "a")
	repoB := filepath.Join(dir, "b")
	cfg := twoProjectCfg(t, repoA, repoB)

	other := filepath.Join(dir, "elsewhere")
	name, _, err := Resolve(cfg, "", other)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if name != "a" {
		t.Errorf("fallback should pick default_project; got %q", name)
	}
}
