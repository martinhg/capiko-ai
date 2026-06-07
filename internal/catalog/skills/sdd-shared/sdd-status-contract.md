# SDD Status and Instructions Contract

Shared OpenSpec-style contract for SDD commands and phase skills. Load this before
acting on a change so orchestration does not guess state, paths, or edit scope.

## Purpose

Commands that select, continue, apply, verify, or archive an SDD change MUST first
produce or consume structured status. The status is the handoff between the
orchestrator and the phase executor.

## Native Engine

capiko ships a native SDD engine. When the `capiko-ai` binary is on PATH, prefer it
for status and routing:

- `capiko-ai sdd-status [change] --cwd <repo> --json` — the authoritative status as
  `capiko.sdd-status` JSON (the same schema below).
- `capiko-ai sdd-continue [change] --cwd <repo>` — the dispatcher routing view.

Treat the native JSON as authoritative over prompt inference. Route only by
`nextRecommended` and the dependency states; never re-derive status from prose when
the binary answered. The engine routes the planning phases deterministically too:
`nextRecommended` may be `propose`, `spec`, `design`, or `tasks` — delegate to the
matching `sdd-<phase>` worker exactly as you would for `apply`/`verify`/`archive`,
without inferring the next planning step from `blockedReasons`. When `blockedReasons`
is non-empty, report it and stop unless `nextRecommended` is `verify` (verification
may run to refresh evidence). When `nextRecommended` is `resolve-blockers` or
`select-change`, report and stop.

Routing invariants (the engine reports state; it never starts work):

- The engine routes changes that already exist under `openspec/changes/`. It never
  fabricates a change. With zero active changes it returns `sdd-new` — that is a
  signal to the orchestrator's triage gate, not an instruction to auto-start a cycle.
- The orchestrator's triage gate runs FIRST, before `sdd-status`. A `nextRecommended`
  of `propose` does not bypass triage; it only applies once a change has been started.

If the binary is unavailable, fall back to this prompt contract: reconstruct status
from the manual schema below by reading the change's OpenSpec artifacts. Manual
fallback output MUST stay shape-compatible with the native JSON so consumers parse
both the same way.

## Change Selection

- If a change name is provided, use that exact change after confirming it exists
  under `openspec/changes/`.
- If no change name is provided, infer only when the active change is unambiguous
  from session state or there is exactly one active change.
- If multiple active changes match or the active change is unclear, ask the user
  to choose. Do not guess.
- If no active changes exist, report that no SDD change is active and suggest
  `/sdd-new <change>`.

## Status Schema

Return status as markdown with these fields, or as equivalent JSON when the host
supports it:

```yaml
schemaName: capiko.sdd-status
schemaVersion: 1
changeName: <change-name-or-null>
artifactStore: openspec
planningHome:
  mode: repo-local
  path: <absolute path to openspec>
changeRoot: <absolute path to openspec/changes/<change> or null>
artifactPaths:
  proposal: [<absolute path>]
  specs: [<absolute paths>]
  design: [<absolute path>]
  tasks: [<absolute path>]
  applyProgress: [<absolute path>]
  verifyReport: [<absolute path>]
artifacts:
  proposal: missing | done | partial
  specs: missing | done | partial
  design: missing | done | partial
  tasks: missing | done | partial
  applyProgress: missing | done | partial
  verifyReport: missing | done | partial
taskProgress:
  total: 0
  completed: 0
  pending: 0
  allComplete: false
dependencies:
  proposal: blocked | ready | all_done
  specs: blocked | ready | all_done
  design: blocked | ready | all_done
  tasks: blocked | ready | all_done
  apply: blocked | ready | all_done
  verify: blocked | ready | all_done
  archive: blocked | ready | all_done
applyState: blocked | all_done | ready
actionContext:
  mode: repo-local
  workspaceRoot: <absolute path>
  allowedEditRoots: [<absolute paths>]
nextRecommended: propose | spec | design | tasks | apply | verify | archive | sdd-new | select-change | resolve-blockers
blockedReasons: []
```

Empty path fields MUST be arrays, not null. `changeName` and `changeRoot` are
nullable; every other section should be present so consumers can parse status the
same way. `nextRecommended` is a bounded routing token, not human prose — route
only by `nextRecommended` and dependency states; human explanation belongs in
`blockedReasons`.

## Apply State

- `blocked`: required apply artifacts are missing, task selection is ambiguous, or
  the action context makes edits unsafe.
- `all_done`: the tasks artifact exists and every implementation task is checked
  `[x]`.
- `ready`: the tasks artifact exists, at least one implementation task remains
  unchecked, and the edit scope is safe.

## Dependency States

- `proposal`, `specs`, `design`, and `tasks` follow the planning DAG
  (proposal → spec/design → tasks). Each is `all_done` when its own artifact is
  complete, `ready` when its prerequisites are complete but its artifact is not yet
  (so it is the next planning step to run), and `blocked` otherwise. `proposal` has
  no prerequisite; `specs` and `design` require `proposal`; `tasks` requires both.
- `apply` is `ready` only when specs, design, and tasks exist and task progress is
  not all done.
- `verify` is `ready` when tasks exist and either apply-progress exists or the
  tasks artifact shows all intended implementation complete. Incomplete tasks
  remain blockers for full verification.
- `archive` is `ready` only when the verify-report exists, is clearly passing, and
  tasks are complete. A clearly passing report needs an explicit PASS/SUCCESS
  signal and no blocker or negation signals such as FAIL, FAILURE, BLOCKED,
  CRITICAL, PENDING, TODO, `not passed`, or `pass: no`. CRITICAL issues have no
  override.

## Action Context Guard

The orchestrator MUST carry `actionContext` into any phase launch.

- If reconstructed context cannot prove edit ownership or allowed edit roots, stop
  before editing.
- If `allowedEditRoots` is present, only edit files within those roots.
- If a command cannot prove a file is inside the authoritative workspace or
  allowed edit roots, stop and ask for clarification.

## Status Output

Every command that acts on a change MUST show status before launching an executor
or performing archive work:

- The active change and how it was resolved.
- Artifact statuses and the paths used as context.
- Task progress and the unchecked task list when tasks exist.
- The next recommended action.
- `blockedReasons` when `nextRecommended` is not `verify`, plus any edit-root
  blockers.
