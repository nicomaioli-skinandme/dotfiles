#!/usr/bin/env bash
# Populate a freshly-created dotfiles worktree with local-only files that
# aren't tracked in git (so chezmoi/claude pick up the same per-machine
# config as the source checkout). Run from inside the new worktree:
#
#   ./worktree_setup.sh ~/Code/dotfiles    # source = primary checkout
#
# Idempotent — skips files that already exist in the target.
#
# Bash 3.2 compatible (macOS system bash). No associative arrays, no
# mapfile, no ${var,,}.

set -e

dir_name="$(cd "$(dirname "$0")" && pwd)"

copy_source_files() {
    SOURCE_DIR="$1"

    if [ ! -d "$SOURCE_DIR" ]; then
        echo "Error: source directory $SOURCE_DIR does not exist" >&2
        exit 1
    fi

    if [ "$SOURCE_DIR" = "$dir_name" ]; then
        echo "Error: source and target are the same directory ($dir_name)" >&2
        exit 1
    fi

    echo "Copying local-only files from $SOURCE_DIR..."

    # Whitelist of files/globs to copy. These are .chezmoiignore'd or
    # otherwise untracked, so a fresh worktree won't have them.
    WHITELIST_PATTERNS=(
        ".claude/settings.local.json"
    )

    for pattern in "${WHITELIST_PATTERNS[@]}"; do
        for item in "$SOURCE_DIR"/$pattern; do
            [ -e "$item" ] || continue

            relative_path="${item#"$SOURCE_DIR/"}"
            target_item="$dir_name/$relative_path"

            if [ -e "$target_item" ]; then
                echo "Skipping $relative_path (already exists)"
                continue
            fi

            target_dir=$(dirname "$target_item")
            mkdir -p "$target_dir"

            echo "Copying $relative_path"
            cp "$item" "$target_item"
        done
    done
}

if [ -z "$1" ]; then
    echo "Usage: $0 <source-checkout-dir>" >&2
    echo "  e.g. $0 ~/Code/dotfiles" >&2
    exit 2
fi

copy_source_files "$1"

echo ""
echo "✓ Worktree configured: $dir_name"
