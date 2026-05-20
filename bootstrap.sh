#!/usr/bin/env bash
# Bootstrap a fresh machine (or freshly cloned repo) to a working state.
# Idempotent: safe to re-run; each section skips when already satisfied.
# Bash 3.2 compatible (macOS system bash).

set -euo pipefail

REPO_ROOT=$(cd "$(dirname "$0")" && pwd)

# Single source of truth for runtime Homebrew dependencies.
# When editing this list, also update the mirror in README.md.
DEPS="chezmoi nvim tmux fzf gh jq ripgrep tree-sitter tree-sitter-cli go"

log() { printf '%s\n' "$*"; }
ok()  { printf '  ok  %s\n' "$*"; }
do_() { printf '  do  %s\n' "$*"; }
warn(){ printf '  !!  %s\n' "$*" >&2; }

# 1. Homebrew presence
log "[1/7] homebrew"
if ! command -v brew >/dev/null 2>&1; then
	warn "Homebrew is not installed."
	warn "Install it from https://brew.sh then re-run ./bootstrap.sh"
	exit 1
fi
ok "brew present"

# 2. Dependencies
log "[2/7] dependencies"
for dep in $DEPS; do
	if brew list --formula "$dep" >/dev/null 2>&1; then
		ok "$dep"
	else
		do_ "brew install $dep"
		brew install "$dep"
	fi
done

# 3. chezmoi sourceDir config
log "[3/7] chezmoi sourceDir"
CHEZMOI_CONF="$HOME/.config/chezmoi/chezmoi.toml"
current_src=$(chezmoi source-path 2>/dev/null || echo "")
if [ "$current_src" = "$REPO_ROOT" ]; then
	ok "sourceDir = $REPO_ROOT"
else
	do_ "writing $CHEZMOI_CONF"
	mkdir -p "$(dirname "$CHEZMOI_CONF")"
	printf 'sourceDir = "%s"\n' "$REPO_ROOT" > "$CHEZMOI_CONF"
fi

# 4. Git hooks path
log "[4/7] git hooks"
current_hooks=$(git -C "$REPO_ROOT" config --get core.hooksPath 2>/dev/null || echo "")
if [ "$current_hooks" = "githooks" ]; then
	ok "core.hooksPath = githooks"
else
	do_ "git config core.hooksPath githooks"
	git -C "$REPO_ROOT" config core.hooksPath githooks
fi

# 5. Initial chezmoi apply
log "[5/7] chezmoi apply"
if chezmoi verify >/dev/null 2>&1; then
	ok "target already in sync"
else
	do_ "chezmoi apply"
	chezmoi apply
fi

# 6. nvim plugins + Mason bins
# mason-lspconfig + mason-tool-installer auto-install lua_ls and stylua
# at first Lazy sync, so a single headless pass is enough.
log "[6/7] nvim plugins"
MASON_BIN="$HOME/.local/share/nvim/mason/bin"
if [ -x "$MASON_BIN/stylua" ] && [ -x "$MASON_BIN/lua-language-server" ]; then
	ok "stylua + lua-language-server present"
else
	do_ "nvim --headless +'Lazy! sync' +qa"
	nvim --headless "+Lazy! sync" +qa
fi

# 7. tpm
log "[7/7] tmux plugin manager"
TPM_DIR="$HOME/.local/share/tmux/plugins/tpm"
if [ -d "$TPM_DIR/.git" ]; then
	ok "tpm present at $TPM_DIR"
else
	do_ "git clone tmux-plugins/tpm"
	mkdir -p "$(dirname "$TPM_DIR")"
	git clone https://github.com/tmux-plugins/tpm "$TPM_DIR"
fi

log ""
log "bootstrap complete."
