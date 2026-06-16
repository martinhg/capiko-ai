---
name: review-reliability
description: "Reliability-focused code review: error handling, edge cases, test coverage. Trigger: reviewing code for correctness, pre-PR reliability check, or /review-reliability."
license: Apache-2.0
metadata:
  author: capiko-ai
  version: "0.1"
---

## Role

Run a focused reliability review on the diff or files provided. Produce structured
findings — not a generic summary.

## Scope

Only flag issues in these categories:

1. **Error handling** — ignored errors, swallowed exceptions, panics in library code,
   missing cleanup on error paths (defer, close, rollback).
2. **Edge cases** — nil/null/zero-value inputs, empty collections, boundary values,
   concurrent access without synchronization.
3. **Data integrity** — partial writes without atomicity, missing validation at system
   boundaries, inconsistent state after failure.
4. **Test coverage** — new behavior without tests, tests that do not assert the
   meaningful outcome, flaky test patterns (time-dependent, order-dependent).
5. **Contract violations** — functions that silently break their documented or implied
   contract (return type, side effects, invariants).

Do NOT flag style, naming, security, or performance — those belong to other review
skills.

## Output format

For each finding, emit:

```
### [SEVERITY] Title
- **Category**: (one of the five above)
- **Location**: file:line
- **What**: one sentence describing the issue
- **Impact**: what breaks and under what conditions
- **Suggested fix**: code snippet or actionable step
```

Severity levels: CRITICAL (data loss or corruption possible), HIGH (silent wrong
behavior), MEDIUM (degraded behavior under edge conditions), LOW (missing coverage
that a test would catch).

## When no issues are found

State: "No reliability findings in the reviewed scope." Do not invent issues.
