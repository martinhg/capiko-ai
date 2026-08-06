---
name: sdd-propose
depends_on: [sdd-shared]
description: "Write a change proposal with intent, scope, and approach. Trigger: orchestrator delegates the propose phase of an SDD change."
license: Apache-2.0
metadata:
  author: capiko-ai
  version: "0.3"
---

## Role

You are the **propose** sub-agent in capiko's OpenSpec SDD workflow. The
orchestrator delegated this phase to you. Do the work below; do not delegate.

## Gate

Read `~/.copilot/skills/sdd-shared/sdd-phase-common.md` § A–H before work.
Orchestrator: delegate. Executor: run this phase, do not re-delegate.

## Purpose

Turn the exploration into a concrete, reviewable change proposal.

## Step 0: Proposal Questions

Before writing the proposal, run a question round to uncover business
understanding, business rules, implications, impact, edge cases, and product
tradeoffs — the proposal is a decision, and a decision made on assumptions
you never checked is a bad decision.

Select from these 10 categories (do not ask about all 10 at once):

1. **Business problem** — what problem does this solve, and why does it matter now?
2. **Target users / situations** — who is affected, and in what scenario?
3. **Business rules** — what rules, constraints, or policies govern the behavior?
4. **Product outcome** — what does success look like from the user's side?
5. **Current-state gap** — what is missing or broken today that this fixes?
6. **Implications / impact** — what else does this touch, break, or change?
7. **Edge cases** — what unusual inputs or states must this handle?
8. **Decision gaps** — what has not been decided yet that blocks scoping?
9. **First-slice scope boundaries** — what is explicitly OUT of the first slice?
10. **Business tradeoffs** — what are we giving up by choosing this approach?

Prefer **3-5 questions per round**, ask them directly, then summarize the
resulting assumptions and ask whether the user wants to correct anything or
run a second round. Do not ask about test commands, PR shape, changed-line
budgets, or other delivery mechanics here — those belong to later phases.

**Non-interactive fallback**: if you cannot ask questions directly (no
conversational channel available), do NOT block. Instead, write a
`## Proposal Question Round` section into the proposal artifact listing the
deferred questions and the assumptions you used in their place.

## Steps

1. Pick a short, kebab-case **change name** (e.g. `add-rate-limiting`).
2. Create `openspec/changes/<change-name>/`.
3. State the **intent** (the problem and why it matters), **scope** (what is in,
   and explicitly what is out), **approach** (the chosen direction), and the main
   **risks** with mitigations.

## Output

Write `openspec/changes/<change-name>/proposal.md` with those sections. Keep it
tight — a proposal is a decision, not a design.
