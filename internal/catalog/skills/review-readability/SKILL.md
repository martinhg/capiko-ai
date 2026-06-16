---
name: review-readability
description: "Readability-focused code review: naming, cognitive load, structure, comments. Trigger: reviewing code for clarity, pre-PR readability check, or /review-readability."
license: Apache-2.0
metadata:
  author: capiko-ai
  version: "0.1"
---

## Role

Run a focused readability review on the diff or files provided. Produce structured
findings — not a generic summary.

## Scope

Only flag issues in these categories:

1. **Naming** — misleading, ambiguous, or inconsistent names for variables, functions,
   types, or files relative to what they actually do.
2. **Cognitive load** — functions or blocks that require too many things in working
   memory: deep nesting, long parameter lists, interleaved concerns, boolean
   combinatorics.
3. **Structure** — code that would be clearer with extraction, reordering, or
   collapsing. Guard clauses instead of nested ifs. Early returns instead of flag
   variables.
4. **Comments** — missing comments where the WHY is non-obvious, or stale/misleading
   comments that contradict the code.
5. **Consistency** — style or pattern breaks within the file or relative to neighboring
   files in the same package.

Do NOT flag security, error handling, performance, or test gaps — those belong to
other review skills.

## Output format

For each finding, emit:

```
### [SEVERITY] Title
- **Category**: (one of the five above)
- **Location**: file:line
- **What**: one sentence describing the issue
- **Suggestion**: concrete rename, restructure, or rewrite
```

Severity levels: HIGH (actively misleading — a reader would misunderstand the code),
MEDIUM (unclear — requires extra effort to understand), LOW (polish — marginal
clarity improvement).

## When no issues are found

State: "No readability findings in the reviewed scope." Do not invent issues.
