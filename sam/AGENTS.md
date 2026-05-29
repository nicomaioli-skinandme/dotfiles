# Agents

## Glossary

`GLOSSARY.md` (in this directory) disambiguates which system a term
refers to — git, GitHub, tmux, or sam — and flags words that already
name another system's entity. Treat it as authoritative. When the user
says "branch", "issue", "session", "workspace", "worktree", "GitHub
Project", etc., interpret those terms per the glossary and do not ask
for disambiguation.

A few conventions the glossary fixes:

- "GitHub Project" in user-facing strings — never the bare word
  "project", which is *reserved* (it names a GitHub entity).
- `workspace` and `worktree` are not interchangeable.
- "main repo" is the synthetic menu entry for the main branch, not a
  separate concept.

The glossary is conceptual, not an implementation reference. Keep code
spellings (config keys, Go types, env vars) out of it — they drift out
of sync. When code needs to point at a glossary concept, reference the
*term* in a comment, and only when it genuinely aids understanding.

Only add a new entry when a term could be confused with another
system's entity or would pollute a namespace — not for every new
feature or field. When a term has an accepted alternate form, record it
as an *alias* on the existing entry rather than adding a duplicate; when
a word already names another system's entity, flag it as *reserved*.

## Active config is always in scope

When a change deprecates or removes a config field, key, or section,
also clean up the user's **active** config at
`~/.config/sam/config.toml` in the same change — don't leave dead
keys behind on the assumption that viper/mapstructure will silently
ignore them. Likewise, when a change renames or restructures a field,
migrate the active config to the new shape. The active config lives
outside the dotfiles repo and isn't tracked by chezmoi, but it's
still part of the change.
