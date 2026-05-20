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
