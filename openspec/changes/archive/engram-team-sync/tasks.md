# Tasks: Engram team sync (managed git hooks) — E-1

**Change:** `engram-team-sync`
**Spec:** `sdd/engram-team-sync/spec` (engram #371)
**Design:** `sdd/engram-team-sync/design` (engram #372)
**TDD mode:** Strict — write failing test first, then implementation.
**Gate per work unit:** `gofmt -l .` (empty) → `go vet ./...` → `go test -race ./...` → `go build ./...`

> **Archive note (2026-06-29):** T-12–T-17 checkboxes were stale (unchecked) in the
> persisted tasks artifact at archive time. Reconciliation performed per the Strict-vs-
> OpenSpec archive policy: apply-progress (engram #374) and verify-report-pr4 (engram
> #379) both confirm WU-6/7/8 are COMPLETE and all 4 PRs merged to main with 0 CRITICAL
> issues. Checkboxes updated here to reflect the true final state.

---

## Spec/design reconciliations applied to these tasks

| Item | Spec says | Design says | Resolved |
|---|---|---|---|
| `TeamSyncRecord` conflict fields | `ConflictDetected bool` + `ConflictReason string` | Single `Conflict string` (empty = none, non-empty = reason) | **Design wins** (ADR-2) |
| `RemoveBlock` on shebang-only result | "does NOT delete the hook file" (REQ-2.4) | "deletes the hook file if only the shebang remains" (ADR-1) | **Design wins** — avoids inert capiko-only hook |
| `.engram/config.json` key for project | `"project"` (REQ-4.1) | `"project_name"` (ADR-5 reuses existing `projectConfig` struct) | **Design wins** — `engram.go:265` uses `project_name` |
| `engramSyncExport`/`engramSyncImport` seams | Mentioned in proposal | Explicitly superseded; apply writes files only, no exec at apply time | **Design wins** (ADR-6) |

---

## Dependency graph

```
WU-1 (githooks) ──────────────────────────────────────────────▶ WU-5 (apply/disable)
WU-2 (state)    ──────────────────────────────────────────────▶ WU-5
WU-3 (engram)   ──────────────────────────────────────▶ WU-4 ─▶ WU-5
                                                         (helpers)

WU-5 ──▶ WU-6 (TUI screen) ──▶ WU-7 (app wiring)
                           ──▶ WU-8 (docs)

WU-1 // WU-2 // WU-3  — all parallel (no inter-deps on this change)
WU-4 after WU-3        — resolveProject calls engram.ReadProjectName
WU-5 after WU-1+WU-2+WU-4
WU-6 after WU-2+WU-3+WU-5
WU-7 // WU-8 after WU-6
```

---

## PR grouping (as shipped — 4 PRs, stacked-to-main)

| PR | Work units | Commit(s) | GitHub |
|---|---|---|---|
| PR-1 | WU-1 (githooks) | 7dfee44 | #146 |
| PR-2 | WU-3 + WU-4 (engram helper + pure helpers) | 13bb261, ff105a2 | #147 |
| PR-3 | WU-2 + WU-5 (state + apply/disable) | 82a9f61, 422bc30, 227a020 | #148 |
| PR-4 | WU-6 + WU-7 + WU-8 (TUI + wiring + docs) | 7835f6b, 04a7072, dc8f527 | #149 |

---

## Work Unit 1 — `internal/githooks` package

**Commit:** feat: add internal/githooks WriteBlock and RemoveBlock (7dfee44)
**PR:** #146 (merged)

### T-01 `[x]` `[RED]` Write `internal/githooks/githooks_test.go`

7 table-driven cases: create-new, inject-existing, idempotent, remove-block,
remove-only-shebang→delete, remove-missing-file, coexist-foreign-block.

### T-02 `[x]` `[GREEN + GATE]` Implement `internal/githooks/githooks.go`

WriteBlock (MkdirAll+seed-shebang+instructions.Inject+chmod0755+atomic-rename)
+ RemoveBlock (Inject-with-empty+delete-if-only-shebang). Gate: all green.

---

## Work Unit 2 — State layer: `TeamSyncRecord` + `SetTeamSync`

**Commit:** feat: add TeamSyncRecord and SetTeamSync to state (82a9f61)
**PR:** #148 (merged)

### T-03 `[x]` `[CONFIRM]` Document JSON key reconciliation

Key is `project_name` (design ADR-5 wins over spec REQ-4.1).

### T-04 `[x]` `[RED]` Write state tests for `TeamSyncRecord` and `SetTeamSync`

5 cases: round-trip, nil-clears, updated-at-advances, conflict-persisted,
omitempty-nil.

### T-05 `[x]` `[GREEN + GATE]` Add `TeamSyncRecord`, `State.TeamSync`, `SetTeamSync`

`TeamSyncRecord{Enabled bool, Workspace string, Project string, Conflict string}`.
Gate: all green.

---

## Work Unit 3 — `engram.ReadProjectName`

**Commit:** feat: add engram.ReadProjectName helper (13bb261)
**PR:** #147 (merged)

### T-06 `[x]` `[RED]` Write `ReadProjectName` tests

5 cases: present-valid, absent, malformed-json, empty-project-name, wrong-key.

### T-07 `[x]` `[GREEN + GATE]` Implement `engram.ReadProjectName`

Reuses existing `projectConfig` struct. Gate: all green.

---

## Work Unit 4 — Pure helpers in `internal/tui/teamsync.go`

**Commit:** feat: add teamsync pure helpers (ff105a2)
**PR:** #147 (merged)

### T-08 `[x]` `[RED]` Write tests for pure helpers

17 cases: shSingleQuote (4), detectHookConflict (7), resolveProject (2),
renderPostMerge/renderPrePush (3).

### T-09 `[x]` `[GREEN + GATE]` Implement pure helpers

Markers, hooksPathRe, shSingleQuote, detectHookConflict, resolveProject,
renderPostMerge, renderPrePush. Gate: all green.

---

## Work Unit 5 — `applyTeamSync` + `disableTeamSync`

**Commits:** feat: add applyTeamSync and disableTeamSync (422bc30) +
fix(teamsync): resolve project name before conflict skip (227a020)
**PR:** #148 (merged)

### T-10 `[x]` `[RED]` Write apply/disable tests

5 cases: happy-path (files written + state set), conflict-husky (no files +
Conflict set + nil return), backup-before-write (CreateFiles before WriteBlock),
disableTeamSync (removes both + Enabled:false), resolveProject-in-apply.

### T-11 `[x]` `[GREEN + GATE]` Implement `applyTeamSync` + `disableTeamSync`

teamSyncDetectConflict seam + applyTeamSync + disableTeamSync +
backupTeamSyncHooks. Gate: all green.

---

## Work Unit 6 — TUI screen `teamSyncScreen`

**Commit:** feat: add teamSyncScreen TUI and goldens (7835f6b)
**PR:** #149 (merged)

### T-12 `[x]` `[RED]` Write `Update()`-driven TUI tests

14 Update()-driven test cases: cursor bounds, toggle enabled/ack, ack gate
(blocks when enabled+!ack, allows when ack set, bypassed when disabling),
backMsg, applied msg transitions (done/failed), view assertions (engram absent
hint, engram present no hint, conflict banner with manual commands), hydrates
from state.

### T-13 `[x]` `[GREEN + GATE]` Implement `teamSyncScreen` struct + seams + `Update` + `View`

Seams: teamSyncGetwd, engramDetected, teamSyncDetectConflict. Types:
teamSyncState, row constants, teamSyncScreen, teamSyncAppliedMsg. Functions:
newTeamSync, Update, toggle, applyCmd, View. Gate: all green (28 packages).

### T-14 `[x]` Generate + inspect golden files

7 golden files generated and inspected: teamsync_editing, teamsync_ack,
teamsync_ack_hint, teamsync_conflict, teamsync_no_engram, teamsync_done,
teamsync_failed. All produced under ASCII color profile.

---

## Work Unit 7 — App menu wiring + main-menu golden

**Commit:** feat: wire Configure team sync menu item (04a7072)
**PR:** #149 (merged)

### T-15 `[x]` Add menu item and dispatch to `internal/tui/app.go`

Added `{"Configure team sync", "team-sync", true}` after code-review;
dispatch case `it.id == "team-sync": a.active = newTeamSync(a.svc)` added.
TestEnterOpensTeamSync added. Cursor positions updated +1 for items
after the new entry (upgrade/upgrade-sync/instructions now at 10/11/12).

### T-16 `[x]` Regenerate + inspect main-menu golden

4 menu goldens regenerated; only "Configure team sync" line added. Gate: all green.

---

## Work Unit 8 — Docs

**Commit:** docs: document engram team sync (dc8f527)
**PR:** #149 (merged)

### T-17 `[x]` Update `README` and `llms.txt`

README: added "Configure team sync" to menu table + "Team memory sync" section
(feature, .engram/ git flow, first-push gap, scope-leak warning + mitigations,
conflict coexistence with manual commands).
llms.txt: Team memory sync bullet + docs link.

---

## Final gate output (PR-4, all green)

- `gofmt -l .` → empty (exit 0)
- `go vet ./...` → clean (exit 0)
- `go test -race -count=1 ./internal/tui ./internal/githooks ./internal/state ./internal/engram` → all ok (tui 7.130s)
- `go build ./...` → exit 0
