---
name: tmux
description: Drive an already-running tmux session from outside — discover panes, send keystrokes, capture pane output, spawn windows/panes/popups.
---

# tmux

For when there's a live tmux session — usually the one the user is in right now — and you want to observe or drive it: list panes, run commands in another pane, capture what's on screen, spawn a new pane/window/popup, watch a long-running process. You are not the user's interactive client; you're a sidecar talking to the same server.

Everything below uses the `tmux` CLI directly. You never send the user's prefix (`C-a` in this config, irrelevant when invoking `tmux` from a shell).

If a target pane is running Neovim and you need to drive *the editor* (not the shell underneath), use the `neovim` skill instead — this skill gets you as far as identifying the right pane and its pid.

## 1. Targeting — how to name a pane

Tmux targets are strings, and there are several flavors:

- `session:window.pane` — readable, e.g. `work:1.2`. Indices can renumber (this config has `renumber-windows on`); don't cache these.
- `%<n>` — unique pane id (e.g. `%12`). Stable for the pane's lifetime.
- `@<n>` — unique window id. `$<n>` — unique session id.
- Bare `<name>` — session by name. `<name>:` — first window of a session. `:<window>` — current session, given window.

**Resolve once, reuse the unique id.** A `session:window.pane` lookup is fine as the first step, but immediately grab `#{pane_id}` and use that for subsequent commands:

```sh
PANE=$(tmux display-message -p -t work:1.2 '#{pane_id}')   # e.g. %12
tmux send-keys -t "$PANE" 'echo hi' Enter
```

`display-message -p` prints a format string; without `-p` it pops a transient status-line message instead.

## 2. Discover state — `list-*` + `display-message -p`

```sh
tmux list-sessions -F '#{session_id} #{session_name} #{?session_attached,attached,}'
tmux list-windows -a -F '#{window_id} #{session_name}:#{window_index} #{window_name} #{?window_zoomed_flag,Z,}'
tmux list-panes  -a -F '#{pane_id} #{pane_pid} #{pane_current_command} #{pane_current_path} #{session_name}:#{window_index}.#{pane_index} #{?pane_active,*,}'
```

`-a` widens to all sessions; drop it for the current session only. Build your own `-F` format from the [FORMATS section of `man tmux`](x-man-page://tmux) — anything in `#{...}` works.

Single-field queries skip the list:

```sh
tmux display-message -p -t "$PANE" '#{pane_current_path}'
tmux display-message -p -t "$PANE" '#{pane_current_command}'
tmux display-message -p              '#{client_session}'       # session of the calling tty, if any
```

`pane_current_command` reflects the *foreground* process. If the user is in a shell, it'll say `zsh`; if they launched `nvim` directly, it'll say `nvim`; if they ran `zsh -c nvim`, it'll still say `zsh` — fall back to walking children with `pgrep -P #{pane_pid}` to find the real process. The `neovim` skill documents this for the nvim-socket case.

## 3. Send keystrokes — `send-keys`

```sh
tmux send-keys -t "$PANE" <args...>
```

Key names are tmux-style, **not** Vim-style: `Enter`, `Escape`, `Tab`, `BSpace`, `Space`, `C-c`, `C-d`, `M-x`, `S-Enter`, `F5`, etc. Literal characters are sent as-is. Each argument is a key (or run of literals); separate multi-step inputs into multiple args:

```sh
# run a command and press Enter
tmux send-keys -t "$PANE" 'pytest -k foo' Enter

# Ctrl-C the foreground process
tmux send-keys -t "$PANE" C-c

# arrow up + Enter (recall last command)
tmux send-keys -t "$PANE" Up Enter
```

`-l` (literal) disables key-name translation — use it when the payload could collide with a key name:

```sh
tmux send-keys -t "$PANE" -l 'Enter the building'   # types the literal string
tmux send-keys -t "$PANE" Enter                     # then presses Enter
```

`send-keys` is **fire-and-forget**: zero exit status only means the keys were queued, not that the command ran. To know when something finished, poll via `capture-pane` for a sentinel (see §4).

### Pasting multi-line / shell-meaningful content

Don't `send-keys` a payload with newlines, quotes, or backslashes — escaping is a minefield. Use the buffer mechanism:

```sh
printf '%s\n' "$payload" | tmux load-buffer -
tmux paste-buffer -t "$PANE"          # pastes, leaves buffer
tmux paste-buffer -d -t "$PANE"       # pastes and deletes the buffer
```

`paste-buffer` injects the text exactly; if the receiving program is in cooked mode (a shell prompt) it'll interpret newlines as Enter and run intermediate lines. To paste *as data* into a TUI that supports bracketed paste, add `-p` (`paste-buffer -p`).

## 4. Capture pane output — `capture-pane`

The primary "read state" mechanism. Coarser than nvim's `--remote-expr` — you see the rendered terminal, not editor state.

```sh
tmux capture-pane -t "$PANE" -p                 # visible screen only
tmux capture-pane -t "$PANE" -p -S -200         # last 200 lines of scrollback
tmux capture-pane -t "$PANE" -p -S - -E -       # entire scrollback
tmux capture-pane -t "$PANE" -p -J              # join wrapped lines into logical lines
tmux capture-pane -t "$PANE" -p -e              # keep ANSI escape sequences (colors)
```

Recipes:

```sh
# tail the last 200 lines of a build running in another pane
tmux capture-pane -t "$PANE" -p -S -200

# poll until a long command finishes
until tmux capture-pane -t "$PANE" -p | grep -q 'PASSED\|FAILED'; do sleep 1; done
```

For a TUI (nvim, lazygit, claude code), `capture-pane` returns the *current frame*, not a log of events. If you need editor state, drive the editor directly via its own RPC (e.g. the `neovim` skill).

## 5. Spawn panes, windows, sessions

All three take `-c <cwd>` and an optional trailing shell command.

```sh
# Split the target pane. -h = side-by-side (new pane to the right); -v = stacked.
tmux split-window -t "$PANE" -h -c "$PWD" 'tail -F /tmp/log'

# New window in a target session, given a name and starting command.
tmux new-window -t work: -c "$PWD" -n logs 'tail -F /tmp/log'

# New detached session — doesn't replace the current shell. -A reuses if it already exists.
tmux new-session -d -A -s scratch -c "$HOME" 'zsh'
```

Always pass `-c` explicitly. The user's config inherits cwd for *interactive* splits (`prefix v`/`prefix b`), but that's the binding doing the work; programmatic invocations don't get it for free.

After spawning, `-P -F '#{pane_id}'` prints the new pane's id so you can target it:

```sh
NEW=$(tmux split-window -t "$PANE" -h -c "$PWD" -P -F '#{pane_id}' 'less /tmp/log')
tmux send-keys -t "$NEW" 'G'   # jump to bottom
```

## 6. Popups / scratchpad — `display-popup`

A popup is a floating window over the current client. It's tied to *the calling client*, not a pane id — if you're not running in an attached tmux client, it'll fail with "no client".

```sh
tmux display-popup -E -w 80% -h 80% 'nvim /tmp/scratch.md'
tmux display-popup -E -d "$PWD" 'lazygit'
```

`-E` closes the popup when the command exits. `-d` sets cwd. Sizes accept `%`, lines/columns, or pixels.

The user has `prefix g` bound to a persistent scratchpad: `display-popup -E -w 80% -h 80% "tmux new-session -A -s scratch"`. Because `scratch` is a real session, you can also `send-keys -t scratch:` to drive it from anywhere — the popup just attaches a client to it on demand.

## 7. Buffers and clipboard

Tmux buffer stack (LIFO; most recent is buffer 0):

```sh
tmux set-buffer 'some text'           # push a buffer
tmux load-buffer -                    # push stdin as a buffer
tmux show-buffer                      # print top buffer to stdout
tmux save-buffer /tmp/out             # write top buffer to file
tmux list-buffers
tmux delete-buffer                    # pop top buffer
```

This config's copy-mode bindings pipe selections to `pbcopy` (macOS system clipboard), so the most recently copied text is reachable two ways:

- `pbpaste` — what the user just yanked in copy mode.
- `tmux show-buffer` — top of the tmux buffer stack (may or may not match `pbpaste` depending on which binding was used).

## 8. Composing with the `neovim` skill

The common pattern: identify which pane is running nvim, then hand off to the `neovim` skill to drive it.

```sh
tmux list-panes -a -F '#{pane_id} #{pane_pid} #{pane_current_command}' | grep ' nvim$'
```

If `pane_current_command` is a shell wrapper instead of `nvim`, walk children:

```sh
pgrep -P "$PANE_PID"        # → nvim pid (or another wrapper to descend further)
```

Once you have the nvim pid, the `neovim` skill's §1 explains how to locate its msgpack-RPC socket (`$TMPDIR/nvim.$USER/<id>/nvim.<pid>.0` on macOS). From there everything is `nvim --server <sock>`, not tmux.

## 9. Caveats

- **`send-keys` is async.** Zero exit ≠ command done. Poll via `capture-pane` for a sentinel string, or — if you control the command — append something like `; echo __DONE__` and grep for it.
- **Indices renumber.** `renumber-windows on` is set in this config, and panes get renumbered on close. Resolve `session:window.pane` to `#{pane_id}` / `#{window_id}` once and reuse the unique id.
- **`capture-pane` shows rendered terminal output.** Useful for shells and line-oriented programs. For a TUI you see the current frame, not history — drive the program directly if it has an RPC (e.g. nvim).
- **Don't hijack the user's shell.** Prefer `split-window` / `new-window` to running things in a pane the user is actively typing in. If you must inject into a shared pane, `capture-pane -p | tail -1` first to confirm the prompt is idle (last non-empty line ends with `$ `, `% `, or similar).
- **Prefix is `C-a` in this config, not `C-b`.** Irrelevant when calling `tmux` directly from a shell — you never send the prefix — but worth knowing if a recipe ever does `send-keys C-b`, it's wrong here.
- **Extended keys are on.** `S-Enter`, `C-Enter`, etc. propagate to inner programs (Claude Code, nvim). No special handling needed; just don't be surprised they work.
- **Popups need a client.** `display-popup` only works when invoked from inside an attached tmux session, not from a detached shell.
