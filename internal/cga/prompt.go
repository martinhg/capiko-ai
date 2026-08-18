// Package cga implements the Capiko Guardian Angel: a native pre-commit code
// review that invokes Copilot as the sole reviewer via a git hook script.
// Everything in this package is pure — no I/O, no exec, no network. The
// pre-commit hook script (rendered by RenderPreCommitHook) is the only piece that
// talks to Copilot, at commit time, on the user's machine. capiko's Go
// binary never calls an AI model directly.
package cga

import (
	"fmt"
	"strings"
)

// Rules returns the curated review-rules prose Copilot is asked to enforce:
// REJECT/REQUIRE/PREFER standards covering architecture, naming, testing,
// error handling, and comments. When personaName is non-empty, a pointer to
// the active persona's standards is appended.
func Rules(personaName string) string {
	var b strings.Builder

	b.WriteString("# Code Review Rules (capiko-managed)\n\n")
	b.WriteString("These standards are enforced by Capiko Guardian Angel on every commit.\n")
	b.WriteString("`REJECT if` is a hard failure, `REQUIRE` is mandatory, `PREFER` is advisory.\n\n")

	b.WriteString("## Architecture\n\n")
	b.WriteString("- REJECT if: business or domain logic lives inside UI components or framework adapters\n")
	b.WriteString("- REQUIRE: dependencies point inward — the domain imports no framework or I/O code\n")
	b.WriteString("- PREFER: small single-responsibility modules over multi-purpose \"god\" files\n\n")

	b.WriteString("## Naming\n\n")
	b.WriteString("- REJECT if: names do not reveal intent (`x`, `tmp`, `data`, `helper`, `manager`)\n")
	b.WriteString("- REQUIRE: booleans read as predicates (`isReady`, `hasAccess`, `canRetry`)\n")
	b.WriteString("- PREFER: domain language over technical jargon\n\n")

	b.WriteString("## Testing\n\n")
	b.WriteString("- REJECT if: a behavior change ships without a test that would catch its regression\n")
	b.WriteString("- REQUIRE: tests assert observable behavior, not implementation details\n")
	b.WriteString("- PREFER: fast deterministic tests; reserve end-to-end coverage for genuinely async flows\n\n")

	b.WriteString("## Error Handling\n\n")
	b.WriteString("- REJECT if: errors are swallowed silently or replaced with a default without a reason\n")
	b.WriteString("- REQUIRE: errors are wrapped with context as they cross a boundary\n")
	b.WriteString("- PREFER: failing fast and explicitly over silent fallbacks\n\n")

	b.WriteString("## Comments\n\n")
	b.WriteString("- REJECT if: a comment merely restates what the code already says\n")
	b.WriteString("- PREFER: comments that explain WHY a non-obvious decision was made\n")

	if personaName != "" {
		fmt.Fprintf(&b, "\n## Persona\n\nReview with the **%s** persona's standards in mind.\n", personaName)
	}

	return b.String()
}

// verdictInstructions tells Copilot how to report its finding: a single JSON
// object with a "verdict" field, so the pre-commit hook can parse it
// mechanically without relying on free-form text.
const verdictInstructions = `## Verdict

Respond with ONLY a single JSON object on the last line of your response, and
nothing else after it. Do not wrap it in a code fence.

- {"verdict":"PASS"} if the diff has no violations of the rules above.
- {"verdict":"FAIL","reason":"<one-sentence reason>"} if it violates a REJECT
  or REQUIRE rule.

Optionally include a "findings" array with per-file details:

{"verdict":"FAIL","reason":"<one-sentence>","findings":[{"file":"path/to/file","line":10,"severity":"CRITICAL","description":"what is wrong"}]}

Severity levels:
- CRITICAL: violates a REJECT rule (hard failure)
- WARNING: violates a REQUIRE rule (mandatory fix)
- SUGGESTION: violates a PREFER rule (advisory)

Omit "line" when the finding applies to the entire file.
If the diff has no violations, you may still include SUGGESTION findings.`

// BuildPrompt combines rules text and a staged diff into a Copilot review
// prompt. It is a pure function: no I/O, no exec. When diff is empty (or
// whitespace-only) there is nothing to review, so it returns ("", false) and
// the caller MAY skip invoking Copilot entirely.
func BuildPrompt(rules, diff string) (string, bool) {
	if strings.TrimSpace(diff) == "" {
		return "", false
	}

	var b strings.Builder
	b.WriteString(rules)
	b.WriteString("\n\n## Staged Diff\n\n```diff\n")
	b.WriteString(diff)
	b.WriteString("\n```\n\n")
	b.WriteString(verdictInstructions)

	return b.String(), true
}
