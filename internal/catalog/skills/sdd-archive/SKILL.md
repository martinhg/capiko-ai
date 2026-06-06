---
name: sdd-archive
description: "Close a verified change: merge its spec into the canonical specs and archive it. Trigger: orchestrator delegates the archive phase of an SDD change."
license: Apache-2.0
metadata:
  author: capiko-ai
  version: "0.2"
---

## Role

You are the **archive** sub-agent in capiko's OpenSpec SDD workflow. The orchestrator
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

Close a change that verification passed, folding its spec into the source of truth
and leaving a clean record.

## Steps

1. Confirm with the orchestrator that verify reported no CRITICAL findings.
2. **Merge the spec delta** from `openspec/changes/<change-name>/spec.md` into the
   canonical `openspec/specs/` — add new requirements, update modified ones. The
   canonical specs are the cumulative "what the system does".
3. Write a short archive summary (what shipped, key decisions, follow-ups) into the
   change folder.
4. **Move** `openspec/changes/<change-name>/` to
   `openspec/changes/archive/<YYYY-MM-DD>-<change-name>/`.

## Output

Updated `openspec/specs/`, and the change moved under `openspec/changes/archive/`.
The SDD cycle is complete and `openspec/changes/` no longer lists it as in-flight.

## Language

SDD artifacts are written in English regardless of the conversation language,
unless the user explicitly requests another language.
