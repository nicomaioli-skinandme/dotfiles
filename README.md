# Dotfiles

## Dependencies

- `nvim`
- `tmux`
- `stow`
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

# Instructions

Clone the repo, cd in the clone directory and run:

```sh
mkdir ~/.config
stow -vt ~ tmux
stow -vt ~ nvim
stow -vt ~ ghostty
```

If you want to use the `tmux/dev.sh` script, make it executable (`chmod +x`) and alias it in your shell rc.

## tmux plugins (tpm)

Plugin runtime state lives outside this repo at `~/.local/share/tmux/plugins/` (set via `TMUX_PLUGIN_MANAGER_PATH` in `tmux.conf`). Bootstrap tpm on a new machine:

```sh
mkdir -p ~/.local/share/tmux/plugins
git clone https://github.com/tmux-plugins/tpm ~/.local/share/tmux/plugins/tpm
```

Then inside tmux, `prefix + I` to install the configured plugins.
