package copilothooks

import "fmt"

// pattern is one of the four v1 guardrail patterns (REQ-3.1): a bash-syntax
// (POSIX ERE, as consumed by `grep -Eiq`) regex, a PowerShell-syntax (.NET
// regex, as consumed by `-imatch`) equivalent, and the exact reason text
// emitted on a match. Patterns are checked in order; the first match wins
// (REQ-3.1).
type pattern struct {
	bashRegexp       string
	powershellRegexp string
	reason           string
}

// patterns is the frozen v1 pattern set (REQ-3). It is baked in and NOT
// user-editable in v1 (ADR-4) — extending this set is a separate spec change
// (REQ-3.4). Both regex variants implement the same match/non-match behavior
// (REQ-5.2, REQ-3.2/3.3) even though POSIX ERE and .NET regex syntax differ.
var patterns = []pattern{
	{
		// P1 — recursive force-delete of root.
		bashRegexp:       `(sudo[[:space:]]+)?\brm\b[[:space:]]+(-[^[:space:]]*r[^[:space:]]*f[^[:space:]]*|-[^[:space:]]*f[^[:space:]]*r[^[:space:]]*|--recursive[[:space:]]+.*--force|--force[[:space:]]+.*--recursive)[[:space:]]+/+\*?[[:space:]]*($|[;&|])`,
		powershellRegexp: `(sudo\s+)?\brm\b\s+(-\S*r\S*f\S*|-\S*f\S*r\S*|--recursive\s+.*--force|--force\s+.*--recursive)\s+/+\*?\s*($|[;&|])`,
		reason:           "Blocked: recursive force-delete targeting root (/) is destructive and irreversible.",
	},
	{
		// P2 — pipe a remote script directly into a shell interpreter.
		bashRegexp:       `\b(curl|wget)\b[^|]*\|[[:space:]]*(sudo[[:space:]]+)?(sh|bash|zsh)\b`,
		powershellRegexp: `\b(curl|wget)\b[^|]*\|\s*(sudo\s+)?(sh|bash|zsh)\b`,
		reason:           "Blocked: piping a remote script directly into a shell interpreter bypasses review.",
	},
	{
		// P3 — force-push to a protected branch.
		bashRegexp:       `\bgit[[:space:]]+push[[:space:]]+(--force(-with-lease)?|-f)\b.*\b(origin[[:space:]]+)?(main|master)\b`,
		powershellRegexp: `\bgit\s+push\s+(--force(-with-lease)?|-f)\b.*\b(origin\s+)?(main|master)\b`,
		reason:           "Blocked: force-pushing to a protected branch (main/master) can overwrite team history.",
	},
	{
		// P4 — world-writable permissions.
		bashRegexp:       `\bchmod[[:space:]]+(-R[[:space:]]+)?0?777\b`,
		powershellRegexp: `\bchmod\s+(-R\s+)?0?777\b`,
		reason:           "Blocked: chmod 777 grants world-writable permissions, a common security misconfiguration.",
	},
}

// decisionFor maps posture to the permissionDecision literal emitted on a
// pattern match (REQ-4.1, REQ-4.2). Only PostureWarn and PostureStrict render
// a script; PostureOff is not a valid render target (REQ-4.4 — off means no
// hook file at all, decided by the caller before it ever reaches this
// package).
func decisionFor(p Posture) (string, error) {
	switch p {
	case PostureStrict:
		return "deny", nil
	case PostureWarn:
		return "ask", nil
	default:
		return "", fmt.Errorf("copilothooks: RenderGuardrails: unsupported posture %q (want %q or %q)", p, PostureWarn, PostureStrict)
	}
}

// RenderGuardrails renders the v1 guardrails hook file for posture. It always
// emits both the bash and the powershell script bodies in a single entry
// (ADR-5, SC-13) — Copilot CLI itself selects which one to execute based on
// its host OS at runtime, so RenderGuardrails takes no OS/goos parameter and
// has no OS discriminator. posture must be PostureWarn or PostureStrict.
func RenderGuardrails(posture Posture) (HookFile, error) {
	decision, err := decisionFor(posture)
	if err != nil {
		return HookFile{}, err
	}
	return HookFile{
		Version: SchemaVersion,
		Hooks: map[string][]Hook{
			EventPreToolUse: {{
				Type:       "command",
				Matcher:    MatcherBash,
				Bash:       renderBashScript(decision),
				PowerShell: renderPowerShellScript(decision),
				TimeoutSec: TimeoutSeconds,
			}},
		},
	}, nil
}

// renderBashScript builds the guardrail script executed via the bash tool
// (ADR-3): it reads the preToolUse payload from stdin, checks it against each
// baked-in pattern in order, and on the first match prints a
// permissionDecision JSON object and exits. No match prints nothing to
// stdout (silent allow, REQ-4.3) and the script still exits 0.
func renderBashScript(decision string) string {
	script := "input=\"$(cat)\"\n"
	for _, pat := range patterns {
		script += fmt.Sprintf(
			"if printf '%%s' \"$input\" | grep -Eiq '%s'; then\n"+
				"  printf '{\"permissionDecision\":\"%s\",\"permissionDecisionReason\":\"%s\"}'\n"+
				"  exit 0\n"+
				"fi\n",
			pat.bashRegexp, decision, pat.reason,
		)
	}
	script += "exit 0\n"
	return script
}

// renderPowerShellScript is the PowerShell-syntax equivalent of
// renderBashScript, implementing the same four patterns and the same
// permissionDecision contract (REQ-5.2). It is always rendered alongside the
// bash script (ADR-5); its runtime behavior is not exercised on CI (it only
// executes on a Windows host) — verified here by rendering/structural
// coverage rather than execution (see design "Risks").
func renderPowerShellScript(decision string) string {
	script := "$input = [Console]::In.ReadToEnd()\n"
	for _, pat := range patterns {
		script += fmt.Sprintf(
			"if ($input -imatch '%s') {\n"+
				"  Write-Output '{\"permissionDecision\":\"%s\",\"permissionDecisionReason\":\"%s\"}'\n"+
				"  exit 0\n"+
				"}\n",
			pat.powershellRegexp, decision, pat.reason,
		)
	}
	script += "exit 0\n"
	return script
}
