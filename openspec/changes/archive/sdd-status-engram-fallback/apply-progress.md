# Apply Progress: sdd-status-engram-fallback (PR1 + PR2 + PR3)

**All slices complete — waiting for fresh-context review before push/PR**

---

## Phase 1: Share-logic refactor (ADR-3) — COMPLETE

- [x] 1.1 (RED) Tests for `countTaskProgressText(string)` — mixed `[ ]`/`[x]`/`[X]`, prose-only, empty string
- [x] 1.2 (RED) Parity regression test: `countTaskProgressText(content)` equals `countTaskProgress` on temp file
- [x] 1.3 (RED) Tests for `reportTextIsClearlyPassing(string)` — pass keyword, fail/CRITICAL, negation, empty/blank; file parity
- [x] 1.4 (GREEN) Extract `countTaskProgressText(content string) TaskProgress`; `countTaskProgress` delegates to it
- [x] 1.5 (GREEN) Extract `reportTextIsClearlyPassing(text string) bool`; `reportIsClearlyPassing` delegates to it
- [x] 1.6 `go test -race ./internal/sddstatus/...` — GREEN

## Phase 2: Gating infrastructure — COMPLETE

- [x] 2.1 (RED) Tests for `shouldTryEngram` — env var, `.engram/` dir, `artifact_store: engram`, `hybrid`, camelCase `artifactStore:`, `openspec` value does NOT gate, all-off returns false, `.yml` extension
- [x] 2.2 (GREEN) `ArtifactStoreEngram = "engram"` const added beside `artifactStore` in `status.go`
- [x] 2.3 (GREEN) `internal/sddstatus/engram.go` created: `artifactStoreRe`, `configArtifactStoreIsEngram`, `shouldTryEngram`
- [x] 2.4 `go test -race ./internal/sddstatus/...` — GREEN

## Phase 3: Engram export seam + observation helpers — COMPLETE

- [x] 3.1 (RED) `withEngram` helper + tests for `inferEngramProject` (env / git HTTPS / git SSH / dir basename)
- [x] 3.2 (RED) Tests for `collectEngramChanges` + `selectEngramChange` — one change, two changes + no request, personal-scope excluded, project mismatch excluded, requested found/not-found
- [x] 3.3 (RED) Tests for `engramArtifactState`, `engramArtifactContent`, `engramArtifactsForChange`, `engramArtifactPaths`
- [x] 3.4 (RED) SC-11 seam isolation test: fatal-seam + gating off → seam never called, result = sdd-new
- [x] 3.5 (GREEN) `engramObservation` struct; `var engramExport = exportEngramObservations`; `exportEngramObservations()` with temp file + exec + json.Unmarshal
- [x] 3.6 (GREEN) `gitOriginURLRe`, `gitOwnerRepoRe`; `projectFromGitConfig(cwd)` (reads .git/config, lowercased); `inferEngramProject(cwd)` (env → git → basename)
- [x] 3.7 (GREEN) `titleRe`; `engramObservationMatchesProject`; `collectEngramChanges` (distinct sorted); `selectEngramChange` (exact-match or single-auto; zero/multi → false)
- [x] 3.8 (GREEN) `engramArtifactLookup`; `engramArtifactContent`; `engramArtifactState` (content-based twin of singleArtifactState); `engramArtifactsForChange` (six-key map); `engramArtifactPaths` (sentinel engram:sdd/<change>/* paths + withArrays finalization)
- [x] 3.9 `go test -race ./internal/sddstatus/...` — all green (30 new tests)

## Phase 4: `resolveEngramStatus` + `Resolve` wiring + full test suite — COMPLETE

- [x] 4.1 (RED) SC-01: fatal-seam + gating OFF → seam not called, nextRecommended = "sdd-new"
- [x] 4.2 (RED) SC-02a–d: each gating trigger ON → seam IS called, ArtifactStore = "engram"
- [x] 4.3 (RED) SC-03: .engram present + OpenSpec change exists → ArtifactStore = "openspec", seam NOT called
- [x] 4.4 (RED) SC-04 (zero OpenSpec + one Engram change: full origin flags, nextRecommended=apply, task progress) and SC-05 (named change in Engram → resolves)
- [x] 4.5 (RED) SC-06: two Engram changes, no request → nextRecommended = "select-change", blockedReasons non-empty, ArtifactStore != "engram"
- [x] 4.6 (RED) SC-07b: table-driven nextRecommended parity (7 cases: partial-proposal/spec/design/tasks/apply-progress/verify/archive) — Engram and file paths return identical NextRecommended, Dependencies, ApplyState
- [x] 4.7 (RED) SC-09a/b/c: seam error / malformed / empty → err=nil, sdd-new, no panic; SC-10: origin flags + JSON round-trip valid, nextRecommended != sdd-new
- [x] 4.8 (GREEN) `resolveEngramStatus(cwd, requested string) (Status, bool)` in `engram.go` — full flow: shouldTryEngram → engramExport seam → inferEngramProject → collectEngramChanges → selectEngramChange (with multi-change select-change branch) → artifacts/taskProgress/verifyPassing → resolveApplyState/resolveDependencies/resolveNextRecommended → baseStatus + 3 origin overrides
- [x] 4.9 (GREEN) `Resolve` wiring in `status.go`: `if blocked == "sdd-new" { if st, ok := resolveEngramStatus(cwd, options.ChangeName); ok { return st, nil } }` before existing `return blockedStatus(...)`
- [x] 4.9b (carry-forward) State-only change (only `sdd/<change>/state` obs) routes sanely — all artifacts missing → nextRecommended = "propose", no panic, ArtifactStore = "engram" (`TestSC_StateOnlyChange_RoutesToPropose`)
- [x] 4.10 `go test -race ./internal/sddstatus/...` — all 86 tests green

## Phase 5: Contract doc + quality gate — COMPLETE

- [x] 5.1 `internal/catalog/skills/sdd-shared/sdd-status-contract.md` — added bounded **Engram read-only fallback** note under Artifact Store section
- [x] 5.2 `go test ./internal/catalog/...` — embed tests green; `go test ./internal/tui/...` — golden files unchanged
- [x] 5.3 `gofmt -l .` — no output; `go vet ./...` — clean
- [x] 5.4 `go test -race ./...` — all 27 packages pass (86 tests in sddstatus)
- [x] 5.5 `go build ./...` — clean

---

## Commits

### PR1 (merged — b6c1049, PR #139)
- `33afb5a` — refactor(sddstatus): extract countTaskProgressText and reportTextIsClearlyPassing text cores
- `f5705f3` — feat(sddstatus): add ArtifactStoreEngram const and shouldTryEngram gating

### PR2 (merged — d3ccd8d, PR #140)
- `8310bed` — feat(sddstatus): engram observation seam, project inference, and artifact helpers

### PR3 (merged — PR #141)
- `08eed4a` — feat(sddstatus): resolveEngramStatus fallback + Resolve wiring (3/3)
- `85c1d76` — docs(sddstatus): add Engram read-only fallback note to sdd-status-contract

## Quality gate (PR3)

- `gofmt -l .` — clean (no output)
- `go vet ./...` — clean (no output)
- `go test -race ./...` — all 27 packages pass (86 tests in sddstatus)
- `go build ./...` — clean
- `go test ./internal/tui/...` — golden files unchanged

## Files changed (PR3)

- `internal/sddstatus/engram.go` — +71 lines: `resolveEngramStatus` + ambiguity-aware blocked status for multiple Engram changes
- `internal/sddstatus/status.go` — +8 lines: `blocked == "sdd-new"` wiring in `Resolve`
- `internal/sddstatus/status_test.go` — +537 lines: SC-01 through SC-10 end-to-end tests + helpers + 4.9b state-only test
- `internal/catalog/skills/sdd-shared/sdd-status-contract.md` — +15 lines: Engram read-only fallback note
