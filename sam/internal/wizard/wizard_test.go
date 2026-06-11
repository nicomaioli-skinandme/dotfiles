package wizard

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/nicomaioli-skinandme/dotfiles/sam/internal/ghx"
)

func TestParseProjectURL(t *testing.T) {
	cases := []struct {
		in      string
		owner   string
		number  int
		wantErr bool
	}{
		{"https://github.com/orgs/acme/projects/7", "acme", 7, false},
		{"https://github.com/users/bob/projects/12", "bob", 12, false},
		{"  https://github.com/orgs/acme/projects/7  ", "acme", 7, false},
		{"https://github.com/acme/projects/7", "", 0, true},        // missing orgs/users
		{"https://github.com/orgs/acme/projects/x", "", 0, true},   // non-numeric
		{"https://github.com/orgs/acme/projects", "", 0, true},     // no number segment
		{"https://github.com/orgs/acme/repos/7", "", 0, true},      // wrong noun
		{"", "", 0, true},
	}
	for _, c := range cases {
		owner, number, err := ParseProjectURL(c.in)
		if c.wantErr {
			if err == nil {
				t.Errorf("ParseProjectURL(%q): expected error, got (%q, %d)", c.in, owner, number)
			}
			continue
		}
		if err != nil {
			t.Errorf("ParseProjectURL(%q): unexpected error %v", c.in, err)
			continue
		}
		if owner != c.owner || number != c.number {
			t.Errorf("ParseProjectURL(%q) = (%q, %d), want (%q, %d)", c.in, owner, number, c.owner, c.number)
		}
	}
}

func TestBacklogPreselect(t *testing.T) {
	opts := []ghx.ProjectStatusOption{
		{ID: "1", Name: "Todo"},
		{ID: "2", Name: "In Progress"},
		{ID: "3", Name: "Done"},
		{ID: "4", Name: "Blocked"},
		{ID: "5", Name: "Cancelled"},
	}
	got := BacklogPreselect(opts, "2")
	want := []string{"1", "4"} // skips in-progress and done-like names
	if !reflect.DeepEqual(got, want) {
		t.Errorf("BacklogPreselect = %v, want %v", got, want)
	}
}

func TestSplitComma(t *testing.T) {
	cases := []struct {
		in   string
		want []string
	}{
		{"a, b ,c", []string{"a", "b", "c"}},
		{"a,,b", []string{"a", "b"}},
		{"  ", []string{}},
		{"", []string{}},
	}
	for _, c := range cases {
		got := SplitComma(c.in)
		if len(got) != len(c.want) {
			t.Errorf("SplitComma(%q) = %v, want %v", c.in, got, c.want)
			continue
		}
		for i := range got {
			if got[i] != c.want[i] {
				t.Errorf("SplitComma(%q) = %v, want %v", c.in, got, c.want)
				break
			}
		}
	}
}

func TestExpandAbs(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatal(err)
	}
	got, err := ExpandAbs("~/projects/x")
	if err != nil {
		t.Fatal(err)
	}
	if want := filepath.Join(home, "projects", "x"); got != want {
		t.Errorf("ExpandAbs(~/projects/x) = %q, want %q", got, want)
	}

	got, err = ExpandAbs("/a/../b/")
	if err != nil {
		t.Fatal(err)
	}
	if got != "/b" {
		t.Errorf("ExpandAbs(/a/../b/) = %q, want /b", got)
	}
}
