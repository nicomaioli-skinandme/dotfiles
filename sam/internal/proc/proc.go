// Package proc wraps process-inspection subprocesses (pgrep, lsof, ps,
// tmux list-panes) used by `sam clankers`. Per-pid errors from external
// commands are swallowed and surfaced as zero values so a single dead
// process or missing fd doesn't tank the listing.
package proc

import (
	"bytes"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
)

// Process is a single running process matched by name.
type Process struct {
	PID  int
	Name string
}

// Pane is one row of `tmux list-panes -a`.
type Pane struct {
	PanePID    int
	Session    string
	WindowIdx  int
	WindowName string
	PaneIdx    int
	PaneTitle  string
}

// Claudes returns all running `claude` processes. The `-a` flag keeps
// our own ancestor in the result when invoked from inside a claude
// session — macOS pgrep silently filters it otherwise.
func Claudes() ([]Process, error) {
	out, err := exec.Command("pgrep", "-ax", "claude").Output()
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok && ee.ExitCode() == 1 {
			return nil, nil
		}
		return nil, fmt.Errorf("pgrep -ax claude: %w", err)
	}
	return parsePgrep(string(out)), nil
}

func parsePgrep(s string) []Process {
	var out []Process
	for _, line := range strings.Split(s, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		pidStr, name, _ := strings.Cut(line, " ")
		pid, err := strconv.Atoi(pidStr)
		if err != nil {
			continue
		}
		out = append(out, Process{PID: pid, Name: name})
	}
	return out
}

// Cwd returns the cwd of pid via `lsof -a -p PID -d cwd -Fn`. Any error
// (including a dead process) yields "", nil to match bash 2>/dev/null.
func Cwd(pid int) (string, error) {
	out, err := exec.Command("lsof", "-a", "-p", strconv.Itoa(pid), "-d", "cwd", "-Fn").Output()
	if err != nil {
		return "", nil
	}
	return parseLsofCwd(string(out)), nil
}

func parseLsofCwd(s string) string {
	for _, line := range strings.Split(s, "\n") {
		if strings.HasPrefix(line, "n") {
			return line[1:]
		}
	}
	return ""
}

// Parent returns pid's ppid via `ps -o ppid= -p PID`. Errors or
// unparseable output yield 0, nil.
func Parent(pid int) (int, error) {
	out, err := exec.Command("ps", "-o", "ppid=", "-p", strconv.Itoa(pid)).Output()
	if err != nil {
		return 0, nil
	}
	ppid, err := strconv.Atoi(strings.TrimSpace(string(out)))
	if err != nil {
		return 0, nil
	}
	return ppid, nil
}

// TmuxPanes returns every pane across every session. Empty slice when
// tmux isn't on PATH or no server is running.
func TmuxPanes() ([]Pane, error) {
	if _, err := exec.LookPath("tmux"); err != nil {
		return nil, nil
	}
	cmd := exec.Command("tmux", "list-panes", "-a", "-F",
		"#{pane_pid}\t#{session_name}\t#{window_index}\t#{window_name}\t#{pane_index}\t#{pane_title}")
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return nil, nil
	}
	return parseTmuxPanes(stdout.String()), nil
}

func parseTmuxPanes(s string) []Pane {
	var out []Pane
	for _, line := range strings.Split(s, "\n") {
		if line == "" {
			continue
		}
		fields := strings.Split(line, "\t")
		if len(fields) < 6 {
			continue
		}
		pid, err := strconv.Atoi(fields[0])
		if err != nil {
			continue
		}
		winIdx, err := strconv.Atoi(fields[2])
		if err != nil {
			continue
		}
		paneIdx, err := strconv.Atoi(fields[4])
		if err != nil {
			continue
		}
		out = append(out, Pane{
			PanePID:    pid,
			Session:    fields[1],
			WindowIdx:  winIdx,
			WindowName: fields[3],
			PaneIdx:    paneIdx,
			PaneTitle:  fields[5],
		})
	}
	return out
}

// FindTmuxPane walks the parent chain from claudePid upward and returns
// the first pane whose PanePID matches. Stops at pid 1 or 0.
func FindTmuxPane(panes []Pane, claudePid int) (Pane, bool) {
	if len(panes) == 0 {
		return Pane{}, false
	}
	cur := claudePid
	for cur > 1 {
		for _, p := range panes {
			if p.PanePID == cur {
				return p, true
			}
		}
		ppid, _ := Parent(cur)
		if ppid == 0 || ppid == cur {
			return Pane{}, false
		}
		cur = ppid
	}
	return Pane{}, false
}
