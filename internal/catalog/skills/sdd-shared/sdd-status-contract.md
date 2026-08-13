# SDD Status and Instructions Contract

Shared status contract for SDD commands and phase skills. Load before acting on a
change so orchestration doesn't guess state, paths, or edit scope — this is the
handoff between orchestrator and phase executor.

## Native Engine

Prefer `capiko-ai` on PATH: `sdd-status [change] --cwd <repo> --json` (the
authoritative `capiko.sdd-status` JSON, schema below) and `sdd-continue [change]
--cwd <repo>` (dispatcher routing view).

Route only by `nextRecommended`/dependency states, never re-derived from prose. All
phases (`propose|spec|design|tasks|apply|verify|archive`) route the same way:
delegate to the matching `sdd-<phase>` worker. Non-empty `blockedReasons` → stop,
except when `nextRecommended` is `verify` (may re-run for fresh evidence);
`resolve-blockers`/`select-change` also stop.

The engine only reports state on changes already under `openspec/changes/` — it
never fabricates one. Zero changes → `sdd-new`, a triage-gate signal (triage runs
BEFORE `sdd-status`), not an auto-start instruction.

No binary → reconstruct status from the JSON example below via the OpenSpec
artifacts; output MUST stay shape-compatible.

## Artifact Store

File-first: phases read/write under `openspec/changes/<change>/`; the engine scans
those files. The Engram *mode* (`hybrid|engram|openspec|none`) only toggles the
cross-session memory backend, never artifact location.

**Engram read-only fallback.** With no matching OpenSpec change on disk, the engine
MAY resolve it from Engram (`sdd/<change>/*`) when gated by env var
`CAPIKO_SDD_STATUS_ENGRAM`, a `.engram/` dir, or `artifact_store: engram|hybrid` in
`openspec/config.yaml`. Then `artifactStore` becomes `"engram"` and
`changeRoot`/`planningHome.path` become `"engram:sdd/<change>"`/`"engram:sdd"` — NOT
filesystem paths; guard path-parsing against the `engram:` prefix. Strictly
read-only: an OpenSpec change on disk always wins.

## Change Selection

- Explicit name → use it after confirming it exists under `openspec/changes/`.
- No name → infer only if unambiguous (one active change, or clear from session).
- Multiple/unclear → ask the user, don't guess.
- None active → report that, suggest `/sdd-new <change>`.

## Status Schema (annotated example)

Mid-cycle example (proposal/specs/design done, tasks in progress) with every field
shown; return this shape as JSON, or markdown if the host can't render JSON.

```jsonc
{
  "schemaName": "capiko.sdd-status", "schemaVersion": 1,
  "changeName": "add-rate-limiter", // null if unresolved
  "artifactStore": "openspec", // "engram" only on read-only fallback (above)
  "planningHome": { "mode": "repo-local", "path": "/repo/openspec" },
  "changeRoot": "<root>", // <root>=/repo/openspec/changes/add-rate-limiter; null if unresolved
  "artifactPaths": { "proposal": ["<root>/proposal.md"], "specs": ["<root>/spec.md"], "design": ["<root>/design.md"], "tasks": ["<root>/tasks.md"], "applyProgress": [], "verifyReport": [] }, // each "<root>/<file>"; [] if missing
  "artifacts": { "proposal": "done", "specs": "done", "design": "done", "tasks": "done", "applyProgress": "partial", "verifyReport": "missing" }, // same 6 keys; each: missing|partial|done
  "taskProgress": { "total": 12, "completed": 5, "pending": 7, "allComplete": false },
  "dependencies": { "proposal": "all_done", "specs": "all_done", "design": "all_done", "tasks": "all_done", "apply": "ready", "verify": "blocked", "archive": "blocked" },
  // DAG proposal->spec/design->tasks: all_done=own artifact done; ready=prereqs done, own isn't (next step); else blocked. specs/design need proposal; tasks needs both.
  // apply ready iff specs+design+tasks done and progress not all done. verify ready iff tasks exist and (apply-progress exists or tasks all complete).
  // archive ready iff verify-report is clearly passing (explicit PASS/SUCCESS, no FAIL/FAILURE/BLOCKED/CRITICAL/PENDING/TODO/negation; CRITICAL never overridden) and tasks complete.
  "applyState": "ready", // blocked=apply artifact missing/ambiguous selection/unsafe edit; ready=tasks exist w/ pending item, safe edit scope; all_done=all checked [x]
  "actionContext": { "mode": "repo-local", "workspaceRoot": "/repo", "allowedEditRoots": ["/repo"] },
  "nextRecommended": "apply", // propose|spec|design|tasks|apply|verify|archive|sdd-new|select-change|resolve-blockers
  "blockedReasons": [] // prose explanation; route by nextRecommended, not this
}
```

`changeName`/`changeRoot` are nullable; every other section is always present.
`nextRecommended` is a bounded routing token, not prose.

## Action Context Guard

Orchestrator MUST carry `actionContext` into every phase launch:

- Can't prove edit ownership/allowed roots → stop before editing.
- Only edit files within `allowedEditRoots`.
- Can't prove a file is in the workspace/edit roots → stop, ask for clarification.

## Status Output

Every command acting on a change MUST show, before launching an executor or
archiving: active change and how resolved; artifact statuses and paths used; task
progress and unchecked list; next recommended action; `blockedReasons` (when
`nextRecommended` isn't `verify`) plus any edit-root blockers.
