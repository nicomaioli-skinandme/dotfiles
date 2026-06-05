package workspace

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/nicomaioli-skinandme/dotfiles/sam/internal/config"
)

// writeConfig writes body to a temp config.toml and returns its path.
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

// These exercise the config.Load → Service.Resolve path end-to-end (the
// hand-built-config cases live in resolve_test.go).

func TestResolve_SingleWorkspaceNoDefault(t *testing.T) {
	body := `
[workspaces.solo]
repo        = "/x"
worktrees   = "/y"
trunk = "main"
`
	cfg, err := config.Load(writeConfig(t, body))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	got, err := Service{}.Resolve(cfg, "", "")
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if got == nil || got.Name != "solo" {
		t.Errorf("Resolve: got %+v", got)
	}
}

func TestResolve_ExplicitFlag(t *testing.T) {
	body := `
[workspaces.andbegin]
repo        = "/a"
worktrees   = "/wa"
trunk = "main"

[workspaces.other]
repo        = "/b"
worktrees   = "/wb"
trunk = "main"
`
	cfg, err := config.Load(writeConfig(t, body))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	got, err := Service{}.Resolve(cfg, "other", "")
	if err != nil {
		t.Fatalf("Resolve(other): %v", err)
	}
	if got == nil || got.Name != "other" {
		t.Errorf("explicit flag should win: got %+v", got)
	}
	if _, err := (Service{}).Resolve(cfg, "ghost", ""); err == nil {
		t.Error("expected error for undefined --workspace")
	}
}

func TestResolve_MultiWorkspaceNoCwdMatch(t *testing.T) {
	body := `
[workspaces.a]
repo        = "/a"
worktrees   = "/wa"
trunk = "main"

[workspaces.b]
repo        = "/b"
worktrees   = "/wb"
trunk = "main"
`
	cfg, err := config.Load(writeConfig(t, body))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	got, err := Service{}.Resolve(cfg, "", "")
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if got != nil {
		t.Errorf("expected unresolved (nil); got %+v", got)
	}
}
