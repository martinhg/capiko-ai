# Design: Engram team sync (managed git hooks) — E-1

Architectural decisions for the opt-in feature that wires `.git/hooks/post-merge`
and `.git/hooks/pre-push` so a team shares LOCAL Engram memory through git. The
proposal's product decisions (`sdd/engram-team-sync/proposal`) are authoritative;
this document settles the HOW and grounds each decision in the existing codebase.

## Architecture approach

Mirror the established "managed per-repo feature" pattern (`applyCodeReview` /
`applyEngramConfig`): a pure apply/disable function pair keyed on a `workspace`
obtained via a `Getwd` seam, plus a Bubbletea screen that builds a state record
and dispatches the apply on a `tea.Cmd`. Two layers:

- **`internal/githooks`** — a new, mechanism-only package. It knows how to inject
  a marker-delimited block into a `.git/hooks/<name>` file with a shell shebang and
  the executable bit. It owns NO policy (no engram, no detection). It reuses
  `internal/instructions.Inject` (a pure function) for the marker logic so the
  marker semantics stay identical to persona/SDD/code-review blocks.
- **`internal/tui/teamsync.go`** — policy + UX. Conflict detection, project-name
  resolution, hook-body rendering, the scope-leak ack gate, and the screen.

Boundary rule: `githooks` is a generic text-injection mechanism; everything
engram-, framework-, and git-config-aware lives in `teamsync.go`. This keeps the
new package small, fully unit-testable with `t.TempDir()`, and free of any
project-specific coupling.

```
teamsync.go (policy/UX)                       githooks (mechanism)
  applyTeamSync(workspace, …) ───────────────▶ WriteBlock(ws, hook, mStart, mEnd, body)
  disableTeamSync(workspace, …) ─────────────▶ RemoveBlock(ws, hook, mStart, mEnd)
  detectHookConflict(workspace) string                │ reuses
  resolveProject(workspace) string                    ▼
  renderPostMerge()/renderPrePush(name)        instructions.Inject (pure)
  shSingleQuote(name) string
  state.SetTeamSync(rec)
```

---

## ADR-1 — `internal/githooks` API and atomic-write strategy

**Decision.** New package `internal/githooks` with two functions:

```go
// WriteBlock injects a marker-delimited block into <workspace>/.git/hooks/<hookName>.
// When the file is absent or empty it is created with a "#!/bin/sh\n" first line.
// Content outside the markers is preserved. The file is made executable (0o755).
// Idempotent: an unchanged block produces identical bytes and no spurious rewrite.
func WriteBlock(workspace, hookName, markerStart, markerEnd, block string) error

// RemoveBlock removes the marker-delimited block from <workspace>/.git/hooks/<hookName>.
// A missing file or absent markers is a no-op. If removing the block leaves only the
// shebang (capiko was the sole owner), the hook file is deleted so disable leaves no
// inert capiko-only hook behind.
func RemoveBlock(workspace, hookName, markerStart, markerEnd string) error
```

- **Marker format (shell comments).** Markers are passed in by the caller (as in
  `applyCodeReview`, which passes `codereview.MarkerStart/End` to
  `instructions.Render`). `teamsync.go` defines shell-comment markers so the shell
  never chokes on them:
  ```
  # >>> capiko:team-sync:post-merge >>>   …   # <<< capiko:team-sync:post-merge <<<
  # >>> capiko:team-sync:pre-push  >>>   …   # <<< capiko:team-sync:pre-push  <<<
  ```
- **Shebang on create.** `WriteBlock` reads the file itself (it cannot use
  `instructions.Render`, which has no shebang/exec-bit notion). When the existing
  content is empty/whitespace it seeds `existing = "#!/bin/sh\n"` BEFORE calling
  `instructions.Inject`. When the file already has content it is left untouched
  (existing hooks already carry their own interpreter line).
- **Marker injection reuse.** Call `instructions.Inject(existing, start, end, block)`
  (internal pkg, pure, lines 17-52 of `internal/instructions/instructions.go`). This
  inherits the exact append-with-markers / replace / preserve-outside behavior used
  everywhere else — no duplicated marker logic.
- **Atomic write + exec bit.** Mirror `instructions.Write` (temp file + `os.Rename`,
  `internal/instructions/instructions.go:66`) and `engram.atomicWrite`
  (`internal/engram/engram.go:289`) BUT with executable perms. `os.WriteFile` is
  subject to umask, so after writing the temp file call `os.Chmod(tmp, 0o755)` to
  guarantee the exec bit, then `os.Rename` (rename preserves mode). `os.MkdirAll`
  the `.git/hooks` dir first.

**Rejected alternatives.**
- *Reuse `instructions.Render`/`Write` directly.* Rejected: `Render` treats a
  missing file as empty with no shebang, and `Write` hardcodes `0o644` — a hook
  written `0o644` will not run. We need shebang seeding + `0o755`.
- *Approach B from exploration (`git config core.hooksPath .capiko/hooks`).*
  Rejected in the proposal: it itself collides with husky/lefthook (which set the
  same key) and rewrites repo git config globally.

---

## ADR-2 — `TeamSyncRecord` shape (settling the proposal's open question)

**Decision.** Store the workspace path. `TeamSyncRecord` is NOT path-less.

```go
// TeamSyncRecord is the managed git-hook team-sync wiring. capiko writes the
// marker-delimited hook blocks itself (unlike code review, which delegates to gga),
// so this record is the ONLY system of record for where the hooks live.
type TeamSyncRecord struct {
    Enabled   bool   `json:"enabled"`
    Workspace string `json:"workspace,omitempty"` // repo root the hooks were written to
    Project   string `json:"project,omitempty"`   // resolved --project used in the pre-push export
    Conflict  string `json:"conflict,omitempty"`  // non-empty: framework/hooksPath detected, hooks NOT written
}
```

Plus `TeamSync *TeamSyncRecord` on `State` and `func (s *Store) SetTeamSync(rec *TeamSyncRecord) error`, mirroring `SetCodeReview` (`internal/state/state.go:281`).

**Why store the path (and why this differs from CodeReview).** `CodeReviewRecord`
is path-less because gga owns the hook and has its own per-repo install state —
capiko can delegate verification to gga. Team-sync has NO external system of
record: capiko writes the block and nothing else remembers which repo was
configured. Without `Workspace`, the deferred `drift.StaleTeamSync` / doctor checks
(proposal Non-goals) become impossible to implement later without a schema
migration, because there is no way to know which `.git/hooks/<name>` to inspect.
Adding one additive, backward-compatible field now unblocks the whole deferred
roadmap at zero MVP cost (RunSync still skips team-sync, exactly like CodeReview).

`Conflict` records the warn-and-continue outcome so a later status/doctor pass can
distinguish "enabled and written" from "enabled but skipped due to husky/lefthook".

**Rejected alternatives.**
- *Path-less, exactly like CodeReview.* Rejected: capiko owns the artifact here, so
  forgetting the location is strictly worse and blocks deferred drift/doctor.
- *Map keyed by workspace (`map[string]TeamSyncRecord`) for true multi-repo.*
  Rejected as over-engineering for the first slice. The single-record "last-wins"
  limitation is identical to CodeReview's today; migrating a struct to a path-keyed
  map later is non-destructive (the `Workspace` field becomes the key).

---

## ADR-3 — `core.hooksPath` + framework conflict detection (no git binary)

**Decision.** A `detectHookConflict(workspace string) string` helper in `teamsync.go`
returns a human-readable reason for the FIRST conflict found, or `""` when
`.git/hooks/` is the live hook directory and no framework is present. Detection
order (most authoritative first):

1. **`core.hooksPath` set to a non-default path** — parse `<workspace>/.git/config`
   with a regex, mirroring `projectFromGitConfig`
   (`internal/sddstatus/engram.go:137`). The `hooksPath` key is unique to the
   `[core]` section, so a key-anchored match is sufficient and avoids fragile
   section parsing:
   ```go
   var hooksPathRe = regexp.MustCompile(`(?mi)^\s*hooksPath\s*=\s*(\S.*?)\s*$`)
   ```
   A match whose value is not `.git/hooks` is a conflict (`.git/hooks/<name>` is
   inert under a custom hooks path).
2. **Framework signal files at the workspace root** — `.husky/` directory,
   `lefthook.yml` or `.lefthook.yml`, `.pre-commit-config.yaml`. Checked with
   `os.Stat`.

When a conflict is detected, `applyTeamSync` records state
(`Enabled`, `Workspace`, `Project`, `Conflict=reason`), SKIPS the hook writes, and
the screen surfaces the reason plus the manual shell commands (the gga
"not installed" banner pattern, `internal/tui/codereview.go:293`). It NEVER refuses.

**Why regex, no git binary.** Consistent with the rest of the repo
(`sddstatus/engram.go`, `configArtifactStoreIsEngram`) — capiko never shells out to
`git` to read config; a narrow regex over the raw file is enough and keeps apply
hermetic/testable.

**Rejected alternative.** *Warn-then-refuse (exploration Approach C).* Rejected by
the proposal: warn-and-continue matches gga's "binary not found" UX and never
leaves the user stuck.

---

## ADR-4 — Hook block contents and safe `<name>` interpolation

**Decision.** Two render helpers in `teamsync.go` produce the block bodies (without
markers; `githooks.WriteBlock` adds markers + shebang):

`renderPostMerge()` (constant):
```sh
engram sync --import || true
```

`renderPrePush(project string)`:
```sh
engram sync --project 'PROJECT' || true
echo 'Remember to commit .engram/ so teammates receive your memories.'
```

- **Non-blocking by design (`|| true`).** A missing `engram` binary or a sync
  failure must never block a merge, checkout, or push. This honors "configure,
  never install" and keeps git operations safe.
- **Export + remind, no auto-commit (settled).** pre-push runs the export and
  PRINTS the reminder; it never creates a commit or alters the push set.
- **Safe `<name>` interpolation.** The resolved project name is wrapped with a
  POSIX single-quote escaper, NOT naive string concatenation:
  ```go
  // shSingleQuote returns s as a single-quoted POSIX sh literal, escaping any
  // embedded single quote as the canonical '\'' sequence.
  func shSingleQuote(s string) string // "a'b" -> 'a'\''b'
  ```
  Single-quoting neutralizes every shell metacharacter (`$`, backtick, `\`, spaces,
  `;`), so no charset rejection is needed. This closes the shell-injection vector
  the explore flagged.

**Note on echo wording.** The as-built echo reminder is `echo 'Remember to commit
.engram/ so teammates receive your memories.'` — the design-era draft showed a
different echo string. REQ-3.2 was updated at archive time to reflect that exact
wording is not mandated (ADR-4 wins; any reminder echo is compliant).

**Rejected alternative.** *Double-quote interpolation (`--project "<name>"`).*
Rejected: double quotes still expand `$`, backticks, and `\`; a crafted or unlucky
project name could break or inject shell. Single-quote escaping is bulletproof.

---

## ADR-5 — Project-name resolution

**Decision.** Add an exported reader to `internal/engram` and resolve with a
two-branch fallback in `teamsync.go`:

```go
// internal/engram — reuses the existing projectConfig struct + .engram/config.json path
func ReadProjectName(repoRoot string) string // "" when absent/malformed

// internal/tui/teamsync.go
func resolveProject(workspace string) string {
    return engram.ReadProjectName(workspace) // ReadProjectName already has filepath.Base fallback
}
```

The `.engram/config.json` schema (`projectConfig{ProjectName json:"project_name"}`)
and path already live in `internal/engram` (`engram.go:264`). The key is
`"project_name"` (not `"project"` as the original spec draft said). `ReadProjectName`
includes the `filepath.Base(workspace)` fallback internally, so `resolveProject` is a
thin delegate that always returns a non-empty string.

**Rejected alternative.** *Re-declare the `projectConfig` struct in `teamsync.go`.*
Rejected: duplicates the config schema across packages; drift between writer and
reader becomes possible.

---

## ADR-6 — TUI screen (state machine, ack gate, conflict view, seams)

**Decision.** `teamSyncScreen` mirrors `headroomScreen` / `codeReviewScreen`.

State machine:
```go
type teamSyncState int
const (
    teamSyncEditing teamSyncState = iota
    teamSyncApplying
    teamSyncDone
    teamSyncFailed
)
```

Rows:
```go
const (
    rowTeamSyncEnabled = iota // toggle: wire both hooks
    rowTeamSyncAck            // toggle: "I understand scope:personal memories will be committed"
    rowTeamSyncApply
    rowTeamSyncBack
    teamSyncRows
)
```

- **Scope-leak ack gate.** Apply is gated: when `enabled && !ack`, selecting Apply
  does NOT transition to `teamSyncApplying`; it stays in `teamSyncEditing` and shows
  an inline hint ("Acknowledge the scope-leak warning before enabling."). Disabling
  (`!enabled`) requires no ack. This is the one structural difference from the
  code-review/headroom screens, which apply unconditionally.
- **Apply writes FILES only — no engram exec at apply time.** `applyTeamSync` writes
  the two hook blocks (the hooks themselves run `engram sync …` later, at git-event
  time). Therefore the screen needs NO export/import exec seams. This explicitly
  supersedes the proposal's mention of `engramSyncExport`/`engramSyncImport` seams,
  which were vestigial from the earlier ExportMode design; the settled architecture
  writes hooks, it does not run sync. Required seams:
  ```go
  var (
      teamSyncGetwd        = os.Getwd                                   // mirror codeReviewGetwd
      engramDetected       = func() bool { _, e := exec.LookPath("engram"); return e == nil }
      teamSyncDetectConflict = detectHookConflict                      // swappable in tests
  )
  ```
- **Conflict-warning view.** `newTeamSync` resolves the workspace via the seam and
  calls `teamSyncDetectConflict`; the reason is stored on the screen. `View` renders
  a persistent warning banner with the manual shell commands when the reason is
  non-empty (gga-banner pattern, `codereview.go:293-296`). On Apply with a conflict,
  `applyTeamSync` records state with `Conflict` set, skips the writes, and the screen
  shows "recorded; hooks not written — run these manually" (success, never an error).
- **engram-not-on-PATH hint.** When `!engramDetected()`, show the install hint
  banner (mirrors headroom `internal/tui/headroom.go:254`), since the hooks need
  `engram` on PATH at git-event time. capiko configures, never installs (A-D1).
- **Apply command.** `applyCmd` builds the `TeamSyncRecord`, resolves the workspace
  via `teamSyncGetwd`, and returns a `tea.Cmd` emitting `teamSyncAppliedMsg{err}` —
  exactly the `codeReviewScreen.applyCmd` shape (`codereview.go:272-286`).
- **Golden snapshots.** Add `internal/tui/testdata/teamsync_*.golden` for: editing
  (default), editing + ack on, conflict-warning, engram-not-detected, done, failed.
  Regenerate with `go test ./internal/tui -update` and INSPECT the diff
  (capiko-dev skill). Note: adding the menu item changes the main-menu golden too —
  regenerate and inspect that one as well.

Menu wiring (`internal/tui/app.go`): add `{"Configure team sync", "team-sync", true}`
to `menuItems` (after code review, line 74) and
`case it.id == "team-sync": a.active = newTeamSync(a.svc)` to the dispatch
(after line 249).

**Rejected alternative.** *Run `engram sync --import` once at apply for an immediate
first import.* Rejected: keeps apply impure and surprising; the proposal scope is
"write hooks," and a pure apply is far easier to test and reason about.

---

## ADR-7 — Test strategy under Strict TDD (seams over mocks, no teatest)

**Decision.** Write the failing test first for every unit, then implement. Seams are
swapped and restored with `t.Cleanup`; the filesystem is always `t.TempDir()`
(go-testing skill). capiko does NOT use `teatest` — TUI flows are driven by calling
`Update(msg)` directly with the `key(...)` helper.

- **`internal/githooks` (table-driven, `t.TempDir()`):** create-new (asserts shebang
  first line + mode `&0o111 != 0`), inject-into-existing (preserves prior content and
  its shebang), idempotent (write twice → identical bytes), remove (block gone),
  remove-leaves-only-shebang → file deleted, remove on missing file → no-op,
  coexist-with-foreign-block (capiko block added without clobbering an unrelated
  block).
- **`teamsync` apply/disable (seam-swapped `teamSyncGetwd`, temp repo with
  `.git/hooks`, temp `state.Store`):** apply writes both blocks + sets the record;
  disable removes both + records `Enabled:false`; conflict case (seed `.husky/` or a
  `.git/config` with `hooksPath`) asserts hooks NOT written, `Conflict` set, no error.
- **`resolveProject`:** config.json present, absent (→ `filepath.Base`), malformed
  (→ fallback). **`shSingleQuote`:** embedded single quote, `$`, spaces.
  **`detectHookConflict`:** each signal + the clean case.
- **TUI flows (`Update`-driven):** cursor up/down bounds, space toggles enabled/ack,
  ack gate blocks Apply when `enabled && !ack` (stays `teamSyncEditing`), Apply emits
  a cmd → call `cmd()` → assert `teamSyncAppliedMsg` → state `teamSyncDone`/`Failed`.
  Build the screen struct directly for hermetic navigation tests (go-testing skill).
- **Golden:** generate with `-update`, inspect the diff before committing; the
  main-menu golden also changes when the item is added.
- **Run order:** `go test ./internal/githooks ./internal/tui` (narrow) then
  `go test -race ./...` before the PR; `gofmt -l .` and `go vet ./...` clean.

---

## Risks (new or sharpened by the design)

- **`|| true` hides engram failures (MEDIUM).** Hooks never block git ops, but a
  failed/absent `engram` import or export is silent — a user may believe memory
  synced when it did not. Accepted tradeoff (never block git); could later append a
  visible `echo` on failure.
- **First-push memory gap (MEDIUM, UX).** pre-push exports to the WORKING TREE; those
  `.engram/` changes are not in the in-flight push. The first push after enabling
  shares nothing until a subsequent commit+push. The reminder echo mitigates but does
  not eliminate this; spec/docs should state it plainly.
- **Single global record is last-wins across repos (LOW).** Identical to CodeReview
  today; documented. Migration to a path-keyed map is non-destructive when multi-repo
  drift is built.
- **Exec bit / umask (LOW).** Mitigated by explicit `os.Chmod(tmp, 0o755)` before
  rename. On filesystems without exec bits the hook will not run — out of scope.
- **Golden churn (LOW).** Adding the menu item changes the main-menu golden plus the
  new screen goldens; reviewer must inspect diffs (capiko-dev skill).
- **Resolved (not residual): shell injection** via project name — closed by
  `shSingleQuote`.

## Decisions explicitly NOT reopened (from the proposal)

Pre-push = export + remind (no auto-commit / no push-set change); personal-scope
leak = warn + ack gate + documented mitigations; hook conflict = warn-and-continue;
single on/off toggle wires both hooks; capiko configures, never installs engram (A-D1).
