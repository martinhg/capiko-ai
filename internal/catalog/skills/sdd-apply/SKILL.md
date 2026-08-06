---
name: sdd-apply
depends_on: [sdd-shared]
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

Read `~/.copilot/skills/sdd-shared/sdd-phase-common.md` § A–H before work.
Orchestrator: delegate. Executor: run this phase, do not re-delegate.

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
