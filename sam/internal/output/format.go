// Package output renders command results in one of the supported formats
// selected by the global --output/-o flag. It is infra: it imports nothing
// from the rest of sam, so every entity's cli/ View can depend on it without
// risking an import cycle.
package output

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"text/tabwriter"
)

// Format is an --output/-o value. Table is the default; json is opt-in.
// Room remains for yaml/toml later without touching call sites.
type Format string

const (
	Table Format = "table"
	JSON  Format = "json"
)

// Parse maps a flag value to a Format. The empty string defaults to Table.
func Parse(s string) (Format, error) {
	switch s {
	case "", string(Table):
		return Table, nil
	case string(JSON):
		return JSON, nil
	default:
		return "", fmt.Errorf("unknown output format %q (want: table, json)", s)
	}
}

// TableData is the human-readable shape: a header row plus aligned columns.
type TableData struct {
	Header []string
	Rows   [][]string
}

// Render writes jsonValue as JSON when f is JSON, otherwise renders table as an
// aligned text table. jsonValue should already be the json-tagged shape the
// command wants to emit (a slice for list commands); table mirrors it for human
// reading. Callers build both from the same controller output.
func Render(w io.Writer, f Format, jsonValue any, table TableData) error {
	if f == JSON {
		return json.NewEncoder(w).Encode(jsonValue)
	}
	tw := tabwriter.NewWriter(w, 1, 1, 2, ' ', 0)
	if len(table.Header) > 0 {
		fmt.Fprintln(tw, strings.Join(table.Header, "\t"))
	}
	for _, row := range table.Rows {
		fmt.Fprintln(tw, strings.Join(row, "\t"))
	}
	return tw.Flush()
}
