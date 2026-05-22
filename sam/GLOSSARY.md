# sam glossary

Terms used in sam's code, config, and CLI. The point of this file is to
keep two easily-confused concepts — **workspace** and **GitHub Project**
— from collapsing back into the same word.

## Workspace

sam's container for working on a codebase. Holds a repo path, a worktrees
directory, the main branch, the tmux layout, the optional
`worktree_setup` hook, and an optional link to a GitHub Project board.

- **Config:** `[workspaces.<name>]` in `~/.config/sam/config.toml`.
- **Default selection:** `default_workspace = "<name>"` at the top of the config.
- **CLI:** `--workspace <name>` to override, `sam workspace add` to create one via the wizard.
- **Go type:** `config.Workspace`.
- **Env (exposed to `worktree_setup`):** `SAM_WORKSPACE`.

Today a workspace points at a single repo. It may grow to span multiple
repositories — the term was chosen with that future in mind.

## Repo

A single git repository belonging to a workspace. Field:
`workspace.repo`. Always an absolute path on the local filesystem.

## Worktree

A [git worktree](https://git-scm.com/docs/git-worktree) — an additional
working tree of a repo, checked out to a branch other than the one in
the repo root. sam creates worktrees under `workspace.worktrees/`, one
per branch, and drives a tmux session per worktree.

Distinct from **workspace** despite the spelling overlap: "worktree" is
git's term for a checkout; "workspace" is sam's term for the container.

## Branch repo

The GitHub `owner/name` slug used by `gh issue develop` when creating
branches from issues. Field: `workspace.branch_repo`. Usually the same
as the origin remote of `workspace.repo`, but split so the issue can
live in a different GitHub repo from where branches are pushed.

## GitHub Project

A [GitHub Project (v2)](https://docs.github.com/en/issues/planning-and-tracking-with-projects)
board — GitHub's issue-organisation feature. **Always** spelled "GitHub
Project" in user-facing strings and `gh_project` in config keys; never
shortened to just "project" in this codebase.

- **Config (optional):** `[workspaces.<name>.gh_project]`.
- **Go type:** `config.GhProject`.
- **Used by:** `sam from-issue`, to read the configured project's backlog and move the picked item to "In Progress".
- **Wraps:** `gh project ...` subcommands; see `internal/ghx`.

## `from-issue` flow

`sam from-issue` end-to-end: pick a backlog issue (from a configured
GitHub Project, or from `gh issue list` when none is configured) →
assign it to the current user → set its project status to "In Progress"
(if a GitHub Project is linked) → create a branch via `gh issue develop`
→ fetch and fast-forward the main branch → create a worktree → run the
`worktree_setup` hook → assemble a tmux session per `workspace.tmux` →
attach.

## `worktree_setup` hook

Optional per-workspace shell command (`workspace.worktree_setup`) run
inside a freshly created worktree, via `sh -c`. Exposed env:
`SAM_BRANCH`, `SAM_WORKTREE`, `SAM_REPO`, `SAM_WORKSPACE`,
`SAM_ISSUE_NUMBER` (empty when no issue is associated, e.g. for
`sam new-worktree`).

Failure bubbles up and aborts the bootstrap; the worktree directory is
left in place so the user can inspect what went wrong.
