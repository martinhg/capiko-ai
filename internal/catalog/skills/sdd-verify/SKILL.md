---
name: sdd-verify
depends_on: [sdd-shared]
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

Read `~/.copilot/skills/sdd-shared/sdd-phase-common.md` § A–I before work.
Orchestrator: delegate. Executor: run this phase, do not re-delegate.

## Purpose

Confirm the implementation satisfies its contract before the change is archived.

## Steps

1. Read the change's `spec.md`, `design.md`, and `tasks.md`, plus the canonical
   `openspec/specs/` it builds on.
2. Check each requirement and scenario against the actual code and tests.
3. Confirm every task is implemented and checked off.
4. Run the test/build command from `openspec/config.yaml` yourself; do not trust a
   claim that it passes.
5. If strict TDD is active, you MUST follow the audit in
   `~/.copilot/skills/sdd-verify/strict-tdd-verify.md` (TDD compliance, theater-test
   detection, changed-file coverage) and fold its findings into the verdict below.

## Output

A verdict for the orchestrator, grouping findings as:
- **CRITICAL** — does not meet the spec; must fix before archive.
- **WARNING** — works but risky or incomplete; should fix.
- **SUGGESTION** — optional improvement.

State plainly whether the change is ready to archive.
