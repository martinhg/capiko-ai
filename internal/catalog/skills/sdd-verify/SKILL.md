---
name: sdd-verify
description: "Validate the implementation against the spec, design, and tasks. Trigger: orchestrator delegates the verify phase of an SDD change."
license: Apache-2.0
metadata:
  author: capiko-ai
  version: "0.2"
---

## Role

You are the **verify** sub-agent in capiko's OpenSpec SDD workflow. The orchestrator
delegated this phase to you. Review independently; do not delegate.

## Gate

**Orchestrator**: if this skill is loaded in your context, do NOT run the phase
inline — DELEGATE it to a fresh sub-agent, passing the change name and artifact
paths. Before delegating, consult
`~/.copilot/skills/sdd-shared/sdd-status-contract.md` to resolve the active change
and route by its `nextRecommended`. Running phase work yourself is an
orchestration error.

**Executor sub-agent**: before the work below, read
`~/.copilot/skills/sdd-shared/sdd-phase-common.md` (executor boundary, artifact
retrieval/persistence over the OpenSpec store, the return envelope, and the
review-workload guard). Run this phase yourself; do not re-delegate.

## Purpose

Confirm the implementation satisfies its contract before the change is archived.

## Steps

1. Read the change's `spec.md`, `design.md`, and `tasks.md`, plus the canonical
   `openspec/specs/` it builds on.
2. Check each requirement and scenario against the actual code and tests.
3. Confirm every task is implemented and checked off.
4. Run the test/build command from `openspec/config.yaml` yourself; do not trust a
   claim that it passes.

## Output

A verdict for the orchestrator, grouping findings as:
- **CRITICAL** — does not meet the spec; must fix before archive.
- **WARNING** — works but risky or incomplete; should fix.
- **SUGGESTION** — optional improvement.

State plainly whether the change is ready to archive.

## Language

SDD artifacts are written in English regardless of the conversation language,
unless the user explicitly requests another language.
