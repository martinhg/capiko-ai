# Proposal: Engram team sync (managed git hooks)

## Summary

Add an opt-in capiko managed feature that configures **git hooks** in a repo so a
team can share Engram memory through git — **local engram only, no cloud**. A new
Configure screen writes two marker-delimited hook blocks into the workspace's
`.git/hooks/`:

- `post-merge` → `engram sync --import` (pull teammates' memories after `git pull`/merge)
- `pre-push` → `engram sync --project <name>` (export this project's memories to
  `.engram/`) then **print a reminder to commit `.engram/`**

Memories travel as content-hashed chunks in `.engram/` committed to the repo.
The feature is strictly opt-in, a single on/off toggle wires both hooks, and capiko
**configures** the hooks — it never installs or manages the `engram` binary (A-D1).

## Why

Engram memory is per-machine. Today a team has no first-class way to share SDD
artifacts and decisions across members without Engram Cloud. `engram sync` already
supports a git-based local flow (export to `.engram/`, import content-hashed chunks
idempotently), but wiring it requires each member to hand-author shell hooks and
remember to run import/export at the right moments. That is exactly the kind of
mechanical, error-prone setup capiko exists to manage.

E-1 turns that manual wiring into a managed, idempotent, reversible feature that
mirrors capiko's existing managed-block patterns (`instructions.Inject`,
`applyCodeReview`), with the privacy and coexistence guardrails the manual approach
lacks.

## What changes

1. **New Configure screen** (`internal/tui/teamsync.go`) — a managed-feature screen
   (mirrors `headroomScreen`/`codereview.go`): an Enabled toggle, an explicit
   scope-leak acknowledgment gate, Apply, and Back. `applyTeamSync(workspace, …)`
   writes both hook blocks; `disableTeamSync(…)` removes them. Exec is behind test
   seams (`engramSyncExport`, `engramSyncImport`). A menu item "Configure team sync"
   is added in `internal/tui/app.go`.

2. **New `internal/githooks` package** — `WriteBlock(workspace, hookName, markerStart,
   markerEnd, block)` and `RemoveBlock(...)`. Marker-delimited, idempotent block
   injection into `.git/hooks/<name>` (mirrors `internal/instructions`), adding a
   `#!/bin/sh` header when creating a new hook file and `chmod +x` on the file. It
   coexists with user- or framework-authored hook content by owning only its marked
   block.

3. **Hook contents** —
   - `post-merge`: `engram sync --import`
   - `pre-push`: `engram sync --project <name>` followed by an echo reminding the
     user to commit `.engram/`. Export only — **no auto-commit, no
     modification of the push set.**
   The `<name>` is the project from `.engram/config.json` if present, else
   `filepath.Base(workspace)`.

4. **Scope-leak guard** — the Configure screen states plainly that `engram sync` has
   no scope filter: `scope: personal` observations for this project **will be
   committed to git**. Enabling requires an explicit acknowledgment. The screen
   documents the two available mitigations: wrap sensitive content in
   `<private>…</private>` tags (stripped at export), or use a separate project name
   for personal memories.

5. **Conflict guard (warn-and-continue)** — before writing, read `.git/config` for
   `core.hooksPath` (regex parse, no git binary — mirrors
   `sddstatus/engram.go`'s `projectFromGitConfig`) and detect framework signals
   (`.husky/`, `lefthook.yml`/`.lefthook.yml`, `.pre-commit-config.yaml`). If a
   non-default `core.hooksPath` or a framework is detected, **record the desired
   state, SKIP writing hooks, and surface the equivalent manual shell commands**
   (same UX as gga's "binary not installed" banner). capiko does not refuse.

6. **State** (`internal/state/state.go`) — new `TeamSyncRecord` + `TeamSync
   *TeamSyncRecord` on `State` + `SetTeamSync` method, mirroring `CodeReviewRecord`.
   Snapshot-before-mutate as usual.

## Settled decisions (do not reopen)

| Decision | Resolution |
| --- | --- |
| Pre-push behavior | Export + remind. `engram sync --project <name>`, then print a reminder to commit `.engram/`. No auto-commit, no push-set changes. |
| Personal-scope leak | Warn + explicit confirmation. Configure screen shows the leak and requires acknowledgment before enabling; documents `<private>…</private>` and separate-project-name mitigations. |
| Hook conflict | Warn-and-continue. Detect `core.hooksPath`/husky/lefthook/pre-commit → record state, skip writing, show manual commands. Never refuse. |
| Toggle granularity | Single on/off enables both `post-merge` import and `pre-push` export together. |
| Engram binary | capiko configures, never installs (A-D1). If `engram` is absent from PATH, show an install hint. |

## Scope

In scope:

- `internal/githooks/githooks.go` (new) — marker-delimited hook block writer/remover
  with `#!/bin/sh` header creation and executable bit.
- `internal/tui/teamsync.go` (new) — Configure screen, `applyTeamSync`/
  `disableTeamSync`, exec test seams, scope-leak ack gate, conflict detection +
  manual-command fallback.
- `internal/state/state.go` — `TeamSyncRecord`, `TeamSync` field, `SetTeamSync`.
- `internal/tui/app.go` — "Configure team sync" menu item.
- Project name resolution: `.engram/config.json` → `filepath.Base(workspace)`.
- Tests: `internal/githooks` block injection/removal/idempotency; `teamsync.go`
  apply/disable/conflict paths via seams (no real `engram` shell-out); TUI goldens
  for the new screen.

## Non-goals

- **No cloud sync.** This is git-based local engram sharing only; `engram sync
  --cloud` is out of scope.
- **No engram-side scope filtering.** capiko does not add a `--scope project` flag to
  `engram sync` (it does not exist); the leak is mitigated by warning + ack + docs,
  not by changing engram.
- **No auto-commit / no push-set modification.** pre-push exports and reminds; it
  never commits or alters what gets pushed.
- **No committed team setup script.** `.git/hooks/` is per-user-per-repo; each member
  configures locally via the Configure screen. No shared bootstrap script in E-1.
- **No `RunSync` re-apply.** Git hooks are per-repo and `RunSync` has no workspace
  context (same choice as CodeReview). Re-apply happens only by re-running the
  Configure screen.
- **No engram binary management.** capiko configures, never installs/upgrades engram.
- Optional `drift.StaleTeamSync` and `doctor` team-sync check are **not** in the
  first slice (documented as later refinement).

## Impact

| Area | Change |
| --- | --- |
| `internal/githooks/githooks.go` | New package: `WriteBlock`/`RemoveBlock`, `#!/bin/sh` header, `chmod +x`, `.git/hooks/` path routing |
| `internal/tui/teamsync.go` | New screen + `applyTeamSync`/`disableTeamSync`, scope-leak ack, conflict detection, manual-command fallback, exec seams |
| `internal/state/state.go` | `TeamSyncRecord` + `TeamSync` field + `SetTeamSync` |
| `internal/tui/app.go` | New menu item |
| `internal/tui/testdata/*.golden` | New golden(s) for the Configure screen |

Behavioral impact:

- Default off. No hooks are written until a user opts in and acknowledges the leak.
- Hooks are per-user-per-repo and not committed; team adoption requires each member
  to configure locally. `.engram/` chunks are what travels via git.
- Idempotent: re-applying overwrites only capiko's marked block; disabling removes
  only that block, leaving user/framework hook content intact.
- Hermetic tests: all `engram` invocations go through seams; no test shells out.

## Risks / open questions

- **Personal-scope leak (HIGH).** `engram sync` has no scope filter; `scope: personal`
  observations for this project will be committed to git once enabled. Mitigated by an
  explicit warning + acknowledgment gate and documented `<private>…</private>` /
  separate-project-name workarounds — but the residual risk is real and product-
  accepted. This is the headline risk.
- **`core.hooksPath` / framework conflict (MEDIUM).** Teams using husky/lefthook set
  `core.hooksPath`, making `.git/hooks/` inert. Mitigated by detect → record → skip →
  show manual commands (warn-and-continue). Edge case: detection misses an
  uncommon framework and capiko writes inert hooks — confirm the detection signal set
  at design/spec time.
- **No `RunSync` re-apply (LOW).** Upgrading capiko won't refresh hook block content;
  the user must re-run Configure. Documented limitation.
- **New package, no analog (MEDIUM effort).** `internal/githooks` is novel;
  `instructions.Inject` is the marker reference but does not handle the shebang,
  executable bit, or `.git/` routing. Keep it small and well-tested.
- **Project-name drift.** If `.engram/config.json` is absent and `filepath.Base`
  differs from how memories were saved, export may target the wrong project name.
  Fail-safe (export simply scopes to that name) but worth a test per resolution branch.

## First slice

Ship the minimum end-to-end opt-in feature:

1. `internal/githooks` package with `WriteBlock`/`RemoveBlock` + tests.
2. `TeamSyncRecord` state plumbing.
3. Configure screen with the scope-leak ack gate, single toggle, conflict
   warn-and-continue + manual commands, and apply/disable writing both hook blocks.
4. Menu item + golden.

Defer to later: `drift.StaleTeamSync`, `doctor` team-sync check, any committed team
bootstrap helper, and `RunSync` re-apply.
