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
shell alias needed. Configuration lives at `~/.config/sam/config.toml`
(template: `private_dot_config/sam/config.toml.tmpl`).

## tmux plugins (tpm)

Plugin runtime state lives outside this repo at
`~/.local/share/tmux/plugins/` (set via `TMUX_PLUGIN_MANAGER_PATH` in
`tmux.conf`, not managed by chezmoi). `bootstrap.sh` clones tpm there.
Inside tmux, `prefix + I` installs the configured plugins.

# Workflow

Run `sam` with no arguments to get the interactive picker (sessions and
worktrees, with `●` marking active sessions). Or invoke a subcommand
directly:

## `sam from-issue`

- Select an issue from the configured GitHub Project
- Assigns it to the currently logged-in user
- Creates a worktree and a tmux session
- Adds a `claude` pane running the project's `claude_prompt` (rendered
  with the issue's number, title, repo, and URL)

The prompt template, pane title, and target window are configured per
project under `[projects.<name>.from_issue]` in
`~/.config/sam/config.toml` — that file is the source of truth.

## `sam new-worktree`

Designed for code reviews:

- Select a local or remote branch from the UI
- Creates a git worktree and a tmux session

## `sam delete`

Deletes a local git worktree (and tears down its tmux session).

## `sam clankers`

Lists running `claude` processes; `--human` for a human-readable view.
