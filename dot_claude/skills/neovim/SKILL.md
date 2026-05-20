---
name: neovim
description: Drive an already-running Neovim instance from outside it (e.g. another tmux pane) via its msgpack-RPC socket.
---

# neovim

For when there's a Neovim already open — usually in another tmux pane — and you want to drive it: open files, send keystrokes, run ex commands, inject text, read editor state back. You are *not* launching a new headless nvim; you're talking to the user's live session.

Everything below uses `nvim --server <sock>` against the running instance's msgpack-RPC socket.

## 1. Find the socket

Every running nvim publishes a socket on launch.

- **macOS:** `$TMPDIR/nvim.$USER/<id>/nvim.<pid>.0`
- **Linux:** `$XDG_RUNTIME_DIR/nvim.$USER/<id>/nvim.<pid>.0`
- **From inside that nvim:** `:echo v:servername`

Quick listing:

```sh
ls -t "${TMPDIR:-/tmp}/nvim.$USER"/*/nvim.*.0 2>/dev/null   # macOS
ls -t "${XDG_RUNTIME_DIR:-/run/user/$(id -u)}/nvim.$USER"/*/nvim.*.0 2>/dev/null   # Linux
```

If only one socket exists, use it. If multiple, **disambiguate by pid** — don't guess by mtime. The pid is in the socket filename (`nvim.<pid>.0`).

Cross-reference against tmux to figure out which pane the user means:

```sh
tmux list-panes -a -F '#{pane_pid} #{pane_current_command} #{session_name}:#{window_index}.#{pane_index}'
```

`pane_current_command` will be `nvim` for direct invocations. If the user runs nvim under a shell wrapper or `zsh -c`, `pane_pid` is the shell — walk its children (`pgrep -P <pane_pid>`) to find the nvim pid, then match it to the socket filename.

Cache the socket path once you've resolved it, but remember: **the socket disappears the moment nvim quits.** A stale path will fail silently or with a connection error — re-resolve if a send fails.

## 2. Send keystrokes — `--remote-send`

```sh
nvim --server "$SOCK" --remote-send '<keys>'
```

Uses Vim keycode notation: `<CR>`, `<Esc>`, `<C-w>`, `<Tab>`, `<BS>`, `<Space>`, etc. Literal characters are sent as-is.

**Always prefix risky sends with `<Esc><Esc>`** to guarantee normal mode before issuing an ex command. Two escapes because one in insert mode just exits to normal — the second is a no-op safety net that also exits operator-pending or visual mode if you happened to be there.

```sh
nvim --server "$SOCK" --remote-send '<Esc><Esc>:write<CR>'
```

`--remote-send` is **fire-and-forget**. It returns instantly even if the ex command is still running (e.g. a long `:make` or a dadbod query). Don't assume completion from a zero exit status. If you need to know when something finishes, poll state via `--remote-expr` (see below).

### Shell-escaping

Single-quote the whole keystroke string. Vim keycodes contain `<` and `>` which the shell would otherwise treat as redirection. If you must include a literal `'`, end the single-quoted run, escape the apostrophe, and resume: `'...don'\''t...'`. Easier: avoid apostrophes in sent text, or use `--remote-expr` with `nvim_buf_set_lines` instead.

## 3. Read state — `--remote-expr`

```sh
nvim --server "$SOCK" --remote-expr '<vimscript-expression>'
```

Returns the evaluated value on stdout. Useful for:

```sh
# current buffer's file path
nvim --server "$SOCK" --remote-expr 'expand("%:p")'

# cwd
nvim --server "$SOCK" --remote-expr 'getcwd()'

# current mode ('n', 'i', 'v', 'V', 'c', ...)
nvim --server "$SOCK" --remote-expr 'mode()'

# current line text
nvim --server "$SOCK" --remote-expr 'getline(".")'

# whole buffer as one string
nvim --server "$SOCK" --remote-expr "join(getbufline('%', 1, '$'), '\n')"
```

For Lua, wrap with `luaeval`:

```sh
nvim --server "$SOCK" --remote-expr 'luaeval("vim.api.nvim_buf_get_name(0)")'
```

For fire-and-forget Lua (no return value needed):

```sh
nvim --server "$SOCK" --remote-send ':lua vim.notify("hi")<CR>'
```

## 4. Recipes

### Open a file in a new tab and run an ex command on it

The shape used to drive vim-dadbod from outside:

```sh
nvim --server "$SOCK" --remote-send '<Esc><Esc>:tabedit /path/to/query.sql<CR>:%DB<CR>'
```

`<Esc><Esc>` guarantees normal mode; `:tabedit` opens the file in a new tab and focuses it; the second ex command runs against the just-opened buffer.

### Inject text into a buffer without typing it character-by-character

Two good options. **Prefer the API** unless you specifically want the text to land at the cursor.

**API (preferred — clean, no cursor side-effects):**

```sh
nvim --server "$SOCK" --remote-expr \
  'nvim_buf_set_lines(0, -1, -1, v:false, ["line one", "line two"])'
```

`(0, -1, -1, …)` appends to the current buffer. Use `(0, 0, -1, …)` to replace the whole buffer.

**Via tmp file + `:read`** (handy when the payload is large or already on disk):

```sh
printf '%s\n' "$payload" > /tmp/nvim-inject.$$
nvim --server "$SOCK" --remote-send '<Esc><Esc>:read /tmp/nvim-inject.'"$$"'<CR>'
rm /tmp/nvim-inject.$$
```

Don't try to `--remote-send` a multi-line payload as keystrokes — newlines, special chars, and shell escaping turn it into a minefield.

### Is nvim ready for an ex command?

```sh
mode=$(nvim --server "$SOCK" --remote-expr 'mode()')
[ "$mode" = "n" ] || nvim --server "$SOCK" --remote-send '<Esc><Esc>'
```

`mode()` returns `'n'` for normal, `'i'` for insert, `'c'` for command-line, `'v'`/`'V'` for visual. If you don't care about preserving an in-progress insert, just `<Esc><Esc>` unconditionally.

### Poll for completion of a long ex command

`--remote-send` won't block. To wait, have the command set a sentinel:

```sh
nvim --server "$SOCK" --remote-send \
  '<Esc><Esc>:let g:done=0 \| MyLongCmd \| let g:done=1<CR>'
until [ "$(nvim --server "$SOCK" --remote-expr 'get(g:, "done", 0)')" = "1" ]; do
  sleep 0.5
done
```

## 5. Caveats

- **Socket lifetime:** dies with nvim. Re-resolve on connection failure rather than caching forever.
- **Shell-escaping:** single-quote keystroke strings; sidestep embedded apostrophes by using `--remote-expr` with the API instead.
- **`--remote-send` is async:** zero exit status ≠ command done. Poll if you need to sequence.
- **Multiple nvims:** match by pid via the socket filename, not by mtime. Cross-reference tmux panes when the user says "the one in my other pane."
- **You're driving the user's live editor.** A stray `:q!` closes their buffers. Prefer `:tabedit` (additive) over `:edit` (replaces current buffer). When in doubt, read state first.
