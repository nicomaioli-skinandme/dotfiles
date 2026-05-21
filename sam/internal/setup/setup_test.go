package setup

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nicomaioli-skinandme/dotfiles/sam/internal/config"
)

// initRepo creates a tiny git repo at dir with one commit on branch
// `main` so we can spawn worktrees off it.
func initRepo(t *testing.T, dir string) {
	t.Helper()
	run := func(args ...string) {
		cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
		var out bytes.Buffer
		cmd.Stdout = &out
		cmd.Stderr = &out
		if err := cmd.Run(); err != nil {
			t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out.String())
		}
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	run("init", "-b", "main")
	run("config", "user.email", "test@example.com")
	run("config", "user.name", "Test")
	if err := os.WriteFile(filepath.Join(dir, "README"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	run("add", ".")
	run("commit", "-m", "initial")
	run("branch", "feature")
}

func TestCreateWorktree_NoHook(t *testing.T) {
	root := t.TempDir()
	repo := filepath.Join(root, "repo")
	wts := filepath.Join(root, "wts")
	initRepo(t, repo)

	proj := &config.Project{Repo: repo, Worktrees: wts, MainBranch: "main"}
	path, err := CreateWorktree(proj, "feature", 0, "demo")
	if err != nil {
		t.Fatalf("CreateWorktree: %v", err)
	}
	if path != filepath.Join(wts, "feature") {
		t.Errorf("path: got %q", path)
	}
	if _, err := os.Stat(path); err != nil {
		t.Errorf("worktree dir missing: %v", err)
	}
}

func TestCreateWorktree_HookRunsWithEnv(t *testing.T) {
	root := t.TempDir()
	repo := filepath.Join(root, "repo")
	wts := filepath.Join(root, "wts")
	initRepo(t, repo)

	// The hook writes the SAM_* env vars and pwd to a file in the
	// worktree dir so we can assert on them.
	hookOut := filepath.Join(root, "hook.out")
	// Use printf rather than printenv so unset vars surface as empty
	// lines instead of being skipped (macOS `printenv` exits 1 and
	// drops unset names silently).
	hook := `printf '%s\n%s\n%s\n%s\n%s\n%s\n' "$SAM_BRANCH" "$SAM_WORKTREE" "$SAM_REPO" "$SAM_PROJECT" "$SAM_ISSUE_NUMBER" "$(pwd)" > ` + hookOut

	proj := &config.Project{
		Repo:          repo,
		Worktrees:     wts,
		MainBranch:    "main",
		WorktreeSetup: hook,
	}
	path, err := CreateWorktree(proj, "feature", 42, "demo")
	if err != nil {
		t.Fatalf("CreateWorktree: %v", err)
	}

	body, err := os.ReadFile(hookOut)
	if err != nil {
		t.Fatalf("read hook output: %v", err)
	}
	lines := strings.Split(strings.TrimSpace(string(body)), "\n")
	want := []string{"feature", path, repo, "demo", "42", path}
	if len(lines) != len(want) {
		t.Fatalf("hook output: got %d lines, want %d:\n%s", len(lines), len(want), body)
	}
	for i, w := range want {
		// On macOS /var is a symlink to /private/var; resolve both
		// sides to a canonical form before comparing path-shaped
		// values.
		got := lines[i]
		if w == path || w == repo {
			gr, _ := filepath.EvalSymlinks(got)
			wr, _ := filepath.EvalSymlinks(w)
			if gr == wr && gr != "" {
				continue
			}
		}
		if got != w {
			t.Errorf("line %d: got %q want %q", i, got, w)
		}
	}
}

func TestCreateWorktree_HookFailureBubblesUp(t *testing.T) {
	root := t.TempDir()
	repo := filepath.Join(root, "repo")
	wts := filepath.Join(root, "wts")
	initRepo(t, repo)

	proj := &config.Project{
		Repo:          repo,
		Worktrees:     wts,
		MainBranch:    "main",
		WorktreeSetup: "exit 7",
	}
	_, err := CreateWorktree(proj, "feature", 0, "demo")
	if err == nil {
		t.Fatal("expected error from failing hook")
	}
	if !strings.Contains(err.Error(), "worktree_setup hook failed") {
		t.Errorf("error should mention worktree_setup: %v", err)
	}
	// Worktree dir should still exist for the user to inspect.
	if _, statErr := os.Stat(filepath.Join(wts, "feature")); statErr != nil {
		t.Errorf("worktree dir should remain after hook failure: %v", statErr)
	}
}

func TestCreateWorktree_IssueZeroLeavesEnvEmpty(t *testing.T) {
	root := t.TempDir()
	repo := filepath.Join(root, "repo")
	wts := filepath.Join(root, "wts")
	initRepo(t, repo)

	hookOut := filepath.Join(root, "issue.out")
	proj := &config.Project{
		Repo:          repo,
		Worktrees:     wts,
		MainBranch:    "main",
		WorktreeSetup: `printf "%s" "$SAM_ISSUE_NUMBER" > ` + hookOut,
	}
	if _, err := CreateWorktree(proj, "feature", 0, "demo"); err != nil {
		t.Fatalf("CreateWorktree: %v", err)
	}
	body, err := os.ReadFile(hookOut)
	if err != nil {
		t.Fatal(err)
	}
	if string(body) != "" {
		t.Errorf("SAM_ISSUE_NUMBER should be empty when issueNumber=0; got %q", body)
	}
}
