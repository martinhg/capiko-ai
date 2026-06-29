# Proposal: engram proactive memory protocol block

## Why

capiko configures the engram MCP server into Copilot CLI, but Copilot never uses
it proactively. The user must explicitly say "search memory" or "save this" every
time. gentle-ai-configured agents search before acting and save on their own.
capiko-configured Copilot does not.

Verified root cause in this codebase:

- `internal/engram/engram.go` only writes the engram MCP **server** entry into
  `~/.copilot/mcp-config.json` (`CopilotCLIEntry` + `MergeMCPEntry`). This makes
  the `mem_*` tools *available* but never tells the agent to *use* them.
- The only behavioral memory guidance lives **inside** the SDD orchestrator block
  (`internal/sdd/sdd.go` ~L122-138, "Memory always on… while working"). It applies
  only inside an SDD cycle, not to general usage.
- capiko injects marker-bound managed blocks into `~/.copilot/copilot-instructions.md`
  via `internal/instructions` (`Inject`/`Render`). Existing managed blocks:
  `capiko:persona`, `capiko:sdd`, `capiko:review`/`capiko:trigger`, `capiko:headroom`,
  `capiko:efficiency`. There is **no** `capiko:memory` block. A repo-wide search for
  "PROACTIVE SAVE", "always active", "do NOT wait" returns zero results.

The fix is the missing behavioral half of the integration: an always-on declarative
protocol that instructs the agent to search-before-acting, save proactively on
defined triggers, summarize at session close, and treat memories as active vs.
needs-review. This mirrors the simplest existing standalone block,
`internal/efficiency` (`Render() string` returning a `const block`).

## What changes

1. **New `capiko:memory` instruction block** — a standalone, declarative,
   always-on engram protocol written into `~/.copilot/copilot-instructions.md`:
   - Search memory before starting work or answering questions that reference
     prior work (search-first).
   - Save proactively on triggers (decisions, bug fixes, conventions, discoveries,
     config changes, gotchas) without being asked.
   - Write a session-close summary before declaring work done.
   - Treat retrieved memories by lifecycle: trust `active`, verify `needs_review`
     before relying on it.
   - Milestone-save advisory in place of a literal timer (see Non-goals).

2. **Gating** — the block is injected **only when engram is configured and
   enabled**, exactly the same condition that governs the engram MCP server install
   (`st.Engram != nil && st.Engram.Enabled`). When engram is absent or disabled the
   block is not written, and is removed if previously present — the same
   empty-content removal that `applyOutputEfficiency`/`disableEngram` already use.

3. **Wiring** — mirror the `internal/efficiency` pattern end to end:
   - New `internal/memory` package: `MarkerStart`/`MarkerEnd`
     (`<!-- capiko:memory:start/end -->`) and `Render() string`.
   - New `internal/tui/memory.go`: `applyMemoryProtocol(host, store, bkp, enabled)`,
     backing up only on change, mirroring `applyOutputEfficiency`.
   - Hook into the engram apply path (`applyEngramConfig` → apply when `rec.Enabled`;
     `disableEngram` → remove) and into the sync re-apply in `internal/tui/sync.go`
     (alongside the existing `if st.Engram != nil && st.Engram.Enabled` re-apply of
     `applyEngram`), so the block tracks the catalog and is removed on disable.

Because the block's presence is fully derived from engram's enabled state, **no new
state field is required** — gating reads the existing `state.EngramRecord.Enabled`.

## Scope / Non-goals

In scope:
- The `capiko:memory` block content, its package, its gating on engram-enabled, the
  apply/disable/sync wiring, and tests.

Out of scope (Non-goals):
- **No literal periodic timer.** Copilot CLI has no runtime hooks or periodic
  reminders — `internal/trigger/trigger.go` documents the model as "declarative and
  advisory… there are no automated hooks." Claude Code's "15-minute reminder" cannot
  be replicated. It is expressed declaratively as a milestone-save advisory.
- **No change to the SDD block.** The existing SDD memory mention
  (`internal/sdd/sdd.go`) stays as-is. `capiko:memory` is the general always-on
  protocol; the SDD block keeps its artifact-specific guidance. Separate scopes,
  no duplication.
- **Not** making the native `sdd-status` engine resolve artifacts from engram. That
  is a separate follow-up the user will do next (ref gentle-ai PR #957).
- No new menu screen or user-facing toggle: the block is auto-gated on engram, not
  independently switchable.

## Impact

Files likely touched:

| Area | File(s) | Change |
|------|---------|--------|
| New block | `internal/memory/memory.go` | `MarkerStart`/`MarkerEnd` + `Render()` returning the protocol `const block` |
| TUI wiring | `internal/tui/memory.go` (new) | `applyMemoryProtocol(...)` mirroring `applyOutputEfficiency` |
| Engram apply | `internal/tui/engram.go` | Call apply in `applyEngramConfig`; remove in `disableEngram` |
| Sync re-apply | `internal/tui/sync.go` | Re-apply under the existing engram-enabled branch |
| Tests | `internal/memory/memory_test.go`, `internal/tui/memory_test.go` | Block content, apply writes, disable removes, coexists with persona/efficiency, sync re-applies, gating off when engram disabled |
| Goldens | `internal/tui/testdata/*.golden` | Only if a status line is added; no new screen expected, so likely untouched. Regenerate with `-update` and diff-inspect if any rendered text changes |

Conventions: branch-first, conventional commits (no Co-Authored-By/AI attribution),
squash-merge, ~400-line PR budget, Strict TDD (`go test -race ./...`).

## Risks / Open questions

- **Instruction-budget creep**: another always-on block grows
  `copilot-instructions.md`. Keep the protocol terse (lead with directives, no
  filler) per cognitive-doc-design, similar length to the efficiency block.
- **Behavioral efficacy is advisory, not enforced**: with no runtime hooks, the
  agent may still under-use memory. This is the platform ceiling; the block raises
  baseline behavior, it cannot guarantee it.
- **Overlap perception with the SDD block**: reviewers may expect consolidation.
  Decision is locked: separate scopes, SDD block unchanged.
