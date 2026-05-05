# Agents

## Bash scripting

When writing bash scripts, ensure they are compatible with the version of bash running on the host. This host uses `bash 3.2.57` (the system bash shipped with macOS), so avoid bash 4+ features such as associative arrays (`declare -A`), `mapfile`/`readarray`, `${var^^}`/`${var,,}` case conversion, and `&>>` redirection.

## Neovim config

Lives in `private_dot_config/nvim/` (chezmoi source for `~/.config/nvim`). Layout:

- `init.lua` — entry point, only `require`s `config.*` modules.
- `lua/config/` — core editor setup: `options.lua`, `keymaps.lua`, `filetypes.lua`, `lazy.lua` (lazy.nvim bootstrap). Edit these for non-plugin behavior.
- `lua/plugins/` — one file per plugin (or tightly related group), each returning a lazy.nvim spec. New plugins go here as a new file; do not lump them into existing files.
- `after/ftplugin/<ft>.lua` — filetype-local overrides (buffer-local options/keymaps). Create this directory if/when needed; prefer it over autocmds in `config/filetypes.lua` for per-buffer settings.

`stylua` and `lua-language-server` are installed via Mason at `~/.local/share/nvim/mason/bin/`.

Run gates against the **applied** tree at `~/.config/nvim`, not the chezmoi source:
`lua-language-server` reads `.luarc.json`, which exists only at the applied path
(the source has `dot_luarc.json`). So the workflow is: edit source → `chezmoi
apply` → run gates.

```sh
chezmoi apply                                                              # sync source → ~/.config/nvim
~/.local/share/nvim/mason/bin/stylua --check ~/.config/nvim                # format check
~/.local/share/nvim/mason/bin/stylua ~/.config/nvim                        # format fix
~/.local/share/nvim/mason/bin/lua-language-server --check ~/.config/nvim   # LSP diagnostics
```

Both must pass before committing nvim changes.
