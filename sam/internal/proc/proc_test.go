package proc

import (
	"reflect"
	"testing"
)

func TestParsePgrep(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want []Process
	}{
		{"empty", "", nil},
		{"whitespace only", "   \n\n", nil},
		{
			"single",
			"12345 claude\n",
			[]Process{{PID: 12345, Name: "claude"}},
		},
		{
			"multiple with args",
			"12345 claude --foo\n67890 claude\n",
			[]Process{
				{PID: 12345, Name: "claude --foo"},
				{PID: 67890, Name: "claude"},
			},
		},
		{
			"malformed line skipped",
			"notapid foo\n12345 claude\n",
			[]Process{{PID: 12345, Name: "claude"}},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := parsePgrep(tc.in)
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("got %#v, want %#v", got, tc.want)
			}
		})
	}
}

func TestParseLsofCwd(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"empty", "", ""},
		{"no n line", "p12345\nfcwd\n", ""},
		{
			"happy path",
			"p12345\nfcwd\nn/Users/foo/Code/dotfiles\n",
			"/Users/foo/Code/dotfiles",
		},
		{
			"first n wins",
			"n/one\nn/two\n",
			"/one",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := parseLsofCwd(tc.in); got != tc.want {
				t.Errorf("got %q, want %q", got, tc.want)
			}
		})
	}
}

func TestParseTmuxPanes(t *testing.T) {
	in := "111\tsystem\t0\thome\t0\tzsh\n" +
		"222\tdotfiles\t1\trepo\t0\tnvim\n" +
		"\n" +
		"badline\n" +
		"333\tdotfiles\t1\trepo\t1\tclaude\n"
	want := []Pane{
		{PanePID: 111, Session: "system", WindowIdx: 0, WindowName: "home", PaneIdx: 0, PaneTitle: "zsh"},
		{PanePID: 222, Session: "dotfiles", WindowIdx: 1, WindowName: "repo", PaneIdx: 0, PaneTitle: "nvim"},
		{PanePID: 333, Session: "dotfiles", WindowIdx: 1, WindowName: "repo", PaneIdx: 1, PaneTitle: "claude"},
	}
	got := parseTmuxPanes(in)
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %#v, want %#v", got, want)
	}
}

func TestFindTmuxPane(t *testing.T) {
	panes := []Pane{
		{PanePID: 100, Session: "a", WindowIdx: 0, WindowName: "w0", PaneIdx: 0, PaneTitle: "t0"},
		{PanePID: 200, Session: "b", WindowIdx: 1, WindowName: "w1", PaneIdx: 0, PaneTitle: "t1"},
	}

	t.Run("match at self", func(t *testing.T) {
		got, ok := FindTmuxPane(panes, 100)
		if !ok || got.Session != "a" {
			t.Errorf("expected match session=a, got %#v ok=%v", got, ok)
		}
	})

	t.Run("empty panes", func(t *testing.T) {
		if _, ok := FindTmuxPane(nil, 100); ok {
			t.Error("expected no match for empty panes")
		}
	})

	t.Run("no match and pid 1 terminates", func(t *testing.T) {
		// pid 1 is never in panes; walk should stop without infinite loop.
		if _, ok := FindTmuxPane(panes, 1); ok {
			t.Error("expected no match starting from pid 1")
		}
	})
}
