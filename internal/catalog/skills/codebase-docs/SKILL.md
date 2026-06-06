---
name: codebase-docs
description: "Generate and maintain a codebase guide so new devs can onboard fast. Trigger: user asks to document the project, onboard new devs, or refresh the codebase docs."
license: Apache-2.0
metadata:
  author: capiko-ai
  version: "0.1"
---

## Role

Generate a **codebase guide** for THIS project under `docs/codebase/`, so a new
developer (or an agent) can understand the architecture and know where code belongs.
You analyze the real code and write the docs; keep them short and scannable.

## What to produce

Create (or refresh) `docs/codebase/` with:

1. **`mental-model.md`** — what the project IS and is NOT, in one screen. Lead with
   a one-paragraph summary, then an "is / is not" table grounded in the actual repo.
2. **`repository-map.md`** — a package/directory **ownership table**: for each top-
   level package, what it owns and what must NOT go there. End with "where common
   changes go" (a new endpoint → here; a new model → there).
3. **`architecture.md`** — the recurring patterns a contributor must match: the main
   flows, key abstractions, how layers talk, and the testing approach.
4. **`README.md`** (index) — links to the three, grouped for users / maintainers /
   contributors.

## How

1. Detect the stack and read `openspec/config.yaml` if present.
2. Map the directory structure and read enough of each package to describe it
   accurately — do not invent packages or responsibilities.
3. Use tables for ownership and "is/is not"; short paragraphs elsewhere. Name things
   exactly as they appear in the code.
4. Keep each file to roughly one screen; link between them.

## Maintaining

When the structure changes, update `repository-map.md` and `architecture.md` in the
same change. Commit the docs so they travel with the repo.

## Language

Docs are written in English unless the user requests another language.
