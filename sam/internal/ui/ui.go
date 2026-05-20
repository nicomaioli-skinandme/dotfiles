// Package ui wraps charmbracelet/huh primitives used by sam's command
// flows: a filterable picker, a yes/no confirm, an editable input, and
// a small label decorator.
package ui

import (
	"errors"
	"fmt"
	"strings"

	"github.com/charmbracelet/huh"
)

// ErrCancelled is returned when the user dismisses the prompt (Esc,
// Ctrl-C). Callers should treat it as a clean abort, not a failure.
var ErrCancelled = errors.New("ui: cancelled")

type Item struct {
	Value string
	Label string
}

// Picker presents a filterable list. Items are matched on Label.
// Returns ErrCancelled if the user dismisses the prompt.
func Picker(title string, items []Item) (Item, error) {
	if len(items) == 0 {
		return Item{}, fmt.Errorf("ui.Picker: no items")
	}
	options := make([]huh.Option[string], len(items))
	byValue := make(map[string]Item, len(items))
	for i, it := range items {
		options[i] = huh.NewOption(it.Label, it.Value)
		byValue[it.Value] = it
	}
	var selected string
	err := huh.NewSelect[string]().
		Title(title).
		Options(options...).
		Filtering(true).
		Value(&selected).
		Run()
	if err != nil {
		if errors.Is(err, huh.ErrUserAborted) {
			return Item{}, ErrCancelled
		}
		return Item{}, err
	}
	return byValue[selected], nil
}

// Confirm prompts yes/no. Defaults to No to match the legacy bash flow.
func Confirm(title string) (bool, error) {
	var v bool
	err := huh.NewConfirm().
		Title(title).
		Affirmative("Yes").
		Negative("No").
		Value(&v).
		Run()
	if err != nil {
		if errors.Is(err, huh.ErrUserAborted) {
			return false, ErrCancelled
		}
		return false, err
	}
	return v, nil
}

// Input shows a prepopulated editable prompt. `header` is shown as a
// description (e.g. the original default the user is overriding).
// Returns the trimmed value.
func Input(title, header, initial string) (string, error) {
	v := initial
	err := huh.NewInput().
		Title(title).
		Description(header).
		Value(&v).
		Run()
	if err != nil {
		if errors.Is(err, huh.ErrUserAborted) {
			return "", ErrCancelled
		}
		return "", err
	}
	return strings.TrimSpace(v), nil
}

// Decorate prepends a bullet to `label` when the named session is
// active. Ports executable_dev.sh:154-161.
func Decorate(name, label string, active bool) string {
	if label == "" {
		label = name
	}
	if active {
		return "● " + label
	}
	return label
}
