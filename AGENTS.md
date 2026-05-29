# Agents

## Bootstrap & hooks

`./bootstrap.sh` (repo root) is the install entry point on a fresh machine.
It's idempotent — safe to re-run; each section skips when already satisfied.

Two tracked git hooks live in `githooks/`, wired in by bootstrap via
`core.hooksPath = githooks`:

- `pre-commit` runs `chezmoi apply --dry-run` and blocks the commit if it
  fails (template error, broken script, malformed source). It does **not**
  block on routine source-vs-target diff.
- `post-commit` runs `chezmoi apply` when `chezmoi verify` reports drift,
  so source advances via `git pull` / `git merge` / direct edits get
  reflected in `~/`. Never fails the hook. Note: if a target file was
  modified outside chezmoi (e.g. you edited `~/.claude/CLAUDE.md` by hand
  and forgot `chezmoi re-add`), `apply` prompts before overwriting. In
  an interactive commit you'll see the prompt; in a non-interactive
  context the hook warns and exits cleanly — re-run `chezmoi apply`
  manually to resolve.

**Runtime dependencies are canonical in `bootstrap.sh`'s `DEPS` variable.**
When adding or removing a dependency, edit `DEPS` and update the
`README.md` Dependencies mirror in the same commit. Don't add ad-hoc
`brew install` calls elsewhere.

## Bash scripting

When writing bash scripts, ensure they are compatible with the version of bash running on the host. This host uses `bash 3.2.57` (the system bash shipped with macOS), so avoid bash 4+ features such as associative arrays (`declare -A`), `mapfile`/`readarray`, `${var^^}`/`${var,,}` case conversion, and `&>>` redirection.

## Neovim config

Lives in `private_dot_config/nvim/` (chezmoi source for `~/.config/nvim`). Layout:

- `init.lua` — entry point, only `require`s `config.*` modules.
- `lua/config/` — core editor setup: `options.lua`, `keymaps.lua`, `filetypes.lua`, `lazy.lua` (lazy.nvim bootstrap). Edit these for non-plugin behavior.
- `lua/plugins/` — one file per plugin (or tightly related group), each returning a lazy.nvim spec. New plugins go here as a new file; do not lump them into existing files. **Exceptions:** all `nvim-mini/mini.*` modules live together in `mini.lua`, and all `folke/snacks.nvim` sub-feature configuration lives together in `snacks.lua`.
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

**Whenever you add or change a `keys = {}` mapping**, also run the keymap
conflict gate. lazy.nvim silently lets the last-loaded spec win when two
plugins declare the same `lhs`+mode, so a clobbered keymap leaves no error —
this catches it by enumerating every declared key across all specs:

```sh
nvim --headless -c 'luafile ~/.config/nvim/scripts/keymap-check.lua' -c 'qa!'   # exits non-zero on conflict
```

It reports the colliding `lhs` and the plugins fighting over it. Resolve by
rebinding one side to a free namespace before committing.

## Claude config

Lives in `dot_claude/` (chezmoi source for `~/.claude/`). Workflow is
edit source → `chezmoi apply`, same as `nvim/`.

Tracked:

- `dot_claude/CLAUDE.md` — global instructions loaded into every session.
- `dot_claude/settings.json` — model, `enabledPlugins`, `extraKnownMarketplaces`,
  `effortLevel`. Hand-curated; safe to commit (no secrets).
- `dot_claude/skills/<name>/SKILL.md` — authored skills go here.
- `dot_claude/commands/`, `dot_claude/agents/`, `dot_claude/hooks/` — same
  pattern when you start authoring custom slash commands, subagents, or
  hooks. Don't pre-create empty dirs; they materialize when the first file
  lands.

Deliberately **not** tracked (runtime, cache, or sensitive — chezmoi only
manages files we explicitly add, so these stay untouched in `~/.claude/`):
`projects/` (conversation transcripts, can contain code/secrets),
`plugins/`, `backups/`, `cache/`, `file-history/`, `paste-cache/`,
`plans/`, `tasks/`, `telemetry/`, `sessions/`, `session-env/`,
`shell-snapshots/`, `downloads/`, `ide/`, `history.jsonl`,
`policy-limits.json`, `mcp-needs-auth-cache.json`, `.last-cleanup`. A
`~/.claude/settings.local.json`, if it ever appears, is per-machine and
should also stay untracked.

The repo root has a separate `.claude/settings.local.json` — that's the
**project-local** Claude settings for this dotfiles working directory (Claude
reads it from cwd), not the global folder. It's excluded from target sync
via `.chezmoiignore` (`/.claude/settings.local.json`). Don't confuse the
two; don't move it under `dot_claude/`.

To add a new skill:

```sh
mkdir -p ~/Code/dotfiles/dot_claude/skills/<name>
$EDITOR ~/Code/dotfiles/dot_claude/skills/<name>/SKILL.md
chezmoi apply        # materializes ~/.claude/skills/<name>/SKILL.md
```

## Sam

`sam/` is a Go CLI for tmux + git-worktree workflows. It has its own
agent guidance and a project glossary that disambiguates which system a
term refers to — git, GitHub, tmux, or sam (branch, issue, session,
workspace, worktree, GitHub Project, etc.). Load it when working on
anything under `sam/`:

@sam/AGENTS.md
