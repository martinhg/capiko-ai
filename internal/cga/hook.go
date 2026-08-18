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

// Marker delimiters for capiko's managed block inside .git/hooks/post-commit,
// consumed by internal/githooks.WriteBlock/RemoveBlock. Distinct from
// MarkerStart/MarkerEnd so the pre-commit and post-commit blocks never
// collide inside the same managed-block scanner.
const (
	PostCommitMarkerStart = "# >>> capiko:cga:post-commit >>>"
	PostCommitMarkerEnd   = "# <<< capiko:cga:post-commit <<<"
)

// defaultTimeoutSeconds is used when RenderPreCommitHook is given a
// non-positive timeout.
const defaultTimeoutSeconds = 120

// defaultRotationCap is the number of most-recent findings-log entries kept
// after each append when RenderPreCommitHook is given a non-positive
// rotationCap.
const defaultRotationCap = 200

// RenderPreCommitHook returns the bash pre-commit script body (without a
// shebang — githooks.WriteBlock seeds that separately) that: collects the
// staged diff, short-circuits when there is nothing to review, invokes
// `copilot -p --output-format json` with the given rules baked in, parses
// the resulting verdict (via jq, falling back to grep when jq is
// unavailable), and exits 0 on PASS or 1 on FAIL. An ambiguous verdict or a
// timed-out/failed copilot invocation is resolved by the StrictMode policy:
// strict blocks the commit (exit 1), non-strict allows it (exit 0).
//
// When logPath is non-empty, the script also appends a findings-log JSONL
// entry (commit_sha "pending" until the post-commit hook patches it — see
// RenderPostCommitHook) after every review outcome, then rotates the log to
// the most recent rotationCap entries (rotationCap <= 0 defaults to 200).
// Findings-log failures never block the commit: they warn to stderr and
// fall through. An empty logPath disables findings-log persistence entirely.
//
// This is Phase 0: bash-only, no PowerShell — git hooks run as POSIX shell
// scripts on every platform, including Windows (Git for Windows ships sh).
func RenderPreCommitHook(rules string, strict bool, timeout int, logPath string, rotationCap int) string {
	if timeout <= 0 {
		timeout = defaultTimeoutSeconds
	}
	if rotationCap <= 0 {
		rotationCap = defaultRotationCap
	}
	strictVal := "false"
	if strict {
		strictVal = "true"
	}

	var b strings.Builder

	b.WriteString("[ \"$CGA_RUNNING\" = \"1\" ] && exit 0\n")
	b.WriteString("export CGA_RUNNING=1\n\n")

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

	b.WriteString(`printf '%s' "$result" | jq -r ` +
		`'.findings[]? | "  [\(.severity)] \(.file)\(if .line and .line > 0 then ":\(.line)" else "" end) - \(.description)"' ` +
		"2>/dev/null >&2\n")

	if logPath != "" {
		b.WriteString(renderFindingsAppendBlock(logPath, rotationCap))
	}

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

// renderFindingsAppendBlock returns the bash block that composes one JSONL
// findings-log entry from $result/$verdict, appends it to logPath (creating
// the parent directory first), and rotates the file to the most recent
// rotationCap lines. Every step is guarded so a log-write failure — a
// missing jq, a read-only filesystem, a full disk — warns to stderr and
// falls through without ever exiting non-zero; findings-log persistence
// must never block a commit.
func renderFindingsAppendBlock(logPath string, rotationCap int) string {
	var b strings.Builder

	b.WriteString("\n# cga: append findings log entry\n")
	fmt.Fprintf(&b, "logPath=%s\n", shellSingleQuote(logPath))
	b.WriteString(`mkdir -p "$(dirname "$logPath")" 2>/dev/null || true` + "\n")
	b.WriteString(`log_entry="$(printf '%s' "$result" | jq -c ` +
		`--arg ts "$(date -u +%Y-%m-%dT%H:%M:%SZ)" ` +
		`--arg parent "$(git rev-parse HEAD 2>/dev/null)" ` +
		`--arg verdict "$verdict" ` +
		`'{"timestamp":$ts,"parent_sha":$parent,"commit_sha":"pending","verdict":$verdict,"reason":(.reason // ""),"findings":(.findings // [])}' ` +
		`2>/dev/null)"` + "\n")
	b.WriteString("if [ -n \"$log_entry\" ]; then\n")
	b.WriteString("  if printf '%s\\n' \"$log_entry\" >> \"$logPath\" 2>/dev/null; then\n")
	b.WriteString(`    tmp_log="$(mktemp "${logPath}.XXXXXX" 2>/dev/null || echo "")"` + "\n")
	b.WriteString("    if [ -n \"$tmp_log\" ]; then\n")
	fmt.Fprintf(&b, "      tail -n %d \"$logPath\" > \"$tmp_log\" 2>/dev/null && mv \"$tmp_log\" \"$logPath\" 2>/dev/null || rm -f \"$tmp_log\" 2>/dev/null\n", rotationCap)
	b.WriteString("    fi\n")
	b.WriteString("  else\n")
	b.WriteString("    echo \"CGA: warning: failed to write findings log to $logPath\" >&2 || true\n")
	b.WriteString("  fi\n")
	b.WriteString("else\n")
	b.WriteString("  echo \"CGA: warning: failed to compose findings log entry\" >&2 || true\n")
	b.WriteString("fi\n")
	b.WriteString("# cga: end findings log entry\n")

	return b.String()
}

// RenderPostCommitHook returns the bash post-commit script body (without a
// shebang — githooks.WriteBlock seeds that separately) that patches the
// findings-log's last JSONL entry with the real commit SHA once the commit
// has landed: pre-commit writes commit_sha:"pending" because the SHA does
// not exist yet, and this hook fills it in via `git rev-parse HEAD`.
//
// The last line is left untouched (no-op) when it does not carry
// "commit_sha":"pending" — e.g. logging was disabled, the log is empty, or
// the entry was already patched by a previous run. The patch itself never
// mutates the file in place: it writes to a temp file and renames it over
// logPath (tmp+mv), which is POSIX-portable, unlike `sed -i` whose in-place
// flag differs between BSD and GNU. As with the pre-commit append, every
// step is guarded so a failure here warns to stderr (or silently no-ops)
// and never fails the hook.
//
// An empty logPath returns "" — the post-commit block is omitted entirely
// when findings-log persistence is disabled.
func RenderPostCommitHook(logPath string) string {
	if logPath == "" {
		return ""
	}

	var b strings.Builder

	fmt.Fprintf(&b, "logPath=%s\n", shellSingleQuote(logPath))
	b.WriteString(`git_dir="$(git rev-parse --git-dir 2>/dev/null)"` + "\n")
	b.WriteString("if [ -z \"$git_dir\" ]; then\n  exit 0\nfi\n")
	b.WriteString("[ -f \"$logPath\" ] || exit 0\n")
	b.WriteString(`last_line="$(tail -n 1 "$logPath" 2>/dev/null)"` + "\n")
	b.WriteString("[ -n \"$last_line\" ] || exit 0\n")
	b.WriteString("case \"$last_line\" in\n")
	b.WriteString(`  *'"commit_sha":"pending"'*) ;;` + "\n")
	b.WriteString("  *) exit 0 ;;\n")
	b.WriteString("esac\n")
	b.WriteString(`commit_sha="$(git rev-parse HEAD 2>/dev/null)"` + "\n")
	b.WriteString("[ -n \"$commit_sha\" ] || exit 0\n")
	b.WriteString(`patched_line="$(printf '%s' "$last_line" | jq -c --arg sha "$commit_sha" '.commit_sha=$sha' 2>/dev/null)"` + "\n")
	b.WriteString("[ -n \"$patched_line\" ] || exit 0\n")
	b.WriteString(`tmp_log="$(mktemp "${logPath}.XXXXXX" 2>/dev/null || echo "")"` + "\n")
	b.WriteString("[ -n \"$tmp_log\" ] || exit 0\n")
	b.WriteString(`total_lines="$(wc -l < "$logPath" 2>/dev/null | tr -d '[:space:]')"` + "\n")
	b.WriteString("case \"$total_lines\" in\n")
	b.WriteString("  ''|*[!0-9]*) rm -f \"$tmp_log\" 2>/dev/null; exit 0 ;;\n")
	b.WriteString("esac\n")
	b.WriteString("if [ \"$total_lines\" -gt 1 ]; then\n")
	b.WriteString(`  head -n "$((total_lines - 1))" "$logPath" > "$tmp_log" 2>/dev/null` + "\n")
	b.WriteString("fi\n")
	b.WriteString(`printf '%s\n' "$patched_line" >> "$tmp_log" 2>/dev/null` + "\n")
	b.WriteString(`mv "$tmp_log" "$logPath" 2>/dev/null || rm -f "$tmp_log" 2>/dev/null` + "\n")
	b.WriteString("exit 0\n")

	return b.String()
}

// shellSingleQuote wraps s in single quotes so it is embedded verbatim in a
// generated bash script, escaping any single quotes already present using
// the standard close-quote/backslash-quote/open-quote technique.
func shellSingleQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}
