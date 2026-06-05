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

// SessionName returns the tmux session name for a branch (or the
// trunk) within a workspace: "<workspace>-<branch>". Prefixing with the
// workspace keeps session names unambiguous when multiple workspaces have
// worktrees for similarly-named branches. See issue #23.
func SessionName(workspace, branch string) string {
	return workspace + "-" + branch
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

// AddClaudePane splits repoWindow vertically and runs `claude -n <title>
// <prompt>` (or `claude <prompt>` when title is empty) in the new pane.
// The pane is created in cwd so Claude starts in the worktree rather than
// tmux's default directory. The window/prompt/title come from the caller's
// flow config (from_issue or from_pr).
//
// An empty promptTmpl means "no Claude pane": AddClaudePane is a no-op
// (the caller has already built the session's tmux layout), so a flow
// with no configured prompt simply doesn't launch Claude. permissionMode,
// when non-empty, is passed through as `claude --permission-mode <mode>`.
func AddClaudePane(session, repoWindow, promptTmpl, titleTmpl, permissionMode string, data ClaudeData, cwd string) error {
	if promptTmpl == "" {
		return nil
	}
	if repoWindow == "" {
		return fmt.Errorf("repo_window is not configured")
	}
	prompt, err := RenderPrompt(promptTmpl, data)
	if err != nil {
		return err
	}
	title, err := RenderPaneTitle(titleTmpl, data)
	if err != nil {
		return err
	}
	target := session + ":" + repoWindow
	pane, err := tmux("split-window", "-v", "-t", target, "-c", cwd, "-P", "-F", "#{pane_id}")
	if err != nil {
		return err
	}
	pane = strings.TrimSpace(pane)

	if _, err := tmux("send-keys", "-t", pane, claudeCommand(permissionMode, title, prompt), "C-m"); err != nil {
		return err
	}
	return nil
}

// claudeCommand builds the shell command run in the Claude pane. An empty
// permissionMode or title omits the corresponding flag. All inputs are
// shell-quoted; the caller has already rendered any templates.
func claudeCommand(permissionMode, title, prompt string) string {
	parts := []string{"claude"}
	if permissionMode != "" {
		parts = append(parts, "--permission-mode", shellQuote(permissionMode))
	}
	if title != "" {
		parts = append(parts, "-n", shellQuote(title))
	}
	parts = append(parts, shellQuote(prompt))
	return strings.Join(parts, " ")
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
