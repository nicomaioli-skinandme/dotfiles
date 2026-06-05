package output

import (
	"bytes"
	"strings"
	"testing"
)

func TestParse(t *testing.T) {
	cases := []struct {
		in      string
		want    Format
		wantErr bool
	}{
		{"", Table, false},
		{"table", Table, false},
		{"json", JSON, false},
		{"yaml", "", true},
	}
	for _, c := range cases {
		got, err := Parse(c.in)
		if c.wantErr {
			if err == nil {
				t.Errorf("Parse(%q): want error, got nil", c.in)
			}
			continue
		}
		if err != nil {
			t.Errorf("Parse(%q): unexpected error %v", c.in, err)
		}
		if got != c.want {
			t.Errorf("Parse(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestRenderJSON(t *testing.T) {
	var buf bytes.Buffer
	val := []map[string]int{{"n": 1}}
	if err := Render(&buf, JSON, val, TableData{}); err != nil {
		t.Fatal(err)
	}
	if got := strings.TrimSpace(buf.String()); got != `[{"n":1}]` {
		t.Errorf("JSON render = %q", got)
	}
}

func TestRenderTable(t *testing.T) {
	var buf bytes.Buffer
	td := TableData{
		Header: []string{"NAME", "TYPE"},
		Rows:   [][]string{{"main", "main"}, {"feat", "linked"}},
	}
	if err := Render(&buf, Table, nil, td); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	for _, want := range []string{"NAME", "TYPE", "main", "feat", "linked"} {
		if !strings.Contains(out, want) {
			t.Errorf("table render missing %q in:\n%s", want, out)
		}
	}
}
