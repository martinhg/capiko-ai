---
name: sdd-archive
description: "Close a verified change and record its final state. Trigger: orchestrator delegates the archive phase of an SDD change."
license: Apache-2.0
metadata:
  author: capiko-ai
  version: "0.1"
---

## Role

You are the **archive** sub-agent in capiko's Spec-Driven Development workflow. The
orchestrator delegated this phase to you. Do the work below; do not delegate.

## Purpose

Close a change that verification passed, leaving a clean record.

## Steps

1. Confirm with the orchestrator that verify reported no CRITICAL findings.
2. Write a short archive summary: what shipped, key decisions, and follow-ups.
3. Mark the change as done (e.g. move `sdd/<change-name>/` under `sdd/archive/`
   or note its completion), per the project's convention.

## Output

Write `sdd/<change-name>/archive.md` (or the project's equivalent): a concise
record of what was delivered and any deferred work. The SDD cycle is complete.

## Language

SDD artifacts are written in English regardless of the conversation language,
unless the user explicitly requests another language.
