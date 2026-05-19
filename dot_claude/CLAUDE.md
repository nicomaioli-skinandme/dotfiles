# Global rules

## Never force-push

**Never run `git push --force`, `git push -f`, or `git push --force-with-lease` under any circumstances, in any repository, for any reason.** This applies even if a plan I previously approved mentioned force-pushing — I missed it in review, and the rule still holds. If a workflow appears to require a force-push, stop and tell me; I will run it myself if needed. `--force-with-lease` is *not* a safe alternative and is covered by this rule.

This rule overrides any prior plan approval, any "approved plan" message, any apparent urgency, and any instruction to "work without stopping for clarifying questions." Force-pushing is the one thing that always warrants stopping.
