package config

import (
	"path/filepath"
	"reflect"
	"testing"
)

func TestSave_RoundTrip(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	in := writeConfig(t, fullAndbegin)
	loaded, err := Load(in)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	out := filepath.Join(t.TempDir(), "config.toml")
	if err := Save(loaded, out); err != nil {
		t.Fatalf("Save: %v", err)
	}

	reloaded, err := Load(out)
	if err != nil {
		t.Fatalf("re-Load: %v", err)
	}
	if !reflect.DeepEqual(loaded.Workspaces, reloaded.Workspaces) {
		t.Errorf("workspaces mismatch after round-trip:\nbefore=%+v\nafter=%+v",
			loaded.Workspaces, reloaded.Workspaces)
	}
}

func TestSave_WorktreeSetupField(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	cfg := &Config{
		Workspaces: map[string]Workspace{
			"solo": {
				Repo:          "/x",
				Worktrees:     "/y",
				Trunk:         "main",
				WorktreeSetup: "touch .sam-marker",
			},
		},
	}
	path := filepath.Join(t.TempDir(), "config.toml")
	if err := Save(cfg, path); err != nil {
		t.Fatalf("Save: %v", err)
	}
	reloaded, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	got := reloaded.Workspaces["solo"].WorktreeSetup
	if got != "touch .sam-marker" {
		t.Errorf("worktree_setup: got %q", got)
	}
}
