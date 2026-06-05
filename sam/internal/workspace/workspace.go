// Package workspace owns active-workspace resolution: given the loaded
// config, an optional explicit --workspace name, and the caller's cwd, it
// decides which workspace an invocation operates on. The config schema and
// its IO live in the config package; resolution *policy* lives here, so
// cmd/sam resolves once per invocation and hands the result to controllers.
package workspace

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/nicomaioli-skinandme/dotfiles/sam/internal/config"
)

// Service resolves and lists workspaces. It is stateless; the zero value is
// ready to use.
type Service struct{}

// Active is a resolved workspace: its key in the config alongside the
// workspace itself.
type Active struct {
	Name string
	WS   *config.Workspace
}

// ErrInsideRepo is returned by Resolve when cwd is a descendant of a
// workspace's repo (but not the repo root) and no other resolution
// mechanism applied. The caller surfaces a guidance message and exits.
type ErrInsideRepo struct {
	Workspace string
	Repo      string
}

func (e *ErrInsideRepo) Error() string {
	return fmt.Sprintf("invoked inside %s repo (%s); cd to the repo root or a worktree",
		e.Workspace, e.Repo)
}

// IsInsideRepo reports whether err is the in-repo-subdir guidance error.
func IsInsideRepo(err error) bool {
	var e *ErrInsideRepo
	return errors.As(err, &e)
}

// LoadConfig loads the config from its default path, returning the path too
// so callers can mention it in error or status messages.
func (Service) LoadConfig() (*config.Config, string, error) {
	path, err := config.DefaultPath()
	if err != nil {
		return nil, "", err
	}
	cfg, err := config.Load(path)
	if err != nil {
		return nil, path, err
	}
	return cfg, path, nil
}

// Resolve picks which workspace to use given an optional explicit name
// (from --workspace) and the caller's cwd. Resolution order:
//  1. explicit
//  2. cwd matches a workspace's repo or sits inside its worktrees dir
//  3. cwd is inside a workspace's repo (but not at root) → ErrInsideRepo
//  4. single-workspace shortcut
//
// When none of the above apply, Resolve returns (nil, nil) — a success
// result meaning "no workspace selected; the caller should prompt." The
// interactive menu treats this as "open the workspace-select view";
// non-interactive commands surface their own error pointing at --workspace.
func (Service) Resolve(cfg *config.Config, explicit, cwd string) (*Active, error) {
	if explicit != "" {
		w, ok := cfg.Workspaces[explicit]
		if !ok {
			return nil, fmt.Errorf("workspace %q not found in config (have: %s)",
				explicit, names(cfg))
		}
		return &Active{Name: explicit, WS: &w}, nil
	}

	if cwd != "" {
		cwd = filepath.Clean(cwd)
		// Pass 1: exact repo match or descendant of worktrees.
		for name, w := range cfg.Workspaces {
			repo := filepath.Clean(w.Repo)
			wt := filepath.Clean(w.Worktrees)
			if cwd == repo {
				return &Active{Name: name, WS: &w}, nil
			}
			if isDescendant(cwd, wt) {
				return &Active{Name: name, WS: &w}, nil
			}
		}
		// Pass 2: descendant of repo (but not at root) → guidance error.
		for name, w := range cfg.Workspaces {
			repo := filepath.Clean(w.Repo)
			if isDescendant(cwd, repo) {
				return nil, &ErrInsideRepo{Workspace: name, Repo: repo}
			}
		}
	}

	if len(cfg.Workspaces) == 1 {
		for name, w := range cfg.Workspaces {
			return &Active{Name: name, WS: &w}, nil
		}
	}
	return nil, nil
}

// List returns all configured workspaces sorted by name.
func (Service) List(cfg *config.Config) []Active {
	out := make([]Active, 0, len(cfg.Workspaces))
	for name := range cfg.Workspaces {
		w := cfg.Workspaces[name]
		out = append(out, Active{Name: name, WS: &w})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

// Names returns the configured workspace names, sorted and comma-joined,
// for use in error messages.
func (Service) Names(cfg *config.Config) string {
	return names(cfg)
}

func names(cfg *config.Config) string {
	out := make([]string, 0, len(cfg.Workspaces))
	for n := range cfg.Workspaces {
		out = append(out, n)
	}
	sort.Strings(out)
	return strings.Join(out, ", ")
}

// isDescendant reports whether child is a strict descendant of parent.
// Both arguments are expected to be Clean'd absolute paths.
func isDescendant(child, parent string) bool {
	if parent == "" || child == parent {
		return false
	}
	sep := string(os.PathSeparator)
	return strings.HasPrefix(child, parent+sep)
}
