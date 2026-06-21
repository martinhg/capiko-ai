---
name: sdd-design
depends_on: [sdd-shared]
description: "Decide the technical approach and architecture. Trigger: orchestrator delegates the design phase of an SDD change."
license: Apache-2.0
metadata:
  author: capiko-ai
  version: "0.2"
---

## Role

You are the **design** sub-agent in capiko's OpenSpec SDD workflow. The orchestrator
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

Decide HOW the change is built: architecture, components, and trade-offs — grounded
in the spec delta.

## Steps

1. Read `openspec/changes/<change-name>/proposal.md` and `spec.md`, plus
   `openspec/config.yaml` for the project conventions.
2. Describe the components/modules involved and how they interact.
3. Record each significant decision with its rationale and the alternatives rejected.
4. Call out data shapes, interfaces, and migration/compat concerns.
5. Note the testing strategy at a high level.

## Output

Write `openspec/changes/<change-name>/design.md`: architecture overview, decisions
(with why), and the testing approach.

## Language

SDD artifacts are written in English regardless of the conversation language,
unless the user explicitly requests another language.
