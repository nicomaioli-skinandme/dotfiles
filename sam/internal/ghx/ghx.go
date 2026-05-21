// Package ghx wraps the `gh` CLI. Output parsing uses encoding/json
// rather than shelling out to `jq`.
package ghx

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os/exec"
	"strconv"
	"strings"

	"github.com/nicomaioli-skinandme/dotfiles/sam/internal/config"
)

type ProjectItem struct {
	ID        string   `json:"id"`
	Status    string   `json:"status"`
	Assignees []string `json:"assignees"`
	Content   struct {
		Number     int    `json:"number"`
		Title      string `json:"title"`
		Repository string `json:"repository"`
	} `json:"content"`
}

type projectItemList struct {
	Items []ProjectItem `json:"items"`
}

func run(args ...string) (string, error) {
	cmd := exec.Command("gh", args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			return "", fmt.Errorf("gh %s: %w", strings.Join(args, " "), err)
		}
		return "", fmt.Errorf("gh %s: %w: %s", strings.Join(args, " "), err, msg)
	}
	return stdout.String(), nil
}

// ProjectItems returns every item on the configured project. Filtering
// by repo or status happens in the caller.
func ProjectItems(cfg config.GhProject) ([]ProjectItem, error) {
	out, err := run(
		"project", "item-list", strconv.Itoa(cfg.Number),
		"--owner", cfg.Owner,
		"--format", "json",
		"--limit", "200",
	)
	if err != nil {
		return nil, err
	}
	var wrap projectItemList
	if err := json.Unmarshal([]byte(out), &wrap); err != nil {
		return nil, fmt.Errorf("parse gh project item-list output: %w", err)
	}
	return wrap.Items, nil
}

func IssueAddAssignee(repo string, num int, who string) error {
	_, err := run("issue", "edit", strconv.Itoa(num),
		"--repo", repo, "--add-assignee", who)
	return err
}

func IssueSwapAssignee(repo string, num int, from, to string) error {
	_, err := run("issue", "edit", strconv.Itoa(num),
		"--repo", repo,
		"--remove-assignee", from,
		"--add-assignee", to)
	return err
}

func IssueDevelop(issueRepo, branchRepo string, num int, name string) error {
	_, err := run("issue", "develop", strconv.Itoa(num),
		"--repo", issueRepo,
		"--branch-repo", branchRepo,
		"--name", name)
	return err
}

// IssueDevelopList returns the first linked branch name for an issue
// (the first whitespace-separated field of the first non-blank line).
// Returns ("", nil) if no branch is linked.
func IssueDevelopList(issueRepo string, num int) (string, error) {
	out, err := run("issue", "develop", "--list", strconv.Itoa(num), "--repo", issueRepo)
	if err != nil {
		return "", err
	}
	for _, line := range strings.Split(out, "\n") {
		fields := strings.Fields(line)
		if len(fields) > 0 {
			return fields[0], nil
		}
	}
	return "", nil
}

func ProjectItemSetStatus(cfg config.GhProject, itemID, optionID string) error {
	_, err := run("project", "item-edit",
		"--project-id", cfg.ID,
		"--id", itemID,
		"--field-id", cfg.StatusFieldID,
		"--single-select-option-id", optionID)
	return err
}

func CurrentUser() (string, error) {
	out, err := run("api", "user", "-q", ".login")
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(out), nil
}

// ProjectMeta fetches the GitHub Project's stable node ID (PVT_...)
// from owner+number via `gh project view`.
func ProjectMeta(owner string, number int) (id string, err error) {
	out, err := run(
		"project", "view", strconv.Itoa(number),
		"--owner", owner,
		"--format", "json",
	)
	if err != nil {
		return "", err
	}
	var meta struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal([]byte(out), &meta); err != nil {
		return "", fmt.Errorf("parse gh project view output: %w", err)
	}
	return meta.ID, nil
}

// ProjectStatusOption is one of the choices on a single-select project
// field (typically the Status field).
type ProjectStatusOption struct {
	ID   string
	Name string
}

// ProjectStatusField captures the IDs needed to set status on items
// and the option list that the wizard uses to ask the user which
// option counts as "in progress" / "backlog".
type ProjectStatusField struct {
	FieldID string
	Options []ProjectStatusOption
}

// ProjectStatusField fetches the Status single-select field plus its
// options for the given project. Returns a typed error when no field
// named "Status" exists.
func StatusField(owner string, number int) (ProjectStatusField, error) {
	out, err := run(
		"project", "field-list", strconv.Itoa(number),
		"--owner", owner,
		"--format", "json",
	)
	if err != nil {
		return ProjectStatusField{}, err
	}
	var resp struct {
		Fields []struct {
			ID      string `json:"id"`
			Name    string `json:"name"`
			Type    string `json:"type"`
			Options []struct {
				ID   string `json:"id"`
				Name string `json:"name"`
			} `json:"options"`
		} `json:"fields"`
	}
	if err := json.Unmarshal([]byte(out), &resp); err != nil {
		return ProjectStatusField{}, fmt.Errorf("parse gh project field-list output: %w", err)
	}
	for _, f := range resp.Fields {
		if f.Name != "Status" {
			continue
		}
		out := ProjectStatusField{FieldID: f.ID}
		for _, o := range f.Options {
			out.Options = append(out.Options, ProjectStatusOption{ID: o.ID, Name: o.Name})
		}
		return out, nil
	}
	return ProjectStatusField{}, fmt.Errorf("project %s/#%d has no Status single-select field", owner, number)
}

// AuthScopes returns the OAuth scopes currently granted to the gh
// token for github.com, parsed from `gh auth status`. Returns an
// empty slice (and no error) when the user isn't logged in to
// github.com — caller should also surface "not logged in" via the
// underlying error for that case.
func AuthScopes() ([]string, error) {
	// `gh auth status` writes to stderr; we capture both.
	cmd := exec.Command("gh", "auth", "status")
	var combined bytes.Buffer
	cmd.Stdout = &combined
	cmd.Stderr = &combined
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("gh auth status: %w: %s", err, strings.TrimSpace(combined.String()))
	}
	out := combined.String()
	// Look for: "- Token scopes: 'a', 'b', 'c'"
	const marker = "Token scopes:"
	idx := strings.Index(out, marker)
	if idx < 0 {
		return nil, nil
	}
	rest := out[idx+len(marker):]
	if nl := strings.IndexByte(rest, '\n'); nl >= 0 {
		rest = rest[:nl]
	}
	parts := strings.Split(rest, ",")
	scopes := make([]string, 0, len(parts))
	for _, p := range parts {
		s := strings.Trim(strings.TrimSpace(p), "'\"")
		if s != "" {
			scopes = append(scopes, s)
		}
	}
	return scopes, nil
}
