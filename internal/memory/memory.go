// Package memory provides the engram proactive-memory protocol instruction block
// capiko can inject into Copilot's instructions file. It tells the agent how to
// use the mem_* tools: search first, save proactively, close sessions with a
// summary, and treat retrieved memories by lifecycle state.
package memory

// MarkerStart and MarkerEnd delimit the capiko-managed memory section so it
// can be injected, refreshed, or removed without touching the user's own content.
const (
	MarkerStart = "<!-- capiko:memory:start -->"
	MarkerEnd   = "<!-- capiko:memory:end -->"
)

// Render returns the engram proactive-memory protocol block.
func Render() string { return block }

const block = `## Memory protocol (engram)

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

- Trust memories marked active. Treat needs_review memories as stale context, not fact: surface them and verify against the current code before relying on them.`
