---
name: sdd-init
description: "Bootstrap the OpenSpec store for a project, once. Trigger: starting SDD in a repo for the first time, or refreshing its context."
license: Apache-2.0
metadata:
  author: capiko-ai
  version: "0.2"
---

## Role

You are running **sdd-init**. Bootstrap capiko's OpenSpec store so the SDD cycle
has memory: a place for in-flight changes, the canonical specs, and an archive.
Run once per project; re-run to refresh the context.

## OpenSpec layout (create this)

```
openspec/
├── config.yaml                 # project context + rules (below)
├── changes/                    # in-flight changes, one folder each
│   └── archive/                # completed changes, dated
└── specs/                      # canonical, accumulated specs (source of truth)
```

## Steps

1. Detect the stack: language(s), framework, package manager, and the **build**
   and **test** commands (read `go.mod` / `package.json` / `pyproject.toml` …).
2. Create the `openspec/`, `openspec/changes/`, `openspec/changes/archive/`, and
   `openspec/specs/` directories.
3. Write `openspec/config.yaml`:

   ```yaml
   schema: spec-driven
   context: |
     Tech stack: <languages, framework, package manager>
     Build: <build command>
     Test: <test command>
     Source/tests: <where code and tests live>
     Conventions: <formatting/lint, key rules>
   strict_tdd: <true|false>   # match the Configure SDD strict-TDD setting
   rules: |
     <project rules the SDD phases must follow>
   ```

Every later phase reads `openspec/config.yaml` instead of re-discovering the
project, so the test command and conventions are decided once, here.

## Language

SDD artifacts are written in English regardless of the conversation language,
unless the user explicitly requests another language.
