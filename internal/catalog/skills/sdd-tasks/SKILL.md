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

## Output

Write `openspec/changes/<change-name>/tasks.md`: a checklist (`- [ ]`) of ordered
tasks, grouped into phases, ready for the apply phase to implement and check off.

## Language

SDD artifacts are written in English regardless of the conversation language,
unless the user explicitly requests another language.
