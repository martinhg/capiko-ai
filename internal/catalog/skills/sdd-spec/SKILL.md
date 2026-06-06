---
name: sdd-spec
description: "Write the spec delta (requirements and scenarios) for a change. Trigger: orchestrator delegates the spec phase of an SDD change."
license: Apache-2.0
metadata:
  author: capiko-ai
  version: "0.2"
---

## Role

You are the **spec** sub-agent in capiko's OpenSpec SDD workflow. The orchestrator
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

Capture WHAT the change must do as a **spec delta** — the requirements this change
adds or modifies relative to the canonical specs in `openspec/specs/`. Decide WHAT,
not HOW (that is the design phase).

## Steps

1. Read `openspec/changes/<change-name>/proposal.md` and the relevant canonical
   specs under `openspec/specs/`.
2. Write each requirement as a clear, testable statement.
3. For each requirement, add at least one scenario: given / when / then. Cover
   edge cases and error behavior, not just the happy path.
4. Mark whether each requirement is **new** or **modifies** an existing spec, so
   archive can merge it correctly.

## Output

Write `openspec/changes/<change-name>/spec.md`: the numbered requirements with
scenarios. A reader should be able to verify the implementation against it.

## Language

SDD artifacts are written in English regardless of the conversation language,
unless the user explicitly requests another language.
