# Dotfiles

## Setup

```sh
git clone <this-repo> ~/Code/dotfiles
cd ~/Code/dotfiles
./bootstrap.sh
```

`bootstrap.sh` is idempotent: dependency install via brew, chezmoi
`sourceDir` config, git hooks (`core.hooksPath = githooks`), initial
`chezmoi apply`, nvim plugin sync, and `tpm` clone. See `AGENTS.md` for
hook behavior.

## Dependencies

Mirror of `bootstrap.sh`'s `DEPS` variable — that's the canonical list.
When updating, edit both in the same commit.

- `nvim`
- `tmux`
- `chezmoi`
- `fzf`
- `gh`
- `jq`
- `ripgrep` (provides `rg`)
- `tree-sitter` (library)
- `tree-sitter-cli` (CLI binary; needed for nvim-treesitter `:TSUpdate`)
- `go` (builds `sam`, the tmux dev session manager, on `chezmoi apply`)

## GitHub

Authenticate and add permissions to access projects:

```sh
gh auth login
gh auth login refresh -s project,read:project
```

## Claude

Global Claude config (`~/.claude/CLAUDE.md`, `~/.claude/settings.json`, and
authored content under `skills/`, `commands/`, `agents/`, `hooks/`) is
managed via `dot_claude/`. `chezmoi apply` keeps `~/.claude/` in sync.
Runtime dirs (`projects/`, `plugins/`, `history.jsonl`, etc.) are
intentionally not tracked — see `AGENTS.md` for the full list.

You will also need the `skinandme` plugins, open `claude` and run:

```
/plugin marketplace add skinandme/claude-plugins
/plugin install dev@skinandme
/reload-plugins
```

# Instructions

This repo is a [chezmoi](https://www.chezmoi.io/) source directory. It's kept
at `~/Code/dotfiles` rather than the chezmoi default; `bootstrap.sh` writes
the `sourceDir` override into `~/.config/chezmoi/chezmoi.toml`.

Workflow: edit files directly in this repo, then run `chezmoi apply` to sync
them into `$HOME`. `chezmoi managed` lists what's tracked. The
`post-commit` hook also applies automatically when drift is present.

`sam` (the tmux dev-session manager) is installed automatically to
`~/.local/bin/sam` by the `run_onchange_install-sam.sh.tmpl` hook
whenever any tracked file under `sam/` changes. Invoke it directly — no
shell alias needed.

Configuration lives at `~/.config/sam/config.toml`. The file is owned
by `sam` itself: on first run (or `sam workspace add`) an interactive
wizard creates and writes it. Edit the file by hand for layout-level
changes (tmux windows, prompt templates).

## tmux plugins (tpm)

Plugin runtime state lives outside this repo at
`~/.local/share/tmux/plugins/` (set via `TMUX_PLUGIN_MANAGER_PATH` in
`tmux.conf`, not managed by chezmoi). `bootstrap.sh` clones tpm there.
Inside tmux, `prefix + I` installs the configured plugins.

# Workflow

Run `sam` with no arguments to get the interactive picker (sessions and
worktrees, with `●` marking active sessions). Or invoke a subcommand
directly. The CLI is **noun→verb** and fully non-interactive: it never
prompts — when it needs input it can't derive (a reassignment, a branch
on collision) it errors and asks for the corresponding flag. Nouns
resolve k8s-style, so `issue` and `issues` are the same command. List
output defaults to a table; pass `-o json` (`--output json`) for JSON.

## `sam issue develop <n>`

- Resolves issue `n` from the configured GitHub Project (or the
  workspace's branch repo); `--repo org/name` overrides the repo
- Assigns it to the currently logged-in user (`--reassign` to take it
  off another assignee), moves its project status to In Progress
- Creates a worktree and a tmux session, then attaches
- Adds a `claude` pane running the workspace's `claude_prompt` (rendered
  with the issue's number, title, repo, and URL)
- `--branch <b>` overrides the derived branch name (required when the
  derived name exceeds the workspace's `max_branch_len`)

`sam issue list` lists the backlog (or open) issues. The prompt
template, pane title, and target window are configured per workspace
under `[workspaces.<name>.from_issue]` in `~/.config/sam/config.toml` —
that file is the source of truth.

## `sam pr review <n>`

Checks out PR `n`'s head branch for review (no GitHub writes):

- Creates a worktree on the head branch and a tmux session, then
  attaches; adds a `claude` review pane per `[workspaces.<name>.from_pr]`
- `--repo org/name` overrides the repo

`sam pr list` lists the PRs awaiting your review.

## `sam worktree add <branch>` / `delete <name>` / `list`

- `add` creates a git worktree and tmux session for an existing branch,
  then attaches (the interactive branch picker lives in the menu)
- `delete` removes a local git worktree and tears down its tmux session
- `list` shows the main worktree plus linked worktrees and which have a
  live session

## `sam session attach <name>`

Attaches to the tmux session for a worktree, building its layout first
if it doesn't exist yet.

## `sam clanker list`

Lists running `claude` processes with their tmux session and cwd.

## `sam workspace add` / `list`

`add` is an interactive wizard that appends a new workspace to
`~/.config/sam/config.toml`. Auto-detects the repo path, main branch,
and origin slug; offers to wire up a GitHub Project (by URL) and an
optional post-worktree setup hook; validates `gh` scopes before
writing. The same wizard runs automatically the first time `sam` is
invoked on a machine with no config file. `list` shows the configured
workspaces.

When a workspace has `worktree_setup` configured (a shell command
string), it runs after `git worktree add` and before the tmux session
is built. The hook's cwd is the new worktree and it sees these env
vars: `SAM_BRANCH`, `SAM_WORKTREE`, `SAM_REPO`, `SAM_WORKSPACE`, and
`SAM_ISSUE_NUMBER` (empty when there's no associated issue).
