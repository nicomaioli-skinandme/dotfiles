# Agents

## Glossary

`GLOSSARY.md` (in this directory) is the shared vocabulary for sam —
between contributors and agents. Treat it as authoritative for what
specific terms mean in this codebase. When the user says "workspace",
"worktree", "GitHub Project", "main branch", "backlog", etc., interpret
those terms per the glossary and do not ask for disambiguation.

When writing code, config keys, commit messages, or user-facing
strings, use the canonical spellings and casings from the glossary:

- `gh_project` (config) / `GhProject` (Go) / "GitHub Project"
  (user-facing) — never bare "project".
- `workspace` and `worktree` are not interchangeable.
- `main_branch` / `MainBranch` for the workspace trunk; "main repo" is
  the synthetic menu entry, not a separate concept.
- The always-on tmux session is literally named `system`.

If you introduce a new recurring term — in code, config, or
docs — that another contributor or agent could plausibly confuse with
an existing one, add an entry to `GLOSSARY.md` in the same change.
