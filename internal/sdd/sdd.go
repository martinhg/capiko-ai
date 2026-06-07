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

// DefaultAssignments maps every phase to DefaultModel until the user configures
// specifics.
func DefaultAssignments() map[string]string {
	m := make(map[string]string, len(Phases))
	for _, p := range Phases {
		m[p] = DefaultModel
	}
	return m
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

// Render builds the orchestrator instruction block for the given assignments.
// When strictTDD is true, the block requires the apply/verify phases to follow
// strict Test-Driven Development.
func Render(assignments map[string]string, strictTDD bool) string {
	a := normalize(assignments)

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
	b.WriteString("Artifacts live in the **OpenSpec store**: in-flight changes under `openspec/changes/<change>/` (proposal, spec, design, tasks), the canonical specs in `openspec/specs/`, and completed changes in `openspec/changes/archive/`. Archive merges the change's spec delta into `openspec/specs/`. Run `sdd-init` once to create it.\n\n")

	b.WriteString("### Model assignments\n\n")
	b.WriteString("Run the session on the most capable assigned model. Delegate each phase to its model via the Task tool's `model` parameter. Copilot honors any model whose cost is ≤ the session model and downgrades anything more expensive, so keep the orchestrator on the top model. `default` means inherit the session model.\n\n")
	b.WriteString("| Phase | Model |\n| --- | --- |\n")
	for _, p := range Phases {
		fmt.Fprintf(&b, "| %s | %s |\n", p, a[p])
	}
	b.WriteString("\n")

	b.WriteString("### Rules\n\n")
	b.WriteString("- Delegate each phase's work to a sub-agent (Task tool); pass the phase's assigned model.\n")
	b.WriteString("- Each phase has a matching `sdd-<phase>` skill (e.g. `sdd-spec`); load it when running that phase.\n")
	b.WriteString("- Run `sdd-init` once per project (creates `sdd/context.md`) before the first cycle; use `sdd-onboard` for a guided walkthrough.\n")
	b.WriteString("- Keep one thin orchestrator thread and synthesize the sub-agents' results.\n")
	b.WriteString("- A phase with `default` runs on the session model.\n")

	b.WriteString("\n### Skill resolution\n\n")
	b.WriteString("Before delegating to a sub-agent, resolve the skills it needs: run `capiko-ai skill-registry` to get the current index of installed skills by trigger and path. Match the relevant ones by their trigger, then pass the exact `SKILL.md` paths in the sub-agent handoff so it loads the full skill before doing any work.\n")
	b.WriteString("Pass paths, not summaries — the `SKILL.md` is the source of truth, and a sub-agent that reads it directly preserves the author's intent. If the `capiko-ai` binary is unavailable, fall back to scanning `~/.copilot/skills` for the matching `SKILL.md` paths.\n")

	if strictTDD {
		b.WriteString("\n### Strict TDD (active)\n\n")
		b.WriteString("The apply and verify phases MUST follow strict Test-Driven Development: write a failing test FIRST, run it to see it fail, then write the minimal code to pass it, then refactor. Do not write any implementation before a failing test exists.\n")
		b.WriteString("\nForward this requirement structurally: when you delegate the apply or verify phase, you MUST forward `strict_tdd: true` and the project's test command in the sub-agent handoff. The worker keys off that forwarded signal to load its strict-TDD protocol — stating the rule here is not enough, the flag has to travel with the delegation.\n")
	}

	return b.String()
}
