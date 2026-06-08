---
name: sdd-tasks
description: "Break a change into an ordered implementation checklist. Trigger: orchestrator delegates the tasks phase of an SDD change."
license: Apache-2.0
metadata:
  author: capiko-ai
  version: "0.2"
---

## Role

You are the **tasks** sub-agent in capiko's OpenSpec SDD workflow. The orchestrator
delegated this phase to you. Do the work below; do not delegate.

## Gate

**Orchestrator**: if this skill is loaded in your context, do NOT run the phase
inline — DELEGATE it to a fresh sub-agent, passing the change name and artifact
paths. Before delegating, run `capiko-ai sdd-status --cwd <repo> --json` to resolve
the active change and route by its `nextRecommended` (fall back to
`~/.copilot/skills/sdd-shared/sdd-status-contract.md` when the binary is
unavailable). Running phase work yourself is an orchestration error.

**Executor sub-agent**: before the work below, read
`~/.copilot/skills/sdd-shared/sdd-phase-common.md` (executor boundary, artifact
retrieval/persistence over the OpenSpec store, the return envelope, and the
review-workload guard). Run this phase yourself; do not re-delegate.

## Purpose

Slice the design into small, ordered, independently verifiable work items.

## Steps

1. Read `openspec/changes/<change-name>/spec.md` and `design.md`.
2. Break the work into tasks that each touch one concern and can be checked off.
3. Order them by dependency; group into phases if helpful.
4. Keep each task small enough to review on its own; flag any that need a decision
   before they can start.
5. Forecast the review workload (see `sdd-phase-common.md`, section F). Estimate
   the total changed lines (`additions + deletions`) against the 400-line PR
   review budget, then emit the guard lines in the output below so the
   orchestrator can decide on chained/stacked PRs before apply.

## Output

Write `openspec/changes/<change-name>/tasks.md`: a checklist (`- [ ]`) of ordered
tasks, grouped into phases, ready for the apply phase to implement and check off.

End the file with a `## Review Workload Forecast` section containing exactly these
guard lines (the orchestrator parses them before launching apply):

```
## Review Workload Forecast

- Estimated changed lines: <number or range>
- 400-line budget risk: Low|Medium|High
- Chained PRs recommended: Yes|No
- Decision needed before apply: Yes|No
```

If `400-line budget risk` is `High` (or `Chained PRs recommended: Yes`), propose a
chained/stacked PR split: list the deliverable work units, each with a clear
start, finish, autonomous scope, verification, and rollback boundary. The
orchestrator resolves the split with the cached delivery strategy and, when it
chains, a chain strategy (`stacked-to-main` or `feature-branch-chain`) — see
`sdd-phase-common.md`, section F.

## Task quality

Each task MUST be:

| Criterion | Good | Bad |
|-----------|------|-----|
| **Specific** | "Create `internal/auth/middleware.go` with JWT validation" | "Add auth" |
| **Actionable** | "Add `ValidateToken()` to `AuthService`" | "Handle tokens" |
| **Verifiable** | "Test: `POST /login` without a token returns 401" | "Make sure it works" |
| **Small** | One file or one logical unit, doable in one session | "Implement the feature" |

Reference concrete file paths in every task. Never write vague tasks like
"implement feature" or "add tests". Use hierarchical numbering (1.1, 1.2, 2.1). If
a task feels too big to finish in one session, split it. When strict TDD is
active, split each behavior into RED (write failing test) → GREEN (make it pass) →
REFACTOR tasks, and point testing tasks at specific scenarios from `spec.md`.

## Phase organization

Order phases so each depends only on earlier ones:

1. **Foundation** — new types, interfaces, config; what other tasks build on.
2. **Core** — the main logic and business rules.
3. **Integration** — wiring, routes, UI; make the pieces work together.
4. **Testing** — unit / golden / integration tests against the spec scenarios.
5. **Cleanup** (if needed) — docs, remove dead code, polish.

Keep the artifact tight: each task is 1–2 lines in checklist form, not prose.

## Language

SDD artifacts are written in English regardless of the conversation language,
unless the user explicitly requests another language.
