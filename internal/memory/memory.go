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

Project detection:

- At session start, call mem_current_project to verify engram sees the correct project. It never fails — it returns project name, detection source, path, and available alternatives.

Search first:

- Before starting a task, before answering a question that references prior work, and whenever the user says "remember", "recall", or "what did we do", call mem_context, then mem_search if more is needed.
- Search results are truncated — retrieve full content with mem_get_observation before relying on it.

Save proactively — do NOT wait to be asked — after any of:

- An architecture or design decision: record the rationale and the rejected alternatives.
- A bug fix: record the root cause, not just the change.
- A new convention, pattern, or workflow that was agreed.
- A non-obvious discovery, gotcha, or edge case in the codebase.
- A configuration or environment change.

Use mem_save with a short, searchable title and a What / Why / Where / Learned body. Project is auto-detected from the server working directory — do not pass project on save calls.

Conflict surfacing:

- After every mem_save, check the response for judgment_required. If true, iterate the candidates array and resolve each one by calling mem_judge with its own judgment_id.
- mem_judge requires: judgment_id, relation (one of: related, compatible, scoped, conflicts_with, supersedes, not_conflict), reason, and confidence (0–1).
- Heuristic: if confidence < 0.7 OR (relation is supersedes or conflicts_with AND type is architecture, policy, or decision), ask the user before resolving. Otherwise resolve silently.
- For batch resolution, use mem_judge_many to resolve 1–32 judgments in a single call.
- mem_compare persists an externally-judged semantic verdict between two memories: call it with memory_id_a, memory_id_b, relation, confidence, and reasoning.

Session close:

- Before declaring work done, call mem_session_summary covering goal, decisions, discoveries, what was accomplished, next steps, and the relevant files. Skipping it starts the next session blind.
- Alternatively, call mem_session_finalize for an atomic close that combines session end and summary in a single operation.

Milestone save (advisory):

- There is no timer. Treat each completed unit of work — a green test suite, a merged change, a resolved decision — as a save point, so progress survives a lost session.

Updating and deleting:

- mem_update and mem_delete both require expected_project — a safety guard confirming the correct project before mutating data. Always pass it.
- mem_delete permanently removes an observation. Use it only when the user explicitly asks to delete a memory.
- mem_consolidate merges related observations into one. It is destructive — always preview before confirming.

Additional tools:

- mem_list_projects lists all known projects with observation counts.
- mem_merge_projects merges project name variants into a canonical name.
- mem_stats returns aggregate memory statistics for a project.
- mem_session_start and mem_session_end manage session boundaries (typically handled by hooks, not called directly).
- mem_suggest_topic_key infers a family/slug topic key from a title and content. Use it when unsure of the right topic_key for an upsert.

Lifecycle guardrails:

Engram observations may carry lifecycle state (active, needs_review). These rules are forward-compatible — a no-op on versions without lifecycle support.

Reading:

- Prefer mem_review for lifecycle-aware queries when available. Fall back to mem_context / mem_search on older versions — absence of lifecycle fields means the observation is implicitly active.
- Treat needs_review memories as stale context: surface them to the user and verify against current code before relying on them.

Writing:

- Never mark an observation as reviewed automatically — only the user may confirm a needs_review observation.
- When saving (mem_save), do not set lifecycle fields — let engram assign the default (active).
- When updating (mem_update), preserve the current lifecycle state unless the user explicitly asks to change it.`
