package doctor

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// gitInit makes dir a git repository (no commit needed — doctor only checks
// `git rev-parse`).
func gitInit(t *testing.T, dir string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("git", "-C", dir, "init", "-b", "main")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git init: %v\n%s", err, out)
	}
}

// writeConfig writes body to a temp config.toml and returns its path.
func writeConfig(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestLocalIssues_Clean(t *testing.T) {
	root := t.TempDir()
	repo := filepath.Join(root, "repo")
	gitInit(t, repo)
	body := "[workspaces.solo]\n" +
		"repo = \"" + repo + "\"\n" +
		"worktrees = \"" + filepath.Join(root, "wt") + "\"\n" +
		"trunk = \"main\"\n"

	cfg, issues := localIssues(writeConfig(t, body))
	if cfg == nil {
		t.Fatal("expected a decoded config")
	}
	if len(issues) != 0 {
		t.Errorf("clean config should have no issues, got: %v", issues)
	}
}

func TestLocalIssues_Problems(t *testing.T) {
	root := t.TempDir()
	notRepo := filepath.Join(root, "plain") // exists but not a git repo
	if err := os.MkdirAll(notRepo, 0o755); err != nil {
		t.Fatal(err)
	}

	cases := []struct {
		name string
		body string
		want string // substring that must appear in some issue
	}{
		{
			name: "unknown key",
			body: "[workspaces.solo]\nrepo=\"" + notRepo + "\"\nworktrees=\"" +
				filepath.Join(root, "wt") + "\"\ntrunk=\"main\"\nbogus=\"x\"\n",
			want: "unknown key",
		},
		{
			name: "missing trunk",
			body: "[workspaces.solo]\nrepo=\"" + notRepo + "\"\nworktrees=\"" +
				filepath.Join(root, "wt") + "\"\n",
			want: "trunk is required",
		},
		{
			name: "invalid color",
			body: "[workspaces.solo]\nrepo=\"" + notRepo + "\"\nworktrees=\"" +
				filepath.Join(root, "wt") + "\"\ntrunk=\"main\"\n[tui.colors]\nprimary=\"notacolor\"\n",
			want: "tui.colors.primary",
		},
		{
			name: "repo not a git repository",
			body: "[workspaces.solo]\nrepo=\"" + notRepo + "\"\nworktrees=\"" +
				filepath.Join(root, "wt") + "\"\ntrunk=\"main\"\n",
			want: "is not a git repository",
		},
		{
			name: "worktrees parent missing",
			body: "[workspaces.solo]\nrepo=\"" + notRepo + "\"\nworktrees=\"" +
				filepath.Join(root, "no", "such", "wt") + "\"\ntrunk=\"main\"\n",
			want: "worktrees parent",
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, issues := localIssues(writeConfig(t, c.body))
			if !containsSubstr(issues, c.want) {
				t.Errorf("issues %v should contain %q", issues, c.want)
			}
		})
	}
}

func TestLocalIssues_MissingFile(t *testing.T) {
	_, issues := localIssues(filepath.Join(t.TempDir(), "absent.toml"))
	if !containsSubstr(issues, "no config file") {
		t.Errorf("missing file should report it, got: %v", issues)
	}
}

func containsSubstr(issues []string, want string) bool {
	for _, i := range issues {
		if strings.Contains(i, want) {
			return true
		}
	}
	return false
}
