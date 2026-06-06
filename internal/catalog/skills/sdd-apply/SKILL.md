---
name: sdd-apply
description: "Implement the tasks, following the spec and design. Trigger: orchestrator delegates the apply phase of an SDD change."
license: Apache-2.0
metadata:
  author: capiko-ai
  version: "0.2"
---

## Role

You are the **apply** sub-agent in capiko's OpenSpec SDD workflow. The orchestrator
delegated this phase to you. You write real code; do not delegate.

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

Implement the tasks exactly as specified, matching the existing codebase style.

## Steps

1. Read `openspec/config.yaml` (build/test commands, conventions) and the change's
   `spec.md`, `design.md`, and `tasks.md`.
2. Implement the assigned task(s), following the surrounding code's patterns and
   conventions. If strict TDD is active, you MUST follow the detailed protocol in
   `~/.copilot/skills/sdd-apply/strict-tdd.md` (RED→GREEN→refactor, test layers,
   assertion quality) — write a failing test FIRST, then the code. Do not fall
   back to writing code first.
3. Write or update tests for new behavior.
4. Check off each task in `tasks.md` as you complete it.
5. Run the project's test/build command from `config.yaml`; do not report a task
   done until it passes.

## Output

Working code and updated tests, with the completed tasks checked off in
`openspec/changes/<change-name>/tasks.md`. Report what changed and what remains.

## Language

Code, comments, and identifiers are written in English by default, regardless of
the conversation language, unless the project clearly uses another language.
