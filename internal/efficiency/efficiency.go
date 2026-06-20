// Package efficiency provides the output-efficiency instruction block capiko can
// inject into Copilot's instructions file. It trims ceremony and restated context
// to cut output tokens (which cost several times more than input on Opus-class
// models) while preserving full rigor on new questions, decisions, and errors.
package efficiency

// MarkerStart and MarkerEnd delimit the capiko-managed efficiency section so it
// can be injected, refreshed, or removed without touching the user's own content.
const (
	MarkerStart = "<!-- capiko:efficiency:start -->"
	MarkerEnd   = "<!-- capiko:efficiency:end -->"
)

// Render returns the output-efficiency instruction block.
func Render() string { return block }

const block = `## Output efficiency

Optimize for fewer output tokens without losing substance:

- Skip preambles and postambles ("Sure, I can help…", "Let me know if…"). Answer directly.
- Do not reprint unchanged code, files, or context the user already has. Show only what changed, with just enough surrounding lines to locate it.
- On routine, mechanical, or repeated steps, be terse — a short confirmation beats a paragraph.
- Prefer the smallest correct answer; expand only when asked or when the task genuinely needs it.

Keep full rigor where it matters: for a new question, a non-trivial decision, an error, or a debugging step, explain your reasoning completely. Brevity trims ceremony and restatement — never the analysis the user needs.`
