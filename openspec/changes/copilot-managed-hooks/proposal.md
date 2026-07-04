# Proposal: Copilot managed hooks (guardrails)

## Summary

Add the first HARD enforcement lever to capiko: capiko-managed GitHub Copilot CLI
hooks. Today all levers are SOFT (instruction blocks, skills, MCP) — the model may
ignore them. This change adds a new managed feature that writes user-level hook files
to `~/.copilot/hooks/*.json` (schema `{"version":1,"hooks":[...]}`), following the
established managed-feature pattern (Record + SetXxx + apply/disable + RunSync gate +
TUI screen + snapshot-before-mutate).

The first (and only v1) preset is **guardrails**: a `preToolUse` hook that
deterministically blocks or asks before dangerous shell commands run — the hard lever
that soft instructions cannot provide.

## Why now

Soft levers are advisory. A `preToolUse` hook with `permissionDecision=deny` is
deterministic: the CLI will not run the matched command regardless of what the model
decides. This is a categorically new capability for capiko and the highest-signal
first use of the Copilot hook surface. `preToolUse` + `permissionDecision` is also the
most complete, bug-free part of the hook spec (unlike `additionalContext`, see Risks).

## Settled decisions (do not reopen)

| Decision | Resolution |
| --- | --- |
| Guardrail posture | USER-SELECTABLE via TUI dropdown: `off` / `warn` (permissionDecision=ask) / `strict` (permissionDecision=deny). User picks at apply time. |
| v1 pattern set | MINIMAL, high-signal only: `rm -rf /` (+equivalents), `curl … \| sh` (pipe-to-shell), `git push --force` to main/master, `chmod 777`. Low false-positive. |
| Matcher | `^(?:bash\|shell\|run_command)$` — targets shell/run tools. |
| Repo-level hooks | DEFERRED. TUI shows a note: "Manages user-level CLI hooks. For GitHub cloud agent enforcement, repo-level hooks are a future feature." |
| Windows | IN scope. Renderer detects OS, emits `bash` (macOS/Linux) or `powershell` (Windows) for command-type hooks. |
| File layout | Individual JSON files, one per preset (`capiko-guardrails.json`). Whole-file ownership; atomic write-to-tmp→rename; disable = `os.Remove`. |
| Drift | Combined SHA-256 of capiko-owned hook files → `CopilotHooksRecord.Checksum`; `drift.StaleCopilotHooks()`. |
| Version | Pin `"version":1` in rendered files. |
| Shell safety | Sanitize any user-influenced shell content via the `shSingleQuote()` pattern (teamsync.go:97–99). |
| Timeout | Cap `timeoutSec` at 5s — guardrail runs on every shell tool call. |

## Scope

### In scope (slice J-1 foundation + J-2 guardrails)

- **Foundation (bundled, settled):**
  - Fix `$COPILOT_HOME` resolution in `copilot.Detect()` (copilot.go:39–58, currently
    hardcodes `~/.copilot`) — prerequisite so hook files land in the right dir (G-CC3).
  - Add `HooksDir string` to `copilot.Host` (copilot.go:20–26).
  - `state.CopilotHooksRecord{Enabled, Checksum, Presets}` + `Store.SetCopilotHooks()`.
- **New `internal/copilothooks/` package:** v1 schema types, `RenderGuardrails()`,
  `WriteHookFile()` (atomic), `RemoveHookFile()`, `HookFileChecksum()`, OS-aware
  bash/powershell command selector.
- **`internal/tui/copilothooks.go`:** `applyCopilotHooks()`, `disableCopilotHooks()`,
  backup-before-mutate, TUI screen (posture dropdown: off/warn/strict).
- **RunSync re-apply gate** (sync.go:109–113) gated on `CopilotHooks.Enabled`.
- **`drift.StaleCopilotHooks()`** (mirrors drift.go:30–43).
- **Menu wiring** (app.go:66–80 menuItems, 225–262 open()).
- Tests: hooks package (render/write/remove/checksum/OS branches) + TUI screen goldens.

### Out of scope (non-goals)

- **Repo-level hooks** (`.github/hooks`, cloud agent) — future slice; TUI note only.
- **Learn-loop / memory-injection hooks** (D-8) — blocked by upstream `additionalContext`
  bug on `postToolUse`/`sessionStart` (#2142, #2980).
- **sessionStart verify-layer hook** (G-CC2) — future, separate slice.
- **Native plugin management** (C-4) — separate feature on the same foundation.
- **Inline hooks in `settings.json`** — rejected in favor of individual files.
- Additional guardrail patterns beyond the four v1 high-signal ones.

## Capabilities

### New Capabilities
- `copilot-managed-hooks`: capiko manages user-level Copilot CLI hook files
  (`~/.copilot/hooks/*.json`), with a selectable guardrail posture (off/warn/strict),
  a minimal high-signal `preToolUse` pattern set, OS-aware command rendering, atomic
  whole-file writes, checksum-based drift detection, and RunSync re-apply.

### Modified Capabilities
- None. `$COPILOT_HOME` resolution in `copilot.Detect()` is a bug fix bundled here, not
  a spec-level behavior change to an existing capability.

## Approach

Approach A from exploration: user-level individual JSON files. Reuse the entire
orchestration layer (Record, SetXxx, apply/disable, RunSync gate, TUI FSM, backup) from
the existing managed-feature pattern; only the JSON writer, v1 schema, and OS-aware
renderer are net-new. `internal/githooks` (shell-marker) is NOT reused — JSON files use
whole-file atomic ownership like `state.Store.Save()` / `instructions.Write()`.

## Affected Areas

| Area | Impact | Description |
| --- | --- | --- |
| `internal/copilot/copilot.go` | Modified | `$COPILOT_HOME` in `Detect()`; `HooksDir` on `Host` |
| `internal/state/state.go` | Modified | `CopilotHooksRecord` + field + `SetCopilotHooks()` |
| `internal/copilothooks/` | New | v1 schema, renderer, atomic writer, checksum, OS selector |
| `internal/tui/copilothooks.go` | New | apply/disable + posture-dropdown screen |
| `internal/tui/sync.go` | Modified | re-apply gate |
| `internal/tui/app.go` | Modified | menu item + open() case |
| `internal/drift/drift.go` | Modified | `StaleCopilotHooks()` |
| `internal/tui/testdata/*.golden` | New | screen goldens |

## Risks

| Risk | Likelihood | Mitigation |
| --- | --- | --- |
| `$COPILOT_HOME` not honored → hooks in wrong dir | High if unfixed | Fixed in foundation (G-CC3) before any hook write |
| Admin policy hooks (`/etc/github-copilot/policy.d/`) override user hooks | Med | Surface in TUI; not a capiko bug |
| `additionalContext` upstream bug bounds future learn-loop | N/A for v1 | preToolUse unaffected; note only, no v1 dependency |
| Shell content injection in command hooks | Low | `shSingleQuote()` sanitization on any user-influenced field |
| Guardrail latency on every shell call | Low | 5s `timeoutSec` cap; minimal pattern set |
| Hook schema version drift | Low | Pin `"version":1`; migration path in Record |

## Rollback Plan

Disable = `disableCopilotHooks()` runs `os.Remove()` on capiko-owned hook file(s) and
clears `CopilotHooksRecord.Enabled`. State snapshot to `~/.capiko/backups/` precedes any
mutation, so the prior state and any pre-existing files are restorable. capiko owns only
its own preset files — user/other hook files are never touched.

## Dependencies

- GitHub Copilot CLI supporting hook schema `version:1` with `preToolUse` +
  `permissionDecision`. Older CLI without preToolUse support silently no-ops.

## Success Criteria

- [ ] With `strict` posture applied, running `rm -rf /` in Copilot CLI is denied.
- [ ] With `warn` posture, the same command prompts (ask) instead of running silently.
- [ ] `off` removes/omits the guardrail; `disable` deletes the file and clears state.
- [ ] Hook files land in `$COPILOT_HOME/hooks` when the env var is set.
- [ ] `drift.StaleCopilotHooks()` detects a hand-edited hook file.
- [ ] Renderer emits `powershell` on Windows, `bash` on macOS/Linux.
- [ ] `go test -race ./...` green; goldens reflect only intended lines.
