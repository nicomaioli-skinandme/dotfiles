package tmuxx

import "testing"

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
