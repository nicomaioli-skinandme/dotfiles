// Package worktree is the worktree entity: it owns listing, creating, and
// removing git worktrees (and their branch candidates) on top of the gitx
// and setup infra, and a Controller that ties worktree actions to the
// session entity (annotating active sessions, attaching on add, killing on
// delete). The Service imports only infra; the cross-entity edge to
// session lives in the Controller.
package worktree

// Type distinguishes the repo-root worktree from the linked worktrees.
type Type string

const (
	Main   Type = "main"
	Linked Type = "linked"
)

// Worktree is one worktree row. The json tags match the legacy CLI output
// so the cli/ View can serialize it directly. SessionActive is annotated by
// the Controller (the Service leaves it false).
type Worktree struct {
	Name          string `json:"name"`
	Path          string `json:"path"`
	Type          Type   `json:"type"`
	SessionActive bool   `json:"session_active"`
}
