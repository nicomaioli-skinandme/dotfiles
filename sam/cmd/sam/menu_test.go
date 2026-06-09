package main

import "testing"

func TestShouldDefaultToMenu(t *testing.T) {
	cases := []struct {
		name string
		args []string
		want bool
	}{
		{"no args", nil, true},
		{"bare subcommand", []string{"issue"}, false},
		{"help long", []string{"--help"}, false},
		{"help short", []string{"-h"}, false},
		{"version", []string{"--version"}, false},
		{"workspace detached then nothing", []string{"--workspace", "dotfiles"}, true},
		{"workspace detached then subcommand", []string{"--workspace", "dotfiles", "issue"}, false},
		// A detached --log-level value must not be mistaken for a subcommand.
		{"log-level detached", []string{"--log-level", "debug"}, true},
		{"log-level attached", []string{"--log-level=debug"}, true},
		{"log-level detached then subcommand", []string{"--log-level", "debug", "issue"}, false},
		{"output detached", []string{"-o", "json"}, true},
	}
	for _, c := range cases {
		if got := shouldDefaultToMenu(c.args); got != c.want {
			t.Errorf("%s: shouldDefaultToMenu(%v) = %v, want %v", c.name, c.args, got, c.want)
		}
	}
}
