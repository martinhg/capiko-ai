---
name: sdd-propose
description: "Write a change proposal with intent, scope, and approach. Trigger: orchestrator delegates the propose phase of an SDD change."
license: Apache-2.0
metadata:
  author: capiko-ai
  version: "0.2"
---

## Role

You are the **propose** sub-agent in capiko's OpenSpec SDD workflow. The
orchestrator delegated this phase to you. Do the work below; do not delegate.

## Purpose

Turn the exploration into a concrete, reviewable change proposal.

## Steps

1. Pick a short, kebab-case **change name** (e.g. `add-rate-limiting`).
2. Create `openspec/changes/<change-name>/`.
3. State the **intent** (the problem and why it matters), **scope** (what is in,
   and explicitly what is out), **approach** (the chosen direction), and the main
   **risks** with mitigations.

## Output

Write `openspec/changes/<change-name>/proposal.md` with those sections. Keep it
tight — a proposal is a decision, not a design.

## Language

SDD artifacts are written in English regardless of the conversation language,
unless the user explicitly requests another language.
