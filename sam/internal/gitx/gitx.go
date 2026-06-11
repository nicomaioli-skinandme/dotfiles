// Package gitx wraps git subprocess invocations. All functions take the
// target repo as their first argument and shell out via `git -C <repo>`.
package gitx

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"sort"
	"strings"
)

func run(repo string, args ...string) (string, error) {
	cmd := exec.Command("git", append([]string{"-C", repo}, args...)...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			return "", fmt.Errorf("git %s: %w", strings.Join(args, " "), err)
		}
		return "", fmt.Errorf("git %s: %w: %s", strings.Join(args, " "), err, msg)
	}
	return stdout.String(), nil
}

func Fetch(repo string) error {
	_, err := run(repo, "fetch", "origin")
	return err
}

// IsRepo reports whether path is the work tree of a git repository.
func IsRepo(path string) bool {
	_, err := run(path, "rev-parse", "--git-dir")
	return err == nil
}

// OriginRepo returns the `owner/name` slug derived from the `origin`
// remote URL. Supports both HTTPS (https://github.com/owner/repo[.git])
// and SSH (git@github.com:owner/repo[.git]) forms. Returns ("", nil)
// when there's no origin remote configured.
func OriginRepo(repo string) (string, error) {
	out, err := run(repo, "remote", "get-url", "origin")
	if err != nil {
		// `git remote get-url` returns non-zero when origin is unset;
		// caller treats "no origin" as a soft miss, not a hard error.
		return "", nil
	}
	url := strings.TrimSpace(out)
	url = strings.TrimSuffix(url, ".git")
	if i := strings.Index(url, "github.com"); i >= 0 {
		rest := url[i+len("github.com"):]
		rest = strings.TrimLeft(rest, ":/")
		return rest, nil
	}
	return "", nil
}

// DefaultBranch returns the branch that `origin/HEAD` points at (the
// repo's default branch as seen by origin). Returns ("", nil) when
// origin/HEAD isn't set locally, so the caller can fall through to a
// prompt.
func DefaultBranch(repo string) (string, error) {
	out, err := run(repo, "symbolic-ref", "refs/remotes/origin/HEAD")
	if err != nil {
		return "", nil
	}
	ref := strings.TrimSpace(out)
	return strings.TrimPrefix(ref, "refs/remotes/origin/"), nil
}

func CurrentBranch(repo string) (string, error) {
	out, err := run(repo, "symbolic-ref", "--short", "HEAD")
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(out), nil
}

// FastForwardTrunk fast-forwards to origin/<trunk> only when the
// working tree is currently on <trunk>. If it isn't, no-op — the
// caller has work in flight on another branch.
func FastForwardTrunk(repo, trunk string) error {
	cur, err := CurrentBranch(repo)
	if err != nil {
		return err
	}
	if cur != trunk {
		return nil
	}
	_, err = run(repo, "merge", "--ff-only", "origin/"+trunk)
	return err
}

// BranchesByRecency returns local and origin/* branches sorted by most
// recent commit. Symbolic refs (e.g. origin/HEAD) are skipped, the
// `origin/` prefix is stripped, and duplicates between local and remote
// are collapsed while preserving first-seen order.
func BranchesByRecency(repo string) ([]string, error) {
	out, err := run(repo,
		"for-each-ref",
		"--sort=-committerdate",
		"refs/heads", "refs/remotes/origin",
		"--format=%(if)%(symref)%(then)%(else)%(refname:short)%(end)",
	)
	if err != nil {
		return nil, err
	}
	seen := map[string]struct{}{}
	var result []string
	for _, line := range strings.Split(out, "\n") {
		ref := strings.TrimSpace(line)
		if ref == "" {
			continue
		}
		name := strings.TrimPrefix(ref, "origin/")
		if _, dup := seen[name]; dup {
			continue
		}
		seen[name] = struct{}{}
		result = append(result, name)
	}
	return result, nil
}

func WorktreeAdd(repo, path, branch string) error {
	_, err := run(repo, "worktree", "add", path, branch)
	return err
}

// WorktreeAddNewBranch creates a new branch at start and checks it out in a
// new worktree at path (git worktree add -b … --no-track). --no-track keeps
// the new branch from adopting start's upstream (e.g. origin/<trunk>): a
// brand-new branch has no upstream until the user pushes it.
func WorktreeAddNewBranch(repo, path, branch, start string) error {
	_, err := run(repo, "worktree", "add", "-b", branch, "--no-track", path, start)
	return err
}

func WorktreeRemoveForce(repo, path string) error {
	_, err := run(repo, "worktree", "remove", "--force", path)
	return err
}

// Worktrees returns the basenames of immediate subdirectories of root,
// sorted lexicographically. Non-directory entries are skipped. A
// missing root is treated as empty (no error).
func Worktrees(root string) ([]string, error) {
	entries, err := os.ReadDir(root)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		names = append(names, e.Name())
	}
	sort.Strings(names)
	return names, nil
}

func RevParse(repo, ref string) (string, error) {
	out, err := run(repo, "rev-parse", ref)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(out), nil
}

// PushRefspec pushes <src> to refs/heads/<dst> on origin. Used by the
// branch-rename flow: push the existing SHA under the new name, then
// delete the old ref.
func PushRefspec(repo, src, dst string) error {
	_, err := run(repo, "push", "origin", src+":refs/heads/"+dst)
	return err
}

func PushDelete(repo, branch string) error {
	_, err := run(repo, "push", "origin", "--delete", branch)
	return err
}
