---
name: ab-pr
description: Open a GitHub pull request following andbegin project conventions.
---

# ab-pr

Lightweight skill for opening a pull request when the implementation work is already done. Assumes the active GitHub issue's number and title are already in conversation context. If they aren't, prompt the user for the issue URL.

This skill does NOT implement, refactor, or commit. It only drafts the PR, gets explicit user approval, and runs `gh pr create`.

## Workflow

### 1. Pre-flight

Run these in parallel:

- `git status` — note any uncommitted changes; warn the user but don't auto-stage or auto-commit.
- `git rev-parse --abbrev-ref HEAD` — current branch.
- `gh repo view --json defaultBranchRef -q .defaultBranchRef.name` — default branch (the PR base).
- `git log <base>..HEAD --oneline` and `git diff <base>...HEAD --stat` — what the PR will contain.
- Look for a PR template, in this order, and read the first one that exists:
  - `.github/PULL_REQUEST_TEMPLATE.md`
  - `.github/pull_request_template.md`
  - `PULL_REQUEST_TEMPLATE.md`
  - `docs/PULL_REQUEST_TEMPLATE.md`

### 2. Resolve issue context

The issue number and title should already be in conversation context. Use them directly.

If they are missing, ask the user for the issue URL, then:

```sh
gh issue view <url> --json number,title -q '"\(.number) \(.title)"'
```

### 3. Draft the title

Format exactly: `<issue> - <issue title>`

Example: `1423 - Allow customers to update their delivery address mid-cycle`

No conventional-commit prefixes, no scope tags, no extra punctuation. Use the issue's title verbatim unless the user has explicitly given a different one.

### 4. Draft the body

Keep it brief and decision-focused — the diff and linked issue carry the detail. A good PR description surfaces *what mattered and why*, not a file-by-file change log.

- If a PR template was found, fill its sections in. If no template exists, use a `## Summary` section followed by `Closes #<issue>`.
- **Summary**: lead with one sentence of context — what this enables and *why* (the problem it solves). Then 2–4 short bullets covering the **meaningful decisions and trade-offs**, e.g.:
  - The key design choice and the reasoning behind it (especially anything non-obvious — why this approach over the alternative).
  - Rollout/compatibility reasoning where relevant (e.g. "backend-only change, honours zero-downtime").
  - Notable scope boundaries (what was deliberately left out and why).
- **Do not** enumerate line-by-line changes, list every file touched, or restate what the diff already shows. If a reviewer can see it in the diff, it doesn't need a bullet.
- Leave optional/“Additional changes” sections empty rather than padding them — that section is reserved for genuinely out-of-scope extras.
- Always include a `Closes #<issue>` reference if the template doesn't already.

> Litmus test: every bullet should tell the reviewer something the diff alone wouldn't — a reason, a trade-off, or a decision. Drop bullets that just narrate the code.

### 5. Ask about Release Actions

Use `AskUserQuestion`:

> Are there release actions for this PR?

Options: **No** / **Yes**.

- **No** → omit the section and the label.
- **Yes** → ask the user (free-form) what they are, then:
  - Append a `### Release Actions` section to the body with a bullet list of the actions. This should be the last section.
  - Queue the `release actions` label for `gh pr create --label "release actions"`.

### 6. Ask about Draft

Use `AskUserQuestion`:

> Open as Draft?

Options: **No** / **Yes**. Always ask — never assume either way.

### 7. Approval gate

Show the user the full proposal as one block:

```
Title: <title>

Body:
<body>

Labels: <labels or "(none)">
Draft:  <yes/no>
```

Ask them to **accept** or **edit**. If they edit, apply the edits and re-show only if something material changed. **Do not call `gh pr create` until the user has explicitly accepted.**

Before you submit, ensure to strip out all additional `\n` artefacts that you're introducing when presenting the PR body as a preview. These show as new-lines in GitHub.

### 8. Create the PR

Push the branch if it has no upstream:

```sh
git push -u origin HEAD   # never --force or --force-with-lease — see global rule
```

If `git status` showed uncommitted changes in step 1, surface this to the user before pushing — do not auto-commit.

Then create the PR with a HEREDOC body:

```sh
gh pr create \
  --base "<base>" \
  --title "[#<issue>] - <issue title>" \
  ${DRAFT:+--draft} \
  ${LABEL:+--label "release actions"} \
  --body "$(cat <<'EOF'
<body>
EOF
)"
```

Return the PR URL to the user.

## Safety

- **Never force-push.** This is a global rule. `git push -u origin HEAD` only — if the remote rejects a normal push, stop and tell the user.
- **Never auto-commit or auto-stage.** If `git status` is dirty, surface it and let the user decide.
- **Never skip the approval gate.** The user must see and accept the title + body + labels + draft status before `gh pr create` runs.
