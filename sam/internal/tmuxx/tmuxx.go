// Package tmuxx wraps tmux subprocess invocations for session/pane
// manipulation. Layout building is driven by config.Workspace.Tmux.
package tmuxx

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/nicomaioli-skinandme/dotfiles/sam/internal/config"
)

// InTmux reports whether sam is running inside a tmux client. This is
// the sole guard against nesting tmux sessions — any code path that
// would attach a new tmux client to the controlling terminal MUST
// short-circuit when InTmux() is true.
func InTmux() bool {
	return os.Getenv("TMUX") != ""
}

func tmux(args ...string) (string, error) {
	cmd := exec.Command("tmux", args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			return "", fmt.Errorf("tmux %s: %w", strings.Join(args, " "), err)
		}
		return "", fmt.Errorf("tmux %s: %w: %s", strings.Join(args, " "), err, msg)
	}
	return stdout.String(), nil
}

func HasSession(name string) bool {
	cmd := exec.Command("tmux", "has-session", "-t", name)
	return cmd.Run() == nil
}

func resolveCwd(baseDir, rel string) string {
	if rel == "" || rel == "." {
		return baseDir
	}
	if filepath.IsAbs(rel) {
		return rel
	}
	return filepath.Join(baseDir, rel)
}

// BuildSession creates `name` and applies workspace.Tmux.Windows. The
// first window is created with new-session; subsequent windows via
// new-window. Each pane is added with split-window.
func BuildSession(name string, workspace *config.Workspace, baseDir string) error {
	if len(workspace.Tmux.Windows) == 0 {
		return fmt.Errorf("workspace has no tmux windows configured")
	}
	for i, w := range workspace.Tmux.Windows {
		winCwd := resolveCwd(baseDir, w.Cwd)
		if i == 0 {
			if _, err := tmux("new-session", "-d", "-s", name, "-n", w.Name, "-c", winCwd); err != nil {
				return err
			}
		} else {
			if _, err := tmux("new-window", "-t", name, "-n", w.Name, "-c", winCwd); err != nil {
				return err
			}
		}
		for _, p := range w.Panes {
			paneCwd := resolveCwd(baseDir, p.Cwd)
			splitFlag := "-h"
			if p.Split == "v" {
				splitFlag = "-v"
			}
			if _, err := tmux("split-window", splitFlag, "-t", name+":"+w.Name, "-c", paneCwd); err != nil {
				return err
			}
		}
	}
	first := workspace.Tmux.Windows[0].Name
	if _, err := tmux("select-window", "-t", name+":"+first); err != nil {
		return err
	}
	return nil
}

// AddClaudePane splits workspace.FromIssue.RepoWindow vertically and runs
// `claude -n <title> <prompt>` (or `claude <prompt>` when title is
// empty) in the new pane. The pane is created in cwd so Claude starts in
// the worktree rather than tmux's default directory.
func AddClaudePane(session string, workspace *config.Workspace, data ClaudeData, cwd string) error {
	if workspace.FromIssue.RepoWindow == "" {
		return fmt.Errorf("from_issue.repo_window is not configured")
	}
	prompt, err := RenderPrompt(workspace.FromIssue.ClaudePrompt, data)
	if err != nil {
		return err
	}
	title, err := RenderPaneTitle(workspace.FromIssue.ClaudePaneTitle, data)
	if err != nil {
		return err
	}
	target := session + ":" + workspace.FromIssue.RepoWindow
	pane, err := tmux("split-window", "-v", "-t", target, "-c", cwd, "-P", "-F", "#{pane_id}")
	if err != nil {
		return err
	}
	pane = strings.TrimSpace(pane)

	var cmd string
	if title != "" {
		cmd = fmt.Sprintf("claude -n %s %s", shellQuote(title), shellQuote(prompt))
	} else {
		cmd = fmt.Sprintf("claude %s", shellQuote(prompt))
	}
	if _, err := tmux("send-keys", "-t", pane, cmd, "C-m"); err != nil {
		return err
	}
	return nil
}

// shellQuote wraps s in single quotes and escapes any internal
// single-quote, matching bash's `printf %q`-equivalent for our inputs.
func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

func KillSession(name string) error {
	_, err := tmux("kill-session", "-t", name)
	return err
}

// SwitchOrAttach switches the current tmux client to `name` when called
// from inside tmux ($TMUX set), otherwise attaches the controlling
// terminal to the session. The outside-tmux branch replaces sam's
// process image with tmux via syscall.Exec so no `sam` process lingers
// as the parent of the tmux client — without this, `killall sam` would
// tear down the user's attached session.
func SwitchOrAttach(name string) error {
	if InTmux() {
		_, err := tmux("switch-client", "-t", name)
		return err
	}
	// Defense in depth: the InTmux() check above already routes inside-
	// tmux callers to switch-client. This redundant check ensures that
	// if a future refactor ever lets us fall through with $TMUX set, we
	// fail loudly instead of silently nesting a tmux client.
	if InTmux() {
		return fmt.Errorf("refusing to attach: $TMUX is set, would nest tmux sessions")
	}
	bin, err := exec.LookPath("tmux")
	if err != nil {
		return err
	}
	return syscall.Exec(bin, []string{"tmux", "attach-session", "-t", name}, os.Environ())
}

// CurrentSession returns the current session name, or "" when not in
// tmux or when tmux exits with an error.
func CurrentSession() (string, error) {
	if !InTmux() {
		return "", nil
	}
	cmd := exec.Command("tmux", "display-message", "-p", "#S")
	out, err := cmd.Output()
	if err != nil {
		return "", nil
	}
	return strings.TrimSpace(string(out)), nil
}
