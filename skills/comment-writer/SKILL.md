---
name: comment-writer
description: "Write warm, direct collaboration comments. Trigger: PR reviews, issue replies, or review feedback."
---

## Principle

Comments are for a person, not a linter. Be warm and direct: respect the author,
be clear about what matters, and make the next step obvious.

## How

- Lead with the point. Say what you'd change and **why**, with evidence (link the
  line, the test, the doc).
- Separate severity: blocking vs nit. Mark nits as nits.
- Prefer questions when you're unsure ("is this intentional?") over assertions.
- Suggest the fix, not just the problem — a concrete diff or example.
- Acknowledge good work; reviews aren't only for faults.

## Avoid

- Vague "this is wrong" without the why.
- Stacking ten nits as if they were blockers.
- Sarcasm. Disagree with the code, never the person.
