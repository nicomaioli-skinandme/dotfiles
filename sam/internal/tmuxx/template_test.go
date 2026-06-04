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

func TestRenderPrompt_PRFields(t *testing.T) {
	data := ClaudeData{
		PRNumber: 123,
		PRTitle:  "Add nudge",
		PRRepo:   "skinandme/andbegin-monorepo",
		PRURL:    "https://github.com/skinandme/andbegin-monorepo/pull/123",
	}
	got, err := RenderPrompt("/review {{ .PRURL }} for {{ .PRTitle }}", data)
	if err != nil {
		t.Fatalf("RenderPrompt: %v", err)
	}
	want := "/review https://github.com/skinandme/andbegin-monorepo/pull/123 for Add nudge"
	if got != want {
		t.Errorf("got %q want %q", got, want)
	}
}

func TestRender_UnknownFieldErrors(t *testing.T) {
	_, err := RenderPrompt("{{ .Nope }}", ClaudeData{})
	if err == nil {
		t.Fatal("expected error for unknown field")
	}
}

// With no prompt configured, AddClaudePane must be a clean no-op (no tmux
// calls, no error) — the flow launches no Claude pane. This returns before
// any tmux subprocess, so it's safe without a running server.
func TestAddClaudePane_NoPromptSkips(t *testing.T) {
	if err := AddClaudePane("session", "repo", "", "", ClaudeData{}, "/tmp"); err != nil {
		t.Errorf("empty prompt should no-op, got %v", err)
	}
	// Empty prompt skips even when repo_window is also unset.
	if err := AddClaudePane("session", "", "", "", ClaudeData{}, "/tmp"); err != nil {
		t.Errorf("empty prompt/window should no-op, got %v", err)
	}
}
