# Proposal: SDD status Engram fallback

## Summary

Add a files-first, Engram-fallback resolution path to capiko's native SDD status
engine. When a change's OpenSpec files are absent on a machine but its Engram
observations (`sdd/<change>/*`) exist, reconstruct the same `capiko.sdd-status`
from Engram instead of returning a blind "no active change" result. OpenSpec files
stay canonical; Engram is consulted only when the files are not there. Ported from
gentle-ai PR #957, adapted to capiko's engine shape.

## Why

`internal/sddstatus.Resolve` is purely file-based. `selectChange` only lists
OpenSpec changes under `openspec/changes/`, and `artifactStore` is the hardcoded
constant `"openspec"`. The engine has no knowledge of Engram.

This breaks a real cross-machine workflow. With Engram Cloud sync, a teammate's
SDD memories (`sdd/<change>/proposal`, `.../tasks`, etc.) arrive on another machine
before — or without — a `git pull` of the `openspec/` files. In that state the
engine is blind:

- `ListActiveOpenSpecChanges` finds nothing under `openspec/changes/`.
- `selectChange` returns the `sdd-new` routing token with
  `"No active OpenSpec changes found under openspec/changes."`
- A requested change that exists only in Engram returns `sdd-new` with
  `"Active OpenSpec change not found: <change>."`

So a change that demonstrably exists (its state lives in synced memory) is reported
as nonexistent, and the orchestrator is told to start a new cycle instead of routing
the in-flight one. The data to answer correctly is already present locally — capiko
just never reads it.

Evidence in-repo: capiko already shells out to the `engram` binary through a tested
seam (`internal/engram/version.go` `runOut`), so an Engram read path is consistent
with existing architecture. `engram export <path>` produces a JSON dump of
observations (`{"observations":[{"title","content","project","scope"}]}`), which is
enough to reconstruct artifact presence and task progress.

## What changes

Add an Engram fallback that runs **only when the file-based path finds no matching
change**, mirroring gentle-ai #957 adapted to capiko's `Resolve`/`selectChange`:

1. **Fallback resolution** — a new `resolveEngramStatus(cwd, requestedChange,
   includeInstructions)` invoked at the two `selectChange` blocked branches:
   (a) no change requested AND zero active OpenSpec changes, and (b) a requested
   change not among the active OpenSpec changes. If the fallback resolves a change
   it short-circuits and returns that status; otherwise the existing blocked status
   (`sdd-new` / `select-change`) is returned unchanged.

2. **Gating** (when the fallback is even attempted) — true when ANY of:
   - the env override `CAPIKO_SDD_STATUS_ENGRAM` is set, OR
   - a `.engram` directory exists at the workspace root, OR
   - `openspec/config.yaml|yml` has `artifact_store:` / `artifactStore:` whose value
     contains `engram` or `hybrid`.

   When gating is off, behavior is byte-for-byte today's behavior.

3. **Reconstruction** — export observations via the `engram` CLI behind a test seam
   (same pattern as `internal/engram`'s `runOut`, so tests never shell out). Infer
   the Engram project (ENGRAM_PROJECT env → git remote owner/repo from `.git/config`
   → lowercased dir basename), match observations by project (case-insensitive) and
   `scope != "personal"`, and parse titles with
   `^sdd/([^/]+)/(proposal|spec|design|tasks|apply-progress|verify-report|state)$`.
   Build the same artifacts map (proposal/specs/design/tasks/applyProgress/
   verifyReport) from observation presence/content and run the **same** dependency /
   next-recommended / apply-state logic the file path already uses.

4. **Origin flag** — when resolved from Engram, set `ArtifactStore = "engram"`,
   `ChangeRoot = "engram:sdd/<change>"`, and `PlanningHome.Path = "engram:sdd"`.
   `nextRecommended` is computed normally and routes exactly like the file path
   (apply/verify/etc.); the origin flag tells consumers the source without
   downgrading routing to a git-pull nudge.

5. **Contract doc note** — document the fallback in
   `internal/catalog/skills/sdd-shared/sdd-status-contract.md`: today the doc asserts
   the artifact store is "ALWAYS file-based" with `artifactStore: openspec`. Add a
   bounded note that the engine MAY report `artifactStore: engram` with an
   `engram:sdd/<change>` root when OpenSpec files are absent but Engram observations
   exist — files-first, Engram only as fallback.

## Scope

In scope:

- `internal/sddstatus/status.go` — the fallback insertion points in `selectChange`'s
  blocked branches, the `resolveEngramStatus` flow, gating, project inference, the
  Engram export seam, and an `ArtifactStoreEngram` const (capiko currently has only
  the hardcoded `artifactStore = "openspec"` and no ArtifactStore enum — verify
  against the real code and add the minimum needed).
- `countTaskProgressText(string)` alongside the existing file-based
  `countTaskProgress(path)`, sharing the `taskCheckbox` regex.
- `internal/sddstatus/status_test.go` — table tests for gating on/off, project
  inference, fallback short-circuit at both branches, artifact/task reconstruction,
  and parity of `nextRecommended` with the file path. Tests drive the export seam,
  never the real `engram` binary.
- A note in `internal/catalog/skills/sdd-shared/sdd-status-contract.md`.

## Non-goals

- **Engram does not become a primary artifact store.** Files stay canonical; Engram
  is read only when no matching OpenSpec change is found.
- **No phase-skill or agent changes.** The SKILLS are not mode-branched — this is an
  engine-layer graceful-degradation fallback only, preserving the load-bearing design
  rule that capiko keeps OpenSpec files canonical and never branches phase reads/
  writes on the artifact-store mode.
- **No new write path.** The fallback only reads/reconstructs status; it never writes
  Engram or OpenSpec artifacts.
- **No `engram` binary management.** capiko shells out to read; it does not install,
  upgrade, or require Engram (absent binary → fallback simply yields nothing and the
  normal blocked status is returned).
- `SchemaName` stays `capiko.sdd-status`; the JSON shape is unchanged.

## Impact

| Area | Change |
| --- | --- |
| `internal/sddstatus/status.go` | Fallback wiring in `selectChange` branches; `resolveEngramStatus`; gating (`shouldTryEngram`); project inference; Engram export seam; `ArtifactStoreEngram` const; `countTaskProgressText`; origin flags (`ArtifactStore`, `ChangeRoot`, `PlanningHome.Path`) |
| `internal/sddstatus/status_test.go` | New tests via the export seam; no real shell-out |
| `internal/catalog/skills/sdd-shared/sdd-status-contract.md` | Bounded note documenting the Engram fallback and the `engram:sdd/<change>` origin |

Behavioral impact:

- **Backward compatible.** With gating off (no `.engram`, no engram/hybrid config, no
  env override) the engine behaves exactly as today.
- **Consumers** of `capiko.sdd-status` may now observe `artifactStore: "engram"` and a
  `changeRoot` prefixed `engram:` — the contract note makes this explicit so parsers
  do not treat the prefixed root as a filesystem path.
- **Test seam** keeps `go test -race ./...` hermetic; no test invokes the `engram`
  binary.
- TUI golden files (`internal/tui/testdata/*.golden`) are expected untouched — confirm
  during apply.

## Risks / open questions

- **Engram export schema drift.** The fallback depends on `engram export <path>`
  emitting `{"observations":[...]}` with `title`/`content`/`project`/`scope`. If that
  schema changes, the fallback silently yields nothing (degrades to today's blocked
  status — never a crash). Confirm the schema at apply time.
- **Project inference mismatch.** If the inferred Engram project differs from how the
  observations were saved (env vs. git remote vs. dir basename), the fallback finds no
  match. This is fail-safe (no false positives) but can fail to recover a change that
  is present under a different project key. Worth a test for each inference branch.
- **Task-progress text parsing.** `countTaskProgressText` parses the tasks observation
  body, which may differ in formatting from a `tasks.md` file. Must reuse the same
  checkbox regex and be covered by tests to keep parity with the file path.
- **Contract doc tension.** The doc currently states the store is "ALWAYS file-based."
  The note must frame Engram strictly as a read-only fallback so the canonical-files
  invariant is not perceived as weakened.
