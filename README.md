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

If you want to use `private_dot_config/tmux/dev.sh`, make it executable
(`chmod +x`) and alias it in your shell rc.

## tmux plugins (tpm)

Plugin runtime state lives outside this repo at
`~/.local/share/tmux/plugins/` (set via `TMUX_PLUGIN_MANAGER_PATH` in
`tmux.conf`, not managed by chezmoi). `bootstrap.sh` clones tpm there.
Inside tmux, `prefix + I` installs the configured plugins.

# Workflow

## + from issue

- Select an issue from the UI
- Assigns the issue to the currently logged-in user
- Creates a worktree
- Creates a tmux session
- Runs claude, in a session named after the issue, with a dedicated prompt

## + new worktree

This is designed for code reviews:

- Select a local or remote branch form the UI
- Creates a git worktree
- Creates a tmux session

## + delete worktree

Deletes a local git worktree.
