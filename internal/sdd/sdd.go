// Package sdd builds the capiko SDD orchestrator instructions block: a
// Spec-Driven Development workflow Copilot runs by delegating each phase to a
// sub-agent (Task tool) with a per-phase model. The model table is configurable;
// Copilot honors any model whose cost is <= the session model and downgrades the
// rest, so the orchestrator (session) should run on the most capable model.
package sdd

import (
	"fmt"
	"strings"
)

// Marker delimiters for the orchestrator block inside the instructions file.
const (
	MarkerStart = "<!-- capiko:sdd:start -->"
	MarkerEnd   = "<!-- capiko:sdd:end -->"
)

// DefaultModel is the assignment meaning "inherit the Copilot session model".
const DefaultModel = "default"

// Phases are the SDD roles, in execution order. "orchestrator" is the
// coordinating session; the rest are delegated.
var Phases = []string{
	"orchestrator",
	"explore",
	"propose",
	"spec",
	"design",
	"tasks",
	"apply",
	"verify",
	"archive",
}

// Models is the curated list offered in the picker. The UI also allows a custom
// entry. DefaultModel inherits the session model.
var Models = []string{
	DefaultModel,
	"claude-opus-4.8",
	"claude-sonnet-4.6",
	"gpt-5.2",
	"gemini-5.4",
}

// Efforts are the reasoning effort levels offered in the picker.
var Efforts = []string{"low", "medium", "high"}

// DefaultAssignments maps every phase to DefaultModel until the user configures
// specifics.
func DefaultAssignments() map[string]string {
	m := make(map[string]string, len(Phases))
	for _, p := range Phases {
		m[p] = DefaultModel
	}
	return m
}

// DefaultEfforts maps every phase to a sensible reasoning effort level.
func DefaultEfforts() map[string]string {
	return map[string]string{
		"orchestrator": "high",
		"explore":      "low",
		"propose":      "medium",
		"spec":         "medium",
		"design":       "high",
		"tasks":        "low",
		"apply":        "medium",
		"verify":       "high",
		"archive":      "low",
	}
}

// normalize returns a full phase→model map, filling missing/empty phases with
// DefaultModel so rendering is stable regardless of the input.
func normalize(a map[string]string) map[string]string {
	out := DefaultAssignments()
	for k, v := range a {
		if _, ok := out[k]; ok && strings.TrimSpace(v) != "" {
			out[k] = v
		}
	}
	return out
}

// normalizeEfforts returns a full phase→effort map, filling missing/empty
// phases with the defaults so rendering is stable regardless of the input.
func normalizeEfforts(e map[string]string) map[string]string {
	out := DefaultEfforts()
	for k, v := range e {
		if _, ok := out[k]; ok && strings.TrimSpace(v) != "" {
			out[k] = v
		}
	}
	return out
}

// Render builds the orchestrator instruction block for the given assignments.
// When strictTDD is true, the block requires the apply/verify phases to follow
// strict Test-Driven Development.
func Render(assignments map[string]string, efforts map[string]string, strictTDD bool) string {
	a := normalize(assignments)
	e := normalizeEfforts(efforts)

	var b strings.Builder
	b.WriteString("## SDD Orchestrator (capiko)\n\n")
	b.WriteString("For substantial changes, act as a coordinator: run the Spec-Driven Development (SDD) workflow and delegate each phase to a sub-agent via the Task tool. Skip the workflow for trivial, single-file changes.\n\n")

	b.WriteString("### When to use SDD (triage)\n\n")
	b.WriteString("Decide before doing anything — spend effort only where it pays off:\n\n")
	b.WriteString("- **Inline** when the change is small: 1–3 files to decide or verify, a mechanical edit you already know how to make, a git/state check, or a single targeted fix. Just do it — no sub-agents, no workflow.\n")
	b.WriteString("- **Delegate an exploration** when understanding the change requires reading 4+ files (the 4-file rule). Compress the reading into one sub-agent, then act on its summary.\n")
	b.WriteString("- **Delegate a writer** when the change touches 2+ non-trivial files with new logic — hand it to a sub-agent via the Task tool instead of editing inline.\n")
	b.WriteString("- **Run the full SDD workflow** only for a genuinely substantial change — then start from the phases with their triggers: proposal → spec/design → tasks → apply → verify → archive, each on its assigned model.\n")
	b.WriteString("- **Fresh review before a PR** when the diff is non-trivial, and after any incident — delegate an adversarial review with fresh context.\n\n")
	b.WriteString("When in doubt, stay inline. Do not open the SDD workflow for something small.\n\n")

	b.WriteString("### Phases (in order)\n\n")
	b.WriteString("explore → propose → spec → design → tasks → apply → verify → archive\n\n")
	b.WriteString("The **OpenSpec store** holds file-based artifacts: in-flight changes under `openspec/changes/<change>/` (proposal, spec, design, tasks), the canonical specs in `openspec/specs/`, and completed changes in `openspec/changes/archive/`. Archive merges the change's spec delta into `openspec/specs/`. Run `sdd-init` once to create it.\n\n")

	b.WriteString("### Artifact store\n\n")
	b.WriteString("The cycle has two layers:\n\n")
	b.WriteString("- **Memory (always on, when engram is configured).** Search local engram for prior context while working (`mem_search` / `mem_context`) and save decisions plus a session summary at close (`mem_save` / `mem_session_summary`). engram is local-first; with Engram Cloud enabled it replicates to teammates in the background.\n")
	b.WriteString("- **Artifact store (per change).** Where the formal proposal/spec/design/tasks live:\n")
	b.WriteString("  - `hybrid` (default when engram is configured) — canonical specs as files in `openspec/` (reviewed via git) **and** the working memory/artifacts in engram.\n")
	b.WriteString("  - `engram` — artifacts as engram observations (`sdd/<change>/<artifact>`); repo-clean, but no canonical spec merge layer.\n")
	b.WriteString("  - `openspec` (default otherwise) — file artifacts only, with the canonical `openspec/specs/` and merge-on-archive.\n")
	b.WriteString("  - `none` — ephemeral; return progress only, persist nothing.\n\n")
	b.WriteString("In `engram`/`hybrid` mode, read artifacts directly with `mem_search` + `mem_get_observation` (search results are truncated previews — always fetch the full observation). The native `sdd-status` engine reads the OpenSpec files only, so in those modes route from engram directly rather than the engine. Engram Cloud syncs engram memories; OpenSpec files travel via git.\n\n")
	b.WriteString("Multi-repo: each repo carries a `.engram/config.json` naming its project, so engram attributes memories to the right repo even when a parent folder is the workspace root.\n\n")

	b.WriteString("### Engram lifecycle guardrails\n\n")
	b.WriteString("Engram observations may carry lifecycle state (`active`, `needs_review`). These guardrails are forward-compatible: they are a no-op on engram versions that do not expose lifecycle, and activate automatically when the server does.\n\n")
	b.WriteString("**Reading memories:**\n\n")
	b.WriteString("- Prefer `mem_review` for lifecycle-aware queries when the tool is available. It surfaces staleness metadata alongside content.\n")
	b.WriteString("- Fall back to `mem_context` / `mem_search` on older engram versions that do not expose `mem_review`. The absence of lifecycle fields means the observation is implicitly `active`.\n")
	b.WriteString("- When an observation is `needs_review`, treat it with extra caution: flag it to the user before acting on it, note that it may be outdated, and prefer corroborating evidence from the codebase or git history.\n\n")
	b.WriteString("**Writing lifecycle state:**\n\n")
	b.WriteString("- Never mark an observation as `reviewed` automatically. Only the user may confirm that a `needs_review` observation is still valid.\n")
	b.WriteString("- When saving new observations (`mem_save`), do not set lifecycle fields — let engram assign the default (`active`).\n")
	b.WriteString("- When updating an existing observation (`mem_update`), preserve its current lifecycle state unless the user explicitly asks to change it.\n\n")

	b.WriteString("### Model assignments\n\n")
	b.WriteString("Run the session on the most capable assigned model. Delegate each phase to its model via the Task tool's `model` parameter. Copilot honors any model whose cost is ≤ the session model and downgrades anything more expensive, so keep the orchestrator on the top model. `default` means inherit the session model.\n\n")
	b.WriteString("The **Effort** column sets reasoning effort (`low`/`medium`/`high`) per phase. Forward it in the sub-agent prompt so the worker calibrates its depth: `low` for structural/mechanical work, `medium` for balanced implementation, `high` for architectural decisions and validation. The orchestrator should include a line like `Reasoning effort: <level>` in every sub-agent handoff.\n\n")
	b.WriteString("| Phase | Model | Effort |\n| --- | --- | --- |\n")
	for _, p := range Phases {
		fmt.Fprintf(&b, "| %s | %s | %s |\n", p, a[p], e[p])
	}
	b.WriteString("\n")

	b.WriteString("### Rules\n\n")
	b.WriteString("- Delegate each phase's work to a sub-agent (Task tool); pass the phase's assigned model and reasoning effort.\n")
	b.WriteString("- Each phase has a matching `sdd-<phase>` skill (e.g. `sdd-spec`); load it when running that phase.\n")
	b.WriteString("- Run `sdd-init` once per project (creates `sdd/context.md`) before the first cycle; use `sdd-onboard` for a guided walkthrough.\n")
	b.WriteString("- Keep one thin orchestrator thread and synthesize the sub-agents' results.\n")
	b.WriteString("- A phase with `default` runs on the session model.\n")

	b.WriteString("\n### Result contract\n\n")
	b.WriteString("Every phase must return these fields: `status` (ok | failed | partial), `executive_summary`, `artifacts` (topic keys or file paths produced), `next_recommended`, `risks`, and `skill_resolution` (paths-injected | fallback-registry | fallback-path | none). A phase that omits any field has a malformed result.\n")

	b.WriteString("\n### Execution mode\n\n")
	b.WriteString("On the first SDD cycle in a session, ask once and cache:\n\n")
	b.WriteString("- **Automatic** — run all phases back-to-back without pausing. The gatekeeper validates between phases.\n")
	b.WriteString("- **Interactive** (default) — pause after each phase, show a concise summary, and ask before launching the next. Accept continue, stop, or specific feedback to adjust.\n")

	b.WriteString("\n### Automatic mode gatekeeper\n\n")
	b.WriteString("In automatic mode, validate every phase result BEFORE launching the next one. Skip in interactive mode — the user reviews manually.\n\n")
	b.WriteString("**Inline checks (all phases):**\n\n")
	b.WriteString("1. Result contract completeness — all six fields present and non-empty.\n")
	b.WriteString("2. Status — must be `ok`; `partial` or `failed` stops the pipeline.\n")
	b.WriteString("3. Artifact retrievability — the produced artifact must be findable in the active store (engram search, file read, or inline content depending on mode).\n")
	b.WriteString("4. Scope consistency — the `next_recommended` phase must be the expected successor in the dependency graph.\n\n")
	b.WriteString("**Fresh-context review (high-risk phases only: `design`, `apply`):**\n\n")
	b.WriteString("After `sdd-design` or `sdd-apply` passes inline checks, delegate a separate review sub-agent with fresh context to verify:\n\n")
	b.WriteString("- Anti-hallucination — file paths mentioned in the output actually exist in the repo.\n")
	b.WriteString("- Routing coherence — the artifacts are internally consistent and the next phase can consume them.\n\n")
	b.WriteString("**On gate failure:**\n\n")
	b.WriteString("- Re-run the failed phase once with corrective feedback describing exactly what failed.\n")
	b.WriteString("- If the retry also fails, stop the pipeline and report both failures to the user.\n")
	b.WriteString("- Never skip a gate failure or proceed silently.\n")

	b.WriteString("\n### Skill resolution\n\n")
	b.WriteString("Before delegating to a sub-agent, resolve the skills it needs: run `capiko-ai skill-registry` to get the current index of installed skills by trigger and path. Match the relevant ones by their trigger, then pass the exact `SKILL.md` paths in the sub-agent handoff so it loads the full skill before doing any work.\n")
	b.WriteString("Pass paths, not summaries — the `SKILL.md` is the source of truth, and a sub-agent that reads it directly preserves the author's intent. If the `capiko-ai` binary is unavailable, fall back to scanning `~/.copilot/skills` for the matching `SKILL.md` paths.\n")

	b.WriteString("\n### Delivery & chain strategy\n\n")
	b.WriteString("Keep the PR size matched to what a reviewer can hold in their head (the 400-line budget). On the first SDD cycle, ask once and cache a **delivery strategy**, then forward it to `sdd-tasks` and `sdd-apply`:\n\n")
	b.WriteString("- `ask-on-risk` (default) — proceed normally, but stop and ask when the forecast says the work is oversized.\n")
	b.WriteString("- `auto-chain` — when oversized, split into chained PRs without asking.\n")
	b.WriteString("- `single-pr` — keep it one PR, but only with an explicitly recorded `size:exception`.\n")
	b.WriteString("- `exception-ok` — proceed as one PR, accepting a `size:exception` for this run.\n\n")
	b.WriteString("**Review Workload Guard.** After `sdd-tasks` returns, read its `Review Workload Forecast` before launching `sdd-apply`. If it reports `Chained PRs recommended: Yes`, `400-line budget risk: High`, or `Decision needed before apply: Yes`, resolve with the cached delivery strategy: `ask-on-risk` → STOP and ask (split into chained PRs or take a `size:exception`); `auto-chain` → do not ask, apply only the next autonomous slice with work-unit commits; `single-pr` → require a recorded `size:exception` first; `exception-ok` → continue as `size:exception`. Never start oversized apply work without resolving this guard.\n\n")
	b.WriteString("When the resolution yields chained PRs, ask once and cache a **chain strategy**, then forward it alongside the delivery strategy:\n\n")
	b.WriteString("- `stacked-to-main` — each PR merges to `main` in order. Fast iteration, fix on the go; best for independent slices.\n")
	b.WriteString("- `feature-branch-chain` — a tracker branch accumulates the integration: PR #1 targets the tracker, each later PR targets the previous PR's branch, and only the tracker merges to `main`. Best for rollback control and a coordinated release.\n")

	if strictTDD {
		b.WriteString("\n### Strict TDD (active)\n\n")
		b.WriteString("The apply and verify phases MUST follow strict Test-Driven Development: write a failing test FIRST, run it to see it fail, then write the minimal code to pass it, then refactor. Do not write any implementation before a failing test exists.\n")
		b.WriteString("\nForward this requirement structurally: when you delegate the apply or verify phase, you MUST forward `strict_tdd: true` and the project's test command in the sub-agent handoff. The worker keys off that forwarded signal to load its strict-TDD protocol — stating the rule here is not enough, the flag has to travel with the delegation.\n")
	}

	return b.String()
}
