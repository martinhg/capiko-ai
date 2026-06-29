# Design: engram proactive memory protocol block

## Context

capiko wires the engram MCP **server** into Copilot CLI (`internal/engram`,
`internal/tui/engram.go`) but never tells the agent to *use* the `mem_*` tools.
The behavioral half is missing. This design adds a standalone, always-on
declarative protocol block (`capiko:memory`) injected into
`~/.copilot/copilot-instructions.md`, gated on engram being enabled, mirroring
the simplest existing managed block end to end: `internal/efficiency` +
`internal/tui/efficiency.go`.

No new architecture is introduced. This is a faithful clone of an established,
tested pattern with one deliberate deviation (no new state field — gating is
derived from `state.EngramRecord.Enabled`).

## Goals / Non-goals

In scope: the `internal/memory` package, the `applyMemoryProtocol` wiring, the
apply/disable/sync hook points, and tests.

Out of scope (locked): no literal periodic timer (Copilot CLI has no runtime
hooks — expressed as a declarative milestone-save advisory); no change to the
`capiko:sdd` block; no new menu screen or independent toggle; not making
`sdd-status` resolve artifacts from engram.

## Architecture approach

Pattern: **marker-bound managed instruction block**, the same one used by
persona, SDD, trigger, headroom, and efficiency. Two layers:

1. **Content layer** — `internal/memory`: a pure, dependency-free package that
   owns the markers and renders the protocol text. Exact shape of
   `internal/efficiency` (no I/O, no state, just `Render() string` over a
   `const block`).
2. **Wiring layer** — `internal/tui/memory.go`: `applyMemoryProtocol`, the
   render-when-changed + backup-on-change + write applier. Exact shape of
   `applyOutputEfficiency`, minus the state setter (no new field).

Boundary: the content package knows nothing about Copilot, state, or backups.
The TUI applier owns all I/O through the existing `internal/instructions`
primitives (`Render`/`Write`). Gating lives entirely at the call sites in
`internal/tui/engram.go` and `internal/tui/sync.go`.

## Component 1 — `internal/memory` package

New file `internal/memory/memory.go`. Mirrors `internal/efficiency/efficiency.go`
exactly.

Exported surface:

```go
package memory

const (
	MarkerStart = "<!-- capiko:memory:start -->"
	MarkerEnd   = "<!-- capiko:memory:end -->"
)

// Render returns the engram proactive-memory protocol block.
func Render() string { return block }

const block = `...` // the protocol text below
```

The package has no imports and no state. Identical contract to `efficiency`:
`MarkerStart`/`MarkerEnd` are distinct and namespaced (`capiko:memory`), and
`Render()` returns a non-empty `const block`.

### Proposed block content (load-bearing artifact)

This is the exact markdown the agent reads. Terse, directive-led, Copilot-
agnostic tool names, similar length to the efficiency block.

```markdown
## Memory protocol (engram)

Engram is persistent memory across sessions. This protocol is always active while engram is configured — apply it without being asked.

Search first:

- Before starting a task, before answering a question that references prior work, and whenever the user says "remember", "recall", or "what did we do", call mem_context, then mem_search if more is needed.
- Search results are truncated — retrieve full content with mem_get_observation before relying on it.

Save proactively — do NOT wait to be asked — after any of:

- An architecture or design decision: record the rationale and the rejected alternatives.
- A bug fix: record the root cause, not just the change.
- A new convention, pattern, or workflow that was agreed.
- A non-obvious discovery, gotcha, or edge case in the codebase.
- A configuration or environment change.

Use mem_save with a short, searchable title and a What / Why / Where / Learned body.

Session close:

- Before declaring work done, call mem_session_summary covering goal, decisions, discoveries, what was accomplished, next steps, and the relevant files. Skipping it starts the next session blind.

Milestone save (advisory):

- There is no timer. Treat each completed unit of work — a green test suite, a merged change, a resolved decision — as a save point, so progress survives a lost session.

Lifecycle-aware reads:

- Trust memories marked active. Treat needs_review memories as stale context, not fact: surface them and verify against the current code before relying on them.
```

Note: `instructions.Render` trims the trailing newline of the block before
injection (`strings.TrimRight(block, "\n")`), so the `const` may end with or
without a trailing newline without affecting output — same as `efficiency`.

## Component 2 — `internal/tui/memory.go`

New file. `applyMemoryProtocol` mirrors `applyOutputEfficiency` exactly in
params and return shape, with the **only** deviation being no state setter
(locked decision: no new state field; the `store` param is retained for
signature parity and call-site symmetry but not used to persist a flag).

```go
package tui

import (
	"fmt"
	"path/filepath"

	"github.com/martinhg/capiko-ai/internal/backup"
	"github.com/martinhg/capiko-ai/internal/copilot"
	"github.com/martinhg/capiko-ai/internal/instructions"
	"github.com/martinhg/capiko-ai/internal/memory"
	"github.com/martinhg/capiko-ai/internal/state"
)

// applyMemoryProtocol injects (enabled) or removes (disabled) the engram
// proactive-memory protocol block in Copilot's instructions file, backing up
// only when it changes. Presence is fully derived from engram's enabled state,
// so unlike applyOutputEfficiency it records no flag of its own. The store
// param is kept for call-site parity with the other appliers.
func applyMemoryProtocol(host *copilot.Host, store *state.Store, bkp *backup.Store, enabled bool) error {
	if host == nil {
		return nil
	}

	var block string
	if enabled {
		block = memory.Render()
	}

	path := filepath.Join(host.ConfigDir, "copilot-instructions.md")
	content, changed, err := instructions.Render(path, memory.MarkerStart, memory.MarkerEnd, block)
	if err != nil {
		return err
	}
	if changed {
		if bkp != nil {
			if _, err := bkp.CreateFiles("memory", Version, []string{path}); err != nil {
				return fmt.Errorf("backup failed, aborting: %w", err)
			}
		}
		if err := instructions.Write(path, content); err != nil {
			return err
		}
	}
	_ = store // no new state field; gating derives from EngramRecord.Enabled
	return nil
}
```

Deviation summary vs `applyOutputEfficiency`:

- Backup label `"memory"` instead of `"efficiency"`.
- No `store.SetOutputEfficiency(enabled)` tail — replaced by the documented
  `_ = store`. Keeping the `store` parameter preserves an identical call
  signature so the three call sites pass the same argument tuple as the other
  appliers.

## Component 3 — Data flow and wiring (exact call sites)

The block's presence is derived state: it is written wherever engram is applied
as enabled, and removed wherever engram is disabled.

### 3a. `internal/tui/engram.go` — `applyEngramConfig` (enabled path)

`applyEngramConfig` already returns early to `disableEngram` when `!rec.Enabled`
(L82-84). So the post-`applyEngram` region is the enabled path. Insert the call
immediately after the `applyEngram` block at L88-90, before the VS Code surface
handling:

```go
	if err := applyEngram(svc.host, svc.state, svc.backup, rec); err != nil {
		return err
	}
	if err := applyMemoryProtocol(svc.host, svc.state, svc.backup, true); err != nil {
		return err
	}
	if hasSurface(rec.Surfaces, "vscode") {
```

Rationale: keep the memory block adjacent to the engram MCP wiring it shadows,
and after the MCP entry so a failure there short-circuits first.

### 3b. `internal/tui/engram.go` — `disableEngram` (remove path)

Add the removal alongside the existing MCP-entry removals, before the state
write at L133. Place it after the VS Code removals:

```go
	if userPath, err := vscodeUserMCPath(); err == nil {
		if err := engram.RemoveMCPEntry(userPath, "servers", "engram"); err != nil {
			return err
		}
	}
	if err := applyMemoryProtocol(svc.host, svc.state, svc.backup, false); err != nil {
		return err
	}
	rec.Checksum = ""
```

`enabled=false` renders an empty block, which `instructions.Inject` removes,
preserving all other managed sections (persona, efficiency, etc.).

### 3c. `internal/tui/sync.go` — `RunSync` re-apply branch

Inside the existing `if st.Engram != nil && st.Engram.Enabled` branch (L92-96),
after the `applyEngram` re-apply, add the memory re-apply so the block tracks
the catalog on every sync and is rewritten only on change:

```go
		if st.Engram != nil && st.Engram.Enabled {
			if err := applyEngram(host, store, bkp, st.Engram); err != nil {
				return len(recorded) + len(agentRecorded), fmt.Errorf("re-applying engram: %w", err)
			}
			if err := applyMemoryProtocol(host, store, bkp, true); err != nil {
				return len(recorded) + len(agentRecorded), fmt.Errorf("re-applying memory protocol: %w", err)
			}
		}
```

Note the call-site argument forms differ by context but the function signature
is identical: `engram.go` passes `svc.host, svc.state, svc.backup`; `sync.go`
passes the local `host, store, bkp`. Both match
`(host *copilot.Host, store *state.Store, bkp *backup.Store, enabled bool)`.

## Ordering / sequencing concerns

- `instructions.Inject` only ever touches its own marker pair, so block order in
  `copilot-instructions.md` is independent and append-on-first-write. The
  `capiko:memory` section lands after whatever already exists (persona, SDD,
  efficiency) and is idempotent across re-applies. No ordering coupling.
- Within `applyEngramConfig`, the memory apply runs after the MCP entry write so
  the more critical wiring fails first; within `disableEngram` it runs with the
  other removals before the state write. Neither creates a dependency on block
  position in the file.
- Sync re-applies persona, SDD, trigger, instructions, engram, **memory**,
  efficiency, headroom in sequence; each is render-when-changed, so the combined
  pass is idempotent and order-insensitive.

## Test strategy (Strict TDD, red-first)

Write each test first, watch it fail, then implement. Run the narrow package,
then `go test -race ./...` before a PR. Mirror the existing efficiency tests.

### `internal/memory/memory_test.go` (clone of `efficiency_test.go`)

- `TestRenderReturnsProtocol` — `Render()` non-empty and contains the load-
  bearing directives, e.g. (lowercased) `"search"`, `"mem_save"`, `"proactiv"`.
- `TestRenderCoversTriggers` — asserts the save triggers and session-close are
  present: `"decision"`, `"root cause"` (bug fix), `"mem_session_summary"`.
- `TestRenderLifecycleAware` — asserts lifecycle handling: `"active"` and
  `"needs_review"` both appear.
- `TestRenderMilestoneAdvisory` — asserts the no-timer advisory: `"no timer"`
  (or `"milestone"`) present, guarding the locked declarative decision.
- `TestMarkersAreDistinctAndNamespaced` — `MarkerStart != MarkerEnd` and both
  contain `"capiko:memory"`.

### `internal/tui/memory_test.go` (clone of `efficiency_test.go` TUI tests)

- `TestApplyMemoryProtocolWritesBlock` — apply with `enabled=true`,
  `bkp=nil`; assert file contains `memory.MarkerStart` and a recognizable
  heading line (`"Memory protocol"`). No state assertion (no new field).
- `TestApplyMemoryProtocolDisabledRemovesBlock` — apply true then false; assert
  `memory.MarkerStart` absent afterward.
- `TestApplyMemoryProtocolPreservesPersonaAndEfficiency` — apply persona, then
  efficiency, then memory; assert all three markers
  (`capiko:persona:start`, `efficiency.MarkerStart`, `memory.MarkerStart`)
  coexist — proves capiko only touches its own section.
- `TestApplyEngramConfigInjectsMemory` (gating ON) — drive `applyEngramConfig`
  with an `EngramRecord{Enabled: true}` against a `t.TempDir()` host and assert
  the memory block is present. Use the existing engram-test seams so it never
  shells out (`cloudConfig`, `cloudEnroll`, `vscodeUserMCPath`).
- `TestDisableEngramRemovesMemory` (gating OFF) — apply enabled, then disable;
  assert the memory block is removed. Confirms derived-state removal.
- `TestRunSyncReappliesMemoryProtocol` — `store.SetEngram(&EngramRecord{
  Enabled: true})`, run `RunSync`, assert the memory block was written by sync.
- `TestRunSyncSkipsMemoryWhenEngramDisabled` — engram disabled (or nil); run
  `RunSync`; assert no memory block — confirms gating is purely engram-derived
  with no new flag.

Filesystem rules: every test uses `t.TempDir()` and never touches the real home
directory; swap seams and restore via `t.Cleanup` where exec/home paths are
involved (here only the engram seams already defined in `engram.go`).

### Golden impact: none expected

The `capiko:memory` block is written to `copilot-instructions.md` only; it is
**not** rendered in any TUI screen. No new screen, no new status line, no
change to `engramScreen.View()` or any other `View()`. The efficiency feature —
the pattern being cloned — likewise has zero golden coverage; its block is
verified by file-content assertions, not goldens. Therefore
`internal/tui/testdata/*.golden` are untouched. If, contrary to this design, any
rendered text changes, regenerate with `go test ./internal/tui -update` and
inspect the diff before committing — but no such change is part of this design.

## ADR-style decisions

### ADR-1: Gate on engram-enabled, add no new state field

- Decision: presence is fully derived from `state.EngramRecord.Enabled`. The
  applier records no flag of its own.
- Rationale: the protocol is meaningless without the `mem_*` tools, which exist
  only when the engram MCP server is wired. Coupling to the existing enabled
  state removes a redundant, drift-prone flag and a second toggle.
- Rejected alternative: a `state.MemoryProtocol bool` mirroring
  `OutputEfficiency`. Rejected — it could desync from engram (block present
  while tools absent) and adds a menu affordance the proposal explicitly
  excludes.
- Consequence: `applyMemoryProtocol` keeps the `store` param for signature
  parity but does not call a setter (`_ = store`).

### ADR-2: Clone the efficiency pattern rather than generalize

- Decision: duplicate the `efficiency` package + applier shape verbatim instead
  of extracting a shared "managed text block" abstraction.
- Rationale: the existing managed blocks (persona, SDD, trigger, headroom,
  efficiency) all duplicate this shape deliberately; each owns its content and
  markers. Following the established repo convention keeps the change small,
  reviewable, and consistent. A premature abstraction would touch five existing
  features for no functional gain.
- Rejected alternative: a generic `instructions.ManagedBlock` registry.
  Rejected as out-of-scope scope-creep; can be a separate refactor later.

### ADR-3: Declarative milestone advisory, not a timer

- Decision: express periodic saving as a declarative "treat each completed unit
  of work as a save point" advisory.
- Rationale: Copilot CLI has no runtime hooks (`internal/trigger/trigger.go`
  documents the model as declarative/advisory). A literal timer is unenforceable
  on this platform.
- Consequence: behavioral efficacy is advisory; the block raises baseline
  behavior, it cannot guarantee it. Accepted as the platform ceiling.

## Risks

- Instruction-budget creep: another always-on block grows
  `copilot-instructions.md`. Mitigated by keeping the block terse and directive-
  led, similar length to the efficiency block.
- Advisory, not enforced: no runtime hooks means the agent may still under-use
  memory. Platform ceiling, accepted.
- Overlap perception with the `capiko:sdd` block: reviewers may expect
  consolidation. Locked: separate scopes, SDD block unchanged.
