package main

import (
	"testing"

	"github.com/nicomaioli-skinandme/dotfiles/sam/internal/ghx"
)

func mkItem(id string, num int, repo, title, status string, assignees ...string) ghx.ProjectItem {
	it := ghx.ProjectItem{ID: id, Status: status, Assignees: assignees}
	it.Content.Number = num
	it.Content.Repository = repo
	it.Content.Title = title
	return it
}

func TestFilterBacklog(t *testing.T) {
	items := []ghx.ProjectItem{
		mkItem("a", 1, "org/api", "in scope", "📋 Backlog"),
		mkItem("b", 2, "org/web", "wrong repo", "📋 Backlog"),
		mkItem("c", 3, "org/api", "wrong status", "🏗 In Progress"),
		mkItem("d", 4, "org/api", "platform", "Platform Backlog"),
	}
	repos := []string{"org/api"}
	statuses := []string{"📋 Backlog", "Platform Backlog"}

	got := filterBacklog(items, repos, statuses)
	if len(got) != 2 {
		t.Fatalf("len=%d, want 2 (%v)", len(got), got)
	}
	if got[0].ID != "a" || got[1].ID != "d" {
		t.Errorf("got ids %s,%s; want a,d", got[0].ID, got[1].ID)
	}

	if r := filterBacklog(nil, repos, statuses); len(r) != 0 {
		t.Errorf("empty input: got len=%d", len(r))
	}
	if r := filterBacklog(items, nil, statuses); len(r) != 0 {
		t.Errorf("no repos: got len=%d", len(r))
	}
}

func TestFindItem(t *testing.T) {
	items := []ghx.ProjectItem{
		mkItem("a", 1, "org/api", "x", "📋 Backlog"),
		mkItem("b", 2, "org/api", "y", "📋 Backlog"),
		mkItem("c", 1, "org/web", "z", "📋 Backlog"),
	}

	got, ok := findItem(items, 2, "org/api")
	if !ok || got.ID != "b" {
		t.Errorf("want b/true, got %s/%v", got.ID, ok)
	}
	got, ok = findItem(items, 1, "org/web")
	if !ok || got.ID != "c" {
		t.Errorf("want c/true, got %s/%v", got.ID, ok)
	}
	if _, ok := findItem(items, 99, "org/api"); ok {
		t.Error("want false for missing num")
	}
	if _, ok := findItem(items, 1, "org/missing"); ok {
		t.Error("want false for missing repo")
	}
}
