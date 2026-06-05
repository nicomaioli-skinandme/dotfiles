package workspace

import (
	"path/filepath"
	"testing"

	"github.com/nicomaioli-skinandme/dotfiles/sam/internal/config"
)

func twoWorkspaceCfg(t *testing.T, repoA, repoB string) *config.Config {
	t.Helper()
	return &config.Config{
		Workspaces: map[string]config.Workspace{
			"a": {
				Repo:      repoA,
				Worktrees: repoA + ".worktrees",
				Trunk:     "main",
			},
			"b": {
				Repo:      repoB,
				Worktrees: repoB + ".worktrees",
				Trunk:     "main",
			},
		},
	}
}

func TestResolve_CwdMatchesRepo(t *testing.T) {
	dir := t.TempDir()
	repoA := filepath.Join(dir, "a")
	repoB := filepath.Join(dir, "b")
	cfg := twoWorkspaceCfg(t, repoA, repoB)

	got, err := Service{}.Resolve(cfg, "", repoB)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if got == nil || got.Name != "b" {
		t.Errorf("cwd in repo b should pick b; got %+v", got)
	}
}

func TestResolve_CwdInsideWorktrees(t *testing.T) {
	dir := t.TempDir()
	repoA := filepath.Join(dir, "a")
	repoB := filepath.Join(dir, "b")
	cfg := twoWorkspaceCfg(t, repoA, repoB)

	cwd := filepath.Join(repoB+".worktrees", "some-branch", "nested")
	got, err := Service{}.Resolve(cfg, "", cwd)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if got == nil || got.Name != "b" {
		t.Errorf("cwd in b's worktrees should pick b; got %+v", got)
	}
}

func TestResolve_CwdInsideRepoSubdir(t *testing.T) {
	dir := t.TempDir()
	repoA := filepath.Join(dir, "a")
	repoB := filepath.Join(dir, "b")
	cfg := twoWorkspaceCfg(t, repoA, repoB)

	cwd := filepath.Join(repoA, "internal", "sub")
	_, err := Service{}.Resolve(cfg, "", cwd)
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
	cfg := twoWorkspaceCfg(t, repoA, repoB)

	got, err := Service{}.Resolve(cfg, "a", repoB)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if got == nil || got.Name != "a" {
		t.Errorf("flag should override cwd; got %+v", got)
	}
}

func TestResolve_UnresolvedReturnsNil(t *testing.T) {
	dir := t.TempDir()
	repoA := filepath.Join(dir, "a")
	repoB := filepath.Join(dir, "b")
	cfg := twoWorkspaceCfg(t, repoA, repoB)

	other := filepath.Join(dir, "elsewhere")
	got, err := Service{}.Resolve(cfg, "", other)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if got != nil {
		t.Errorf("expected unresolved (nil); got %+v", got)
	}
}

func TestResolve_SingleWorkspaceShortcut(t *testing.T) {
	dir := t.TempDir()
	repoA := filepath.Join(dir, "a")
	cfg := &config.Config{
		Workspaces: map[string]config.Workspace{
			"only": {
				Repo:      repoA,
				Worktrees: repoA + ".worktrees",
				Trunk:     "main",
			},
		},
	}

	other := filepath.Join(dir, "elsewhere")
	got, err := Service{}.Resolve(cfg, "", other)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if got == nil || got.Name != "only" || got.WS == nil {
		t.Errorf("single-workspace shortcut should pick the only workspace; got %+v", got)
	}
}
