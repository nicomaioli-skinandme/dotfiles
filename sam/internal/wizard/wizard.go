// Package wizard owns the non-interactive half of the workspace-setup
// flow that produces a validated entry in `~/.config/sam/config.toml`:
// pure helpers (URL parsing, path expansion, backlog preselection) and
// [Service], the I/O seam the TUI's add-workspace form drives. The
// interaction itself lives in internal/tui (the in-TUI form).
package wizard

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/nicomaioli-skinandme/dotfiles/sam/internal/config"
	"github.com/nicomaioli-skinandme/dotfiles/sam/internal/ghx"
	"github.com/nicomaioli-skinandme/dotfiles/sam/internal/gitx"
)

// ErrMissingScopes is returned when `gh` is missing scopes the user's
// selections require. The setup flow surfaces err.Error() (which carries
// the `gh auth refresh` remedy) without writing config.
type ErrMissingScopes struct {
	Missing []string
}

func (e *ErrMissingScopes) Error() string {
	return fmt.Sprintf(
		"gh is missing required scope(s) %s — run: gh auth refresh -s %s",
		strings.Join(quoted(e.Missing), ", "),
		strings.Join(e.Missing, ","),
	)
}

func quoted(in []string) []string {
	out := make([]string, len(in))
	for i, s := range in {
		out[i] = "'" + s + "'"
	}
	return out
}

// Service is the I/O surface of the workspace-setup flow: repo probing,
// GitHub Project lookup, gh scope validation, and the final config write.
// internal/tui declares its consumer-side interface (SetupService) so
// tests can substitute a fake; this is the real implementation.
type Service struct{}

// ProbeRepo expands p to an absolute path, verifies it is a git
// repository, and returns it along with the detected trunk ("main" when
// origin/HEAD gives nothing) and the origin's owner/name slug (empty
// when there is no usable origin remote).
func (Service) ProbeRepo(p string) (repo, trunk, originSlug string, err error) {
	repo, err = ExpandAbs(p)
	if err != nil {
		return "", "", "", err
	}
	if !gitx.IsRepo(repo) {
		return "", "", "", fmt.Errorf("%s is not a git repository", repo)
	}
	trunk, _ = gitx.DefaultBranch(repo)
	if trunk == "" {
		trunk = "main"
	}
	originSlug, _ = gitx.OriginRepo(repo)
	return repo, trunk, originSlug, nil
}

// FetchProject resolves the GitHub Project's node ID and its Status
// single-select field, erroring when the field has no options to pick
// from.
func (Service) FetchProject(owner string, number int) (string, ghx.ProjectStatusField, error) {
	projectID, err := ghx.ProjectMeta(owner, number)
	if err != nil {
		return "", ghx.ProjectStatusField{}, err
	}
	fld, err := ghx.StatusField(owner, number)
	if err != nil {
		return "", ghx.ProjectStatusField{}, err
	}
	if len(fld.Options) == 0 {
		return "", ghx.ProjectStatusField{}, fmt.Errorf("Status field on project %s/#%d has no options", owner, number)
	}
	return projectID, fld, nil
}

// CheckScopes reports (via ErrMissingScopes) any of the required gh
// OAuth scopes the current `gh auth` token lacks.
func (Service) CheckScopes(required []string) error {
	return CheckScopes(required)
}

// SaveWorkspace re-loads the config from disk (tolerating a missing file
// on first run), re-checks the name for duplicates, appends the
// workspace, and saves. It returns the written path.
func (Service) SaveWorkspace(name string, ws config.Workspace) (string, error) {
	path, err := config.DefaultPath()
	if err != nil {
		return "", err
	}
	var cfg *config.Config
	if _, statErr := os.Stat(path); statErr == nil {
		cfg, err = config.Load(path)
		if err != nil {
			return "", err
		}
	} else if !errors.Is(statErr, os.ErrNotExist) {
		return "", statErr
	} else {
		cfg = &config.Config{Workspaces: map[string]config.Workspace{}}
	}
	if cfg.Workspaces == nil {
		cfg.Workspaces = map[string]config.Workspace{}
	}
	if _, exists := cfg.Workspaces[name]; exists {
		return "", fmt.Errorf("workspace %q already exists; pick a different name or edit %s by hand", name, path)
	}
	cfg.Workspaces[name] = ws
	if err := config.Save(cfg, path); err != nil {
		return "", err
	}
	return path, nil
}

// ParseProjectURL extracts (owner, number) from URLs like
// https://github.com/orgs/<owner>/projects/<n> or
// https://github.com/users/<owner>/projects/<n>.
func ParseProjectURL(s string) (string, int, error) {
	s = strings.TrimSpace(s)
	u, err := url.Parse(s)
	if err != nil {
		return "", 0, fmt.Errorf("parse url: %w", err)
	}
	parts := strings.Split(strings.Trim(u.Path, "/"), "/")
	// Expect: ["orgs"|"users", owner, "projects", number]
	if len(parts) != 4 || (parts[0] != "orgs" && parts[0] != "users") || parts[2] != "projects" {
		return "", 0, fmt.Errorf("not a GitHub Project URL: %s", s)
	}
	owner := parts[1]
	num, err := strconv.Atoi(parts[3])
	if err != nil {
		return "", 0, fmt.Errorf("project number: %w", err)
	}
	return owner, num, nil
}

var doneLike = regexp.MustCompile(`(?i)\b(done|complete|completed|shipped|closed|cancelled|canceled)\b`)

// BacklogPreselect returns the option IDs that should start checked in
// the "which statuses count as backlog" multi-select: everything except
// the in-progress choice and obvious "done" shapes.
func BacklogPreselect(opts []ghx.ProjectStatusOption, inProgressID string) []string {
	pre := make([]string, 0, len(opts))
	for _, o := range opts {
		if o.ID == inProgressID {
			continue
		}
		if doneLike.MatchString(o.Name) {
			continue
		}
		pre = append(pre, o.ID)
	}
	return pre
}

// CheckScopes reports (via ErrMissingScopes) any of the required gh
// OAuth scopes the current `gh auth` token lacks.
func CheckScopes(required []string) error {
	have, err := ghx.AuthScopes()
	if err != nil {
		return err
	}
	got := map[string]bool{}
	for _, s := range have {
		got[s] = true
	}
	var missing []string
	for _, r := range required {
		if !got[r] {
			missing = append(missing, r)
		}
	}
	if len(missing) == 0 {
		return nil
	}
	return &ErrMissingScopes{Missing: missing}
}

// SplitComma splits a comma-separated list, trimming whitespace and
// dropping empties.
func SplitComma(s string) []string {
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

// ExpandAbs expands a leading ~ and resolves the path to a cleaned
// absolute form.
func ExpandAbs(p string) (string, error) {
	if strings.HasPrefix(p, "~") {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		if p == "~" {
			p = home
		} else if strings.HasPrefix(p, "~/") {
			p = filepath.Join(home, p[2:])
		}
	}
	abs, err := filepath.Abs(p)
	if err != nil {
		return "", err
	}
	return filepath.Clean(abs), nil
}
