#!/usr/bin/env bash
MONOREPO="$HOME/Code/andbegin-monorepo"
WORKTREES_DIR="$HOME/Code/andbegin-monorepo.worktrees"

# GitHub project IDs for the "+ from issue" flow.
# Regenerate with: gh project field-list 45 --owner skinandmeprojects --format json
GH_PROJECT_OWNER="skinandmeprojects"
GH_PROJECT_NUMBER=45
GH_PROJECT_ID="PVT_kwDOBLqSu84AVoEL"
GH_STATUS_FIELD_ID="PVTSSF_lADOBLqSu84AVoELzgN0d5A"
GH_STATUS_IN_PROGRESS_ID="15c99605"
GH_REPO="skinandmeprojects/andbegin"         # issues live here
GH_BRANCH_REPO="skinandme/andbegin-monorepo" # branches/PRs live here

create_system_session() {
  if tmux has-session -t "system" 2>/dev/null; then
    return
  fi

  tmux new-session -d -s "system" -n "home" -c "$HOME"
  tmux new-window -t "system" -n "dotfiles" -c "$HOME/Code/dotfiles"
  tmux select-window -t "system:home"
}

create_andbegin_session() {
  local session="$1"
  local project_dir="$2"

  tmux new-session -d -s "$session" -n "repo" -c "$project_dir"

  tmux new-window -t "$session" -n "local" -c "$project_dir/backend"
  tmux split-window -h -t "$session:local" -c "$project_dir/store-ui"

  tmux new-window -t "$session" -n "uat" -c "$project_dir/deployment/uat"

  tmux select-window -t "$session:repo"
}

# Collect existing tmux sessions into a colon-delimited string for lookup
active_sessions=":"
while IFS= read -r s; do
  [ -z "$s" ] && continue
  active_sessions+="$s:"
done < <(tmux list-sessions -F '#S' 2>/dev/null)

decorate() {
  local name="$1" label="${2:-$1}"
  if [[ "$active_sessions" == *":$name:"* ]]; then
    printf '● %s' "$label"
  else
    printf '%s' "$label"
  fi
}

# Build menu options
options=("$(decorate system)" "$(decorate master "master  (main repo)")")
paths=("$HOME" "$MONOREPO")
sessions=("system" "master")

for dir in "$WORKTREES_DIR"/*/; do
  [ -d "$dir" ] || continue
  branch=$(basename "$dir")
  options+=("$(decorate "$branch")")
  paths+=("${dir%/}")
  sessions+=("$branch")
done

options+=("+ from issue")
options+=("+ new worktree")
options+=("- delete worktree")

# Prompt user to pick a worktree interactively
selected=$(printf '%s\n' "${options[@]}" | fzf --prompt="Select a worktree: " --height=~50% --reverse --no-info --cycle --bind=tab:down,btab:up)

if [ -z "$selected" ]; then
  echo "No selection made."
  exit 1
fi

# Handle "from issue" selection
if [ "$selected" = "+ from issue" ]; then
  command -v jq >/dev/null || {
    echo "jq required" >&2
    exit 1
  }

  echo "Fetching current gh user..."
  me=$(gh api user -q .login)

  # Fetch Backlog + Platform Backlog issues in the monorepo.
  # Tab-delimited: num \t status \t assignees(csv) \t itemId \t title
  echo "Fetching project $GH_PROJECT_NUMBER items from $GH_PROJECT_OWNER..."
  items=$(gh project item-list "$GH_PROJECT_NUMBER" --owner "$GH_PROJECT_OWNER" --format json --limit 200 |
    jq -r --arg repo "$GH_REPO" '
        .items[]
        | select(.content.repository == $repo)
        | select(.status == "📋 Backlog" or .status == "Platform Backlog")
        | [(.content.number|tostring), .status, ((.assignees // []) | join(",")), .id, .content.title]
        | @tsv')
  # Surface upstream failures that would otherwise leave $items empty and
  # look like "no matching issues".
  gh_status=${PIPESTATUS[0]}
  jq_status=${PIPESTATUS[1]}
  if [ "$gh_status" -ne 0 ] || [ "$jq_status" -ne 0 ]; then
    echo "Failed to fetch project items (gh=$gh_status jq=$jq_status)." >&2
    exit 1
  fi

  if [ -z "$items" ]; then
    echo "No Backlog or Platform Backlog issues in $GH_REPO."
    exit 0
  fi
  echo "Found $(printf '%s\n' "$items" | wc -l | tr -d ' ') matching issues."

  # Show columns 1 (#num), 2 (status), 5 (title); keep all 5 for post-selection parsing
  selected_item=$(printf '%s\n' "$items" | fzf \
    --prompt="Select issue: " \
    --height=~50% --reverse --no-info --cycle \
    --bind=tab:down,btab:up \
    --delimiter=$'\t' --with-nth=1,2,5)

  if [ -z "$selected_item" ]; then
    echo "No issue selected."
    exit 1
  fi

  issue_num=$(printf '%s' "$selected_item" | cut -f1)
  issue_status=$(printf '%s' "$selected_item" | cut -f2)
  issue_assignees=$(printf '%s' "$selected_item" | cut -f3)
  item_id=$(printf '%s' "$selected_item" | cut -f4)

  # Assignment: unassigned → self; already mine → no-op; else → confirm and swap
  if [ -z "$issue_assignees" ]; then
    echo "Assigning issue #$issue_num to @me..."
    gh issue edit "$issue_num" --repo "$GH_REPO" --add-assignee "@me" >/dev/null
  elif [ "$issue_assignees" = "$me" ]; then
    :
  else
    confirm=$(printf 'No\nYes\n' | fzf \
      --prompt="Issue is assigned to $issue_assignees. Reassign to you? " \
      --height=~50% --reverse --no-info --cycle \
      --bind=tab:down,btab:up,y:last+accept,n:first+accept)
    if [ "$confirm" != "Yes" ]; then
      echo "Aborted."
      exit 1
    fi
    echo "Reassigning issue #$issue_num to @me..."
    gh issue edit "$issue_num" --repo "$GH_REPO" \
      --remove-assignee "$issue_assignees" --add-assignee "@me" >/dev/null
  fi

  # Status → In Progress (if not already)
  if [ "$issue_status" != "🏗 In Progress" ]; then
    echo "Moving issue #$issue_num to In Progress..."
    gh project item-edit \
      --project-id "$GH_PROJECT_ID" \
      --id "$item_id" \
      --field-id "$GH_STATUS_FIELD_ID" \
      --single-select-option-id "$GH_STATUS_IN_PROGRESS_ID" >/dev/null
  fi

  # Pick up an already-linked branch if one exists; otherwise derive the
  # default name (issuenum-slug) that `gh issue develop` would create, so
  # we can offer a rename BEFORE creating the remote branch.
  existing_branch=$(gh issue develop --list "$issue_num" --repo "$GH_REPO" 2>/dev/null | awk 'NR==1 && NF>0 {print $1}')
  if [ -n "$existing_branch" ]; then
    branch="$existing_branch"
  else
    issue_title=$(printf '%s' "$selected_item" | cut -f5)
    slug=$(printf '%s' "$issue_title" | tr '[:upper:]' '[:lower:]' | tr -c 'a-z0-9' '-' | sed -E 's/-+/-/g; s/^-//; s/-$//')
    branch="${issue_num}-${slug}"
  fi

  # Gate: if the name is unwieldy, offer to edit it. For a not-yet-created
  # branch this is free (we just pass --name). For an existing linked branch
  # we have to rename the remote ref.
  if [ ${#branch} -gt 20 ]; then
    action=$(printf 'Keep as is\nManually edit\n' | fzf \
      --prompt="Branch is ${#branch} chars (> 20). " \
      --height=~50% --reverse --no-info --cycle \
      --bind=tab:down,btab:up)
    if [ "$action" = "Manually edit" ]; then
      # fzf-as-input: macOS bash 3.2 lacks `read -i`, so use fzf's
      # --print-query to get a prepopulated, editable prompt.
      new_branch=$(fzf --print-query \
        --query="${issue_num}-" \
        --prompt="New branch name: " \
        --header="Default: $branch" \
        --height=~5% --reverse --no-info \
        --bind='enter:print-query' < /dev/null)
      new_branch=$(printf '%s' "$new_branch" | tr -d '[:space:]')
      if [ -n "$new_branch" ] && [ "$new_branch" != "$branch" ]; then
        if [ -n "$existing_branch" ]; then
          echo "Renaming on origin: $branch → $new_branch"
          git -C "$MONOREPO" fetch origin
          sha=$(git -C "$MONOREPO" rev-parse "origin/$branch")
          git -C "$MONOREPO" push origin "$sha:refs/heads/$new_branch"
          git -C "$MONOREPO" push origin --delete "$branch"
        fi
        branch="$new_branch"
      fi
    fi
  fi

  if [ -z "$existing_branch" ]; then
    echo "Creating linked branch $branch for issue #$issue_num..."
    develop_output=$(gh issue develop "$issue_num" --repo "$GH_REPO" --branch-repo "$GH_BRANCH_REPO" --name "$branch" 2>&1)
    if [ $? -ne 0 ]; then
      echo "Failed to create linked branch for issue #$issue_num." >&2
      echo "$develop_output" >&2
      exit 1
    fi
  fi
  echo "Branch: $branch"

  git -C "$MONOREPO" fetch origin

  if [ ! -d "$WORKTREES_DIR/$branch" ]; then
    echo "Creating worktree for branch: $branch"
    git -C "$MONOREPO" worktree add "$WORKTREES_DIR/$branch" "$branch"
  fi

  create_system_session
  if ! tmux has-session -t "$branch" 2>/dev/null; then
    create_andbegin_session "$branch" "$WORKTREES_DIR/$branch"
  fi

  if [ -n "$TMUX" ]; then
    tmux switch-client -t "$branch"
  else
    tmux attach-session -t "$branch"
  fi
  exit 0
fi

# Handle "new worktree" selection
if [ "$selected" = "+ new worktree" ]; then
  # Collect existing worktree branch names for filtering.
  # Colon-delimited string instead of an associative array so this works
  # under stock macOS bash 3.2 (which lacks `declare -A`).
  existing_worktrees=":master:"
  for dir in "$WORKTREES_DIR"/*/; do
    [ -d "$dir" ] || continue
    existing_worktrees+="$(basename "$dir"):"
  done

  # Fetch local + remote branches sorted by most recent commit
  seen=":"
  filtered=()
  while IFS= read -r ref; do
    # Blank for symbolic refs (e.g. origin/HEAD -> origin/master), skip those
    [ -z "$ref" ] && continue
    # Strip origin/ prefix from remote branches
    name="${ref#origin/}"
    [[ "$seen" == *":$name:"* ]] && continue
    [[ "$existing_worktrees" == *":$name:"* ]] && continue
    seen+="$name:"
    filtered+=("$name")
  done < <(git -C "$MONOREPO" for-each-ref \
    --sort=-committerdate \
    refs/heads refs/remotes/origin \
    --format='%(if)%(symref)%(then)%(else)%(refname:short)%(end)')

  branch=$(printf '%s\n' "${filtered[@]}" | fzf --prompt="Select branch for new worktree: " --height=~50% --reverse --no-info --cycle --bind=tab:down,btab:up)

  if [ -z "$branch" ]; then
    echo "No branch selected."
    exit 1
  fi

  echo "Creating worktree for branch: $branch"
  git -C "$MONOREPO" worktree add "$WORKTREES_DIR/$branch" "$branch"

  create_system_session
  create_andbegin_session "$branch" "$WORKTREES_DIR/$branch"

  if [ -n "$TMUX" ]; then
    tmux switch-client -t "$branch"
  else
    tmux attach-session -t "$branch"
  fi
  exit 0
fi

# Handle "delete worktree" selection
if [ "$selected" = "- delete worktree" ]; then
  deletable=()
  for dir in "$WORKTREES_DIR"/*/; do
    [ -d "$dir" ] || continue
    deletable+=("$(basename "$dir")")
  done

  if [ ${#deletable[@]} -eq 0 ]; then
    echo "No worktrees to delete."
    exit 0
  fi

  target=$(printf '%s\n' "${deletable[@]}" | fzf --prompt="Select worktree to delete: " --height=~50% --reverse --no-info --cycle --bind=tab:down,btab:up)

  if [ -z "$target" ]; then
    echo "No selection made."
    exit 1
  fi

  current_session=""
  if [ -n "$TMUX" ]; then
    current_session=$(tmux display-message -p '#S')
  fi

  if [ "$current_session" = "$target" ]; then
    confirm=$(printf 'No\nYes\n' | fzf --prompt="Currently in '$target'. Delete anyway? " --height=~50% --reverse --no-info --cycle --bind=tab:down,btab:up,y:last+accept,n:first+accept)
    if [ "$confirm" != "Yes" ]; then
      echo "Aborted."
      exit 1
    fi
    create_system_session
    tmux switch-client -t "system"
  fi

  if tmux has-session -t "$target" 2>/dev/null; then
    tmux kill-session -t "$target"
  fi

  git -C "$MONOREPO" worktree remove --force "$WORKTREES_DIR/$target"

  echo "Deleted worktree: $target"
  exit 0
fi

# Find the index of the selected option
idx=-1
for i in "${!options[@]}"; do
  if [ "${options[$i]}" = "$selected" ]; then
    idx=$i
    break
  fi
done

if [ "$idx" -eq -1 ]; then
  echo "Invalid selection."
  exit 1
fi
SESSION="${sessions[$idx]}"
PROJECT_DIR="${paths[$idx]}"

# If the session already exists, attach/switch to it
if tmux has-session -t "$SESSION" 2>/dev/null; then
  if [ -n "$TMUX" ]; then
    tmux switch-client -t "$SESSION"
  else
    tmux attach-session -t "$SESSION"
  fi
  exit 0
fi

# Handle system session
if [ "$SESSION" = "system" ]; then
  create_system_session
else
  # Ensure system session exists in the background
  create_system_session
  create_andbegin_session "$SESSION" "$PROJECT_DIR"
fi

if [ -n "$TMUX" ]; then
  tmux switch-client -t "$SESSION"
else
  tmux attach-session -t "$SESSION"
fi
