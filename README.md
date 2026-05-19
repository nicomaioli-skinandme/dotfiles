# Dotfiles

## Dependencies

- `nvim`
- `tmux`
- `chezmoi`
- `fzf`
- `gh`
- `jq`
- `rg`
- `tree-sitter`
- `tree-sitter-cli`

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

This repo is a [chezmoi](https://www.chezmoi.io/) source directory. It's kept at
`~/Code/dotfiles` rather than the chezmoi default, so `sourceDir` is overridden
in `~/.config/chezmoi/chezmoi.toml`.

```sh
brew install chezmoi
git clone <this-repo> ~/Code/dotfiles
mkdir -p ~/.config/chezmoi
printf 'sourceDir = "%s/Code/dotfiles"\n' "$HOME" > ~/.config/chezmoi/chezmoi.toml

chezmoi diff      # preview what will change in $HOME
chezmoi apply -v  # materialize files into ~/.config/{nvim,tmux,ghostty} and ~/.claude
```

Workflow: edit files directly in this repo, then run `chezmoi apply` to sync
them into `$HOME`. `chezmoi managed` lists what's tracked.

If you want to use `private_dot_config/tmux/dev.sh`, make it executable
(`chmod +x`) and alias it in your shell rc.

## tmux plugins (tpm)

Plugin runtime state lives outside this repo at `~/.local/share/tmux/plugins/` (set via `TMUX_PLUGIN_MANAGER_PATH` in `tmux.conf`, not managed by chezmoi). Bootstrap tpm on a new machine:

```sh
mkdir -p ~/.local/share/tmux/plugins
git clone https://github.com/tmux-plugins/tpm ~/.local/share/tmux/plugins/tpm
```

Then inside tmux, `prefix + I` to install the configured plugins.

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
