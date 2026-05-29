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
- **Not** eager. An entity sam doesn't actually use (pull requests,
  labels, milestones, GitHub organizations) gets no entry.

Two annotations appear on entries:

- ***Alias*** — an accepted alternate form of the same term (e.g. `repo`
  ↔ `repository`). Interchangeable; recorded on the one entry, never
  duplicated as a second one.
- ***Reserved*** — a word that already names another system's entity and
  must not be reused for a sam concept (e.g. `project`). Forbidden, not
  interchangeable — the opposite of an alias.

## git

- **branch** — a git branch.
- **repo** — the git repository a workspace points at. *Alias:*
  repository.
- **worktree** — a [git worktree](https://git-scm.com/docs/git-worktree):
  an additional checkout of a repo on its own branch. Not a
  **workspace** — see below.

## GitHub

- **issue** — a GitHub issue.
- **project** — ***reserved***. "Project" means a
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
- **main branch** vs **main repo** — "main branch" is the git branch sam
  treats as a workspace's trunk. "Main repo" is the synthetic menu entry
  for that trunk — the repo root checked out to the main branch — not a
  real git worktree.
- **clanker** — a running Claude process, often inside a tmux pane.
  Listed by `sam clankers`.
