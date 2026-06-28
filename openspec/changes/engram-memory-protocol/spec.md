# engram-memory-protocol Specification

## Purpose

This spec defines what MUST be true after `engram-memory-protocol` is applied.
It covers the `capiko:memory` managed instruction block: its content, its package,
its gating on `state.EngramRecord.Enabled`, and its apply/disable/sync wiring.

Delta against current state: no `capiko:memory` block exists anywhere in the
codebase. The SDD block carries memory guidance scoped to SDD cycles only.
This change adds the missing always-on general protocol for Copilot.

---

## Requirements

### Requirement: capiko:memory Package

A new `internal/memory` package MUST expose:

- `MarkerStart` — the string `<!-- capiko:memory:start -->`
- `MarkerEnd`   — the string `<!-- capiko:memory:end -->`
- `Render() string` — returns the instruction block content as a constant string

The markers MUST be distinct, MUST both contain the namespace prefix `capiko:memory`,
and MUST NOT overlap with any marker already used by another managed block.

#### Scenario: Markers are distinct and namespaced

- GIVEN the `internal/memory` package
- WHEN `MarkerStart` and `MarkerEnd` are compared
- THEN they are not equal, both contain `capiko:memory`, and neither matches any
  marker from `internal/efficiency`, `internal/sdd`, or `internal/persona`

#### Scenario: Render returns a non-empty string

- GIVEN the `internal/memory` package
- WHEN `Render()` is called
- THEN the result is a non-empty string

---

### Requirement: Block Content — Search Before Acting

The block returned by `Render()` MUST instruct the agent to search memory before
starting work or answering questions that reference prior work.
The directive MUST be present in lower-case form for keyword matching in tests.

#### Scenario: Block carries search-first directive

- GIVEN the string returned by `memory.Render()`
- WHEN its lowercase content is inspected
- THEN it contains the word `search` and language indicating a pre-work or
  before-acting trigger (e.g. "before", "starting", "first", or similar)

---

### Requirement: Block Content — Proactive Save on Triggers

The block MUST instruct the agent to save to memory proactively — without being
asked — when defined trigger events occur. The trigger list MUST include at minimum:
decisions, bug fixes, conventions, and discoveries.
The block MUST NOT instruct the agent to wait for the user to request a save.

#### Scenario: Block carries proactive-save directive

- GIVEN the string returned by `memory.Render()`
- WHEN its lowercase content is inspected
- THEN it contains the word `proactive` or `proactively`, and the word `save`

#### Scenario: Block names concrete trigger categories

- GIVEN the string returned by `memory.Render()`
- WHEN its lowercase content is inspected
- THEN it contains at least two of: `decision`, `bug`, `convention`, `discovery`

---

### Requirement: Block Content — Session-Close Summary

The block MUST instruct the agent to write a session-close summary before
declaring work done. The summary requirement MUST be expressed as a pre-"done"
action, not a post-session hook.

#### Scenario: Block carries session-close directive

- GIVEN the string returned by `memory.Render()`
- WHEN its lowercase content is inspected
- THEN it contains `session` and (`summary` or `close` or `done`)

---

### Requirement: Block Content — Lifecycle-Aware Reads

The block MUST instruct the agent to treat retrieved memories by lifecycle state:
trust `active` memories, verify `needs_review` memories before relying on them.

#### Scenario: Block carries lifecycle-aware read directive

- GIVEN the string returned by `memory.Render()`
- WHEN its lowercase content is inspected
- THEN it contains `active` and `needs_review`

---

### Requirement: Block Content — No Literal Periodic Timer

The block MUST NOT claim the existence of a periodic timer, a recurring reminder,
or any runtime hook that fires on a schedule. Periodic or milestone-save guidance
MUST be expressed as a declarative advisory only.
Rationale: Copilot CLI has no runtime hooks; claiming a timer misrepresents the
platform ceiling (see `internal/trigger/trigger.go`).

#### Scenario: Block does not promise a literal timer

- GIVEN the string returned by `memory.Render()`
- WHEN its lowercase content is inspected
- THEN it does NOT contain `every 15` or `every fifteen` or `timer` or `reminder`
  as a runtime promise (advisory phrasing such as "milestone" or "periodically"
  is allowed)

---

### Requirement: Gating ON — Block Written When Engram Is Enabled

When `applyMemoryProtocol` is called with `enabled = true`, it MUST inject the
`capiko:memory` block into `~/.copilot/copilot-instructions.md` between
`memory.MarkerStart` and `memory.MarkerEnd`. The instructions file MUST be
created if it does not exist.

No new `state` field is required: gating derives entirely from the caller passing
`enabled = true`, which reflects `state.EngramRecord.Enabled`.

#### Scenario: First apply with engram enabled

- GIVEN an empty `copilot-instructions.md` (or no file at the path)
- WHEN `applyMemoryProtocol(host, store, bkp, true)` is called
- THEN the file exists and contains `memory.MarkerStart`
- AND the file contains the heading or keyword from `memory.Render()`

---

### Requirement: Gating OFF — Block Absent or Removed When Engram Is Disabled

When `applyMemoryProtocol` is called with `enabled = false`, it MUST remove the
`capiko:memory` block from `copilot-instructions.md` using empty-content injection
(the same mechanism as `applyOutputEfficiency` with `enabled = false`). User
content outside the markers MUST NOT be modified.

If the block was never present, the call MUST be a no-op (no write, no error).

#### Scenario: Disable after a prior enable removes the block

- GIVEN `copilot-instructions.md` contains the `capiko:memory` block
- WHEN `applyMemoryProtocol(host, store, bkp, false)` is called
- THEN `copilot-instructions.md` no longer contains `memory.MarkerStart`
- AND user content outside the markers is present and unchanged

#### Scenario: Disable with no prior block is a no-op

- GIVEN `copilot-instructions.md` does not contain `memory.MarkerStart`
- WHEN `applyMemoryProtocol(host, store, bkp, false)` is called
- THEN the file is not written (mtime unchanged or file still absent)
- AND no error is returned

#### Scenario: Nil host is a no-op

- GIVEN `host` is nil
- WHEN `applyMemoryProtocol(nil, store, bkp, true)` is called
- THEN no file operations occur and nil is returned

---

### Requirement: Idempotency

Calling `applyMemoryProtocol` twice in succession with the same `enabled` value
MUST NOT duplicate the block. The second call MUST detect no change and skip
the file write (and therefore skip the backup).

#### Scenario: Re-apply enabled does not duplicate the block

- GIVEN `applyMemoryProtocol(host, store, bkp, true)` has already been called once
- WHEN `applyMemoryProtocol(host, store, bkp, true)` is called a second time
- THEN `memory.MarkerStart` appears exactly once in `copilot-instructions.md`

---

### Requirement: Coexistence with Other Managed Blocks

`applyMemoryProtocol` MUST operate only within the `capiko:memory:start` /
`capiko:memory:end` marker pair. It MUST NOT remove, overwrite, or shift content
from any other managed block (`capiko:persona`, `capiko:sdd`, `capiko:efficiency`,
`capiko:trigger`, `capiko:headroom`) or from user-authored content outside markers.

#### Scenario: Memory block coexists with persona block

- GIVEN `copilot-instructions.md` already contains the `capiko:persona` block
- WHEN `applyMemoryProtocol(host, store, bkp, true)` is called
- THEN the file contains both `capiko:persona:start` and `memory.MarkerStart`
- AND the persona block content is unchanged

#### Scenario: Memory block coexists with efficiency block

- GIVEN `copilot-instructions.md` already contains the `capiko:efficiency` block
- WHEN `applyMemoryProtocol(host, store, bkp, true)` is called
- THEN the file contains both `efficiency.MarkerStart` and `memory.MarkerStart`

#### Scenario: Disabling memory does not touch efficiency block

- GIVEN `copilot-instructions.md` contains both the `capiko:memory` and
  `capiko:efficiency` blocks
- WHEN `applyMemoryProtocol(host, store, bkp, false)` is called
- THEN the file no longer contains `memory.MarkerStart`
- AND the file still contains `efficiency.MarkerStart`

---

### Requirement: Backup on Change

When `applyMemoryProtocol` produces a content change (enabled causes a write, or
disable causes a removal), it MUST call `bkp.CreateFiles` with label `"memory"`
BEFORE writing. When there is no content change, it MUST NOT create a backup.
A nil `bkp` MUST be handled gracefully (skip backup, proceed with write).

#### Scenario: Backup is created on first enable

- GIVEN a non-nil backup store and a pre-existing `copilot-instructions.md`
- WHEN `applyMemoryProtocol(host, store, bkp, true)` produces a change
- THEN a backup entry labeled `"memory"` exists in the backup store

#### Scenario: No backup on no-op apply

- GIVEN `applyMemoryProtocol(host, store, bkp, true)` has already been called
  (block is already present and content is identical)
- WHEN `applyMemoryProtocol(host, store, bkp, true)` is called again
- THEN no new backup entry is created

#### Scenario: Nil backup store does not cause an error

- GIVEN `bkp` is nil
- WHEN `applyMemoryProtocol(host, store, nil, true)` is called
- THEN no error is returned and `copilot-instructions.md` is written correctly

---

### Requirement: Sync Re-Apply

`RunSync` MUST re-apply the `capiko:memory` block when `state.EngramRecord.Enabled`
is true. The memory re-apply MUST occur within the existing
`if st.Engram != nil && st.Engram.Enabled` branch in `internal/tui/sync.go`,
alongside the existing `applyEngram` call. When engram is disabled or absent,
`RunSync` does NOT write the block; removing a previously-written block is owned
by `disableEngram` (see the Wiring requirement below), not by sync — so sync
never operates on a disabled-engram state.

#### Scenario: Sync re-applies block when engram is enabled

- GIVEN state records engram as enabled (`st.Engram.Enabled = true`)
- WHEN `RunSync` is called
- THEN `copilot-instructions.md` contains `memory.MarkerStart` after sync

#### Scenario: Sync does not write the block when engram is disabled

- GIVEN state records engram as disabled (`st.Engram.Enabled = false`)
- WHEN `RunSync` is called
- THEN `RunSync` does not re-apply the `capiko:memory` block (removal of any
  pre-existing block is handled by `disableEngram`, not by sync)

#### Scenario: Sync with no engram state does not write the block

- GIVEN `st.Engram` is nil
- WHEN `RunSync` is called
- THEN `copilot-instructions.md` does NOT contain `memory.MarkerStart`

---

### Requirement: Wiring in applyEngramConfig / disableEngram

`applyEngramConfig` MUST call `applyMemoryProtocol` with `enabled = true` when
`rec.Enabled` is true (after the existing MCP wiring calls). `disableEngram` MUST
call `applyMemoryProtocol` with `enabled = false`. Both MUST propagate any error
returned by `applyMemoryProtocol`.

#### Scenario: applyEngramConfig writes the memory block

- GIVEN a valid `host`, `store`, and `rec` with `rec.Enabled = true`
- WHEN `applyEngramConfig(svc, workspace, rec)` is called
- THEN `copilot-instructions.md` contains `memory.MarkerStart`

#### Scenario: disableEngram removes the memory block

- GIVEN `copilot-instructions.md` contains `memory.MarkerStart`
- WHEN `disableEngram(svc, workspace, rec)` is called
- THEN `copilot-instructions.md` does NOT contain `memory.MarkerStart`

---

### Requirement: Test Suite Passes Under Strict TDD

All new and modified packages MUST pass `go test -race ./...` with zero failures.
No new code path MUST be introduced without a corresponding test. `gofmt -l .`
MUST produce empty output. `go vet ./...` MUST produce no diagnostics.

Tests MUST use `t.TempDir()` for all filesystem interactions. Tests MUST NOT
touch the real `~/.copilot` directory. Tests MUST swap package-level seams
(function vars) where exec or home-dir resolution is needed, restoring via
`t.Cleanup`.

#### Scenario: CI gate passes

- GIVEN all changes from this capability are applied
- WHEN `go test -race ./...` is executed
- THEN the exit code is 0 and no race conditions are reported

#### Scenario: Formatting gate passes

- GIVEN all new `.go` files
- WHEN `gofmt -l .` is executed
- THEN the output is empty (no unformatted files)
