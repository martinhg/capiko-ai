# Archive Report: engram-team-sync (E-1)

**Archived:** 2026-06-29
**Status:** COMPLETE — all 4 PRs (#146/#147/#148/#149) merged to main.

---

## What shipped

An opt-in "Configure team sync" feature that wires two marker-delimited managed blocks
into a workspace's `.git/hooks/`:

- `post-merge`: `engram sync --import || true` — imports team memories after merge/pull
- `pre-push`: `engram sync --project '<name>' || true` + echo reminder — exports
  memories before push and reminds the user to commit `.engram/`

Key properties: default off, single Enabled toggle wires both hooks atomically, scope-
leak acknowledgment gate required before enabling, conflict guard detects competing hook
frameworks (husky/lefthook/pre-commit/core.hooksPath) and surfaces manual commands
rather than writing inert hooks (warn-and-continue, never refuse), hermetic tests with
seam-driven isolation, snapshot-before-mutate backups.

---

## Delivery

4 chained PRs, stacked-to-main strategy:

| PR | Work units | Commits | Scope |
|---|---|---|---|
| #146 | WU-1 | 7dfee44 | `internal/githooks` — WriteBlock/RemoveBlock |
| #147 | WU-3+WU-4 | 13bb261, ff105a2 | `engram.ReadProjectName` + teamsync pure helpers |
| #148 | WU-2+WU-5 | 82a9f61, 422bc30, 227a020 | state.TeamSyncRecord + applyTeamSync/disableTeamSync |
| #149 | WU-6+WU-7+WU-8 | 7835f6b, 04a7072, dc8f527 | teamSyncScreen TUI + app wiring + docs |

---

## Verification summary

| PR | Verdict | CRITICAL | WARNING | SUGGESTION |
|---|---|---|---|---|
| PR-1 (#146) | PASS | 0 | 0 | 0 |
| PR-2 (#147) | PASS | 0 | 2 | 0 |
| PR-3 (#148) | PASS | 0 | 0 | 0 |
| PR-4 (#149) | PASS | 0 | 0 | 2 |

PR-2 warnings (non-blocking): `renderPrePush` tests asserted literal echo wording
from the design-era draft rather than the final wording — resolved before PR-3.

PR-4 suggestions (non-blocking, carried as deferred follow-ups below):
1. Conflict-golden: multi-line `renderPrePush` echo wraps without indentation in
   the manual-command block (cosmetic misalignment in golden only).
2. README conflict example shows design-era echo wording; actual `renderPrePush`
   echo differs — minor doc/code drift in the echo string only.

---

## Spec reconciliations made at archive time

All 4 reconciliations were verified against actual shipped code before editing
`openspec/changes/engram-team-sync/spec.md` and the promoted main spec.

### REQ-1.5 — `.git/hooks/` directory

**Was:** "missing `.git/hooks/` directory" listed as an error condition.
**As-built:** `githooks.go:23` — `os.MkdirAll(filepath.Dir(hookPath), 0o755)` creates
the directory when absent. It is never an error condition.
**Fix:** Reworded to say `WriteBlock` creates `.git/hooks/` via `MkdirAll`.

### REQ-2.4 — hook file deletion on disable

**Was:** "does NOT delete the hook file even if removing the block leaves the file
containing only `#!/bin/sh` or only whitespace."
**As-built:** `githooks.go:75` — `if remaining == "" || strings.TrimSpace(remaining) == "#!/bin/sh" { return os.Remove(hookPath) }` — file IS deleted (ADR-1, "leave no inert hook behind").
**Fix:** Reworded to match as-built delete-on-shebang-only behavior.

### REQ-3.2 — pre-push echo wording

**Was:** echo line MUST reference both `git add .engram` and `git commit`.
**As-built:** `teamsync.go:117-120` — `renderPrePush` emits
`echo 'Remember to commit .engram/ so teammates receive your memories.'` — no literal
`git add .engram` or `git commit` references (ADR-4).
**Fix:** Exact wording no longer mandated; as-built echo documented as the reference.

### REQ-4.1 — `.engram/config.json` key

**Was:** key is `"project"`.
**As-built:** `engram.go:264-265` — `projectConfig struct { ProjectName string \`json:"project_name"\` }` — key is `"project_name"` (ADR-5, reuses existing struct).
**Fix:** Key updated to `"project_name"` everywhere (REQ-4.1, REQ-4.2, SC-06, SC-12,
SC-15, REQ-11.2).

### Additional consistency fixes (same pass)

- REQ-7.1 table: `ConflictDetected bool` + `ConflictReason string` → `Conflict string`
  (ADR-2 wins; single-field design).
- REQ-6.3, SC-10, SC-12, SC-15: `ConflictDetected`/`ConflictReason` references
  updated to `Conflict`.
- SC-06: JSON example `{"project": "..."}` → `{"project_name": "..."}`.
- SC-06 assertion: "referencing git add .engram and git commit" → "reminding to
  commit .engram/" (consistent with REQ-3.2 fix).

---

## Stale-checkbox reconciliation

Tasks artifact (engram #373, openspec tasks.md) had T-12–T-17 unchecked (WU-6/7/8)
at archive time. Exceptional reconciliation performed:

- **Proof of completion:** apply-progress (engram #374) confirms ALL tasks complete
  for WU-6/7/8 with commit hashes. verify-report-pr4 (engram #379) confirms PASS,
  0 CRITICAL, 0 WARNING for the final TUI slice.
- **Root cause:** The persisted tasks artifact was not updated during the PR-4 apply
  pass (apply-progress was updated instead).
- **Action:** Checkboxes corrected in the archived `tasks.md`; noted in the archive
  note at the top of that file.

---

## Engram observation IDs (full traceability)

| Artifact | Engram ID |
|---|---|
| proposal | #370 |
| spec | #371 |
| design | #372 |
| tasks | #373 |
| apply-progress | #374 |
| verify-report-pr1 | #376 |
| verify-report-pr2 | #377 |
| verify-report-pr3 | #378 |
| verify-report-pr4 | #379 |
| archive-report | (this document, topic: sdd/engram-team-sync/archive-report) |

---

## Main spec promoted

`openspec/specs/engram-team-sync/spec.md` created as the canonical post-archive spec.
The reconciled spec is the authoritative source of truth for this capability going
forward.

---

## Files changed (code)

| File | Change |
|---|---|
| `internal/githooks/githooks.go` | New package: WriteBlock/RemoveBlock |
| `internal/githooks/githooks_test.go` | 7 table-driven cases |
| `internal/engram/engram.go` | Added `ReadProjectName` |
| `internal/engram/engram_test.go` | 5 cases for ReadProjectName |
| `internal/state/state.go` | TeamSyncRecord + State.TeamSync + SetTeamSync |
| `internal/state/state_test.go` | 5 round-trip cases |
| `internal/tui/teamsync.go` | Pure helpers + applyTeamSync/disableTeamSync + teamSyncScreen |
| `internal/tui/teamsync_helpers_test.go` | 17 pure-helper cases |
| `internal/tui/teamsync_apply_test.go` | 5 apply/disable cases |
| `internal/tui/teamsync_screen_test.go` | 14 Update()-driven TUI cases |
| `internal/tui/app.go` | "Configure team sync" menu item + dispatch |
| `internal/tui/app_test.go` | TestEnterOpensTeamSync + cursor position updates |
| `internal/tui/testdata/teamsync_*.golden` | 7 golden files (editing/ack/ack_hint/conflict/no_engram/done/failed) |
| `internal/tui/testdata/menu*.golden` | 4 menu goldens updated (new team-sync line) |
| `README.md` | "Configure team sync" in menu table + "Team memory sync" section |
| `llms.txt` | Team memory sync bullet |

---

## Deferred follow-ups (NOT implemented — future work)

These items were intentionally out of scope for E-1 and are recorded here for future
planning:

1. **`drift.StaleTeamSync` + `doctor` team-sync check.** When the drift/doctor system
   is built, `TeamSyncRecord.Workspace` already persists the target repo path, making
   implementation straightforward. This was the primary reason to store `Workspace`
   now (ADR-2).

2. **Conflict-golden indentation cosmetic.** The `teamsync_conflict.golden` manual-
   command block wraps the `renderPrePush` echo without indentation alignment
   (verify-report-pr4 SUGGESTION-1). No functional impact; fix in a future cosmetic
   pass if needed.

3. **README echo wording drift.** The README conflict example still references the
   design-era echo string instead of the actual `renderPrePush` output
   (verify-report-pr4 SUGGESTION-2). The `engram sync` commands are correct; only the
   echo example text differs slightly. Fix in a future docs-only pass.

---

## SDD cycle complete

Change `engram-team-sync` (E-1): planned → implemented → verified → archived.
