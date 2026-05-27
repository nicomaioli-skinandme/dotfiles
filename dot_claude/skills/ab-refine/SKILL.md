---
name: ab-refine
description: Technical-only refinement of a GitHub issue — assess codebase support and clarity, surface disambiguating questions, following andbegin conventions.
---

# ab-refine

Validate whether a GitHub issue is **technically** ready to work on. This is a
narrow, implementation-focused refinement: assume the idea is valid and
well-thought-out, assume any success metrics will be tracked, and do **not**
question the product value. Focus on whether *this* codebase can support the
request, whether the request is **clear enough to build from**, and what to ask
to remove any remaining ambiguity.

**Read-only.** This skill never comments on the issue, never edits code, never
commits. Output goes to the user in conversation only.

**Default outcome is "Ready as is."** Most issues are fine. Only escalate when
something is genuinely unclear or structurally risky — never manufacture
concerns to justify a worse verdict.

## Scope

In scope:

- Whether the existing codebase supports the request.
- Whether it forces **re-architecting** (the highest-priority signal).
- Whether it requires **new dependencies**.
- **Clarity** — is the request unambiguous enough to implement?
- **Design without Figma** — does it imply UI work with no Figma attached?
- **Experiment hygiene** — for experiments: a name is present, and the work
  won't nest inside another live experiment.
- Specific spots where implementation would be tricky.

Explicitly out of scope — do not raise these:

- The validity, priority, or value of the idea.
- Whether metrics/tracking are defined (assume they are).
- Acceptance-criteria completeness or product wording.

## Workflow

### 1. Resolve the issue

- If an issue URL/number was passed as an argument, use it.
- Else if an issue is already in conversation context, use that.
- Else ask the user for the issue URL.

Fetch it:

```sh
gh issue view <url-or-number> --json number,title,body,labels
```

### 2. Investigate the codebase

Dispatch read-only **Explore** agents (in parallel) to map the areas the issue
touches — the relevant modules, data flow, extension points, and current
dependencies. Use the conclusions, not the file dumps. Put the most effort into:

1. Does this fit the current architecture, or does it require re-architecting?
2. Does it need a dependency the project doesn't already have?

**If it's an experiment** — infer from labels/title/body keywords
(`experiment`, `A/B`, `variant`, `test`); if genuinely ambiguous, ask the
operator. When it is an experiment:

- Check the target code path isn't already inside another live experiment
  (**nested experiment** — confounds results; flag it).
- Confirm the issue carries an **experiment name**. If missing, call it out.

### 3. Clarify (interactive loop)

When the issue, or how it fits the codebase, is genuinely unclear, ask the
operator via `AskUserQuestion`. Take the answers; if they open new uncertainty,
ask follow-ups. Bias toward *not* asking — only ask when an answer would change
the verdict or resolve real ambiguity.

Always ask the operator about **design + no Figma** when the issue implies UI
work but has no Figma attached ("design seems needed but none is attached — is
one required?") — this is operator judgment, not automatically a blocker.

Anything still unresolved after the loop becomes the report's **Open questions**
(for the issue author/PM).

### 4. Report

Brief verdict + sections. **Be terse** — bullets, not paragraphs. Omit any
section with nothing to say rather than padding it.

```markdown
## Technical refinement: #<number> — <title>

**Verdict:** Ready as is | Ready, with open questions | Blocked — needs re-architecting

### Open questions
- <residual ambiguity to take to the author; omit if none>

### Codebase support
- <does what exists support this; where it plugs in>

### Re-architecting risk
- <what, if anything, has to change structurally>

### New dependencies
- <none, or the specific deps and why>

### Design
- <only if relevant: design implied? Figma attached?>

### Experiment
- <only if an experiment: name present? nesting risk?>

### Tricky spots
- <concrete implementation hazards>
```

If the issue is clear, fits cleanly, and needs no new deps, say exactly that —
**Ready as is** — in a line or two. That is the expected default, not a fallback.
