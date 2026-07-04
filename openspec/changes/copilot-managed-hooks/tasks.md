# Tasks: Copilot managed hooks (guardrails) — J-1/J-2

**Change:** `copilot-managed-hooks`
**Spec:** `sdd/copilot-managed-hooks/spec` (engram #393, verified against #395)
**Design:** `sdd/copilot-managed-hooks/design` (engram #394, verified against #395)
**TDD mode:** Strict — write failing test first, then implementation.
**Gate per work unit:** `gofmt -l .` (empty) → `go vet ./...` → `go test -race ./...` → `go build ./...`

## Review Workload Forecast

| Field | Value |
|-------|-------|
| Estimated changed lines | ~1400–1900 total (new package + state + TUI screen + drift + wiring + full test suite + goldens) |
| 400-line budget risk | High |
| Chained PRs recommended | Yes |
| Suggested split | PR-1 → PR-2 → PR-3 → PR-4 → PR-5 → PR-6 (6 slices, stacked-to-main suggested) |
| Delivery strategy | ask-on-risk |
| Chain strategy | pending |

Decision needed before apply: Yes
Chained PRs recommended: Yes
Chain strategy: pending
400-line budget risk: High

### Suggested Work Units (PR slices)

| Unit | Goal | Likely PR | Est. lines | Notes |
|------|------|-----------|-----------|-------|
| WU-1+WU-2 | `$COPILOT_HOME`/`HooksDir` (copilot.go) + `CopilotHooksRecord`/`SetCopilotHooks` (state.go) | PR-1 | ~150–200 | Independent of each other; no import cycle |
| WU-3+WU-4 | `internal/copilothooks` schema types + `RenderGuardrails` (bash+powershell, 4 patterns, decision output) | PR-2 | ~500–650 | Largest single unit — dual-shell renderer + doubled pattern tests; do not split further (one cohesive decision) |
| WU-5 | `internal/copilothooks` atomic writer + checksum | PR-3 | ~200–250 | Depends on WU-3 types only |
| WU-6+WU-7+WU-8 | TUI `applyCopilotHooks`/`disableCopilotHooks`/`backupCopilotHooks` + `drift.StaleCopilotHooks` + RunSync gate | PR-4 | ~250–300 | Orchestration glue; depends on PR-1/2/3 |
| WU-9+WU-10 | TUI screen FSM + goldens + menu wiring | PR-5 | ~400–500 | Depends on PR-4 |
| WU-11 | Docs (README + llms.txt) | PR-6 | ~50 | Depends on PR-5 |

Base-branch note if `feature-branch-chain` is chosen: PR-1 targets the tracker
branch; PR-2 targets PR-1's branch; PR-3→PR-2; PR-4→PR-3; PR-5→PR-4; PR-6→PR-5.
If `stacked-to-main`, each merges to `main` in order before the next starts.

---

## Dependency graph

```
WU-1 (copilot.HooksDir)  ─┐
WU-2 (state record)      ─┼──────────────────────────▶ WU-6 (apply/disable/backup)
WU-3 (schema+Marshal) ──▶ WU-4 (renderer) ─┐
                                           WU-5 (writer+checksum) ──┘
WU-6 ──▶ WU-7 (drift) // WU-8 (RunSync gate) ──▶ WU-9 (screen FSM) ──▶ WU-10 (menu wiring) ──▶ WU-11 (docs)

WU-1 // WU-2 // WU-3 — all parallel (no inter-deps)
WU-4 after WU-3; WU-5 after WU-3 (WU-4/WU-5 independent of each other)
WU-6 after WU-1+WU-2+WU-4+WU-5
WU-7 after WU-5+WU-2; WU-8 after WU-6
WU-9 after WU-6+WU-2; WU-10 after WU-9
```

---

## Work Unit 1 — `$COPILOT_HOME` resolution + `HooksDir` (`internal/copilot`)

### T-01 `[x]` `[RED]` Write `copilot_test.go` cases

`COPILOT_HOME` set → `HooksDir` under it, `userHomeDir` NOT called; unset →
`<home>/.copilot/hooks`; `HooksDir` always populated. (REQ-1, SC-01, SC-02)

### T-02 `[x]` `[GREEN + GATE]` Implement `HooksDir` + env resolution

Add `HooksDir string` to `Host`, `copilotHomeEnv` const, replace hardcoded
`cfg := filepath.Join(home, ".copilot")` in `Detect()` (copilot.go:20-26,
39-58) per ADR-1. Gate: green.

---

## Work Unit 2 — State layer: `CopilotHooksRecord` + `SetCopilotHooks`

### T-03 `[x]` `[RED]` Write state tests

Round-trip enabled+posture+presets+checksum; nil clears the field;
`omitempty` backward-compat load of a `state.json` without `copilot_hooks`.
(REQ-8)

### T-04 `[x]` `[GREEN + GATE]` Add record + setter

`CopilotHooksRecord{Enabled,Posture,Presets,Checksum}` (ADR-7),
`State.CopilotHooks` field after `TeamSync` (state.go:18-55, ~line 49),
`Store.SetCopilotHooks` mirroring `SetTeamSync` (state.go:358-369). Gate: green.

---

## Work Unit 3 — `internal/copilothooks`: v1 schema types + `Marshal`

### T-05 `[ ]` `[RED]` Write schema/golden tests

`HookFile`/`Hook` JSON shape: `hooks` object keyed by `preToolUse`, single
entry carries both `bash`+`powershell`, `version:1` pinned, exactly one entry
in `Hooks["preToolUse"]`. (REQ-2, ADR-2)

### T-06 `[ ]` `[GREEN + GATE]` Implement types + `Marshal`

New package `internal/copilothooks/copilothooks.go`: `Posture` enum
(off/warn/strict), consts (`GuardrailsFile`, `FilePrefix`, `SchemaVersion`,
`TimeoutSeconds`, `MatcherBash`, `EventPreToolUse`), `HookFile`/`Hook`
structs, `Marshal` (`json.MarshalIndent` + trailing newline). Add
`testdata/guardrails_strict.json` + `guardrails_warn.json` goldens. Gate: green.

---

## Work Unit 4 — `internal/copilothooks`: `RenderGuardrails` (bash+powershell)

### T-07 `[ ]` `[RED]` Write renderer tests

Table-driven: 4 patterns × match/non-match (REQ-3.2/3.3) × both shells ×
posture (`warn`→`ask`, `strict`→`deny`); REQ-4.3 silent-allow for
non-matches; `RenderGuardrails` takes no OS param and output has non-empty
`bash` AND `powershell` fields in one entry (SC-13); `matcher=bash`,
`timeoutSec=5` pinned.

### T-08 `[ ]` `[GREEN + GATE]` Implement `RenderGuardrails`

`internal/copilothooks/render.go`: baked-in 4-pattern alternation (ADR-4),
`renderBashScript`/`renderPowerShellScript` (ADR-3/ADR-5), decision JSON
output per REQ-4. Gate: green.

---

## Work Unit 5 — `internal/copilothooks`: atomic writer + checksum

### T-09 `[ ]` `[RED]` Write writer/checksum tests

`WriteHookFile`: creates `HooksDir`, atomic tmp-then-rename, mode `0o644`,
overwrite. `RemoveHookFile`: idempotent on missing, path-guard rejects `..`
and separators. `HookFileChecksum`/`CombinedChecksum`: absent dir → `""`,
single/two-file, order-independent, changes on byte change.

### T-10 `[ ]` `[GREEN + GATE]` Implement writer + checksum

`internal/copilothooks/writer.go`: `WriteHookFile`/`RemoveHookFile` (ADR-8,
path guard mirrors copilot.go:132-149), `HookFileChecksum`/`CombinedChecksum`
(ADR-6). Gate: green.

---

## Work Unit 6 — TUI orchestration: `applyCopilotHooks`/`disableCopilotHooks`/`backupCopilotHooks`

### T-11 `[ ]` `[RED]` Write apply/disable tests

`strict`/`warn` write file + update state; `off` routes to disable;
pre-existing non-capiko file untouched (REQ-6.5); posture downgrade
strict→warn→off leaves state/file consistent; backup called before write
when file exists, write aborted on backup error (SC-15).

### T-12 `[ ]` `[GREEN + GATE]` Implement orchestration functions

`internal/tui/copilothooks.go`: `applyCopilotHooks`/`disableCopilotHooks`/
`backupCopilotHooks` (REQ-9.4/9.5, mirrors `applyHeadroom`). Gate: green.

---

## Work Unit 7 — `drift.StaleCopilotHooks`

### T-13 `[ ]` `[RED]` Write drift tests

Unmanaged/disabled → false; matching checksum → false; hand-edited file →
true; missing file while enabled → true. (REQ-7.3, SC-08/SC-09)

### T-14 `[ ]` `[GREEN + GATE]` Implement `StaleCopilotHooks`

`internal/drift/drift.go:30-43`, mirroring `StaleHeadroom`. Gate: green.

---

## Work Unit 8 — RunSync re-apply gate

### T-15 `[ ]` `[RED]` Write RunSync tests

`state.CopilotHooks` enabled → `RunSync` calls `applyCopilotHooks` with
stored posture (SC-10); nil/disabled → hooks dir untouched (SC-11).

### T-16 `[ ]` `[GREEN + GATE]` Add re-apply gate

`internal/tui/sync.go:107-113`, mirroring the headroom/engram gates
(ADR-10). Gate: green.

---

## Work Unit 9 — TUI screen FSM + goldens

### T-17 `[ ]` `[RED]` Write `Update()`-driven screen tests

Posture cycle off→warn→strict→off; Apply from warn/strict emits cmd →
`copilotHooksAppliedMsg` → Done; Apply from off routes to disable;
failure → Failed; hydrates from `state.CopilotHooks.Posture` (default off).

### T-18 `[ ]` `[GREEN + GATE]` Implement `copilotHooksScreen`

`internal/tui/copilothooks.go` (ADR-9): posture dropdown row, Apply/Back
rows, repo-level future-note banner (REQ-9.3), `newCopilotHooks(svc)`
constructor (REQ-9.7). Gate: green.

### T-19 `[ ]` Generate + inspect goldens

`copilothooks_{editing_off,editing_warn,editing_strict,done,failed}.golden`
(REQ-11.3, SC-17). Inspect diffs per capiko-dev skill.

---

## Work Unit 10 — App menu wiring

### T-20 `[ ]` `[RED]` Write `TestEnterOpensCopilotHooks`

Menu item id `copilot-hooks` positioned after "Configure team sync",
`ready:true`; enter sets `a.active` to the Copilot hooks screen (SC-16).

### T-21 `[ ]` `[GREEN + GATE]` Wire menu item + `open()`

`{"Configure Copilot hooks","copilot-hooks",true}` in `menuItems`
(app.go:65-80) + `case it.id == "copilot-hooks"` in `open()` (app.go:225-262).
Regenerate + inspect main-menu golden diff.

---

## Work Unit 11 — Docs

### T-22 `[ ]` Update `README.md` and `llms.txt`

README: menu table entry + "Copilot hooks (guardrails)" section (posture
levels, four patterns, user-level-only scope note, fail-open timeout
caveat). llms.txt: bullet + docs link.

---

## Final gate (run before every PR)

- `gofmt -l .` → empty
- `go vet ./...` → clean
- `go test -race ./...` → all green
- `go build ./...` → exit 0
