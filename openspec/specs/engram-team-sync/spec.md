# Spec: Engram team sync (managed git hooks) — E-1

**Change:** `engram-team-sync`
**Affected packages:** `internal/githooks` (new), `internal/tui/teamsync.go` (new),
`internal/state/state.go`, `internal/tui/app.go`

---

## What this change delivers

After this change, capiko manages an opt-in git hooks feature that configures two
marker-delimited blocks in a workspace's `.git/hooks/` so that team members can share
Engram memory through git (local, no cloud). A single toggle wires both a `post-merge`
import hook and a `pre-push` export-and-remind hook. The feature is off by default,
requires explicit acknowledgment of a scope-leak risk before activation, detects and
coexists with competing hook frameworks, and follows capiko's existing
snapshot-before-mutate and managed-block patterns.

---

## Non-negotiable invariants

| Invariant | Rule |
|---|---|
| Default off | `TeamSync` is nil in state until the user explicitly enables and applies |
| No binary management | capiko NEVER installs or upgrades the `engram` binary (A-D1) |
| No auto-commit | The pre-push hook NEVER commits or modifies the git push set |
| Marker ownership | Hook writes and removals touch only the capiko-owned marker block; content outside those markers is always preserved |
| No git shell-out | Conflict detection and hook file path resolution NEVER invoke an external `git` process |
| Hermetic tests | No test in `go test -race ./...` invokes `engram`, `git`, or any external binary |
| Snapshot-before-mutate | Hook files that exist before a write or removal are backed up via `backup.Store` first |

---

## REQ-1 — `internal/githooks` package: WriteBlock

`WriteBlock(workspace, hookName, markerStart, markerEnd, block string) error` writes
a marker-delimited block into `<workspace>/.git/hooks/<hookName>`.

**REQ-1.1** When the hook file does not exist, `WriteBlock` creates the file with
`#!/bin/sh` as the first line, followed by a blank line, followed by the marker-
delimited block (markerStart, block content, markerEnd). The file is given executable
permissions (mode 0755 or equivalent; the execute bit MUST be set) before returning.

**REQ-1.2** When the hook file exists and does NOT already contain `markerStart`,
`WriteBlock` appends the marker-delimited block to the end of the existing content.
Pre-existing content is preserved verbatim. A second `#!/bin/sh` line is NOT added.

**REQ-1.3** When the hook file exists and already contains a block between `markerStart`
and `markerEnd` (inclusive), `WriteBlock` replaces only that block with the new content.
All content before `markerStart` and after `markerEnd` is preserved. This operation is
idempotent: calling `WriteBlock` twice with identical arguments produces an identical
file.

**REQ-1.4** `WriteBlock` MUST NOT invoke any external process. The hook file path is
computed as `filepath.Join(workspace, ".git", "hooks", hookName)` — it does not read
`core.hooksPath` or call `git rev-parse`.

**REQ-1.5** `WriteBlock` creates `.git/hooks/` via `os.MkdirAll` when the directory
is absent (githooks.go:23). On any other filesystem error (permission denied, etc.),
`WriteBlock` returns a non-nil error and leaves the hook file unchanged.

---

## REQ-2 — `internal/githooks` package: RemoveBlock

`RemoveBlock(workspace, hookName, markerStart, markerEnd string) error` removes
capiko's marker-delimited block from the hook file.

**REQ-2.1** All content outside the markers (before `markerStart` or after `markerEnd`)
is preserved verbatim in the resulting file.

**REQ-2.2** When the hook file does not exist, `RemoveBlock` is a no-op and returns nil.

**REQ-2.3** When the hook file exists but does not contain `markerStart`, `RemoveBlock`
is a no-op and returns nil (the file is left unchanged).

**REQ-2.4** When removing the block leaves the file containing only the
capiko-seeded `#!/bin/sh` shebang (or only whitespace), `RemoveBlock` DELETES the
hook file rather than leaving an inert shebang-only file behind (githooks.go:75,
"leave no inert hook behind" — ADR-1). When other content remains outside the
markers, the file is rewritten with that content preserved.

**REQ-2.5** `RemoveBlock` MUST NOT invoke any external process.

---

## REQ-3 — Hook block contents

**REQ-3.1** The block written into `post-merge` by `applyTeamSync` contains:

```
engram sync --import
```

No other commands are included in the post-merge block.

**REQ-3.2** The block written into `pre-push` by `applyTeamSync` contains:
1. `engram sync --project <name>` (where `<name>` is the resolved project name per REQ-4)
2. An `echo` line reminding the user to commit `.engram/` so teammates receive the
   memories. The exact wording is not mandated; the as-built reminder is
   `echo 'Remember to commit .engram/ so teammates receive your memories.'`
   (teamsync.go:119, ADR-4). References to `git add .engram` and `git commit` in
   the echo text are NOT required.

**REQ-3.3** The pre-push block does NOT call `git`, does NOT commit anything, and
does NOT modify what is being pushed. It only runs `engram sync --project <name>` and
prints a reminder.

**REQ-3.4** The `<name>` value is embedded as a literal string in the hook file at
apply time (not resolved dynamically at hook-execution time).

---

## REQ-4 — Project-name resolution

**REQ-4.1** When `<workspace>/.engram/config.json` exists and is valid JSON containing
a top-level string key `"project_name"` with a non-empty value, that value is used as
`<name>` in the pre-push hook. The key is `"project_name"` (not `"project"`), reusing
the existing `projectConfig` struct in `internal/engram/engram.go:264` (ADR-5).

**REQ-4.2** When `<workspace>/.engram/config.json` does not exist, cannot be read, is
not valid JSON, or does not contain a non-empty `"project_name"` key, `<name>` is
`filepath.Base(workspace)`.

**REQ-4.3** Project-name resolution is performed once at apply time. The resolved value
is written literally into the hook file.

---

## REQ-5 — Scope-leak acknowledgment gate

**REQ-5.1** The Configure team sync screen displays a clearly visible warning explaining
that `engram sync` has no scope filter and that `scope:personal` observations for this
project WILL be committed to git once the feature is enabled.

**REQ-5.2** The screen displays two documented mitigations:
(a) wrapping sensitive content in `<private>…</private>` tags, and
(b) using a separate project name for personal memories.

**REQ-5.3** The screen contains a distinct acknowledgment row (separate from the
Enabled toggle) that the user must explicitly set before Apply will proceed when
`Enabled` is true. Pressing Apply with `Enabled: on` and the acknowledgment unset
does NOT write any hook files. Instead the screen transitions to an error or
prompt state that communicates that acknowledgment is required.

**REQ-5.4** When `Enabled` is false, pressing Apply proceeds without requiring the
acknowledgment (disabling does not need leak confirmation).

**REQ-5.5** Acknowledgment state is local to the current screen session. Re-entering
the Configure screen resets the ack to unset. This is intentional: each enable action
requires a fresh, conscious confirmation.

---

## REQ-6 — Conflict guard (warn-and-continue)

**REQ-6.1** Before writing any hook file, the apply logic checks whether a competing
hook manager is active in the workspace. Detection uses ONLY file-system reads — no
external process is invoked.

**REQ-6.2** Conflict signals checked (any one is sufficient to trigger the guard):
- `<workspace>/.git/config` contains a `core.hooksPath` value that is not the default
  `.git/hooks` path (read and parsed without invoking git).
- `<workspace>/.husky/` directory exists.
- `<workspace>/lefthook.yml` or `<workspace>/.lefthook.yml` file exists.
- `<workspace>/.pre-commit-config.yaml` file exists.

**REQ-6.3** When a conflict signal is detected:
1. `TeamSyncRecord.Enabled` is set to `true` and recorded in state (the desired state
   is persisted).
2. `TeamSyncRecord.Conflict` is set to a human-readable description of what triggered
   the skip (non-empty value signals conflict; ADR-2).
3. Hook files (`post-merge`, `pre-push`) are NOT written.
4. The screen renders the equivalent manual shell commands the user can run to register
   the hooks in their framework configuration (see REQ-6.4).
5. The operation is NOT treated as an error — Apply returns success (the warning/manual-
   commands view is the success state for conflicted workspaces).

**REQ-6.4** The manual commands displayed MUST show the exact shell lines that would
have been written by `WriteBlock`, so the user can paste them into their hook
framework's configuration. The displayed text MUST include both the post-merge command
(`engram sync --import`) and the pre-push command (`engram sync --project <name>`
with the resolved name).

---

## REQ-7 — State persistence

**REQ-7.1** A new `TeamSyncRecord` struct is added to `internal/state/state.go` with
at least the following fields:

| Field | Type | Purpose |
|---|---|---|
| `Enabled` | `bool` | Whether team sync is enabled |
| `Workspace` | `string` | Workspace path for which hooks were configured |
| `Project` | `string` | Resolved project name embedded in the pre-push hook |
| `Conflict` | `string` | Empty when no conflict; non-empty human-readable description when hooks were skipped (ADR-2 — single field supersedes two-field ConflictDetected+ConflictReason split) |

**REQ-7.2** `State` gains a new field `TeamSync *TeamSyncRecord` tagged
`json:"team_sync,omitempty"`. A nil value means the feature is unmanaged.

**REQ-7.3** `Store` gains a new method `SetTeamSync(rec *TeamSyncRecord) error` that
mirrors `SetCodeReview`: loads the current state, sets `st.TeamSync = rec`, stamps
`st.UpdatedAt = time.Now().UTC()`, and saves. A nil argument clears the field
(unmanaged).

**REQ-7.4** Before any hook file write or removal, files that already exist on disk
are snapshotted via `backup.Store.CreateFiles` with the tag `"team-sync"`. If the
backup fails, the write/removal is aborted and the error is returned. (Mirrors
`backupCodeReviewFiles`.)

---

## REQ-8 — Single toggle semantics

**REQ-8.1** The Configure screen exposes exactly one Enabled toggle. Setting it to
`true` and applying writes BOTH the `post-merge` and `pre-push` hook blocks (subject
to REQ-5 ack and REQ-6 conflict guard). Setting it to `false` and applying removes
BOTH hook blocks.

**REQ-8.2** There is no partial apply (e.g., writing only one hook). The two hooks are
atomic with respect to the toggle: either both are written or neither is (on enable),
and both are removed together (on disable).

---

## REQ-9 — A-D1: engram binary handling

**REQ-9.1** When `engram` is not found on PATH, the Configure screen renders a clearly
visible install hint (e.g., pointing to the engram project or install instructions).
The hint is informational; it does NOT prevent the user from enabling or applying the
feature.

**REQ-9.2** capiko NEVER invokes a package manager, curl, or any installer to install
the `engram` binary. The hint is rendered text only.

**REQ-9.3** PATH detection for `engram` is exposed as a package-level function variable
(seam) in `teamsync.go` (the same pattern as `ggaDetected` in `codereview.go`) so
tests can control whether the binary appears available without touching PATH.

---

## REQ-10 — TUI screen

**REQ-10.1** A new file `internal/tui/teamsync.go` defines a screen struct that
implements the `screen` interface (`Update(tea.Msg) (screen, tea.Cmd)` + `View() string`).

**REQ-10.2** The screen presents at minimum the following interactive rows (in display
order):
1. `Enabled` — boolean toggle; default `false` when unmanaged.
2. Scope-leak acknowledgment — must be set to `true` before Apply proceeds when Enabled
   is true.
3. `Apply` — triggers `applyTeamSync` or `disableTeamSync`.
4. `Back` — returns to the main menu via `backMsg`.

**REQ-10.3** `applyTeamSync(workspace string, store *state.Store, bkp *backup.Store, rec *state.TeamSyncRecord) error`
is a package-level function (not a method) that:
- Runs the conflict guard (REQ-6).
- If no conflict: backs up existing hook files, calls `WriteBlock` for both hooks, calls
  `store.SetTeamSync(rec)`.
- If conflict: sets conflict fields on `rec` and calls `store.SetTeamSync(rec)` without
  writing hook files.
- Returns nil on conflict (warn-and-continue); returns an error only on unexpected
  failures (backup failure, filesystem write failure, etc.).

**REQ-10.4** `disableTeamSync(workspace string, store *state.Store, bkp *backup.Store) error`
is a package-level function that calls `RemoveBlock` for both hooks (backing up first
if they exist) and calls `store.SetTeamSync(&TeamSyncRecord{Enabled: false})`.

**REQ-10.5** All exec operations (specifically: any future runtime invocation of
`engram`) are placed behind package-level function variables (seams) in `teamsync.go`.
Note: capiko itself does NOT invoke `engram` at apply time — it only writes the hook
scripts. Seams are required for any binary-detection or potential future test scenarios.

**REQ-10.6** A "Configure team sync" entry is added to `menuItems` in
`internal/tui/app.go` with `id: "team-sync"` and `ready: true`. Its position in the
menu is after "Configure code review".

**REQ-10.7** The `open` function in `app.go` handles `it.id == "team-sync"` by setting
`a.active = newTeamSync(a.svc)`.

**REQ-10.8** `newTeamSync(svc services) screen` reads `svc.state` on construction to
populate the screen with the current `TeamSyncRecord` values (mirroring `newCodeReview`).

---

## REQ-11 — Tests

**REQ-11.1** `internal/githooks` contains table-driven tests covering:
- `WriteBlock` on a missing file: creates `#!/bin/sh` header, correct marker block,
  executable bit set on the resulting file.
- `WriteBlock` on an existing file without markers: appends block, pre-existing content
  intact, no duplicate shebang.
- `WriteBlock` idempotency: two calls with the same arguments produce an identical file.
- `RemoveBlock`: removes only the capiko block; content outside markers is preserved.
- `RemoveBlock` on a missing file: no error, no file created.
- All test cases use `t.TempDir()`. No real home directory or `.git/` directory is used
  outside the temp dir.

**REQ-11.2** TUI/apply tests for `teamsync.go` cover:
- `applyTeamSync` happy path (no conflict): hook files are written with correct content.
- `applyTeamSync` conflict path: state is saved with `Conflict` set to a non-empty
  reason string, hook files are NOT written.
- `disableTeamSync`: hook files are removed; state records `Enabled: false`.
- Scope-leak ack gate: `Update` with Apply pressed when `Enabled: true` and ack unset
  does not invoke `applyTeamSync`.
- Engram not on PATH: `View()` output contains the install hint text.
- Project-name resolution: `.engram/config.json` present → config name used; absent →
  `filepath.Base` fallback.
- All tests use `t.TempDir()` as the workspace. No real home directory is touched.

**REQ-11.3** TUI flow tests drive `Update()` directly with `tea.Msg` values (key
messages, applied messages). No real Bubbletea program is launched.

**REQ-11.4** At least one golden file in `internal/tui/testdata/*.golden` covers the
Configure team sync screen in its default editing state (after `newTeamSync` construction
with no prior state). The golden is generated with `go test ./internal/tui -update` and
produced under the ASCII color profile enforced by `TestMain`.

**REQ-11.5** `go test -race ./...` passes with all new tests included. No test invokes
`engram`, `git`, `husky`, `lefthook`, or any external binary.

---

## Acceptance scenarios

All scenarios are hermetic. All exec operations are controlled via seams.

### SC-01: WriteBlock — new hook file (REQ-1.1)

```
Given   a temp workspace with .git/hooks/ present but no post-merge file
When    WriteBlock(workspace, "post-merge", markerStart, markerEnd, "engram sync --import") is called
Then    .git/hooks/post-merge exists
And     its first line is "#!/bin/sh"
And     the file contains markerStart and markerEnd with the block between them
And     the file is executable (os.Stat mode & 0111 != 0)
```

### SC-02: WriteBlock — inject into existing file (REQ-1.2)

```
Given   .git/hooks/post-merge exists with content "# user content\nsome-other-hook\n"
And     the file does not contain markerStart
When    WriteBlock is called with the capiko marker and block
Then    the file still contains "# user content\nsome-other-hook\n"
And     the capiko block appears after the existing content
And     no second "#!/bin/sh" is added
```

### SC-03: WriteBlock — idempotent re-apply (REQ-1.3)

```
Given   WriteBlock has been called once and succeeded
When    WriteBlock is called a second time with the same arguments
Then    the resulting file is byte-for-byte identical to the file after the first call
```

### SC-04: RemoveBlock — removes only capiko block (REQ-2.1)

```
Given   a hook file containing "# user line\n" followed by capiko's marker block
When    RemoveBlock is called
Then    the file contains "# user line\n"
And     markerStart and markerEnd are no longer present
And     the file is not deleted
```

### SC-05: Hook contents — post-merge block (REQ-3.1)

```
Given   applyTeamSync is called with Enabled: true, no conflict, ack given
When    the post-merge hook file is read
Then    the capiko block contains exactly "engram sync --import"
And     no other commands appear inside the markers
```

### SC-06: Hook contents — pre-push block with config.json name (REQ-3.2, REQ-4.1)

```
Given   <workspace>/.engram/config.json exists and contains {"project_name": "my-team-project"}
And     applyTeamSync is called with Enabled: true, no conflict, ack given
When    the pre-push hook file is read
Then    the capiko block contains "engram sync --project my-team-project"
And     the block contains an echo line reminding to commit .engram/
```

### SC-07: Hook contents — pre-push block with filepath.Base fallback (REQ-4.2)

```
Given   <workspace>/.engram/config.json does not exist
And     workspace path ends in "capiko-ai"
And     applyTeamSync is called with Enabled: true, no conflict, ack given
When    the pre-push hook file is read
Then    the capiko block contains "engram sync --project capiko-ai"
```

### SC-08: Scope-leak ack gate — Apply blocked without ack (REQ-5.3)

```
Given   a teamSyncScreen with Enabled: true
And     the ack row has NOT been set
When    the user presses enter on the Apply row
Then    no hook files are written
And     View() does not show a success message
And     View() communicates that acknowledgment is required
```

### SC-09: Scope-leak ack gate — Apply proceeds with ack (REQ-5.3)

```
Given   a teamSyncScreen with Enabled: true
And     the ack row HAS been set by the user
When    the user presses enter on the Apply row
Then    applyTeamSync is called
And     (if no conflict) hook files are written
```

### SC-10: Conflict guard — husky detected, hooks not written (REQ-6.2, REQ-6.3)

```
Given   <workspace>/.husky/ directory exists
And     applyTeamSync is called with Enabled: true, ack given
When    the call completes
Then    no file is written to <workspace>/.git/hooks/
And     state.TeamSync.Enabled = true
And     state.TeamSync.Conflict is non-empty
And     applyTeamSync returns nil (not an error)
```

### SC-11: Conflict guard — core.hooksPath detected, manual commands shown (REQ-6.3, REQ-6.4)

```
Given   <workspace>/.git/config contains:
        [core]
            hooksPath = .config/hooks
And     applyTeamSync is called with Enabled: true, ack given
And     the result is a conflict-detected success
When    the screen renders View()
Then    View() contains the manual post-merge shell command ("engram sync --import")
And     View() contains the manual pre-push shell command ("engram sync --project <name>")
```

### SC-12: Conflict guard — no conflict, lefthook absent (REQ-6.2)

```
Given   no .husky/, no lefthook.yml, no .lefthook.yml, no .pre-commit-config.yaml
And     .git/config does not contain a non-default core.hooksPath
When    applyTeamSync is called with Enabled: true, ack given
Then    hook files ARE written
And     state.TeamSync.Conflict is empty ("")
```

### SC-13: Disable — removes both hooks, records off (REQ-8.2)

```
Given   both post-merge and pre-push files contain capiko's marker block
When    disableTeamSync is called
Then    both hook files no longer contain markerStart or markerEnd
And     any pre-existing content outside the markers is preserved
And     state.TeamSync.Enabled = false
```

### SC-14: A-D1 — engram absent from PATH (REQ-9.1, REQ-9.2)

```
Given   the engramDetected seam returns false
When    newTeamSync constructs the screen and View() is called
Then    View() contains a visible install hint for engram
And     no package manager or installer is invoked
And     Apply can still be pressed (the screen does not block on binary absence)
```

### SC-15: State round-trip — TeamSyncRecord persisted and loaded (REQ-7.2, REQ-7.3)

```
Given   a Store backed by t.TempDir()
When    SetTeamSync is called with {Enabled: true, Workspace: "/repo", Conflict: ""}
And     Load is called
Then    st.TeamSync is non-nil
And     st.TeamSync.Enabled = true
And     st.TeamSync.Workspace = "/repo"
```

### SC-16: Snapshot-before-mutate — backup called before write (REQ-7.4)

```
Given   a post-merge hook file already exists
And     applyTeamSync is called (no conflict, ack given)
When    the apply logic runs
Then    backup.Store.CreateFiles is called with the existing hook file path
        BEFORE WriteBlock is called
And     if CreateFiles returns an error, WriteBlock is NOT called and the error is returned
```

### SC-17: Menu item wired (REQ-10.6, REQ-10.7)

```
Given   the App menu is rendered
When    the menu items are inspected
Then    a "Configure team sync" item with id "team-sync" and ready: true is present
And     pressing enter on that item sets a.active to a *teamSyncScreen
```

### SC-18: Golden renders default screen (REQ-11.4)

```
Given   newTeamSync(svc) is called with svc.state = nil
When    View() is called
Then    the output matches internal/tui/testdata/teamsync.golden
And     the golden was produced with the ASCII color profile
```

### SC-19: Hermetic — no external binary in any test (REQ-11.5)

```
Given   all package-level seams are set to in-process stubs
When    go test -race ./... runs
Then    no test invokes exec.Command("engram", ...) or exec.Command("git", ...)
And     all tests pass (no data races)
```

---

## Out of scope

- Cloud sync (`engram sync --cloud` or any remote endpoint)
- Engram-side scope filtering (no `--scope` flag on `engram sync`)
- Auto-commit or push-set modification
- Committed team bootstrap script (`.git/hooks/` is per-user-per-repo)
- `RunSync` re-apply (git hooks are per-repo; RunSync has no workspace context)
- Engram binary installation, upgrade, or version management
- `drift.StaleTeamSync` drift detection
- `doctor` team-sync health check
- A second granularity toggle (e.g., post-merge-only without pre-push)
