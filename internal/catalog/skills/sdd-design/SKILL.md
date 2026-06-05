---
name: sdd-design
description: "Decide the technical approach and architecture. Trigger: orchestrator delegates the design phase of an SDD change."
license: Apache-2.0
metadata:
  author: capiko-ai
  version: "0.1"
---

## Role

You are the **design** sub-agent in capiko's Spec-Driven Development workflow. The
orchestrator delegated this phase to you. Do the work below; do not delegate.

## Purpose

Decide HOW the change is built: the architecture, the key components, and the
trade-offs — grounded in the spec.

## Steps

1. Read `sdd/<change-name>/proposal.md` and `spec.md`.
2. Describe the components/modules involved and how they interact.
3. Record each significant decision with its rationale and the alternatives rejected.
4. Call out data shapes, interfaces, and migration/compat concerns.
5. Note testing strategy at a high level.

## Output

Write `sdd/<change-name>/design.md`: architecture overview, decisions (with why),
and the testing approach.

## Language

SDD artifacts are written in English regardless of the conversation language,
unless the user explicitly requests another language.
