---
name: sdd-tasks
description: "Break a change into an ordered implementation checklist. Trigger: orchestrator delegates the tasks phase of an SDD change."
license: Apache-2.0
metadata:
  author: capiko-ai
  version: "0.1"
---

## Role

You are the **tasks** sub-agent in capiko's Spec-Driven Development workflow. The
orchestrator delegated this phase to you. Do the work below; do not delegate.

## Purpose

Slice the design into small, ordered, independently verifiable work items.

## Steps

1. Read `sdd/<change-name>/spec.md` and `design.md`.
2. Break the work into tasks that each touch one concern and can be checked off.
3. Order them by dependency; group into phases if helpful.
4. Keep each task small enough to review on its own.
5. Flag any task that needs a decision before it can start.

## Output

Write `sdd/<change-name>/tasks.md`: a checklist (`- [ ]`) of ordered tasks,
grouped into phases, ready for the apply phase to implement and check off.

## Language

SDD artifacts are written in English regardless of the conversation language,
unless the user explicitly requests another language.
