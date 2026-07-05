# Design: Copilot managed hooks (guardrails) — J-1 + J-2

Architectural decisions for capiko's first HARD enforcement lever: capiko-managed
GitHub Copilot CLI hook files under `$COPILOT_HOME/hooks/*.json`. The proposal's
product decisions (`sdd/copilot-managed-hooks/proposal`) are authoritative; this
document settles the HOW and grounds each decision in the existing codebase. All
anchors below were re-read against the current tree and confirmed accurate.

## Architecture approach

Mirror the established "managed feature" pattern used by headroom/engram/team-sync
(Record + `SetXxx` + apply/disable + RunSync gate + TUI screen + snapshot-before-
mutate). Copilot hook files are discrete whole JSON files, so the file layer follows
the **whole-file atomic ownership** sub-pattern (`state.Store.Save`,
`instructions.Write`, `engram.atomicWrite`), NOT the shell-marker sub-pattern
(`internal/githooks.WriteBlock`). `internal/githooks` is neither reused nor touched.

Two layers with a hard boundary:

- **`internal/copilothooks/`** (NEW) — mechanism + schema only. Owns the v1 hook
  schema types, the guardrails renderer (which embeds both `bash` and `powershell`
  script bodies in one entry — Copilot selects at runtime, ADR-5), atomic whole-file
  write/remove, and checksum. Knows nothing about the TUI, `state.Store` orchestration,
  or backups. Fully unit-testable with `t.TempDir()`.
- **`internal/tui/copilothooks.go`** (NEW) — policy + UX. `applyCopilotHooks` /
  `disableCopilotHooks` / `backupCopilotHooks` and the posture-dropdown Bubbletea
  screen. Orchestrates backup → write → record, exactly like `applyHeadroom`.

```
tui/copilothooks.go (policy/UX)                 copilothooks (mechanism/schema)
  applyCopilotHooks(host, store, bkp, rec) ────▶ RenderGuardrails(posture) → HookFile
  disableCopilotHooks(host, store, bkp) ───────▶ Marshal(HookFile) → []byte
  backupCopilotHooks(bkp, hooksDir) ───────────▶ WriteHookFile(dir, name, data)  (atomic)
  screen: posture dropdown FSM                   RemoveHookFile(dir, name)        (idempotent)
  state.SetCopilotHooks(rec)                     CombinedChecksum(dir) / HookFileChecksum(p)
```

Boundary rule: everything Copilot-, OS-, and JSON-schema-aware lives in
`copilothooks`; everything backup-, state-, and screen-aware lives in `tui`. `drift`
depends only on `copilothooks.CombinedChecksum` + `state`. No import cycles:
`copilothooks → state` (leaf), `drift → copilothooks → state`, `tui → copilothooks`.

## Component / data-flow map

| Component | File (anchor) | Change |
| --- | --- | --- |
| `copilot.Host.HooksDir` + `$COPILOT_HOME` | `internal/copilot/copilot.go:20-26`, `39-58` | Modified |
| `state.CopilotHooksRecord` + `State` field + `SetCopilotHooks` | `internal/state/state.go:18-55`, `100-105`, `358-369` | Modified |
| `internal/copilothooks/` schema+renderer+writer | new package | New |
| `applyCopilotHooks`/`disableCopilotHooks`/screen | `internal/tui/copilothooks.go` | New |
| RunSync re-apply gate | `internal/tui/sync.go:107-113` | Modified |
| `drift.StaleCopilotHooks` | `internal/drift/drift.go:30-43` | Modified |
| Menu item + `open()` case | `internal/tui/app.go:65-80`, `225-262` | Modified |
| Goldens | `internal/tui/testdata/*.golden`, `internal/copilothooks/testdata/*.json` | New |

Apply data flow (enable, posture=strict):
`screen.applyCmd → applyCopilotHooks(host, store, bkp, rec) → RenderGuardrails(strict)
→ Marshal → (if changed) backupCopilotHooks + WriteHookFile → rec.Checksum =
CombinedChecksum(HooksDir); rec.Posture=strict; rec.Presets=["guardrails"] →
store.SetCopilotHooks(rec)`.

Disable / posture=off flow:
`applyCopilotHooks sees posture off or !Enabled → disableCopilotHooks →
backupCopilotHooks + RemoveHookFile → store.SetCopilotHooks({Enabled:false,
Posture:"off"})`.

Drift: `StaleCopilotHooks(HooksDir, st) = st.CopilotHooks.Enabled &&
CombinedChecksum(HooksDir) != st.CopilotHooks.Checksum`.

---

## ADR-1 — `$COPILOT_HOME` resolution and `HooksDir` on `copilot.Host`

**Decision.** Add `HooksDir string` to `Host` and resolve the config root through a
`$COPILOT_HOME` override in `Detect()`. Introduce a package const and read the env
var directly (test via `t.Setenv`, consistent with the existing `userHomeDir`/
`lookPath` seams — no new seam required).

```go
const copilotHomeEnv = "COPILOT_HOME"

type Host struct {
    BinPath       string
    ConfigDir     string // $COPILOT_HOME or ~/.copilot
    SkillsDir     string
    AgentsDir     string
    MCPConfigPath string
    HooksDir      string // <ConfigDir>/hooks
}

// in Detect(), replacing the hardcoded cfg := filepath.Join(home, ".copilot"):
cfg := os.Getenv(copilotHomeEnv)
if cfg == "" {
    home, err := userHomeDir()
    if err != nil {
        return nil, err
    }
    cfg = filepath.Join(home, ".copilot")
}
// existing os.Stat(cfg) gate is unchanged; add:
//   HooksDir: filepath.Join(cfg, "hooks"),
```

`userHomeDir()` is only called when the env var is empty, preserving the current
"no home dir → error" contract for the default path. The `os.Stat(cfg)` "installed
but never logged in → (nil,nil)" gate is untouched and now correctly honors
`$COPILOT_HOME`.

**Why bundle this bug fix here (G-CC3).** Without it, hook files land in `~/.copilot/hooks`
for users whose real config dir is `$COPILOT_HOME` — the feature would silently write
to a directory Copilot never reads. It is a prerequisite, not scope creep.

**Rejected alternatives.**
- *A dedicated `copilotHome` function seam.* Rejected: `os.Getenv` + `t.Setenv`
  already gives hermetic tests; a seam adds surface for no benefit.
- *Expand `~` inside `$COPILOT_HOME`.* Rejected: Copilot itself does not; we treat the
  env value verbatim to match host behavior.

---

## ADR-2 — v1 hook schema types (`{"version":1,"hooks":{"preToolUse":[...]}}`)

**Decision.** Model the settled schema as flat Go structs so `encoding/json`
round-trips deterministically for goldens. `Hooks` is a `map[string][]Hook` keyed by
EVENT NAME — the event is the map key, not a field on `Hook` — per the verified
Copilot CLI hooks-reference schema (engram `reference/copilot-hook-config-schema`,
#395).

```go
type Posture string

const (
    PostureOff    Posture = "off"
    PostureWarn   Posture = "warn"   // permissionDecision = "ask"
    PostureStrict Posture = "strict" // permissionDecision = "deny"
)

const (
    GuardrailsFile  = "capiko-guardrails.json"
    FilePrefix      = "capiko-" // ownership marker: capiko only ever touches capiko-*.json
    SchemaVersion   = 1
    TimeoutSeconds  = 5
    MatcherBash     = "bash" // confirmed tool name; Copilot anchors this internally
    EventPreToolUse = "preToolUse"
)

type HookFile struct {
    Version int               `json:"version"`
    Hooks   map[string][]Hook `json:"hooks"` // keyed by event name, e.g. "preToolUse"
}

type Hook struct {
    Type       string `json:"type"`       // "command"
    Matcher    string `json:"matcher"`    // MatcherBash
    Bash       string `json:"bash"`       // bash-syntax script (ADR-5)
    PowerShell string `json:"powershell"` // PowerShell-syntax script (ADR-5)
    TimeoutSec int    `json:"timeoutSec"` // TimeoutSeconds (cap 5s)
}
```

`Marshal` uses `json.MarshalIndent(f, "", "  ")` plus a trailing newline for a stable,
golden-friendly byte sequence. `Version` is pinned to `SchemaVersion` (1) by the
renderer; it is a struct field (not derived) so a future schema migration is a one-line
change plus a new golden. `Hooks["preToolUse"]` holds exactly one entry in v1 (the
guardrails preset); a future multi-preset slice appends more entries to that same slice
rather than restructuring the map.

**Verified against the live contract (was flagged as a risk, now resolved).** The
project's earlier draft of this ADR modeled `hooks` as a flat array with an `event`
field per entry and a single OS-selected `command` field, and explicitly REJECTED the
`hooks: {preToolUse: [...]}` shape as a "Claude-Code-style" alternative. That was
wrong: `docs.github.com/en/copilot/reference/hooks-reference` plus local Copilot CLI
v1.0.60 confirm the object-keyed shape above, and confirm a single entry carries BOTH
`bash` and `powershell` fields rather than one OS-selected `command` field. See engram
`reference/copilot-hook-config-schema` (#395) for the full verification note.

**Rejected alternative.** *Flat `"hooks":[{event:"preToolUse", ...}]` array (this
project's original, inferred draft).* Rejected now that the object-keyed shape is
verified against the real schema — an array with an inline `event` field is not what
Copilot CLI reads.

---

## ADR-3 — Guardrail script strategy: grep-stdin, no `jq`

**Decision.** The command-type hook reads the tool-call payload from **stdin**, greps
it for a single baked-in alternation regex, and — on a match — prints a
`permissionDecision` JSON object on stdout; otherwise it stays silent and exits 0
(allow). No `jq`, no JSON parsing, no external dependency beyond `grep`/`bash` (or the
PowerShell equivalent).

bash script body (posture interpolated as the literal `ask` or `deny`):
```sh
input="$(cat)"
if printf '%s' "$input" | grep -Eiq 'PATTERN_ALTERNATION'; then
  printf '{"permissionDecision":"DECISION","permissionDecisionReason":"capiko guardrail: dangerous command blocked (REASON)"}'
fi
exit 0
```

**Why grep the whole stdin payload rather than extract the command field.** Extracting
a specific JSON field portably without `jq` is fragile across shells and Copilot
payload shapes. For a deny/ask guardrail, matching the raw `preToolUse` payload is
sufficient and robust: the payload for a shell tool call is dominated by the command
string, and a false-positive match only ever ASKS or DENIES for that invocation (never
silently allows). This is orthogonal to the SEPARATE, verified fail-open behavior of
`timeoutSec` (engram #395): a non-zero exit or crash from the script is fail-CLOSED
(Copilot denies), but the script exceeding the `timeoutSec` budget is fail-OPEN
(Copilot proceeds as if no hook fired). The 5s cap bounds *intended* latency, but if the
script actually hangs past that budget the command is NOT blocked — keeping the script
trivial (grep on stdin, no external calls, no network) is what keeps that risk low, not
the timeout value itself.

**Accepted tradeoff.** JSON escaping in the payload (e.g. `\/`, `\"`) can theoretically
mask a pattern; the four patterns match on tokens (`rm`, `curl`, `chmod`, `git push
--force`) that are not JSON-escaped, so this is low-risk for v1. Documented as a residual
risk; a `jq`-based extraction is a future hardening, not a v1 requirement.

**Rejected alternative.** *Parse stdin with `jq` and match only the command field.*
Rejected: introduces a hard `jq` dependency capiko cannot guarantee is installed, and a
missing `jq` would make the hook error rather than fail-safe.

---

## ADR-4 — Baked-in patterns; zero user-influenced shell in v1

**Decision.** The four high-signal patterns are compile-time constants in
`copilothooks`, assembled into one extended-regex alternation. They are NOT user-editable
in v1. The ONLY user choice is `Posture`, a closed enum mapped to the literal strings
`ask` / `deny`. Consequently **no user-supplied text is interpolated into the hook
script in v1** — the shell-injection surface is empty.

Representative baked-in alternation (exact escaping finalized in implementation; the SET
is frozen for v1):
```
(\brm\b[[:space:]]+(-[^[:space:]]*[rf][^[:space:]]*[[:space:]]+)+/([[:space:]]|$))   # rm -rf / (+ equivalents)
|((curl|wget)[^|]*\|[[:space:]]*(sh|bash))                                            # curl … | sh (pipe-to-shell)
|(git[[:space:]]+push[[:space:]]+.*--force([[:space:]]|=).*[[:space:]](main|master)) # git push --force to main/master
|(chmod[[:space:]]+([^[:space:]]+[[:space:]]+)*777)                                   # chmod 777
```

**On `shSingleQuote` (`internal/tui/teamsync.go:97-99`).** It is the sanitizer of record
for ANY future preset that embeds user-supplied content (e.g. a custom-pattern preset)
into a shell script. In v1 it is intentionally NOT exercised because there is nothing
user-influenced to quote — the honest position, rather than pretending to sanitize a
constant. The moment a user-editable field reaches a rendered script, it MUST pass
through `shSingleQuote` (POSIX single-quote escaping neutralizes `$`, backtick, `\`, `;`,
whitespace). The PowerShell branch will need its own quoting helper at that point.

**Why frozen patterns in v1.** A closed, curated set keeps false positives near zero,
keeps the renderer output deterministic (goldens stable), and removes the injection
vector entirely. User-editable patterns are a deliberate future slice behind their own
UX and sanitization.

**Rejected alternative.** *User-editable pattern list in the TUI now.* Rejected: expands
scope, reintroduces the injection surface, and destabilizes goldens for marginal v1 value.

---

## ADR-5 — Both `bash` and `powershell` in one entry; no render-time OS selection

**Decision.** The verified Copilot CLI schema (engram #395) puts BOTH script variants
in the SAME hook entry — `"bash": "<script>"` and `"powershell": "<script>"` — and
Copilot CLI itself picks which one to execute based on the host OS at RUNTIME.
`RenderGuardrails` therefore takes no `goos` parameter, has no OS discriminator, and
needs no `commandSpec`-style selector: it always emits both fields for a given posture.

```go
func RenderGuardrails(posture Posture) (HookFile, error) {
    bash, err := renderBashScript(posture)
    if err != nil {
        return HookFile{}, err
    }
    ps, err := renderPowerShellScript(posture)
    if err != nil {
        return HookFile{}, err
    }
    return HookFile{
        Version: SchemaVersion,
        Hooks: map[string][]Hook{
            EventPreToolUse: {{
                Type:       "command",
                Matcher:    MatcherBash,
                Bash:       bash,
                PowerShell: ps,
                TimeoutSec: TimeoutSeconds,
            }},
        },
    }, nil
}
```

The bash script body (ADR-3, unchanged) and the PowerShell-equivalent script body (reads
stdin, regex match, emits the same decision JSON) are both authored and both ALWAYS
rendered — only their destination field (`bash` vs `powershell` on the same `Hook`)
differs, never whether they exist in the output:

```powershell
$input = [Console]::In.ReadToEnd()
if ($input -imatch 'PATTERN_ALTERNATION') {
  Write-Output '{"permissionDecision":"DECISION","permissionDecisionReason":"capiko guardrail: dangerous command blocked"}'
}
```

**Why no `goos` seam.** The project's earlier draft of this ADR introduced a `goos`
package var and a `commandSpec` selector to pick ONE interpreter at render time, on the
(inferred) assumption that the config file's `command` field was OS-selected ahead of
time. The verified contract shows Copilot CLI reads both fields from every `preToolUse`
entry and dispatches by its own host OS at runtime — capiko never needs to know or guess
the target OS when rendering. This removes a seam and a whole rejected-alternative axis:
golden tests need only one rendered file per posture (containing both scripts), not a
bash/powershell pair.

**Rejected alternative (superseded).** *A `goos` package var selecting one interpreter at
render time (this project's original, inferred draft of this ADR).* Rejected now that
the verified schema confirms both fields are always present in the same entry; a
render-time OS selector would produce a file only half of Copilot's possible hosts could
execute correctly.

---

## ADR-6 — Combined checksum, not a per-file map

**Decision.** Store ONE combined SHA-256 of all capiko-owned hook files
(`capiko-*.json`, sorted by name) in `CopilotHooksRecord.Checksum`. Drift recomputes the
same combined value and compares. For v1 (single `capiko-guardrails.json`) this is the
hash of that one file's contribution.

```go
// CombinedChecksum hashes every capiko-*.json in hooksDir, sorted by name, as
// name+"\n"+bytes concatenations. Missing dir or no capiko files → "".
func CombinedChecksum(hooksDir string) string

// HookFileChecksum returns state.Checksum(fileBytes), or "" if the file is absent.
func HookFileChecksum(path string) string
```

`CombinedChecksum` is the SINGLE source of the stored/compared value: `applyCopilotHooks`
sets `rec.Checksum = CombinedChecksum(HooksDir)` AFTER writing, and
`drift.StaleCopilotHooks` compares against `CombinedChecksum(HooksDir)` — the two can
never diverge because they call the identical function. `HookFileChecksum` is used only
for the backup-only-when-changed optimization (compare on-disk file bytes to the freshly
rendered bytes before deciding to snapshot+write), mirroring `applyHeadroom`'s
checksum-gated write.

**Why combined, not `map[string]string`.** v1 has one preset. A combined hash keeps the
record flat and the drift check a single equality. When multi-preset arrives, the
`Presets []string` field already scopes which files the combined hash covers — no schema
migration, no map. A per-preset map would be premature and complicate drift for zero v1
benefit.

**Rejected alternative.** *`map[preset]checksum`.* Rejected as over-engineering for a
single-file v1; `Presets` + combined checksum captures the same information additively.

---

## ADR-7 — `CopilotHooksRecord`: store the posture, don't derive it

**Decision.** Persist `Posture` and `Presets` alongside `Enabled` and `Checksum`.

```go
// CopilotHooksRecord is the managed user-level Copilot hook wiring. capiko owns the
// capiko-*.json files under $COPILOT_HOME/hooks entirely. Enabled false records a
// deliberately-disabled wiring so sync does not re-apply it. Mirrors HeadroomRecord.
type CopilotHooksRecord struct {
    Enabled  bool     `json:"enabled"`
    Posture  string   `json:"posture,omitempty"`  // off|warn|strict — drives RenderGuardrails
    Presets  []string `json:"presets,omitempty"`  // active preset ids, e.g. ["guardrails"]
    Checksum string   `json:"checksum,omitempty"` // CombinedChecksum of capiko-*.json
}
```

Plus `CopilotHooks *CopilotHooksRecord` on `State` (after `TeamSync`, `state.go:49`) and
`func (s *Store) SetCopilotHooks(rec *CopilotHooksRecord) error` mirroring
`SetTeamSync` exactly (`state.go:358-369`): snapshot-before-mutate is the caller's job;
the setter only persists.

**Why store `Posture` rather than re-derive it from the file.** RunSync re-apply and any
future status/doctor pass must reproduce the SAME file without reverse-parsing the
rendered JSON to guess `ask` vs `deny`. Storing the intended posture makes re-apply a
pure `RenderGuardrails(rec.Posture)` and makes state the single source of intent. It also
lets `StaleCopilotHooks` distinguish "user hand-edited the file" (checksum drift) from a
capiko-driven posture change (which we rewrite deliberately). `Presets` scopes the
combined checksum (ADR-6) and unblocks multi-preset later.

**Rejected alternative.** *Derive posture by parsing `permissionDecision` out of the
file.* Rejected: couples state recovery to script-body parsing, breaks the moment the
script format changes, and cannot represent `off` (no file) cleanly.

---

## ADR-8 — Atomic whole-file write, idempotent remove, path guard

**Decision.** `copilothooks` owns the file I/O with the repo's standard atomic primitive,
adapted to a filename-in-a-dir API (unlike `instructions.Write`, which takes a full path).

```go
// WriteHookFile atomically writes data to <hooksDir>/<name>: MkdirAll(hooksDir,0o755),
// write <name>.tmp (0o644), Rename over <name>. JSON files are data, not executable.
func WriteHookFile(hooksDir, name string, data []byte) error

// RemoveHookFile removes <hooksDir>/<name>. Idempotent (IsNotExist → nil). Path-guarded:
// refuses any name resolving outside hooksDir or into a subdir (mirrors Host.UninstallAgent).
func RemoveHookFile(hooksDir, name string) error
```

- **0o644, not 0o755.** The JSON file is read by Copilot, not executed; the `command`
  (`bash`/`powershell`) is the executable. This differs deliberately from
  `githooks.WriteBlock`, which needs 0o755 because the hook file itself is the script.
- **Path guard.** `name` is always a capiko constant in v1, but the guard (reject `..`
  and path separators, mirror `copilot.go:132-149`) makes a bad name incapable of
  deleting arbitrary files — defense in depth for future dynamic preset names.
- **Backup before mutate.** `backupCopilotHooks(bkp, hooksDir)` snapshots any existing
  `capiko-*.json` via `bkp.CreateFiles("copilot-hooks", Version, existing)` before write
  or remove — the team-sync `backupTeamSyncHooks` pattern (`teamsync.go:239-260`). A
  first write has nothing to snapshot and is skipped.

**Rejected alternative.** *Reuse `instructions.Write` directly.* Rejected: it takes a full
path and has instruction-file semantics; a small dir+name atomic writer keeps
`copilothooks` self-contained and its tests trivially hermetic.

---

## ADR-9 — TUI screen: posture dropdown FSM

**Decision.** `copilotHooksScreen` mirrors `headroomScreen`/`teamSyncScreen`: the
editing→applying→done/failed state machine, `applyCmd` goroutine emitting a
`copilotHooksAppliedMsg{err}`, and hermetic construction (no FS/exec at build time).

```go
type copilotHooksState int
const (
    copilotHooksEditing copilotHooksState = iota
    copilotHooksApplying
    copilotHooksDone
    copilotHooksFailed
)

const (
    rowCopilotHooksPosture = iota // dropdown: off / warn / strict (space or ←/→ cycles)
    rowCopilotHooksApply
    rowCopilotHooksBack
    copilotHooksRows
)
```

- **Posture dropdown.** `rowCopilotHooksPosture` cycles `off → warn → strict → off`.
  `newCopilotHooks` seeds it from `st.CopilotHooks.Posture` (default `off` when
  unmanaged). No ack gate — unlike team-sync there is no scope-leak; guardrails only make
  the session safer.
- **`applyCmd`.** Builds the record from the selected posture and dispatches
  `applyCopilotHooks(svc.host, svc.state, svc.backup, rec)` on a `tea.Cmd`. `off` routes
  to `disableCopilotHooks`; `warn`/`strict` write the file with
  `Presets:["guardrails"]`. Returns `copilotHooksAppliedMsg{err}`.
- **Informational banners (no seams needed — static text).**
  1. Repo-level future note: "Manages user-level CLI hooks. GitHub cloud-agent
     enforcement (repo-level `.github/hooks`) is a future feature."
  2. Admin-policy override note: "Org policy hooks (`/etc/github-copilot/policy.d/`) can
     override user hooks."
  These are constant strings, so `View()` stays deterministic with no filesystem access
  (the team-sync banner pattern, minus the conflict detection).
- **Goldens.** `internal/tui/testdata/copilothooks_{editing_off,editing_warn,
  editing_strict,done,failed}.golden`. Adding the menu item also changes the main-menu
  golden — regenerate with `go test ./internal/tui -update` and INSPECT both diffs
  (capiko-dev skill).

Menu wiring (`internal/tui/app.go`): add
`{"Configure Copilot hooks", "copilot-hooks", true}` to `menuItems` after team-sync
(line 75) and `case it.id == "copilot-hooks": a.active = newCopilotHooks(a.svc)` to
`open()` after the team-sync case (line 252).

**Rejected alternative.** *Three separate toggle rows (off/warn/strict as booleans).*
Rejected: a single cycling dropdown matches the "user picks one posture" mental model,
avoids illegal multi-select states, and yields a simpler FSM.

---

## ADR-10 — RunSync re-apply gate + drift

**Decision.** Add one gate to `RunSync` after the headroom block (`sync.go:107-113`),
inside the existing `if st, err := store.Load(); err == nil` block, gated on
`Enabled` — mirroring every other managed feature:

```go
if st.CopilotHooks != nil && st.CopilotHooks.Enabled {
    if err := applyCopilotHooks(host, store, bkp, st.CopilotHooks); err != nil {
        return len(recorded) + len(agentRecorded), fmt.Errorf("re-applying copilot hooks: %w", err)
    }
}
```

Re-apply is idempotent: `applyCopilotHooks` rewrites only when the rendered bytes differ
from disk (ADR-6 change gate). `drift.StaleCopilotHooks` mirrors `StaleHeadroom`
(`drift.go:30-43`):

```go
func StaleCopilotHooks(hooksDir string, st *state.State) bool {
    if st == nil || st.CopilotHooks == nil || !st.CopilotHooks.Enabled {
        return false
    }
    return copilothooks.CombinedChecksum(hooksDir) != st.CopilotHooks.Checksum
}
```

**Rejected alternative.** *Skip RunSync re-apply (team-sync style, which is per-repo and
NOT re-applied by RunSync).* Rejected: Copilot hooks are user-level global config, like
headroom/engram — they SHOULD track the catalog on every sync so an upgraded pattern set
propagates. This matches the headroom precedent, not team-sync.

---

## Concrete rendered example — `capiko-guardrails.json` (strict)

```json
{
  "version": 1,
  "hooks": {
    "preToolUse": [
      {
        "type": "command",
        "matcher": "bash",
        "bash": "input=\"$(cat)\"; if printf '%s' \"$input\" | grep -Eiq '(\\brm\\b[[:space:]]+(-[^[:space:]]*[rf][^[:space:]]*[[:space:]]+)+/([[:space:]]|$))|((curl|wget)[^|]*\\|[[:space:]]*(sh|bash))|(git[[:space:]]+push[[:space:]]+.*--force([[:space:]]|=).*[[:space:]](main|master))|(chmod[[:space:]]+([^[:space:]]+[[:space:]]+)*777)'; then printf '{\"permissionDecision\":\"deny\",\"permissionDecisionReason\":\"capiko guardrail: dangerous command blocked\"}'; fi; exit 0",
        "powershell": "$input = [Console]::In.ReadToEnd(); if ($input -imatch 'PATTERN_ALTERNATION') { Write-Output '{\"permissionDecision\":\"deny\",\"permissionDecisionReason\":\"capiko guardrail: dangerous command blocked\"}' }",
        "timeoutSec": 5
      }
    ]
  }
}
```

Both `bash` and `powershell` are ALWAYS present in the same entry; Copilot CLI selects
which one to execute based on its own host OS at runtime — capiko never renders an
OS-specific file (ADR-5). For `warn`, the only diff is `"deny"` → `"ask"` inside BOTH
script bodies. `off` produces NO file (removal).

---

## Test strategy (Strict TDD — write the failing test first)

Per the go-testing skill: table-driven, `t.TempDir()`, seams over mocks
(`userHomeDir`, `COPILOT_HOME` via `t.Setenv`), `t.Cleanup` restores. capiko does
NOT use `teatest` — TUI flows are driven by calling `Update(msg)` directly with the
`key(...)` helper. Every unit gets its failing test BEFORE implementation.

- **`internal/copilothooks` (unit + golden):**
  - `RenderGuardrails`: `off` → error/empty (removal path, not rendered); `warn` → both
    the `bash` and `powershell` fields contain `"ask"`; `strict` → both contain
    `"deny"`; matcher, timeoutSec, version pinned; `Hooks["preToolUse"]` has exactly
    one entry.
  - `Marshal`: deterministic bytes → golden `testdata/guardrails_strict.json`,
    `guardrails_warn.json` — each contains both `bash` and `powershell` fields in the
    same entry (no per-OS golden pair, no `goos` seam needed).
  - `WriteHookFile`: creates `HooksDir`, atomic tmp+rename, mode `0o644`, overwrite.
  - `RemoveHookFile`: removes, idempotent on missing, path-guard rejects `..`/separators.
  - `HookFileChecksum` / `CombinedChecksum`: absent dir → `""`, single file, two files
    (order-independent), changes when bytes change.
- **`internal/copilot` (`Detect`):** `$COPILOT_HOME` set → `HooksDir` under it (seam
  `userHomeDir` NOT called); unset → `~/.copilot/hooks`; `HooksDir` always populated.
- **`internal/state`:** `SetCopilotHooks` round-trip (enabled+posture+presets+checksum),
  nil-clear, load of a state.json without the field (backward-compat, `omitempty`).
- **`internal/drift`:** `StaleCopilotHooks` — unmanaged/disabled → false; matching
  checksum → false; hand-edited file → true; missing file while enabled → true.
- **`internal/tui` (`Update`-driven):** posture cycle off→warn→strict→off; Apply from
  `warn`/`strict` emits a cmd → `cmd()` → `copilotHooksAppliedMsg` → `Done`; Apply from
  `off` routes to disable; failure → `Failed`. Build the screen struct directly for
  hermetic navigation. Goldens for editing(off/warn/strict)/done/failed + the changed
  main-menu golden.
- **Run order:** `go test ./internal/copilothooks ./internal/tui ./internal/drift`
  (narrow) then `go test -race ./...`; `gofmt -l .` and `go vet ./...` clean before PR.

---

## Risks (new or sharpened by the design)

- **Upstream schema field names — RESOLVED.** Originally flagged MEDIUM (ADR-2's field
  names/stdin contract came from the settled proposal, not a live check). Now verified
  against `docs.github.com/en/copilot/reference/hooks-reference` + local Copilot CLI
  v1.0.60 (engram #395): the object-keyed `hooks.preToolUse[]` shape and the sibling
  `bash`/`powershell` fields are confirmed. Residual LOW risk: matcher tool names beyond
  `bash` (e.g. `edit`) are not exercised by this feature and are not needed for v1.
- **grep-on-raw-JSON false negatives via escaping (LOW).** ADR-3 tradeoff; token patterns
  are not JSON-escaped, so a matched command still gets ask/deny. Future `jq` hardening
  optional. Note this is independent of the timeout fail-open risk below.
- **PowerShell parity untested on CI (LOW-MEDIUM).** Every rendered entry carries the
  PowerShell script unconditionally (ADR-5), but it is not executed on a Linux runner.
  Golden covers rendering; runtime behavior needs a manual Windows check.
- **Admin policy override (LOW, surfaced).** Org policy hooks can override user hooks;
  TUI note only, not a capiko bug.
- **Guardrail latency / timeout fail-open (LOW, sharpened).** 5s `timeoutSec` cap +
  minimal pattern set; grep of a small payload is sub-millisecond. Verified fail-mode
  (engram #395): exceeding `timeoutSec` is FAIL-OPEN (Copilot proceeds as if the hook
  never fired), unlike a script crash/non-zero-exit, which is fail-CLOSED (denies). The
  script MUST stay trivial — a hang, not just a slow response, is what defeats the
  guardrail.
- **Golden churn (LOW).** New screen goldens + the main-menu golden change; reviewer
  inspects diffs (capiko-dev skill).
- **Resolved (not residual): shell injection.** v1 interpolates no user input into the
  script (ADR-4); the surface is empty until user-editable patterns arrive.

## Decisions explicitly NOT reopened (from the proposal)

User-selectable posture (off/warn/strict); minimal four-pattern set; matcher `bash`;
individual JSON files with whole-file atomic ownership; combined SHA-256 drift; pinned
`"version":1`; 5s timeout cap; repo-level hooks deferred (TUI note only); Windows in
scope via an always-rendered `powershell` field (not render-time OS selection);
`$COPILOT_HOME` fix bundled as foundation.
