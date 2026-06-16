---
name: review-resilience
description: "Resilience-focused code review: failure modes, recovery, graceful degradation. Trigger: reviewing code for fault tolerance, pre-PR resilience check, or /review-resilience."
license: Apache-2.0
metadata:
  author: capiko-ai
  version: "0.1"
---

## Role

Run a focused resilience review on the diff or files provided. Produce structured
findings — not a generic summary.

## Scope

Only flag issues in these categories:

1. **Failure modes** — what happens when a dependency is unavailable, slow, or returns
   unexpected data. Missing timeouts, unbounded retries, no circuit breakers on
   external calls.
2. **Recovery** — whether the system can return to a healthy state after failure.
   Missing health checks, no retry-with-backoff, no fallback behavior.
3. **Graceful degradation** — hard failures where partial service would be acceptable.
   All-or-nothing designs that could serve stale data, skip optional enrichment, or
   disable non-critical features instead.
4. **Resource management** — unbounded queues, goroutine/thread leaks, connection pool
   exhaustion, missing rate limiting on inbound requests.
5. **Observability gaps** — failures that happen silently without logging, metrics, or
   alerts. A failure that nobody notices is a failure that nobody fixes.

Do NOT flag style, naming, correctness logic, or test structure — those belong to
other review skills.

## Output format

For each finding, emit:

```
### [SEVERITY] Title
- **Category**: (one of the five above)
- **Location**: file:line
- **What**: one sentence describing the failure mode
- **Blast radius**: what breaks and how far the damage spreads
- **Suggested fix**: timeout, fallback, circuit breaker, or design change
```

Severity levels: CRITICAL (cascading failure or total outage), HIGH (single-service
outage with no recovery), MEDIUM (degraded performance or delayed recovery),
LOW (missing observability or hardening opportunity).

## When no issues are found

State: "No resilience findings in the reviewed scope." Do not invent issues.
