# sam glossary

This glossary has one job: **fix which system a word refers to** — git,
GitHub, tmux, or sam itself — and **flag words that already name another
system's entity** so we don't pollute the namespace. When a term here
appears in conversation, code, config, or user-facing strings, it means
what's defined here; no further disambiguation is needed.

It is deliberately narrow:

- **Not** feature or implementation documentation. Config keys, Go
  types, env var names, and flow walk-throughs live in the code, the
  config, and the README — not here, where they would drift out of sync.
- **Not** eager. An entity sam doesn't actually use (labels, milestones,
  GitHub organizations) gets no entry.

Two annotations appear on entries:

- **_Alias_** — an accepted alternate form of the same term (e.g. `repo`
  ↔ `repository`). Interchangeable; recorded on the one entry, never
  duplicated as a second one.
- **_Reserved_** — a word that already names another system's entity and
  must not be reused for a sam concept (e.g. `project`). Forbidden, not
  interchangeable — the opposite of an alias.

## git

- **branch** — a git branch.
- **repo** — the git repository a workspace points at. _Alias:_
  repository.
- **worktree** — a [git worktree](https://git-scm.com/docs/git-worktree):
  an additional checkout of a repo on its own branch. A repo has one
  **main worktree** (the repo-root checkout) and any number of **linked
  worktrees** (the per-branch checkouts sam creates under a workspace's
  worktrees dir); sam tags each row in the worktrees view with its
  _worktree type_, `main` or `linked`. Not interchangeable with a
  **workspace**.

## GitHub

- **issue** — a GitHub issue.
- **column** — a column of a **GitHub Project** board: one option of its Status
  field (e.g. "Backlog", "In Progress", "Done"). The issues view's filter
  sidebar toggles which columns are shown. _Alias:_ status (the Project field a
  column is an option of). Not a database/table column.
- **pull request** — a GitHub pull request. _Alias:_ PR. sam surfaces the
  open PRs in a workspace's repo that request you as a reviewer via the
  **prs** view; selecting one creates a **worktree** on the PR's head
  branch for review.
- **project** — **_reserved_**. "Project" means a
  [GitHub Project (v2)](https://docs.github.com/en/issues/planning-and-tracking-with-projects)
  board. Never reuse the bare word for a sam concept; always write
  "GitHub Project" in user-facing text. If you need to name a new sam
  grouping, pick a different word.

## tmux

- **session** — a tmux session.
- **window** / **pane** — a tmux window / pane.

## sam

- **workspace** — sam's own container for working on a codebase. The
  classic confusion is with git's **worktree**: a worktree is a checkout
  on disk, a workspace is sam's configuration unit. Not interchangeable.
- **trunk** — the git branch sam treats as a workspace's mainline (e.g.
  `main`, `master`), configured per workspace. The **main worktree**
  normally has it checked out; the worktrees view excludes it from the
  branch picker since it already lives in the main worktree.
- **clanker** — a running Claude process, often inside a tmux pane.
  Listed by `sam clanker list`.
- **log** / **logs** — sam's own diagnostic record for a menu session:
  the errors, warnings, and activity shown in the `:logs` view (and teed
  to a temp file). Unrelated to **`git log`** (commit history); when the
  commit history is meant, say so explicitly.
