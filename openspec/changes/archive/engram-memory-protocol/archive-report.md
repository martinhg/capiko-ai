# Archive Report: engram-memory-protocol

## Status

COMPLETE — shipped and merged via PR #137. Archived retroactively (the change
was implemented and merged before its OpenSpec artifacts were archived).

## What shipped

A standalone, always-on `capiko:memory` instruction block injected into Copilot's
`copilot-instructions.md`, gated on engram being enabled. It gives the agent the
proactive-memory protocol that capiko previously omitted: search-before-acting,
proactive save on triggers, session-close summary, milestone-save advisory, and
lifecycle-aware reads (`active` vs `needs_review`).

## Why

capiko wired the engram MCP server into Copilot (tools available) but never the
behavioral protocol, so Copilot did not use memory proactively — the user had to be
explicit every time, unlike gentle-ai-configured agents. This change added the
missing behavioral half.

## Delivery

- Single PR #137 (merged). ~398 lines.
- New `internal/memory` package (`MarkerStart`/`MarkerEnd` + `Render()`), mirroring
  the `internal/efficiency` managed-block pattern.
- New `internal/tui/memory.go` `applyMemoryProtocol`, wired into `applyEngramConfig`
  (inject), `disableEngram` (remove), and `RunSync` (re-apply).
- Gating derives entirely from `state.EngramRecord.Enabled` — no new state field.

## Final verified state

- Fresh-context verify: PASS (0 critical).
- Two follow-ups were closed before merge: a backup-on-change test, and a spec wording
  fix for the sync re-apply scenario.

## Platform note (carried in the design)

Copilot CLI has no runtime hooks, so Claude Code's literal "15-minute reminder" timer
cannot be replicated. It is expressed declaratively as a milestone-save advisory. The
block raises baseline behavior; it cannot enforce it.

## Artifacts

proposal.md, spec.md, design.md, tasks.md, apply-progress.md (this folder).
Canonical spec promoted to `openspec/specs/engram-memory-protocol/spec.md`.
