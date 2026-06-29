# Archive Report: sdd-status-engram-fallback

**Status:** CLOSED — all PRs merged, implementation verified, 0 critical findings.
**Archived:** 2026-06-29
**Engram topic:** `sdd/sdd-status-engram-fallback/archive-report`

---

## What shipped

`internal/sddstatus` now has a files-first, Engram-fallback resolution path. When
`selectChange` returns `sdd-new` (no matching OpenSpec change on disk) and gating
conditions are met, `Resolve` reconstructs a full `capiko.sdd-status` from synced
Engram observations (`sdd/<change>/*`) instead of returning a blind blocked result.
OpenSpec files remain canonical; Engram is consulted only as a graceful-degradation
fallback when the files are absent.

---

## Delivery

Delivered as 3 chained PRs, stacked-to-main strategy.

| PR | GitHub | Merge commit | Slice |
|---|---|---|---|
| PR 1 | #139 | b6c1049 | Phases 1–2: ADR-3 text-core refactor + gating infrastructure |
| PR 2 | #140 | d3ccd8d | Phase 3: Engram export seam + observation helpers + project inference |
| PR 3 | #141 | (merged) | Phases 4–5: `resolveEngramStatus` + `Resolve` wiring + full test suite + contract doc |

---

## Files changed

| File | Change |
|---|---|
| `internal/sddstatus/status.go` | Extract `countTaskProgressText`/`reportTextIsClearlyPassing` text cores (Phase 1); add `ArtifactStoreEngram` const (Phase 2); insert Engram fallback wiring in `Resolve` (Phase 4) |
| `internal/sddstatus/engram.go` | New file: `shouldTryEngram`, `configArtifactStoreIsEngram`, `engramExport` seam, `exportEngramObservations`, `inferEngramProject`, `projectFromGitConfig`, observation helpers, `resolveEngramStatus` |
| `internal/sddstatus/status_test.go` | 86 tests total (+56 new across Phases 1–4); SC-01 through SC-11 end-to-end; all hermetic via seam |
| `internal/catalog/skills/sdd-shared/sdd-status-contract.md` | Bounded **Engram read-only fallback** note added under Artifact Store section |

---

## Verified state (PR3 verify — 0 CRITICAL)

**Verdict:** PASS, ready for PR. All quality gates green:
- `gofmt -l .` — clean
- `go vet ./...` — clean
- `go build ./...` — clean
- `go test -race ./...` — all 27 packages pass (86 tests in `internal/sddstatus`)
- `go test ./internal/tui/...` — golden files UNCHANGED (zero diff)
- `go test ./internal/catalog/...` — embed tests green

**Findings at verify:**
- 0 CRITICAL
- 1 WARNING (non-blocking, resolved at archive): design.md was stale on the
  multiple-Engram-changes case. Design pseudocode said `ok=false` → `sdd-new`;
  implementation correctly returned `select-change` per spec SC-06. Reconciliation
  note added to design.md in this archive.
- 1 SUGGESTION (not acted on): SC-07b "partial-proposal" test case used a
  non-equivalent artifact state. Functional behavior correct; test is conservative.

---

## Key design decisions

| ADR | Decision |
|---|---|
| ADR-1 | Fallback fires inside `Resolve` at the `sdd-new` branch, not inside `selectChange`. Keeps `selectChange` pure. |
| ADR-2 | `ArtifactStoreEngram = "engram"` const + post-`baseStatus` override. No enum; no JSON shape change. |
| ADR-3 | `countTaskProgress`/`reportIsClearlyPassing` split into IO-free text cores shared by both paths. Structural parity guarantee. |
| ADR-4 | Dropped gentle-ai's `includeInstructions` parameter. Not present in capiko's `ResolveOptions`. |

---

## Carried-forward notes

- **design.md ambiguity reconciliation** — applied in this archive. No code change needed.
- **SC-07b partial-proposal test** — conservative but functionally correct. No action needed.
- **Export schema drift** — the fallback depends on `engram export` emitting
  `{"observations":[{"title","content","project","scope"}]}`. Schema drift degrades
  to today's blocked status (never a crash). Monitor if Engram CLI changes.

---

## Engram artifact observation IDs

| Artifact | Engram ID |
|---|---|
| verify-report (PR1) | #360 |
| verify-report (PR2) | #362 |
| verify-report (PR3) | #363 |
