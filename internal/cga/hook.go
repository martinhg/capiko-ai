package cga

import (
	"fmt"
	"strings"
)

// Marker delimiters for capiko's managed block inside .git/hooks/pre-commit,
// consumed by internal/githooks.WriteBlock/RemoveBlock.
const (
	MarkerStart = "# >>> capiko:cga:pre-commit >>>"
	MarkerEnd   = "# <<< capiko:cga:pre-commit <<<"
)

// defaultTimeoutSeconds is used when RenderHook is given a non-positive
// timeout.
const defaultTimeoutSeconds = 120

// RenderHook returns the bash pre-commit script body (without a shebang —
// githooks.WriteBlock seeds that separately) that: collects the staged diff,
// short-circuits when there is nothing to review, invokes
// `copilot -p --output-format json` with the given rules baked in, parses
// the resulting verdict (via jq, falling back to grep when jq is
// unavailable), and exits 0 on PASS or 1 on FAIL. An ambiguous verdict or a
// timed-out/failed copilot invocation is resolved by the StrictMode policy:
// strict blocks the commit (exit 1), non-strict allows it (exit 0).
//
// This is Phase 0: bash-only, no PowerShell — git hooks run as POSIX shell
// scripts on every platform, including Windows (Git for Windows ships sh).
func RenderHook(rules string, strict bool, timeout int) string {
	if timeout <= 0 {
		timeout = defaultTimeoutSeconds
	}
	strictVal := "false"
	if strict {
		strictVal = "true"
	}

	var b strings.Builder

	fmt.Fprintf(&b, "STRICT=%s\n", strictVal)
	fmt.Fprintf(&b, "TIMEOUT=%d\n", timeout)
	fmt.Fprintf(&b, "rules=%s\n", shellSingleQuote(rules))
	fmt.Fprintf(&b, "verdict_instructions=%s\n", shellSingleQuote(verdictInstructions))

	b.WriteString(`diff="$(git diff --cached --no-color)"` + "\n")
	b.WriteString("if [ -z \"$diff\" ]; then\n")
	b.WriteString("  exit 0\n")
	b.WriteString("fi\n")

	b.WriteString("prompt=\"$rules\n\n## Staged Diff\n\n```diff\n$diff\n```\n\n$verdict_instructions\"\n")

	b.WriteString(`result="$(printf '%s' "$prompt" | timeout "$TIMEOUT" copilot -p --output-format json 2>/dev/null)"` + "\n")
	b.WriteString("rc=$?\n")
	b.WriteString("if [ $rc -ne 0 ]; then\n")
	b.WriteString("  if [ \"$STRICT\" = \"true\" ]; then\n")
	b.WriteString("    echo \"CGA: copilot review timed out or failed; blocking commit (StrictMode).\" >&2\n")
	b.WriteString("    exit 1\n")
	b.WriteString("  fi\n")
	b.WriteString("  exit 0\n")
	b.WriteString("fi\n")

	b.WriteString(`verdict="$(printf '%s' "$result" | jq -r '.verdict // empty' 2>/dev/null)"` + "\n")
	b.WriteString("if [ -z \"$verdict\" ]; then\n")
	b.WriteString(`  verdict="$(printf '%s' "$result" | grep -o '"verdict"[[:space:]]*:[[:space:]]*"[A-Za-z]*"' | grep -o '"[A-Za-z]*"$' | tr -d '"' | tr '[:lower:]' '[:upper:]')"` + "\n")
	b.WriteString("fi\n")

	b.WriteString("case \"$verdict\" in\n")
	b.WriteString("  PASS)\n")
	b.WriteString("    exit 0\n")
	b.WriteString("    ;;\n")
	b.WriteString("  FAIL)\n")
	b.WriteString(`    reason="$(printf '%s' "$result" | jq -r '.reason // empty' 2>/dev/null)"` + "\n")
	b.WriteString("    echo \"CGA: commit blocked by review. $reason\" >&2\n")
	b.WriteString("    exit 1\n")
	b.WriteString("    ;;\n")
	b.WriteString("  *)\n")
	b.WriteString("    if [ \"$STRICT\" = \"true\" ]; then\n")
	b.WriteString("      echo \"CGA: ambiguous review verdict; blocking commit (StrictMode).\" >&2\n")
	b.WriteString("      exit 1\n")
	b.WriteString("    fi\n")
	b.WriteString("    exit 0\n")
	b.WriteString("    ;;\n")
	b.WriteString("esac\n")

	return b.String()
}

// shellSingleQuote wraps s in single quotes so it is embedded verbatim in a
// generated bash script, escaping any single quotes already present using
// the standard close-quote/backslash-quote/open-quote technique.
func shellSingleQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}
