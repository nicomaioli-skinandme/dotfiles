package gitx

import "testing"

func TestSlugify(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"Add multi-step nudge", "add-multi-step-nudge"},
		{"Fix: bug in foo/bar (urgent!)", "fix-bug-in-foo-bar-urgent"},
		{"  leading and trailing  ", "leading-and-trailing"},
		{"---only-dashes---", "only-dashes"},
		{"already-lower-1", "already-lower-1"},
		{"UPPER CASE", "upper-case"},
		{"emoji 🎉 keep ascii", "emoji-keep-ascii"},
		{"", ""},
		{"!!!", ""},
		{"a", "a"},
		{"a___b", "a-b"},
	}
	for _, c := range cases {
		got := Slugify(c.in)
		if got != c.want {
			t.Errorf("Slugify(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}
