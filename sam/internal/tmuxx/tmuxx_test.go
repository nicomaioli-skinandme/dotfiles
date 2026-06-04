package tmuxx

import "testing"

func TestClaudeCommand(t *testing.T) {
	cases := []struct {
		name                  string
		permissionMode, title string
		prompt, want          string
	}{
		{"plain", "", "", "/review", `claude '/review'`},
		{"title", "", "REVIEW x", "/review", `claude -n 'REVIEW x' '/review'`},
		{"mode", "auto", "", "/review", `claude --permission-mode 'auto' '/review'`},
		{"mode+title", "auto", "REVIEW x", "/review", `claude --permission-mode 'auto' -n 'REVIEW x' '/review'`},
	}
	for _, c := range cases {
		if got := claudeCommand(c.permissionMode, c.title, c.prompt); got != c.want {
			t.Errorf("%s: got %q want %q", c.name, got, c.want)
		}
	}
}

func TestSessionName(t *testing.T) {
	cases := []struct {
		workspace, branch, want string
	}{
		{"dotfiles", "main", "dotfiles-main"},
		{"skinandme", "23-add-prefix", "skinandme-23-add-prefix"},
	}
	for _, c := range cases {
		if got := SessionName(c.workspace, c.branch); got != c.want {
			t.Errorf("SessionName(%q, %q) = %q, want %q", c.workspace, c.branch, got, c.want)
		}
	}
}
