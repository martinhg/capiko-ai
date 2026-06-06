---
name: sdd-shared
description: "Shared SDD status contract and phase protocol consumed by the SDD phase skills. Not invoked directly."
disable-model-invocation: true
user-invocable: false
license: Apache-2.0
metadata:
  author: capiko-ai
  version: "0.1"
---

## Purpose

`sdd-shared` is a support package, not a runnable skill. It holds the reference
documents the SDD phase skills load before they act, so orchestration does not
re-derive state, paths, or edit scope from loose prose:

- `sdd-status-contract.md` — the structured status object exchanged between the
  orchestrator and each phase executor (schema, dependency states, edit-scope
  guard).
- `sdd-phase-common.md` — the boilerplate every SDD phase agent shares: the
  executor boundary (gate), artifact retrieval/persistence, the return envelope,
  and the review-workload guard.

## Not Invokable

Do not invoke `sdd-shared` as a skill. The SDD phase skills reference its files
by path (`~/.copilot/skills/sdd-shared/<file>.md`). It exists only so those
references resolve on disk.
