# sdd-status-engram-fallback Specification

## Purpose

This spec defines what MUST be true after `sdd-status-engram-fallback` is applied.
It covers the Engram-fallback capability in `internal/sddstatus`: when OpenSpec
change files are absent but Engram observations (`sdd/<change>/*`) exist, `Resolve`
reconstructs and returns a full `capiko.sdd-status` from those observations instead
of returning a blind blocked result. OpenSpec files remain canonical.

Delivered via PRs #139, #140, #141 (all merged to main, 2026-06-29).

---

## Requirements

### REQ-1: Gating — off by default

When none of the gating conditions are satisfied, `Resolve` behavior is byte-for-byte
identical to the pre-change implementation. The Engram export seam is never invoked.

Gating is OFF when all of the following are absent:
- `CAPIKO_SDD_STATUS_ENGRAM` env var
- `.engram/` directory at the workspace root
- `openspec/config.yaml` / `openspec/config.yml` with `artifact_store: engram` or `hybrid`

### REQ-2: Gating — on via any single trigger

| Trigger | Condition |
|---|---|
| Env override | `CAPIKO_SDD_STATUS_ENGRAM` set (any non-empty value) |
| Engram dir | `.engram/` exists at `<cwd>/.engram` |
| Config: engram | `openspec/config.yaml|yml` has `artifact_store: engram` (case-insensitive) |
| Config: hybrid | Same file has `artifact_store: hybrid` |

### REQ-3: Files-first precedence

When a matching OpenSpec change exists on disk, the Engram fallback is NOT consulted
even if gating is ON. The file-path result is returned as-is.

### REQ-4: Fallback at branch (a) — zero OpenSpec, one Engram change

Returns a fully resolved status with:
- `ArtifactStore = "engram"`
- `ChangeRoot = "engram:sdd/<change>"`
- `PlanningHome.Path = "engram:sdd"`
- `NextRecommended` reflecting the reconstructed artifact state
- `BlockedReasons = []`

### REQ-5: Fallback at branch (b) — named change absent from OpenSpec but in Engram

When a `ChangeName` is requested and absent from OpenSpec but present in Engram,
returns a fully resolved status with the same origin flags as REQ-4.

### REQ-6: Ambiguity — zero OpenSpec, multiple Engram changes

When no `ChangeName` is requested and Engram holds two or more distinct changes,
returns `NextRecommended = "select-change"` with non-empty `BlockedReasons` listing
the change names. `ArtifactStore` is NOT `"engram"` for this blocked status.

### REQ-7: Artifact reconstruction — parity with file path

| Artifact key | Present when |
|---|---|
| `proposal` | `sdd/<change>/proposal` observation has non-empty content |
| `specs` | `sdd/<change>/spec` observation has non-empty content |
| `design` | `sdd/<change>/design` observation has non-empty content |
| `tasks` | `sdd/<change>/tasks` observation has non-empty content |
| `applyProgress` | `sdd/<change>/apply-progress` observation has non-empty content |
| `verifyReport` | `sdd/<change>/verify-report` observation has non-empty content |

Task progress uses the same `taskCheckbox` regex as the file path.
`NextRecommended` computed from Engram observations MUST equal `NextRecommended`
from the file path for an equivalent artifact state.

### REQ-8: Project matching

Inference chain (first match wins):
1. `ENGRAM_PROJECT` env var
2. `owner/repo` from `[remote "origin"]` url in `<cwd>/.git/config` (lowercased)
3. Lowercased basename of `cwd`

Only observations where `project` matches (case-insensitive) AND `scope != "personal"`
are considered.

### REQ-9: Degradation safety

Any failure in the Engram path (binary absent, non-zero exit, malformed JSON, schema
drift, no matching observations) returns the same blocked status the file path would
have returned. Error is never surfaced to the caller. No panic.

### REQ-10: Origin flag is explicit and consumer-safe

Engram-resolved status always carries:
- `ArtifactStore = "engram"`
- `ChangeRoot = "engram:sdd/<change>"` (not a filesystem path)
- `PlanningHome.Path = "engram:sdd"` (not a filesystem path)

Routing via `nextRecommended` continues to work identically.

### REQ-11: Contract doc update

`internal/catalog/skills/sdd-shared/sdd-status-contract.md` documents:
- Engine MAY report `artifactStore: "engram"` when OpenSpec files are absent.
- `changeRoot` prefixed `engram:` is NOT a filesystem path.
- Engram is strictly a read-only fallback; canonical-files invariant is NOT weakened.

### REQ-12: Test seam requirement

`engramExport` is a package-level function variable (seam) in `internal/sddstatus`.
Tests swap it to inject fixture JSON. The real `engram` binary is never called during
`go test -race ./...`.

---

## Non-negotiable invariants

| Invariant | Rule |
|---|---|
| `SchemaName` | Always `"capiko.sdd-status"` |
| `SchemaVersion` | Always `1` |
| JSON shape | Unchanged — no new top-level fields |
| Files stay canonical | File-path result is returned when a matching OpenSpec change exists |
| No crash path | Engram path failures always degrade to the normal blocked status |
| No write path | Fallback only reads; never writes Engram or OpenSpec |
| TUI golden files | `internal/tui/testdata/*.golden` — untouched |

---

## Out of scope

- Engram as a primary artifact store
- Phase-skill or agent changes
- New write paths to Engram or OpenSpec
- `engram` binary management (install / upgrade)
- Any change to `SchemaName`, `SchemaVersion`, or JSON top-level shape
- TUI changes or golden file updates
