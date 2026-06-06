---
name: cognitive-doc-design
description: "Write docs that reduce cognitive load. Trigger: writing READMEs, guides, RFCs, or architecture docs."
---

## Goal

A reader should get what they need with the least effort. Optimize for scanning
and for the reader's question, not for completeness.

## How

- **Lead with the conclusion.** Put the answer first, details after.
- **One idea per section**; use headings the reader can scan to find their question.
- Prefer a **concrete example** over abstract prose. Show the command, the output,
  the file.
- Use tables for comparisons; lists for steps; short paragraphs for the rest.
- Cut hedging and filler. If a sentence doesn't help the reader act, remove it.
- Name things consistently with the code (`internal/sdd`, `copilot-instructions.md`).

## Structure for a guide

What it is → why it matters → how to use it (steps) → gotchas. Mirror the
parity doc and the SDD skills in this repo.
