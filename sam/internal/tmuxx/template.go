package tmuxx

import (
	"bytes"
	"fmt"
	"text/template"
)

// ClaudeData is the field set exposed to claude_prompt and
// claude_pane_title templates. Add fields here when the underlying flow
// gains new context. The Issue* and PR* groups are populated by their
// respective flows; the other group stays zero (templates reference only
// the fields relevant to their flow).
type ClaudeData struct {
	IssueNumber int
	IssueTitle  string
	IssueRepo   string
	IssueURL    string

	PRNumber int
	PRTitle  string
	PRRepo   string
	PRURL    string
	PRAuthor string
	PRBranch string
}

func render(name, tmpl string, data ClaudeData) (string, error) {
	t, err := template.New(name).Option("missingkey=error").Parse(tmpl)
	if err != nil {
		return "", fmt.Errorf("%s: parse: %w", name, err)
	}
	var buf bytes.Buffer
	if err := t.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("%s: execute: %w", name, err)
	}
	return buf.String(), nil
}

func RenderPrompt(tmpl string, data ClaudeData) (string, error) {
	return render("claude_prompt", tmpl, data)
}

func RenderPaneTitle(tmpl string, data ClaudeData) (string, error) {
	return render("claude_pane_title", tmpl, data)
}
