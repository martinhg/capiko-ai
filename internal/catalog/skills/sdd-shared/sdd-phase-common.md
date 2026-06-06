# SDD Phase — Common Protocol

Boilerplate shared by every SDD phase skill. Sub-agents MUST load this alongside
their phase-specific `SKILL.md`.

## A. Executor Boundary (Gate)

Every SDD phase agent is an **EXECUTOR**, not an orchestrator. Do the phase work
yourself. Do NOT launch sub-agents, do NOT delegate or bounce work back, and do
NOT call orchestration tools. Stop and report a blocker only when your phase skill
explicitly says to. If you were handed a phase, you run its body — you do not
re-plan or hand it to someone else.

## B. Skill Loading

1. If the orchestrator injected a `## Skills to load before work` block in your
   launch prompt, read those exact `SKILL.md` files before task-specific work.
2. Otherwise, if the project ships a skill registry (`.atl/skill-registry.md` or
   equivalent), match your task's triggers to it and read the listed `SKILL.md`
   paths.
3. If neither exists, proceed with your phase skill only.

Loading the registry is skill loading, not delegation. The preferred path is (1) —
exact paths chosen by the orchestrator.

## C. Artifact Retrieval (OpenSpec store)

Read every artifact you depend on directly from the change folder under
`openspec/changes/<change-name>/` — `proposal.md`, `specs/`, `design.md`,
`tasks.md`, and any `apply-progress`/`verify-report`. Read the full files; never
act on a summary you did not read. Also read `openspec/config.yaml` for the
project's build/test commands and conventions.

## D. Artifact Persistence

Every phase that produces an artifact MUST write it to its file under
`openspec/changes/<change-name>/`. Skipping this BREAKS the pipeline — downstream
phases will not find your output. Write the file during the phase's main step; no
extra action is needed afterward.

## E. Return Envelope

> **Response ordering**: your FINAL output MUST be the return envelope as text, not
> a tool call. If your last action is a tool call, the orchestrator receives only
> the tool result and your analysis is lost.

Every phase MUST return a structured envelope to the orchestrator:

- `status`: `success`, `partial`, or `blocked`
- `executive_summary`: 1–3 sentences on what was done
- `artifacts`: the artifact paths written
- `next_recommended`: the next SDD phase to run, or `none`
- `risks`: risks discovered, or `None`
- `skill_resolution`: how skills were loaded — `paths-injected`,
  `fallback-registry`, or `none`

Example:

```markdown
**Status**: success
**Summary**: Implemented the tasks for `add-auth`; tests pass.
**Artifacts**: openspec/changes/add-auth/tasks.md (all checked)
**Next**: sdd-verify
**Risks**: None
**Skill Resolution**: paths-injected — 2 skills (go-testing, branch-pr)
```

## F. Review Workload Guard

SDD must protect reviewer cognitive load, not only generate tasks.

- The default PR review budget is **400 changed lines** (`additions + deletions`).
- The orchestrator caches a delivery strategy: `ask-on-risk` (default),
  `auto-chain`, `single-pr`, or `exception-ok`, and passes it to `sdd-tasks` and
  the resolved decision to `sdd-apply`.
- `sdd-tasks` MUST forecast whether the planned work may exceed that budget, with
  exact guard lines: `Decision needed before apply: Yes|No`, `Chained PRs
  recommended: Yes|No`, and `400-line budget risk: Low|Medium|High`.
- If the forecast is high, recommend chained/stacked PRs using deliverable work
  units, each with a clear start, finish, autonomous scope, verification, and
  rollback.
- `sdd-apply` MUST NOT start oversized work unless the strategy resolves to
  chained/stacked slices or an explicitly accepted `size:exception`.

This guard reduces reviewer burnout and keeps delivery safe. It is not optional
process noise.
