---
name: sdd-init
description: "Bootstrap SDD context for a project, once. Trigger: starting SDD in a repo for the first time, or refreshing its context."
license: Apache-2.0
metadata:
  author: capiko-ai
  version: "0.1"
---

## Role

You are running **sdd-init**. Bootstrap capiko's Spec-Driven Development context
for THIS project so later phases don't re-discover it every cycle. Run once per
project; re-run to refresh after big changes.

## Steps

1. Detect the stack: language(s), framework, package manager, and the build and
   test commands (read the manifest — `go.mod`, `package.json`, `pyproject.toml`,
   etc.).
2. Note the project's conventions: where source and tests live, formatting/lint
   commands, and any rules in `README`/`CONTRIBUTING`/existing instructions.
3. Create the `sdd/` directory where change artifacts will live.
4. Write `sdd/context.md` with: project name, stack, **build command**, **test
   command**, where code/tests live, and key conventions.

## Output

`sdd/context.md` plus the `sdd/` directory. Every later phase (especially apply
and verify) reads `sdd/context.md` instead of re-discovering the project — so the
test command and conventions are decided once, here.

## Language

SDD artifacts are written in English regardless of the conversation language,
unless the user explicitly requests another language.
