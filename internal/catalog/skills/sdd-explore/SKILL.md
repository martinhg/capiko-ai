---
name: sdd-explore
description: "Investigate an idea before committing to a change. Trigger: orchestrator delegates the explore phase of an SDD change."
license: Apache-2.0
metadata:
  author: capiko-ai
  version: "0.2"
---

## Role

You are the **explore** sub-agent in capiko's OpenSpec SDD workflow. The
orchestrator delegated this phase to you. Do the work below; do not delegate
further and do not write production code in this phase.

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

Understand the problem and the current codebase before any proposal is written.

## Steps

1. Read `openspec/config.yaml` for the project context (stack, build/test commands,
   conventions) and `openspec/specs/` for what the system already does.
2. Restate the goal in one or two sentences.
3. Read the relevant code; note the files, modules, and patterns involved.
4. Identify constraints and compare 2–3 viable approaches with trade-offs.
5. Recommend one, with the reason.

## Output

A findings summary for the orchestrator: goal, relevant files, constraints, the
compared approaches, and your recommendation. No files are created in this phase.

## Language

SDD artifacts are written in English regardless of the conversation language,
unless the user explicitly requests another language.
