---
name: review-risk
description: "Security-focused code review: auth, data exposure, injection, secrets. Trigger: reviewing code for security risks, pre-PR security check, or /review-risk."
license: Apache-2.0
metadata:
  author: capiko-ai
  version: "0.1"
---

## Role

Run a focused security and risk review on the diff or files provided. Produce
structured findings — not a generic summary.

## Scope

Only flag issues in these categories:

1. **Authentication & authorization** — missing or bypassable auth checks, privilege
   escalation paths, broken access control.
2. **Data exposure** — secrets in code or config, PII leaks in logs or responses,
   overly broad API responses.
3. **Injection** — SQL, command, template, path traversal, or any unsanitized input
   reaching a sink.
4. **Cryptography** — weak algorithms, hardcoded keys, missing TLS validation,
   predictable tokens.
5. **Dependency risk** — known-vulnerable versions, unnecessary transitive exposure.

Do NOT flag style, naming, performance, or test coverage — those belong to other
review skills.

## Output format

For each finding, emit:

```
### [SEVERITY] Title
- **Category**: (one of the five above)
- **Location**: file:line
- **What**: one sentence describing the issue
- **Why it matters**: the concrete risk if left unfixed
- **Suggested fix**: code snippet or actionable step
```

Severity levels: CRITICAL (exploitable now), HIGH (exploitable with effort),
MEDIUM (defense-in-depth gap), LOW (hardening opportunity).

## When no issues are found

State: "No risk findings in the reviewed scope." Do not invent issues.
