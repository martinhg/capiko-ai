# Spec: Copilot managed hooks (guardrails) — J-1/J-2

**Change:** `copilot-managed-hooks`
**Affected packages:** `internal/copilot` (modified), `internal/copilothooks` (new),
`internal/state/state.go` (modified), `internal/tui/copilothooks.go` (new),
`internal/tui/sync.go` (modified), `internal/tui/app.go` (modified),
`internal/drift/drift.go` (modified)

---

## What this change delivers

After this change, capiko manages a new **user-level** GitHub Copilot CLI hooks
feature: a capiko-owned JSON file at `$COPILOT_HOME/hooks/capiko-guardrails.json`
(default `~/.copilot/hooks/`) that registers a `preToolUse` hook. The hook
deterministically intercepts four high-signal dangerous shell command patterns
before they run, and responds with a Copilot CLI `permissionDecision` that the
user selects via a TUI posture dropdown: `off` (feature disabled, no file),
`warn` (`permissionDecision=ask`), or `strict` (`permissionDecision=deny`).
This is capiko's first HARD enforcement lever — unlike instruction/skill/MCP
guidance, the CLI cannot ignore a `deny` decision.

The feature follows capiko's established managed-feature pattern: a `*Record` in
state, `SetXxx`/apply/disable functions, a RunSync re-apply gate, a TUI screen,
snapshot-before-mutate, and checksum-based drift detection.

---

## Non-negotiable invariants

| Invariant | Rule |
|---|---|
| Off by default | `CopilotHooks` is nil in state until the user explicitly enables and applies |
| Whole-file ownership | capiko owns `capiko-guardrails.json` in full; it NEVER edits or removes any other file in the hooks directory |
| User-level only | Hooks are written under `$COPILOT_HOME/hooks/` only; no repo-level (`.github/hooks/`) file is ever written by this change |
| Minimal pattern set | Exactly the four v1 patterns (REQ-3) are matched; no additional patterns are added without a spec change |
| Atomic write | The hook file is written via write-to-temp-file-then-rename; a failed write never leaves a partial/corrupt file |
| Snapshot-before-mutate | An existing `capiko-guardrails.json` is backed up via `backup.Store` before any write or removal |
| No injected free text | v1 pattern messages and commands are fixed strings authored by capiko; no user-supplied value is interpolated into the rendered shell/PowerShell script |
| Version pin | Every rendered file has top-level `"version": 1` |

---

## REQ-1 — `$COPILOT_HOME` resolution and `HooksDir`

**REQ-1.1** `copilot.Detect()` MUST resolve the Copilot config directory as
`$COPILOT_HOME` when that environment variable is set and non-empty, and MUST
fall back to `filepath.Join(home, ".copilot")` when it is unset or empty. This
replaces the current hardcoded `~/.copilot` (copilot.go:39–58).

**REQ-1.2** `copilot.Host` gains a `HooksDir string` field, set to
`filepath.Join(cfg, "hooks")` where `cfg` is the resolved config directory from
REQ-1.1.

**REQ-1.3** `HooksDir` is never created by `Detect()` itself. Directory creation
is the responsibility of the writer (REQ-6.2).

---

## REQ-2 — v1 hook file schema

**REQ-2.1** A rendered hook file is a single JSON object with exactly this
top-level shape. `hooks` is an OBJECT keyed by event name — the event name is
the KEY, not a field inside the entry — and a single command entry carries
BOTH a `bash` and a `powershell` script (verified against the real Copilot CLI
hooks-reference schema, see REQ-5):

```json
{
  "version": 1,
  "hooks": {
    "preToolUse": [
      {
        "type": "command",
        "matcher": "bash",
        "bash": "<bash guardrail script, see REQ-5>",
        "powershell": "<PowerShell guardrail script, see REQ-5>",
        "timeoutSec": 5
      }
    ]
  }
}
```

**REQ-2.2** `matcher` is exactly `bash` for the guardrails preset (Copilot
compiles the matcher anchored internally, so no `^(?:...)$` wrapping is
authored here). The tokens `shell`/`run_command` used in an earlier draft of
this spec are UNVERIFIED against real Copilot CLI tool names and are dropped;
only the confirmed `bash` tool is matched in v1.

**REQ-2.3** `timeoutSec` is exactly `5`. The guardrail script MUST complete
within this budget since it runs on every matched shell tool call.
**Fail-mode note (verified):** a non-zero exit or crash from the script is
FAIL-CLOSED (Copilot denies), but exceeding `timeoutSec` is FAIL-OPEN (Copilot
proceeds with its normal permission flow as if the hook were absent). The
guardrail script MUST stay trivial and fast — a slow or hung script does not
deny, it silently lets the command through.

**REQ-2.4** `version` is exactly the integer `1` (not a string). A future
schema change MUST bump this value and add a migration path to
`CopilotHooksRecord` rather than silently changing field shapes under the same
version number.

---

## REQ-3 — Guardrail pattern set (v1, minimal/high-signal)

**REQ-3.1** The guardrail script checks the invoked command string (as
supplied by Copilot CLI's `preToolUse` hook input for the matched tool call)
against exactly these four patterns, in order, and stops at the first match:

| # | Intent | Pattern (RE2/POSIX-ERE compatible, case-insensitive) | Reason text |
|---|---|---|---|
| P1 | Recursive force-delete of root | `(sudo\s+)?rm\s+(-\S*r\S*f\S*\|-\S*f\S*r\S*\|--recursive\s+.*--force\|--force\s+.*--recursive)\s+/+\*?\s*($\|[;&\|])` | "Blocked: recursive force-delete targeting root (/) is destructive and irreversible." |
| P2 | Pipe-to-shell | `\b(curl\|wget)\b[^\|]*\|\s*(sudo\s+)?(sh\|bash\|zsh)\b` | "Blocked: piping a remote script directly into a shell interpreter bypasses review." |
| P3 | Force-push to protected branch | `\bgit\s+push\s+(--force(-with-lease)?\|-f)\b.*\b(origin\s+)?(main\|master)\b` | "Blocked: force-pushing to a protected branch (main/master) can overwrite team history." |
| P4 | World-writable permissions | `\bchmod\s+(-R\s+)?0?777\b` | "Blocked: chmod 777 grants world-writable permissions, a common security misconfiguration." |

**REQ-3.2** Match examples (MUST match): `rm -rf /`, `rm -fr /`, `sudo rm -rf /`,
`rm -rf /*`; `curl https://x/install.sh | sh`, `wget -O- https://x | bash`;
`git push --force origin main`, `git push -f master`; `chmod 777 file`,
`chmod -R 777 .`, `chmod 0777 script.sh`.

**REQ-3.3** Non-match examples (MUST NOT match, low false-positive): `rm -rf
./build`, `rm -rf /tmp/foo`; `curl https://x -o file.sh`; `git push --force
origin feature/foo`, `git push origin main` (no force flag); `chmod 755 file`.

**REQ-3.4** No pattern beyond these four is added in v1. Extending the set is
a separate spec change.

---

## REQ-4 — `permissionDecision` output contract per posture

**REQ-4.1** When posture is `strict` and the command matches one of the four
patterns (REQ-3), the script prints exactly one JSON object to stdout and exits
`0`:

```json
{"permissionDecision": "deny", "permissionDecisionReason": "<reason text from REQ-3.1>"}
```

**REQ-4.2** When posture is `warn` and the command matches, the script prints:

```json
{"permissionDecision": "ask", "permissionDecisionReason": "<reason text from REQ-3.1>"}
```

**REQ-4.3** When posture is `strict` or `warn` and the command does NOT match
any of the four patterns, the script prints nothing to stdout and exits `0`
(Copilot CLI applies its default behavior — the command runs).

**REQ-4.4** When posture is `off`, no hook file exists at all (REQ-6.4) — the
guardrail preset is fully absent, not a no-op file. There is no
`permissionDecision` output because the hook is never registered.

**REQ-4.5** `permissionDecision` is one of exactly `"deny"` or `"ask"` in
rendered output. The value `"allow"` is never emitted; allow is expressed by
silence (REQ-4.3).

---

## REQ-5 — Cross-platform command rendering

**REQ-5.1** `RenderGuardrails(posture Posture) (string, error)` (or
equivalent package-level function) renders a SINGLE hook entry that carries
BOTH a `bash` field (bash-syntax inline script) and a `powershell` field
(PowerShell-syntax inline script) for the given posture. There is no OS
discriminator input and no render-time OS selection: Copilot CLI itself picks
`bash` or `powershell` at runtime based on the host OS, so the renderer always
emits both fields in the same entry.

**REQ-5.2** Both variants implement the same four-pattern checks (REQ-3) and
the same `permissionDecision` output contract (REQ-4) for a given posture.
Regex syntax differs by shell (POSIX ERE for bash, .NET regex for PowerShell)
but the match/non-match behavior for the examples in REQ-3.2/3.3 is identical
across both.

**REQ-5.3** `RenderGuardrails` takes no OS/`goos` parameter and has no
per-platform error path; a `goos`-style seam is unnecessary because both
scripts are always rendered together in one entry.

---

## REQ-6 — Atomic write, disable, and directory handling

**REQ-6.1** `WriteHookFile(path string, content []byte) error` writes via
write-to-a-temporary-file-in-the-same-directory followed by an atomic rename
over `path`. A write failure never leaves a partially-written file at `path`.

**REQ-6.2** When `HooksDir` (REQ-1.2) does not exist, `WriteHookFile` (or its
caller) creates it via `os.MkdirAll` before writing. Any other filesystem error
is returned unchanged; no file is written.

**REQ-6.3** Before any write to an existing `capiko-guardrails.json`,
`backup.Store.CreateFiles` snapshots the current file content. If the backup
fails, the write is aborted and the error is returned.

**REQ-6.4** `RemoveHookFile(path string) error` (disable) removes
`capiko-guardrails.json` via `os.Remove`. Removing a file that does not exist
is a no-op (idempotent), not an error. No other file in `HooksDir` is ever
touched by write or remove.

**REQ-6.5** Neither `WriteHookFile` nor `RemoveHookFile` inspects, modifies, or
deletes any file in `HooksDir` other than `capiko-guardrails.json`. A
pre-existing, non-capiko-owned file in the same directory (e.g. a hand-authored
`my-hooks.json`) is left byte-for-byte unchanged by any apply or disable call.

---

## REQ-7 — Checksum and drift detection

**REQ-7.1** `HookFileChecksum(path string) (string, bool)` returns the SHA-256
hex digest of the current on-disk content of `capiko-guardrails.json`, and
`false` when the file is absent (mirrors `engram.CLIEntryChecksum`).

**REQ-7.2** `CopilotHooksRecord.Checksum` stores the combined SHA-256 of all
capiko-owned hook files under `HooksDir` at the time of the last successful
apply. For v1 (one preset, one file) this is exactly the checksum of
`capiko-guardrails.json`.

**REQ-7.3** `drift.StaleCopilotHooks(hooksDir string, st *state.State) bool`
returns `false` when `st.CopilotHooks` is nil or `st.CopilotHooks.Enabled` is
false. Otherwise it returns `true` when the file is missing or its current
checksum differs from `st.CopilotHooks.Checksum` (mirrors `StaleHeadroom`,
drift.go:30–43).

---

## REQ-8 — State persistence

**REQ-8.1** `state.State` gains `CopilotHooks *CopilotHooksRecord` tagged
`json:"copilot_hooks,omitempty"`. A nil value means unmanaged.

**REQ-8.2** `CopilotHooksRecord` has at least:

| Field | Type | Purpose |
|---|---|---|
| `Enabled` | `bool` | Whether the guardrails preset is active |
| `Posture` | `string` | One of `"off"`, `"warn"`, `"strict"` |
| `Checksum` | `string` | Combined SHA-256 per REQ-7.2 |
| `Presets` | `[]string` | Enabled preset names, e.g. `["guardrails"]` |

**REQ-8.3** `Store.SetCopilotHooks(rec *CopilotHooksRecord) error` mirrors
`SetTeamSync`/`SetCodeReview`: loads state, sets `st.CopilotHooks = rec`, stamps
`UpdatedAt`, saves. A nil argument clears the field (unmanaged).

**REQ-8.4** Setting `Posture` to `"off"` is equivalent to disabling: it MUST
result in `Enabled: false`, no hook file present (REQ-4.4, REQ-6.4), and
`Checksum` cleared to `""`.

---

## REQ-9 — TUI screen and menu wiring

**REQ-9.1** A new file `internal/tui/copilothooks.go` defines a screen struct
implementing the `screen` interface (`Update(tea.Msg) (screen, tea.Cmd)` +
`View() string`).

**REQ-9.2** The screen presents a posture selector with exactly three options
— `off`, `warn`, `strict` — plus an `Apply` row and a `Back` row (returns via
`backMsg`).

**REQ-9.3** The screen displays a note stating that this feature manages
user-level Copilot CLI hooks only, and that repo-level/cloud-agent hook
enforcement is a future feature not covered by this screen.

**REQ-9.4** `applyCopilotHooks(host *copilot.Host, store *state.Store, bkp
*backup.Store, posture string) error` is a package-level function that: for
`posture != "off"`, backs up any existing file (REQ-6.3), renders the
guardrail script (REQ-5) for the given posture (REQ-4), writes it atomically (REQ-6.1),
computes and stores the checksum (REQ-7.2), and calls `SetCopilotHooks`; for
`posture == "off"`, delegates to `disableCopilotHooks`.

**REQ-9.5** `disableCopilotHooks(host *copilot.Host, store *state.Store, bkp
*backup.Store) error` backs up the existing file if present, removes it
(REQ-6.4), and calls `SetCopilotHooks(&CopilotHooksRecord{Enabled: false,
Posture: "off"})`.

**REQ-9.6** A "Configure Copilot hooks" entry is added to `menuItems` in
`internal/tui/app.go` with `id: "copilot-hooks"` and `ready: true`, positioned
after "Configure team sync". `open()` handles `it.id == "copilot-hooks"` by
setting `a.active = newCopilotHooks(a.svc)`.

**REQ-9.7** `newCopilotHooks(svc services) screen` reads `svc.state` on
construction to pre-select the current `Posture` (default `"off"` when
`CopilotHooks` is nil).

---

## REQ-10 — RunSync re-apply gate

**REQ-10.1** `RunSync` (sync.go) re-applies the guardrails hook only when
`st.CopilotHooks != nil && st.CopilotHooks.Enabled`, calling
`applyCopilotHooks` with the stored `Posture` — mirroring the headroom/engram
re-apply gates (sync.go:92–113).

**REQ-10.2** When `st.CopilotHooks` is nil or `Enabled` is false, `RunSync`
does not touch the hooks directory.

---

## REQ-11 — Tests

**REQ-11.1** `internal/copilothooks` has table-driven tests covering: schema
rendering shape (REQ-2), all four pattern match/non-match examples (REQ-3.2,
REQ-3.3) for both bash and PowerShell variants, `permissionDecision` output for
`warn` and `strict` (REQ-4.1, REQ-4.2), silent-allow for non-matching commands
(REQ-4.3), atomic write behavior, `RemoveHookFile` idempotency, and checksum
computation. All tests use `t.TempDir()`.

**REQ-11.2** TUI tests for `copilothooks.go` cover: apply for each posture
writes/removes the file correctly and updates state; a pre-existing
non-capiko file in the hooks directory is untouched after apply/disable
(REQ-6.5); posture downgrade (`strict`→`warn`→`off`) at each step leaves state
and file consistent with REQ-4/REQ-8.4; `$COPILOT_HOME` override changes the
write target directory (REQ-1.1).

**REQ-11.3** At least one golden file in `internal/tui/testdata/*.golden`
covers the Configure Copilot hooks screen in its default state.

**REQ-11.4** `go test -race ./...` passes with all new tests included. No test
invokes the real `copilot` binary or writes outside `t.TempDir()`.

---

## Acceptance scenarios

### SC-01: `$COPILOT_HOME` override changes `HooksDir` (REQ-1.1, REQ-1.2)

```
Given   COPILOT_HOME is set to /custom/copilot-home
When    copilot.Detect() runs
Then    Host.ConfigDir == /custom/copilot-home
And     Host.HooksDir == /custom/copilot-home/hooks
```

### SC-02: `$COPILOT_HOME` unset falls back to default (REQ-1.1)

```
Given   COPILOT_HOME is unset
When    copilot.Detect() runs
Then    Host.HooksDir == <home>/.copilot/hooks
```

### SC-03: Apply with strict posture denies a matching command (REQ-4.1)

```
Given   applyCopilotHooks is called with posture "strict"
When    the rendered hook script is invoked with command "rm -rf /"
Then    stdout is exactly {"permissionDecision":"deny","permissionDecisionReason":"..."}
And     the process exits 0
```

### SC-04: Apply with warn posture asks instead of denying (REQ-4.2)

```
Given   applyCopilotHooks is called with posture "warn"
When    the rendered hook script is invoked with command "chmod -R 777 ."
Then    stdout is exactly {"permissionDecision":"ask","permissionDecisionReason":"..."}
```

### SC-05: Non-matching command produces no decision (REQ-4.3)

```
Given   posture is "strict" or "warn"
When    the rendered hook script is invoked with command "rm -rf ./build"
Then    stdout is empty
And     the process exits 0
```

### SC-06: Off posture — no file written, feature disabled (REQ-4.4, REQ-9.5)

```
Given   CopilotHooks was previously enabled with posture "strict"
When    the user selects posture "off" and presses Apply
Then    capiko-guardrails.json no longer exists in HooksDir
And     state.CopilotHooks.Enabled == false
And     state.CopilotHooks.Posture == "off"
And     state.CopilotHooks.Checksum == ""
```

### SC-07: Disable removes only the capiko-owned file (REQ-6.4, REQ-6.5)

```
Given   HooksDir contains capiko-guardrails.json and a hand-authored other-hook.json
When    disableCopilotHooks is called
Then    capiko-guardrails.json no longer exists
And     other-hook.json is unchanged (same bytes, same mtime is not asserted, content is)
```

### SC-08: Drift detected on hand-edited file (REQ-7.3)

```
Given   CopilotHooks.Enabled is true with a recorded Checksum
And     the on-disk capiko-guardrails.json is hand-edited after apply
When    drift.StaleCopilotHooks(hooksDir, st) is called
Then    it returns true
```

### SC-09: No drift when file matches recorded checksum (REQ-7.3)

```
Given   CopilotHooks.Enabled is true
And     the on-disk file is unchanged since the last apply
When    drift.StaleCopilotHooks(hooksDir, st) is called
Then    it returns false
```

### SC-10: RunSync re-applies an enabled guardrail (REQ-10.1)

```
Given   state.CopilotHooks == {Enabled: true, Posture: "strict", ...}
When    RunSync executes
Then    applyCopilotHooks is called with posture "strict"
And     capiko-guardrails.json reflects the current renderer output
```

### SC-11: RunSync does not touch hooks when unmanaged (REQ-10.2)

```
Given   state.CopilotHooks is nil
When    RunSync executes
Then    no file in HooksDir is created, modified, or removed
```

### SC-12: Posture downgrade strict → warn → off (REQ-4, REQ-8.4)

```
Given   posture is "strict" and applied
When    the user changes posture to "warn" and applies
Then    the file content changes to the warn-posture script (REQ-4.2)
And     state.CopilotHooks.Posture == "warn"
When    the user then changes posture to "off" and applies
Then    the file no longer exists
And     state.CopilotHooks.Enabled == false
```

### SC-13: Rendered entry carries both bash and powershell scripts (REQ-5.1)

```
Given   RenderGuardrails is called for a given posture
When    the rendered hook entry is inspected
Then    the entry has a non-empty "bash" field with a bash-syntax script
And     the entry has a non-empty "powershell" field with a PowerShell-syntax
        script
And     both implement the same four patterns (REQ-3) and decision contract
        (REQ-4) for that posture
And     no OS discriminator was passed to produce this output
```

### SC-14: Missing hooks directory is created on first apply (REQ-6.2)

```
Given   HooksDir does not exist on disk
When    applyCopilotHooks is called with posture "strict"
Then    HooksDir is created
And     capiko-guardrails.json is written inside it
```

### SC-15: Snapshot-before-mutate on re-apply (REQ-6.3)

```
Given   capiko-guardrails.json already exists
When    applyCopilotHooks is called again (e.g. posture change)
Then    backup.Store.CreateFiles is called with the existing file path
        BEFORE the new content is written
And     if CreateFiles returns an error, the write does not proceed and the
        error is returned
```

### SC-16: Menu item wired (REQ-9.6)

```
Given   the App menu is rendered
When    the menu items are inspected
Then    a "Configure Copilot hooks" item with id "copilot-hooks" and
        ready: true is present, after "Configure team sync"
And     pressing enter on that item sets a.active to the Copilot hooks screen
```

### SC-17: Golden renders default screen (REQ-11.3)

```
Given   newCopilotHooks(svc) is called with svc.state.CopilotHooks == nil
When    View() is called
Then    the output matches internal/tui/testdata/copilothooks.golden
And     the posture selector defaults to "off"
And     the repo-level/cloud-agent future-feature note is visible
```

### SC-18: False-positive check — safe commands are not blocked (REQ-3.3)

```
Given   posture is "strict"
When    the rendered hook script is invoked with "rm -rf ./build",
        "curl https://x -o file.sh", "git push --force origin feature/foo",
        and "chmod 755 file"
Then    stdout is empty for every case (no permissionDecision emitted)
```

---

## Out of scope

- Repo-level hooks (`.github/hooks/`) and cloud-agent enforcement — future slice
- Additional guardrail patterns beyond the four v1 high-signal ones
- `postToolUse`, `sessionStart`, or any event other than `preToolUse`
- Learn-loop / memory-injection hooks (blocked upstream by `additionalContext`
  bugs #2142/#2980)
- Native Copilot plugin management
- Inline hooks in `settings.json`
- Admin/org policy hook interaction — capiko cannot override or detect
  `/etc/github-copilot/policy.d/` precedence
