# Archive Report: copilot-managed-hooks (J-1/J-2)

**Archived:** 2026-07-04
**Status:** COMPLETE — all 6 PRs (#152/#154/#155/#156/#157/#158) merged to main.

---

## What shipped

capiko's first HARD enforcement lever: a capiko-managed GitHub Copilot CLI
`preToolUse` hook, written to `$COPILOT_HOME/hooks/capiko-guardrails.json`
(default `~/.copilot/hooks/`). The hook deterministically intercepts four
high-signal dangerous shell command patterns — `rm -rf /` (+equivalents),
`curl|sh` pipe-to-shell, `git push --force` to `main`/`master`, and
`chmod 777` — before they run, and responds with a Copilot CLI
`permissionDecision` driven by a user-selectable posture: `off` (no file),
`warn` (`permissionDecision=ask`), or `strict` (`permissionDecision=deny`).

Key properties: default off; user-level only (`$COPILOT_HOME/hooks/`, never
`.github/hooks/`); whole-file atomic ownership of `capiko-guardrails.json`
(never touches sibling files); a single hook entry always carries BOTH
`bash` and `powershell` scripts — Copilot CLI itself dispatches by host OS
at runtime, so no render-time OS selection is needed; combined SHA-256
checksum drift detection (`drift.StaleCopilotHooks`); RunSync re-apply gate
(mirrors headroom/engram, unlike team-sync which is per-repo and not
re-applied); snapshot-before-mutate backups; zero user-influenced shell
content in v1 (patterns are frozen constants — no injection surface).

Bundled foundation fix: `copilot.Detect()` now honors `$COPILOT_HOME` when
set (previously hardcoded `~/.copilot`), preventing hook files from landing
in the wrong directory.

---

## Delivery

6 PRs, stacked-to-main strategy (as forecast in tasks.md's Review Workload
Forecast):

| PR | Work units | Scope |
|---|---|---|
| #152 | WU-1+WU-2 | `$COPILOT_HOME`/`HooksDir` (`internal/copilot/copilot.go`) + `CopilotHooksRecord`/`SetCopilotHooks` (`internal/state/state.go`) |
| #154 | WU-3+WU-4 | `internal/copilothooks/` — v1 schema types, `Marshal`, `RenderGuardrails` (bash+powershell, 4 patterns, decision output) |
| #155 | WU-5 | `internal/copilothooks/` — atomic `WriteHookFile`/`RemoveHookFile` + `HookFileChecksum`/`CombinedChecksum` |
| #156 | WU-6+WU-7+WU-8 | `internal/tui/copilothooks.go` orchestration (`applyCopilotHooks`/`disableCopilotHooks`/`backupCopilotHooks`) + `drift.StaleCopilotHooks` + RunSync re-apply gate |
| #157 | WU-9+WU-10 | TUI posture-dropdown screen FSM + goldens + `app.go` menu wiring |
| #158 | WU-11 | README.md + llms.txt docs |

All 22 tasks (T-01..T-22) across all 11 work units are marked `[x]` in the
persisted tasks artifact — task completion gate passed with no stale
checkboxes and no exceptional reconciliation needed.

---

## Verification summary

Adversarial fresh-context verify was run per-PR before each PR was opened
(PR-1 through PR-4). PR-5 (#157) and PR-6 (#158) do not have a corresponding
engram verify-report observation — see "Gaps in traceability" below. Per the
working context for this archive, the gate output at every implemented
slice (gofmt/go vet/go test -race/go build) was green, and all 6 PRs merged
to main; verification is treated as PASS for the full change with 0
CRITICAL outstanding.

| PR | Verdict | CRITICAL | WARNING | SUGGESTION |
|---|---|---|---|---|
| PR-1 (#152) | PASS | 0 | 0 | 2 |
| PR-2 (#154) | PASS | 0 | 1 | 4 |
| PR-3 (#155) | PASS | 0 | 0 | 4 |
| PR-4 (#156) | PASS | 0 | 1 | 2 |
| PR-5 (#157) | not verified in engram (see gap note) | — | — | — |
| PR-6 (#158) | not verified in engram (see gap note) | — | — | — |

PR-2 WARNING (non-blocking, spec-frozen not code defect): P3 (force-push
pattern) under-matches flag-after-branch order and over-matches
hyphen-adjacent branch names in the ORIGINAL single-regex form. Fixed by a
spec + code revision (see "Spec reconciliations" below) before PR-2 merged.

PR-4 WARNING (non-blocking, reconciled at archive): spec REQ-9.4's literal
signature `applyCopilotHooks(..., posture string)` was stale vs the
implemented `applyCopilotHooks(..., rec *state.CopilotHooksRecord)` — the
record-based signature is the design-approved shape (lets apply own
`Presets` defaulting + persistence). Reconciled in the promoted main spec
(see below).

---

## Settled decisions / ADRs (do not reopen)

From the proposal and design, carried through unchanged to the shipped
code:

- Guardrail posture is user-selectable via TUI dropdown: off / warn
  (`permissionDecision=ask`) / strict (`permissionDecision=deny`).
- v1 pattern set is exactly four, frozen: `rm -rf /` (+equivalents),
  `curl|wget ... | sh/bash`, `git push --force` to main/master, `chmod 777`.
  No pattern beyond these four without a separate spec change.
- Matcher is `bash` (verified against real Copilot CLI tool names, not the
  originally inferred `^(?:bash|shell|run_command)$`).
- Repo-level hooks (`.github/hooks`, cloud-agent enforcement) are DEFERRED —
  TUI note only.
- Windows is in scope via an always-rendered `powershell` field in the same
  hook entry — NOT a render-time OS selector (ADR-5, schema-verified).
- File layout: individual JSON file (`capiko-guardrails.json`), whole-file
  atomic ownership, write-to-tmp→rename, disable = `os.Remove`.
- Drift: combined SHA-256 of capiko-owned hook files →
  `CopilotHooksRecord.Checksum`; `drift.StaleCopilotHooks()`.
- `"version":1` pinned in every rendered file.
- No user-influenced shell content interpolated in v1 (`shSingleQuote` is
  the sanitizer of record for a FUTURE user-editable preset, not exercised
  in v1 since there's nothing user-supplied to quote).
- `timeoutSec` capped at 5s. Verified fail-mode: non-zero exit/crash is
  FAIL-CLOSED (deny); exceeding `timeoutSec` is FAIL-OPEN (proceeds as if
  no hook fired) — this is a genuine residual risk, not a bug (see Risks).
- Schema verified against `docs.github.com/en/copilot/reference/hooks-reference`
  + local Copilot CLI v1.0.60 (engram #395): `hooks` is a map keyed by event
  name (`preToolUse`), and a single entry carries BOTH `bash` and
  `powershell` fields — Copilot CLI dispatches by host OS at runtime. This
  corrected an earlier inferred draft that had modeled a flat array with a
  render-time OS-selected `command` field (ADR-2, ADR-5 reversal).
- P3 (force-push) is three AND-ed conditions, not one alternation, because
  `grep -E`/PowerShell `-imatch` have no lookahead — makes flag-vs-branch
  order irrelevant and rejects hyphen-adjacent branch names
  (`release-main`, `mainline`, `main-backup`).

---

## Spec reconciliations made at archive time

Two reconciliations were verified against actual shipped code before
promoting `openspec/specs/copilot-managed-hooks/spec.md`:

### REQ-6.1 — `WriteHookFile` signature

**Was (interim spec draft):** `WriteHookFile(path string, content []byte) error`.
**As-built:** `internal/copilothooks/writer.go` —
`WriteHookFile(hooksDir, name string, data []byte) error`, per design ADR-8
(design line ~400), explicitly referenced by task T-10. Confirmed correct
and design-approved by verify-report-pr3 (engram #401), which called the
spec text "stale, NOT a code defect."
**Fix:** REQ-6.1/REQ-6.4 updated to the two-argument `(hooksDir, name)`
signature in the promoted main spec and this archived spec.

### REQ-9.4 — `applyCopilotHooks` signature

**Was (interim spec draft):** `applyCopilotHooks(host, store, bkp,
posture string) error`.
**As-built:** `internal/tui/copilothooks.go` —
`applyCopilotHooks(host *copilot.Host, store *state.Store, bkp
*backup.Store, rec *state.CopilotHooksRecord) error`. Confirmed
design-approved (record-based signature lets apply own `Presets`
defaulting + persistence) by verify-report-pr4 (engram #402), flagged there
as a non-blocking WARNING pending archive-time reconciliation.
**Fix:** REQ-9.4 updated to the record-based signature in the promoted main
spec and this archived spec.

No other reconciliations were needed — the P3 pattern hardening (three
AND-ed conditions) and the schema correction (object-keyed `hooks`, both
`bash`+`powershell` in one entry) were already applied to the spec and
design DURING planning (engram #393/#394, both explicitly verified against
#395), so they did not require archive-time reconciliation.

---

## Gaps in traceability

- **No engram verify-report for PR-5 (#157, screen+goldens+menu wiring) or
  PR-6 (#158, docs).** Per the working context for this archive: verify was
  PASS-treated because every task (T-17..T-22) is marked `[x]` in the
  persisted tasks artifact, both PRs merged to main, and the mandatory gate
  (`gofmt`/`go vet`/`go test -race`/`go build`) was green at every slice per
  the repo's PR discipline established by PR-1..PR-4's verified pattern.
  This is a traceability gap, not a known defect — if a fresh audit of
  PR-5/PR-6 is desired later, it can be run against the merged commits on
  `main` directly.

---

## Engram observation IDs (full traceability)

| Artifact | Engram ID |
|---|---|
| proposal | #392 |
| spec (schema + P3 hardening correction) | #393 |
| design (schema correction) | #394 |
| tasks | #396 |
| apply-progress | #397 |
| verify-report-pr1 | #398 |
| verify-report-pr2 | #400 |
| verify-report-pr3 | #401 |
| verify-report-pr4 | #402 |
| verify-report-pr5 | none found (gap, see above) |
| verify-report-pr6 | none found (gap, see above) |
| archive-report | (this document, topic: sdd/copilot-managed-hooks/archive-report) |

---

## Main spec promoted

`openspec/specs/copilot-managed-hooks/spec.md` created as the canonical
post-archive spec (no prior main spec existed for this domain — the delta
spec was copied and reconciled directly). The reconciled spec is the
authoritative source of truth for this capability going forward.

---

## Files changed (code, across all 6 PRs)

| File | Change |
|---|---|
| `internal/copilot/copilot.go` | `$COPILOT_HOME` resolution + `Host.HooksDir` |
| `internal/copilot/copilot_test.go` | `$COPILOT_HOME` override/unset cases |
| `internal/state/state.go` | `CopilotHooksRecord` + `State.CopilotHooks` + `SetCopilotHooks` |
| `internal/state/state_test.go` | Round-trip, nil-clear, omitempty backward-compat cases |
| `internal/copilothooks/copilothooks.go` | v1 schema types (`HookFile`/`Hook`), `Marshal` |
| `internal/copilothooks/copilothooks_test.go` | Schema/golden shape tests |
| `internal/copilothooks/render.go` | `RenderGuardrails` — 4 baked-in patterns, bash+powershell |
| `internal/copilothooks/render_test.go` | Table-driven match/non-match × both shells × posture |
| `internal/copilothooks/writer.go` | `WriteHookFile`/`RemoveHookFile`/`HookFileChecksum`/`CombinedChecksum` |
| `internal/copilothooks/writer_test.go` | Atomic write, idempotent remove, path guard, checksum cases |
| `internal/copilothooks/testdata/guardrails_{strict,warn}.json` | Golden rendered files |
| `internal/tui/copilothooks.go` | `applyCopilotHooks`/`disableCopilotHooks`/`backupCopilotHooks` + `copilotHooksScreen` FSM |
| `internal/tui/copilothooks_apply_test.go` | Apply/disable orchestration cases |
| `internal/tui/copilothooks_screen_test.go` | `Update()`-driven screen FSM cases |
| `internal/tui/sync.go` | RunSync re-apply gate |
| `internal/tui/sync_test.go` | RunSync gate cases (enabled re-applies, disabled/nil skips) |
| `internal/drift/drift.go` | `StaleCopilotHooks` |
| `internal/drift/drift_test.go` | Drift detection cases |
| `internal/tui/app.go` | "Configure Copilot hooks" menu item + `open()` dispatch |
| `internal/tui/app_test.go` | `TestEnterOpensCopilotHooks` |
| `internal/tui/testdata/copilothooks_{editing_off,editing_warn,editing_strict,done,failed}.golden` | Screen goldens |
| `internal/tui/testdata/menu*.golden` | Main-menu goldens updated (new line) |
| `README.md` | Menu table entry + "Copilot hooks (guardrails)" section |
| `llms.txt` | Copilot hooks bullet + docs link |

---

## Deferred follow-ups (NOT implemented — future work)

These items were intentionally out of scope for J-1/J-2 and are recorded
here for future planning:

1. **Repo-level `.github/hooks` enforcement (cloud-agent hooks).** Deferred
   per the proposal's non-goals. The TUI displays a note only. Would need
   its own spec covering repo-level file ownership, a different trust
   boundary (shared repo vs. user-level), and cloud-agent-specific schema
   considerations.

2. **A `doctor` copilot-hooks check + hook profiles.** `drift.StaleCopilotHooks`
   exists and is wired into RunSync, but there is no standalone `doctor`
   command surfacing drift status outside of sync, and no concept of
   multiple named "profiles" beyond the single `guardrails` preset. The
   `Presets []string` field on `CopilotHooksRecord` (ADR-6/ADR-7) was
   deliberately added now so a future multi-preset/profile slice does not
   require a state schema migration.

3. **PowerShell runtime parity untested on CI.** Every rendered hook entry
   carries a PowerShell script unconditionally (ADR-5), and it is
   structurally verified (regex syntax, decision JSON, `-imatch` presence)
   but never executed — there is no Windows runner in CI. Golden tests
   cover rendering only. A manual Windows check (or a future Windows CI
   job) is needed to confirm runtime behavior matches the bash variant.

4. **`jq`-based payload extraction.** v1 greps the raw stdin JSON payload
   for pattern matches (ADR-3) rather than parsing and extracting the
   command field specifically, to avoid a hard `jq` dependency. This is
   documented as a low-risk accepted tradeoff (JSON escaping could
   theoretically mask a pattern, but the four patterns match on tokens that
   are not JSON-escaped). A `jq`-based extraction is optional future
   hardening, not a v1 requirement.

5. **v2 pattern hardening (already flagged in the promoted spec, REQ-3.4).**
   `rm -rf --no-preserve-root`/quoted-root, `chmod` sticky/setuid
   (1777/4777), and `curl|sh` echo over-match are explicitly deferred to a
   future pattern-spec revision — not fixed in this pass, since REQ-3.4
   freezes the four v1 patterns.

---

## SDD cycle complete

Change `copilot-managed-hooks` (J-1/J-2): planned → implemented (6 chained
PRs, stacked-to-main) → verified (PR-1..PR-4 adversarial fresh-context
verify, PR-5/PR-6 gap noted above) → archived.
