# sam glossary

A shared vocabulary for sam — between contributors and AI agents. When
these terms appear in conversation, code, config, or user-facing
strings, they mean exactly what's defined here; no further
disambiguation is needed. Two pairs deserve special vigilance because
they're easy to collapse: **workspace** vs. **GitHub Project**, and
**workspace** vs. **worktree**.

## Workspace

sam's container for working on a codebase. Holds a repo path, a worktrees
directory, the main branch, the tmux layout, the optional
`worktree_setup` hook, and an optional link to a GitHub Project board.

- **Config:** `[workspaces.<name>]` in `~/.config/sam/config.toml`.
- **Selection:** sam resolves the active workspace from `--workspace` or from cwd (the repo root or anywhere under the worktrees dir). When neither resolves and there's more than one workspace configured, the menu opens on the workspace-select view; non-interactive commands error and ask for `--workspace`.
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

## Main branch

The branch sam treats as the workspace's trunk — the one it fetches and
fast-forwards before creating worktrees, and the one shown as the "main
repo" entry in `sam list` / `sam menu`.

- **Config:** `workspace.main_branch`.
- **Go type:** `config.Workspace.MainBranch`.
- The "main repo" menu entry is **synthetic**: it's a tmux session
  attached to `workspace.repo` itself (the repo root, checked out to
  `main_branch`), not a real git worktree under `workspace.worktrees/`.

Distinct from git's notion of a default branch — sam doesn't infer it,
it reads it from config and treats it as a first-class navigation
target.

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

## Backlog / In Progress

GitHub Project status concepts used by `sam from-issue`. Two sides of
the same transition: read the backlog, write In Progress.

- **Backlog** in sam = whichever GitHub Project statuses match
  `workspace.gh_project.backlog_statuses` (a list, e.g.
  `["📋 Backlog", "Platform Backlog"]`). Anything with a matching
  status is shown in the `from-issue` picker.
- **In Progress** = the status sam writes back after a pick, identified
  by `workspace.gh_project.in_progress_id` — a GitHub Project field
  *option ID*, not a human-readable label.
- The exact label strings are project-specific and configurable; the
  *concepts* (backlog set, in-progress target) are stable.
- **Go types:** `config.GhProject.BacklogStatuses`,
  `config.GhProject.InProgressID`.

## tmux layout

The structured per-workspace description of the tmux session sam builds
for each worktree.

- **Config:** `[workspaces.<name>.tmux]` with one or more
  `[[workspaces.<name>.tmux.windows]]` entries (each with `name`,
  `cwd`) and nested `[[...windows.panes]]` (each with `split` = `h` or
  `v`, and `cwd`).
- **Go types:** `config.TmuxLayout` → `config.Window` → `config.Pane`.
- **Built by:** `tmuxx.BuildSession` — first window via `new-session`,
  rest via `new-window`, each pane via `split-window`.
- `cwd` values are resolved relative to the worktree's base directory
  (absolute paths pass through).
- `workspace.from_issue.repo_window` references a window `name` from
  this layout; config load fails if the reference doesn't resolve.

## tmux session name

The name of the tmux session sam creates for a worktree (or the
synthetic main repo). Canonical format: `<workspace>-<branch>` — the
workspace name, a hyphen, then the branch / worktree name (the main repo
uses `main_branch`).

- **Built by:** `tmuxx.SessionName(workspace, branch)`; every site that
  creates, looks up, kills, or attaches to a sam session derives the name
  through it, so they all agree.
- **Why the prefix:** with multiple workspaces, bare branch names collide
  in tmux's cross-workspace views (`tmux ls`, status bar,
  `switch-client`); the workspace prefix disambiguates them.
- **Distinct from the branch / worktree name:** sam's own `list` and the
  TUI still show the bare branch (those views are already scoped to one
  workspace); the prefix lives only in the underlying tmux session name.
- **Exempt:** the always-on `system` session and any clanker session
  sam attaches to are real, externally-named sessions — sam uses their
  names verbatim and never applies the prefix.

## TUI settings

Top-level, workspace-independent settings for the interactive menu (the
`sam menu` full-screen front end), as opposed to per-workspace config.

- **Config:** `[tui]` in `~/.config/sam/config.toml` (a sibling of the
  `[workspaces.*]` tables, not nested under a workspace).
- **Go type:** `config.Tui`.
- Currently holds `[tui.autocomplete]` (`config.Autocomplete`) with `max`
  — the most entries shown at once in the `:` command popup (default
  `config.DefaultAutocompleteMax`, 5).

Distinct from a workspace's `tmux` layout: `[tui]` configures sam's own
on-screen menu; `[workspaces.<name>.tmux]` configures the tmux session
sam builds for a worktree.

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
