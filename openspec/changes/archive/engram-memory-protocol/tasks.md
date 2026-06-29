# Tasks: engram-memory-protocol

## Metadata

- **Change**: `engram-memory-protocol`
- **Delivery**: single PR (see Review Workload Forecast below)
- **TDD mode**: Strict — red-first for every logic unit
- **Test runner**: `go test -race ./...`
- **Branch convention**: branch from `main` before first edit

---

## Review Workload Forecast

| Metric | Estimate |
|---|---|
| `internal/memory/memory.go` | ~35 lines |
| `internal/memory/memory_test.go` | ~40 lines |
| `internal/tui/memory.go` | ~30 lines |
| `internal/tui/memory_test.go` | ~130 lines (7 tests) |
| `internal/tui/engram.go` wiring | ~6 lines (2 insertions) |
| `internal/tui/sync.go` wiring | ~4 lines (1 insertion) |
| **Total** | **~245 lines** |

**Chained PRs recommended: No**
**400-line budget risk: Low**
**Decision needed before apply: No**

Rationale: this is a faithful clone of an established pattern across two new files plus three small wiring insertions. No golden files are affected.

---

## Dependency Notes

- Work units 1–4 are sequential: each unit depends on the previous being committed and tests green.
- Within each work unit, tests are written first (red), then implementation (green). They ship in the same commit.
- The quality gate (work unit 5) is a blocking prerequisite for the PR step.

---

## Work Unit 1 — `internal/memory` package (content layer)

Satisfies: spec §capiko:memory Package, §Block Content – Search Before Acting, §Block Content – Proactive Save on Triggers, §Block Content – Session-Close Summary, §Block Content – Lifecycle-Aware Reads, §Block Content – No Literal Periodic Timer.
Design: Component 1.

### 1.1 — Branch

- [x] Create branch `feat/engram-memory-protocol` from a fresh `git pull main`.
  - _Ref: capiko-dev skill — branch first, never commit to main._

### 1.2 — Red: write failing tests (`internal/memory/memory_test.go`)

- [x] Create `internal/memory/memory_test.go` (package `memory`) with these test functions — all expected to fail at this point:
  - `TestMarkersAreDistinctAndNamespaced` — asserts `MarkerStart != MarkerEnd`, both contain `"capiko:memory"`, neither equals any marker from `efficiency`, `sdd`, or `persona`.
    - _Spec: §capiko:memory Package – Markers are distinct and namespaced_
  - `TestRenderReturnsProtocol` — asserts `Render()` is non-empty and lowercased content contains `"search"`, `"mem_save"`, and `"proactiv"`.
    - _Spec: §Render returns a non-empty string, §Block Content – Search Before Acting, §Block Content – Proactive Save on Triggers_
  - `TestRenderCoversTriggers` — asserts lowercased `Render()` contains `"decision"`, `"root cause"`, and `"mem_session_summary"`.
    - _Spec: §Block carries proactive-save directive, §Block names concrete trigger categories, §Block carries session-close directive_
  - `TestRenderLifecycleAware` — asserts `"active"` and `"needs_review"` are present.
    - _Spec: §Block carries lifecycle-aware read directive_
  - `TestRenderMilestoneAdvisory` — asserts `"no timer"` (or `"milestone"`) is present and `"timer"` does not appear as a runtime promise; also asserts `"every 15"` and `"reminder"` are absent.
    - _Spec: §Block does not promise a literal timer_
- [x] Run `go test ./internal/memory/...` — confirm all five tests fail to compile (package does not exist yet). This is the red gate.

### 1.3 — Green: implement `internal/memory/memory.go`

- [x] Create `internal/memory/memory.go` (package `memory`) with:
  - `MarkerStart = "<!-- capiko:memory:start -->"`
  - `MarkerEnd   = "<!-- capiko:memory:end -->"`
  - `func Render() string { return block }`
  - `const block = \`...\`` — the exact block content from design §Component 1 (Proposed block content).
- [x] Run `go test -race ./internal/memory/...` — all five tests must pass. This is the green gate.

### 1.4 — Commit

- [x] `gofmt -l ./internal/memory/` — output must be empty.
- [x] Commit: `feat(memory): add capiko:memory instruction block and markers`
  - Include both `memory.go` and `memory_test.go` in the same commit.

---

## Work Unit 2 — `applyMemoryProtocol` applier (`internal/tui/memory.go`)

Satisfies: spec §Gating ON – Block Written When Engram Is Enabled, §Gating OFF – Block Absent or Removed When Engram Is Disabled, §Idempotency, §Coexistence with Other Managed Blocks, §Backup on Change.
Design: Component 2.

### 2.1 — Red: write failing tests (`internal/tui/memory_test.go`)

- [x] Create `internal/tui/memory_test.go` (package `tui`) with:
  - `TestApplyMemoryProtocolWritesBlock` — call `applyMemoryProtocol(host, store, nil, true)` with a `t.TempDir()` host; assert `copilot-instructions.md` contains `memory.MarkerStart` and `"Memory protocol"`.
    - _Spec: §First apply with engram enabled, §Nil backup store does not cause an error_
  - `TestApplyMemoryProtocolDisabledRemovesBlock` — apply enabled then disabled; assert `memory.MarkerStart` absent afterward.
    - _Spec: §Disable after a prior enable removes the block_
  - `TestApplyMemoryProtocolNilHostIsNoop` — call with `nil` host; assert no error and no file created.
    - _Spec: §Nil host is a no-op_
  - `TestApplyMemoryProtocolIdempotent` — call enabled twice; assert `memory.MarkerStart` appears exactly once.
    - _Spec: §Re-apply enabled does not duplicate the block_
  - `TestApplyMemoryProtocolPreservesPersonaAndEfficiency` — apply `applyPersona`, then `applyOutputEfficiency`, then `applyMemoryProtocol` with `enabled=true`; assert all three markers coexist and persona content is unchanged.
    - _Spec: §Memory block coexists with persona block, §Coexistence with Other Managed Blocks_
- [x] Run `go test ./internal/tui/...` — the new tests must fail to compile (function `applyMemoryProtocol` does not exist). Red gate confirmed.

### 2.2 — Green: implement `internal/tui/memory.go`

- [x] Create `internal/tui/memory.go` (package `tui`) with `applyMemoryProtocol` following the exact design shape in §Component 2:
  - Early return on `host == nil`.
  - Set `block = memory.Render()` when `enabled`, empty string otherwise.
  - Call `instructions.Render(path, memory.MarkerStart, memory.MarkerEnd, block)`.
  - On `changed`: backup if `bkp != nil` (label `"memory"`), then `instructions.Write`.
  - `_ = store` (no new state field — derived from caller; keep param for signature parity).
- [x] Run `go test -race ./internal/tui/...` — all five new tests must pass. Green gate.

### 2.3 — Commit

- [x] `gofmt -l ./internal/tui/memory.go` — output must be empty.
- [x] Commit: `feat(tui): add applyMemoryProtocol applier`
  - Include `memory.go` and `memory_test.go` in the same commit.

---

## Work Unit 3 — Wiring: `applyEngramConfig` / `disableEngram`

Satisfies: spec §Wiring in applyEngramConfig / disableEngram, §Backup on Change (propagation), §Gating ON / OFF integration.
Design: Component 3, §3a and §3b.

### 3.1 — Red: write failing gating tests in `internal/tui/memory_test.go`

- [x] Add to `internal/tui/memory_test.go`:
  - `TestApplyEngramConfigInjectsMemory` — call `applyEngramConfig(svc, workspace, &EngramRecord{Enabled: true})` with `t.TempDir()` host and workspace (swap `cloudConfig`, `cloudEnroll` seams via `t.Cleanup`); assert `copilot-instructions.md` contains `memory.MarkerStart`.
    - _Spec: §applyEngramConfig writes the memory block_
  - `TestDisableEngramRemovesMemory` — enable via `applyEngramConfig`, then call `disableEngram`; assert `memory.MarkerStart` absent.
    - _Spec: §disableEngram removes the memory block_
- [x] Run `go test ./internal/tui/...` — new tests must fail (wiring not yet added). Red gate.

### 3.2 — Green: wire into `internal/tui/engram.go`

- [x] In `applyEngramConfig`, immediately after the `applyEngram(...)` call (line ~88-90) and before the `hasSurface` vscode block, insert:
  ```go
  if err := applyMemoryProtocol(svc.host, svc.state, svc.backup, true); err != nil {
      return err
  }
  ```
  _Design: §3a — keep memory block adjacent to engram MCP wiring._

- [x] In `disableEngram`, after the `vscodeUserMCPath` removal block and before `rec.Checksum = ""` (line ~133), insert:
  ```go
  if err := applyMemoryProtocol(svc.host, svc.state, svc.backup, false); err != nil {
      return err
  }
  ```
  _Design: §3b — remove block alongside other removals, before state write._

- [x] Run `go test -race ./internal/tui/...` — all tests (existing + new gating tests) must pass. Green gate.

### 3.3 — Commit

- [x] `gofmt -l ./internal/tui/engram.go` — output must be empty.
- [x] Commit: `feat(tui): wire memory block into applyEngramConfig and disableEngram`
  - Include `engram.go` and the updated `memory_test.go` in the same commit.

---

## Work Unit 4 — Wiring: `RunSync` re-apply branch

Satisfies: spec §Sync Re-Apply — all three scenarios.
Design: Component 3, §3c.

### 4.1 — Red: write failing sync gating tests in `internal/tui/memory_test.go`

- [x] Add to `internal/tui/memory_test.go`:
  - `TestRunSyncReappliesMemoryProtocol` — call `store.SetEngram(&EngramRecord{Enabled: true})`, then `RunSync(host, testCatalog(), nil, store, nil)`; assert `copilot-instructions.md` contains `memory.MarkerStart`.
    - _Spec: §Sync re-applies block when engram is enabled_
  - `TestRunSyncSkipsMemoryWhenEngramDisabled` — set engram disabled (or nil); run `RunSync`; assert `memory.MarkerStart` absent.
    - _Spec: §Sync removes block when engram is disabled, §Sync with no engram state does not write the block_
- [x] Run `go test ./internal/tui/...` — new tests must fail. Red gate.

### 4.2 — Green: wire into `internal/tui/sync.go`

- [x] Inside the `if st.Engram != nil && st.Engram.Enabled` block in `RunSync` (line ~92-96), immediately after the `applyEngram(...)` call, insert:
  ```go
  if err := applyMemoryProtocol(host, store, bkp, true); err != nil {
      return len(recorded) + len(agentRecorded), fmt.Errorf("re-applying memory protocol: %w", err)
  }
  ```
  _Design: §3c — tracks the catalog on every sync, rewritten only on change._

- [x] Run `go test -race ./internal/tui/...` — all tests must pass. Green gate.

### 4.3 — Commit

- [x] `gofmt -l ./internal/tui/sync.go` — output must be empty.
- [x] Commit: `feat(tui): re-apply memory protocol in RunSync`
  - Include `sync.go` and the updated `memory_test.go` in the same commit.

---

## Work Unit 5 — Quality gate and PR

Satisfies: spec §Test Suite Passes Under Strict TDD — CI gate and formatting gate scenarios.

### 5.1 — Full quality gate

- [x] `gofmt -l .` — output must be empty (no unformatted files).
- [x] `go vet ./...` — no diagnostics.
- [x] `go test -race ./...` — exit code 0, no race conditions.
- [x] `go build ./...` — clean build.

### 5.2 — PR

- [ ] Push branch `feat/engram-memory-protocol`.
- [ ] Open PR targeting `main` with a conventional commit title (e.g. `feat(memory): add engram proactive-memory protocol block`).
- [ ] Confirm CI (quality gate) passes before squash-merge.
- [ ] Squash-merge, delete branch, sync local `main`.

---

## Task → Spec / Design Traceability

| Task | Spec Requirement | Design Section |
|---|---|---|
| 1.2–1.4 | §capiko:memory Package, §Block Content (all four) | Component 1 |
| 2.1–2.3 | §Gating ON/OFF, §Idempotency, §Coexistence, §Backup on Change | Component 2 |
| 3.1–3.3 | §Wiring in applyEngramConfig / disableEngram | Component 3 §3a, §3b |
| 4.1–4.3 | §Sync Re-Apply | Component 3 §3c |
| 5.1 | §Test Suite Passes Under Strict TDD | Test strategy |
| 5.2 | Delivery convention | capiko-dev skill |
