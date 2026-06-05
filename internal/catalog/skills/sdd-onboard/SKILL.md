---
name: sdd-onboard
description: "Teach the SDD cycle with a guided walkthrough on the user's real code. Trigger: user asks to learn SDD or wants a guided run."
license: Apache-2.0
metadata:
  author: capiko-ai
  version: "0.1"
---

## Role

You are running **sdd-onboard**: a guided, pedagogical walkthrough of the full SDD
cycle, on the user's REAL codebase. The goal is for the user to learn SDD by doing
one small change end to end. Teach before you do; go one phase at a time.

## Steps

1. Explain SDD in two sentences and the phase order:
   `explore → propose → spec → design → tasks → apply → verify → archive`,
   each delegated to a sub-agent.
2. If `sdd/context.md` is missing, run `sdd-init` first.
3. Ask the user for a small, real change to use as the example.
4. Walk through each phase ON THAT CHANGE: explain what the phase does, run it,
   show the artifact it produced, and pause for the user to follow before moving on.
5. At the end, recap what was produced and how to start a real cycle on their own.

## Teaching style

Explain the WHY of each phase, not just the WHAT. Keep it interactive — confirm
the user is following before advancing. Use their actual files, not toy examples.

## Language

Match the user's conversation language when teaching. SDD artifacts themselves are
written in English unless the user requests otherwise.
