# Tasks: SDD status Engram fallback

## Review Workload Forecast

| Field | Value |
|---|---|
| Estimated changed lines | 780–870 |
| 400-line budget risk | High |
| Chained PRs recommended | Yes |
| Suggested split | PR 1 (refactor+infra) → PR 2 (seam+helpers) → PR 3 (fallback+tests+doc) |
| Delivery strategy | ask-on-risk |
| Chain strategy | pending |

Decision needed before apply: Yes
Chained PRs recommended: Yes
Chain strategy: pending
400-line budget risk: High

### Suggested Work Units

| Unit | Goal | Likely PR | Notes |
|---|---|---|---|
| 1 | ADR-3 text cores + const + gating tests (Phases 1–2) | PR 1 | Base: `main`; no behavior change; ~190 lines |
| 2 | Export seam + observation helpers + project inference (Phase 3) | PR 2 | Base: PR 1 branch; seam only, no `Resolve` wiring; ~260 lines |
| 3 | `resolveEngramStatus` + `Resolve` wiring + SC-01–SC-11 tests + doc + gate (Phases 4–5) | PR 3 | Base: PR 2 branch; all spec behavior lands here; ~370 lines |

---

## Phase 1: Share-logic refactor (ADR-3)

*Satisfies: REQ-7 (shared regex) | Design: ADR-3, `countTaskProgressText`, `reportTextIsClearlyPassing`*

- [x] 1.1 **(RED)** In `internal/sddstatus/status_test.go`: write failing tests for `countTaskProgressText(string)` — mixed `[ ]`/`[x]`/`[X]`, prose-only content, empty string (maps to SC-07a).
- [x] 1.2 **(RED)** In `status_test.go`: write failing regression test asserting `countTaskProgressText(content)` result equals `countTaskProgress` applied to a `t.TempDir()` temp file with the same content (file-path parity).
- [x] 1.3 **(RED)** In `status_test.go`: write failing tests for `reportTextIsClearlyPassing(string)` — pass keyword, fail/CRITICAL keyword, negation pattern, empty/blank.
- [x] 1.4 **(GREEN)** In `internal/sddstatus/status.go`: extract `countTaskProgressText(content string) TaskProgress` (moves the loop + `taskCheckbox` scan); update `countTaskProgress` to `os.ReadFile` then call it — early-return semantics unchanged.
- [x] 1.5 **(GREEN)** In `status.go`: extract `reportTextIsClearlyPassing(text string) bool` (moves the line-scan loop); update `reportIsClearlyPassing` to `os.ReadFile` then call it — early-return semantics unchanged.
- [x] 1.6 Run `go test -race ./internal/sddstatus/...` — all green.

## Phase 2: Gating infrastructure

*Satisfies: REQ-1, REQ-2 | Design: `shouldTryEngram`, `configArtifactStoreIsEngram`, `ArtifactStoreEngram` const*

- [x] 2.1 **(RED)** In `status_test.go`: write failing tests for `shouldTryEngram` — env var set, `.engram/` dir present, `openspec/config.yaml` with `artifact_store: engram`, same with `hybrid`, camelCase `artifactStore:`, config value `openspec` does NOT gate, all-off returns false.
- [x] 2.2 **(GREEN)** In `status.go`: add `ArtifactStoreEngram = "engram"` const beside `artifactStore`.
- [x] 2.3 **(GREEN)** Create `internal/sddstatus/engram.go` (package `sddstatus`): implement `artifactStoreRe` regex, `configArtifactStoreIsEngram(cwd string) bool` (reads `openspec/config.yaml` then `.yml`, no YAML dep), `shouldTryEngram(cwd string) bool`.
- [x] 2.4 Run `go test -race ./internal/sddstatus/...` — all green.

## Phase 3: Engram export seam + observation helpers

*Satisfies: REQ-8, REQ-9, REQ-12 | Design: seam, `inferEngramProject`, `projectFromGitConfig`, observation helper chain*

- [x] 3.1 **(RED)** In `status_test.go`: add `withEngram(t *testing.T, obs []engramObservation)` helper (swaps seam, restores via `t.Cleanup`); write failing tests for `inferEngramProject` — `ENGRAM_PROJECT` env, git `.git/config` origin URL → `owner/repo`, dir-basename fallback (case-insensitive).
- [x] 3.2 **(RED)** Write failing tests for `collectEngramChanges` + `selectEngramChange` — one change returns it, two changes + no request returns `("", false)`, personal-scope excluded, project mismatch excluded (maps to SC-08a–e).
- [x] 3.3 **(RED)** Write failing tests for `engramArtifactState`, `engramArtifactContent`, `engramArtifactsForChange`, `engramArtifactPaths` — present/non-empty → `done`, absent → `missing`, empty → `partial`; six-key map; sentinel `engram:sdd/<change>/` path prefix for present artifacts.
- [x] 3.4 **(RED)** Write failing test SC-11 (seam isolation): install a seam that calls `t.Fatal` if invoked; confirm every existing test still passes without touching the real `engram` binary.
- [x] 3.5 **(GREEN)** In `engram.go`: define `engramObservation` struct; add `var engramExport = exportEngramObservations`; implement `exportEngramObservations()` — temp file, `exec.Command("engram","export",path)`, `os.ReadFile`, `json.Unmarshal` into `doc.Observations`.
- [x] 3.6 **(GREEN)** In `engram.go`: implement `inferEngramProject(cwd string) string` and `projectFromGitConfig(cwd string) string` — read `.git/config`, extract `[remote "origin"]` url via narrow regex `[:/]([^/:]+/[^/]+?)(?:\.git)?$`, lowercase result.
- [x] 3.7 **(GREEN)** In `engram.go`: add `titleRe` regex; implement `engramObservationMatchesProject`, `collectEngramChanges` (distinct sorted change names), `selectEngramChange` (exact-match or single-auto; zero/multi + no-request → false).
- [x] 3.8 **(GREEN)** In `engram.go`: implement `engramArtifactContent`, `engramArtifactState`, `engramArtifactsForChange` (six-key map), `engramArtifactPaths` (sentinel `engram:sdd/<change>/<artifact>` for non-missing artifacts; `.withArrays()` finalization).
- [x] 3.9 Run `go test -race ./internal/sddstatus/...` — all green (30 new tests).

## Phase 4: `resolveEngramStatus` + `Resolve` wiring + full test suite

*Satisfies: REQ-3–REQ-10 | Design: `resolveEngramStatus`, insertion point in `Resolve` (ADR-1)*

- [x] 4.1 **(RED)** Write failing test SC-01: install a seam that calls `t.Fatal`; no gating triggers set; assert `Resolve` returns `nextRecommended = "sdd-new"` and seam is never called.
- [x] 4.2 **(RED)** Write failing tests SC-02a–d: each gating trigger ON; seam returns one change with `proposal` done; assert `ArtifactStore == "engram"` and seam IS called.
- [x] 4.3 **(RED)** Write failing test SC-03: `.engram` present; OpenSpec change exists on disk; seam returns same change with all artifacts done; assert `ArtifactStore == "openspec"` and seam NOT called.
- [x] 4.4 **(RED)** Write failing tests SC-04 (zero OpenSpec, one Engram change — full origin flags + `nextRecommended == "apply"` + task progress counts) and SC-05 (named change absent from OpenSpec, present in Engram → resolves).
- [x] 4.5 **(RED)** Write failing test SC-06: two distinct Engram changes, no `ChangeName` request → `nextRecommended == "select-change"`, `BlockedReasons` non-empty, `ArtifactStore != "engram"`.
- [x] 4.6 **(RED)** Write failing table-driven test SC-07b: `nextRecommended` parity — for each planning-phase artifact state (propose/spec/design/tasks/apply-progress/verify/archive), assert Engram path and file path return identical `NextRecommended`, `Dependencies`, `ApplyState`.
- [x] 4.7 **(RED)** Write failing tests SC-09a–c (degradation: seam returns error / `"not-json"` / `{"observations":[]}` → `err == nil`, `nextRecommended == "sdd-new"`, no panic) and SC-10 (origin flags: `ChangeRoot` starts with `"engram:sdd/"`, `PlanningHome.Path == "engram:sdd"`, valid `json.Marshal`).
- [x] 4.8 **(GREEN)** In `engram.go`: implement `resolveEngramStatus(cwd, requested string) (Status, bool)` — calls `shouldTryEngram`, `engramExport` seam, `inferEngramProject`, `collectEngramChanges`, `selectEngramChange`, builds artifacts/taskProgress/verifyPassing via shared text cores, calls `resolveApplyState`/`resolveDependencies`/`resolveNextRecommended`, assembles via `baseStatus` + three origin overrides.
- [x] 4.9 **(GREEN)** In `internal/sddstatus/status.go` `Resolve`: insert `if blocked == "sdd-new" { if st, ok := resolveEngramStatus(cwd, options.ChangeName); ok { return st, nil } }` before the existing `return blockedStatus(...)`.
- [x] 4.9b **(carry-forward from PR2 verify, WARNING-2)** Add a routing test: a change discovered ONLY via its `sdd/<change>/state` observation (no proposal/spec/etc.) must route sanely — all artifacts `missing` → `nextRecommended == "propose"`, no panic. `collectEngramChanges` already discovers state-only changes (locked by `TestCollectEngramChanges_StateOnlyTitleIsDiscovered`); this confirms `resolveEngramStatus` handles them.
- [x] 4.10 Run `go test -race ./internal/sddstatus/...` — all 86 test functions green.

## Phase 5: Contract doc + quality gate + PR

*Satisfies: REQ-11 | Design: contract doc note, capiko-dev workflow*

- [x] 5.1 In `internal/catalog/skills/sdd-shared/sdd-status-contract.md`: add bounded note under **Artifact Store** section — engine MAY report `artifactStore: "engram"` with `changeRoot: "engram:sdd/<change>"` and `planningHome.path: "engram:sdd"` only when no matching OpenSpec change exists; read-only fallback; parsers MUST guard `engram:` prefix as non-filesystem origin marker.
- [x] 5.2 Run `go test ./internal/catalog/...` — embed tests green; confirm `internal/tui` golden files unchanged (`go test ./internal/tui`).
- [x] 5.3 `gofmt -l .` — no output; `go vet ./...` — clean.
- [x] 5.4 `go test -race ./...` — all 27 packages green.
- [x] 5.5 `go build ./...` — clean.
- [x] 5.6 Delivered as 3 chained PRs (stacked-to-main): PR #139 (Phases 1–2), PR #140 (Phase 3), PR #141 (Phases 4–5). All merged to main.
