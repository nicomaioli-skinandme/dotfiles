package tmuxx

import (
	"strings"
	"testing"
)

func TestRenderPrompt_Substitutes(t *testing.T) {
	data := ClaudeData{
		IssueNumber: 42,
		IssueTitle:  "Add multi-step nudge",
		IssueRepo:   "skinandmeprojects/andbegin",
		IssueURL:    "https://github.com/skinandmeprojects/andbegin/issues/42",
	}
	tmpl := "/plan pull the context from {{ .IssueURL }}, including comments."
	got, err := RenderPrompt(tmpl, data)
	if err != nil {
		t.Fatalf("RenderPrompt: %v", err)
	}
	want := "/plan pull the context from https://github.com/skinandmeprojects/andbegin/issues/42, including comments."
	if got != want {
		t.Errorf("got %q want %q", got, want)
	}
}

func TestRenderPaneTitle_Substitutes(t *testing.T) {
	data := ClaudeData{IssueTitle: "Add multi-step nudge"}
	got, err := RenderPaneTitle("IMPL {{ .IssueTitle }}", data)
	if err != nil {
		t.Fatalf("RenderPaneTitle: %v", err)
	}
	if got != "IMPL Add multi-step nudge" {
		t.Errorf("got %q", got)
	}
}

// Backticks and dollar signs in the title should render verbatim — shell
// escaping is the caller's job, not the template's.
func TestRender_PreservesShellMetacharacters(t *testing.T) {
	data := ClaudeData{IssueTitle: "fix `eval` and $HOME"}
	got, err := RenderPaneTitle("IMPL {{ .IssueTitle }}", data)
	if err != nil {
		t.Fatalf("RenderPaneTitle: %v", err)
	}
	if !strings.Contains(got, "`eval`") || !strings.Contains(got, "$HOME") {
		t.Errorf("expected verbatim metacharacters, got %q", got)
	}
}

func TestRender_UnknownFieldErrors(t *testing.T) {
	_, err := RenderPrompt("{{ .Nope }}", ClaudeData{})
	if err == nil {
		t.Fatal("expected error for unknown field")
	}
}
