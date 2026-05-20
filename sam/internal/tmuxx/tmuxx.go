// Package tmuxx wraps tmux subprocess invocations for session/pane
// manipulation. Layout building is driven by config.Project.Tmux.
package tmuxx

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/nicomaioli-skinandme/dotfiles/sam/internal/config"
)

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

// EnsureSystemSession creates the always-on "system" session if missing.
// Two windows: home (cwd $HOME) and dotfiles (cwd ~/Code/dotfiles).
// Ports executable_dev.sh:16-24.
func EnsureSystemSession() error {
	if HasSession("system") {
		return nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	if _, err := tmux("new-session", "-d", "-s", "system", "-n", "home", "-c", home); err != nil {
		return err
	}
	dotfiles := filepath.Join(home, "Code", "dotfiles")
	if _, err := tmux("new-window", "-t", "system", "-n", "dotfiles", "-c", dotfiles); err != nil {
		return err
	}
	if _, err := tmux("select-window", "-t", "system:home"); err != nil {
		return err
	}
	return nil
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

// BuildSession creates `name` and applies project.Tmux.Windows. The
// first window is created with new-session; subsequent windows via
// new-window. Each pane is added with split-window.
func BuildSession(name string, project *config.Project, baseDir string) error {
	if len(project.Tmux.Windows) == 0 {
		return fmt.Errorf("project has no tmux windows configured")
	}
	for i, w := range project.Tmux.Windows {
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
	first := project.Tmux.Windows[0].Name
	if _, err := tmux("select-window", "-t", name+":"+first); err != nil {
		return err
	}
	return nil
}

// AddClaudePane splits project.FromIssue.RepoWindow vertically and runs
// `claude -n <title> <prompt>` (or `claude <prompt>` when title is
// empty) in the new pane.
func AddClaudePane(session string, project *config.Project, data ClaudeData) error {
	if project.FromIssue.RepoWindow == "" {
		return fmt.Errorf("from_issue.repo_window is not configured")
	}
	prompt, err := RenderPrompt(project.FromIssue.ClaudePrompt, data)
	if err != nil {
		return err
	}
	title, err := RenderPaneTitle(project.FromIssue.ClaudePaneTitle, data)
	if err != nil {
		return err
	}
	target := session + ":" + project.FromIssue.RepoWindow
	pane, err := tmux("split-window", "-v", "-t", target, "-P", "-F", "#{pane_id}")
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
// terminal to the session.
func SwitchOrAttach(name string) error {
	if os.Getenv("TMUX") != "" {
		_, err := tmux("switch-client", "-t", name)
		return err
	}
	cmd := exec.Command("tmux", "attach-session", "-t", name)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// CurrentSession returns the current session name, or "" when not in
// tmux or when tmux exits with an error.
func CurrentSession() (string, error) {
	if os.Getenv("TMUX") == "" {
		return "", nil
	}
	cmd := exec.Command("tmux", "display-message", "-p", "#S")
	out, err := cmd.Output()
	if err != nil {
		return "", nil
	}
	return strings.TrimSpace(string(out)), nil
}
