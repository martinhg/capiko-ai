package copilothooks

import (
	"fmt"
	"strings"
)

// pattern is one of the four v1 guardrail patterns (REQ-3.1): a set of
// bash-syntax (POSIX ERE, as consumed by `grep -Eiq`) conditions and a set of
// PowerShell-syntax (.NET regex, as consumed by `-imatch`) conditions that
// implement the same intent, plus the exact reason text emitted on a match.
// Patterns are checked in order; the first match wins (REQ-3.1).
//
// bashConds/powershellConds are AND-ed together (grep -E and -imatch have no
// lookahead, so an order-independent, multi-clause intent like P3's
// "force flag anywhere + branch token anywhere, regardless of order" is
// expressed as multiple ANDed single-clause conditions rather than one
// alternation). Most patterns need only one condition; P3 needs three.
type pattern struct {
	bashConds       []string
	powershellConds []string
	reason          string
}

// patterns is the frozen v1 pattern set (REQ-3). It is baked in and NOT
// user-editable in v1 (ADR-4) — extending this set is a separate spec change
// (REQ-3.4). Both regex variants implement the same match/non-match behavior
// (REQ-5.2, REQ-3.2/3.3) even though POSIX ERE and .NET regex syntax differ.
var patterns = []pattern{
	{
		// P1 — recursive force-delete of root.
		bashConds:       []string{`(sudo[[:space:]]+)?\brm\b[[:space:]]+(-[^[:space:]]*r[^[:space:]]*f[^[:space:]]*|-[^[:space:]]*f[^[:space:]]*r[^[:space:]]*|--recursive[[:space:]]+.*--force|--force[[:space:]]+.*--recursive)[[:space:]]+/+\*?[[:space:]]*($|[;&|])`},
		powershellConds: []string{`(sudo\s+)?\brm\b\s+(-\S*r\S*f\S*|-\S*f\S*r\S*|--recursive\s+.*--force|--force\s+.*--recursive)\s+/+\*?\s*($|[;&|])`},
		reason:          "Blocked: recursive force-delete targeting root (/) is destructive and irreversible.",
	},
	{
		// P2 — pipe a remote script directly into a shell interpreter.
		bashConds:       []string{`\b(curl|wget)\b[^|]*\|[[:space:]]*(sudo[[:space:]]+)?(sh|bash|zsh)\b`},
		powershellConds: []string{`\b(curl|wget)\b[^|]*\|\s*(sudo\s+)?(sh|bash|zsh)\b`},
		reason:          "Blocked: piping a remote script directly into a shell interpreter bypasses review.",
	},
	{
		// P3 — force-push to a protected branch. Expressed as three ANDed
		// conditions (grep -E / -imatch have no lookahead) so the match is
		// commutative — the force flag may appear before or after the
		// branch name — and the branch token is whitespace/EOL-anchored so
		// hyphen-adjacent names like `release-main`, `mainline`, or
		// `main-backup` are correctly rejected (REQ-3.2/REQ-3.3).
		bashConds: []string{
			`git[[:space:]]+push`,
			`(^|[[:space:]])(--force(-with-lease)?|-f)([[:space:]]|$)`,
			`(^|[[:space:]])(main|master)([[:space:]]|$)`,
		},
		powershellConds: []string{
			`git\s+push`,
			`(^|\s)(--force(-with-lease)?|-f)(\s|$)`,
			`(^|\s)(main|master)(\s|$)`,
		},
		reason: "Blocked: force-pushing to a protected branch (main/master) can overwrite team history.",
	},
	{
		// P4 — world-writable permissions.
		bashConds:       []string{`\bchmod[[:space:]]+(-R[[:space:]]+)?0?777\b`},
		powershellConds: []string{`\bchmod\s+(-R\s+)?0?777\b`},
		reason:          "Blocked: chmod 777 grants world-writable permissions, a common security misconfiguration.",
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
		conds := make([]string, len(pat.bashConds))
		for i, c := range pat.bashConds {
			conds[i] = fmt.Sprintf(`printf '%%s' "$input" | grep -Eiq '%s'`, c)
		}
		script += fmt.Sprintf(
			"if %s; then\n"+
				"  printf '{\"permissionDecision\":\"%s\",\"permissionDecisionReason\":\"%s\"}'\n"+
				"  exit 0\n"+
				"fi\n",
			strings.Join(conds, " && "), decision, pat.reason,
		)
	}
	script += "exit 0\n"
	return script
}

// RenderSessionCheck renders the v1 session-verification hook file (G-CC2).
// Unlike RenderGuardrails it takes no posture: the verification hook is
// always the same script whenever hooks are enabled at all, and it never
// blocks — it only writes a report of what it found. The hook fires on
// EventSessionStart (once per Copilot CLI session) and counts capiko-owned
// files under the resolved Copilot config dir, writing the result to
// <config_dir>/.capiko-verified.json for `capiko-ai doctor` to read.
func RenderSessionCheck() HookFile {
	return HookFile{
		Version: SchemaVersion,
		Hooks: map[string][]Hook{
			EventSessionStart: {{
				Type:       "command",
				Matcher:    "", // sessionStart is a session-level event, not tool-scoped
				Bash:       renderSessionCheckBashScript(),
				PowerShell: renderSessionCheckPowerShellScript(),
				TimeoutSec: TimeoutSeconds,
			}},
		},
	}
}

// renderSessionCheckBashScript builds the verification script executed via
// the bash tool. It resolves the Copilot config dir, counts capiko-owned
// skills/agents/hooks files, checks the MCP config and instructions file for
// capiko wiring, and writes a JSON report. It always exits 0 (REQ:
// verification is non-blocking — a failed check here must never break a
// Copilot session).
func renderSessionCheckBashScript() string {
	return `config_dir="${COPILOT_HOME:-$HOME/.copilot}"
skills=$(find "$config_dir/skills" -maxdepth 1 -name 'capiko-*.md' 2>/dev/null | wc -l | tr -d ' ')
agents=$(find "$config_dir/agents" -maxdepth 1 -name 'capiko-sdd-*.agent.md' 2>/dev/null | wc -l | tr -d ' ')
hooks=$(find "$config_dir/hooks" -maxdepth 1 -name 'capiko-*.json' 2>/dev/null | wc -l | tr -d ' ')
mcp_ok=false
if [ -f "$config_dir/mcp-config.json" ] && grep -q '"engram"' "$config_dir/mcp-config.json" 2>/dev/null; then mcp_ok=true; fi
instructions_ok=false
if [ -f "$config_dir/copilot-instructions.md" ] && grep -q 'capiko:' "$config_dir/copilot-instructions.md" 2>/dev/null; then instructions_ok=true; fi
printf '{"verified_at":"%s","skills":%d,"agents":%d,"hooks":%d,"mcp_engram":%s,"instructions":%s}\n' "$(date -u +%Y-%m-%dT%H:%M:%SZ)" "$skills" "$agents" "$hooks" "$mcp_ok" "$instructions_ok" > "$config_dir/.capiko-verified.json"
exit 0
`
}

// renderSessionCheckPowerShellScript is the PowerShell-syntax equivalent of
// renderSessionCheckBashScript (ADR-5): same resolution, same counts, same
// report shape, same non-blocking exit 0.
func renderSessionCheckPowerShellScript() string {
	return `$configDir = if ($env:COPILOT_HOME) { $env:COPILOT_HOME } else { Join-Path $env:USERPROFILE ".copilot" }
$skills = (Get-ChildItem -Path (Join-Path $configDir "skills") -Filter "capiko-*.md" -File -ErrorAction SilentlyContinue).Count
$agents = (Get-ChildItem -Path (Join-Path $configDir "agents") -Filter "capiko-sdd-*.agent.md" -File -ErrorAction SilentlyContinue).Count
$hooks = (Get-ChildItem -Path (Join-Path $configDir "hooks") -Filter "capiko-*.json" -File -ErrorAction SilentlyContinue).Count
$mcpOk = $false
$mcpConfigPath = Join-Path $configDir "mcp-config.json"
if (Test-Path $mcpConfigPath) {
  if (Select-String -Path $mcpConfigPath -Pattern '"engram"' -Quiet -ErrorAction SilentlyContinue) { $mcpOk = $true }
}
$instructionsOk = $false
$instructionsPath = Join-Path $configDir "copilot-instructions.md"
if (Test-Path $instructionsPath) {
  if (Select-String -Path $instructionsPath -Pattern 'capiko:' -Quiet -ErrorAction SilentlyContinue) { $instructionsOk = $true }
}
$verifiedAt = (Get-Date).ToUniversalTime().ToString("yyyy-MM-ddTHH:mm:ssZ")
$report = [ordered]@{
  verified_at = $verifiedAt
  skills = $skills
  agents = $agents
  hooks = $hooks
  mcp_engram = $mcpOk
  instructions = $instructionsOk
}
($report | ConvertTo-Json -Compress) | Set-Content -Path (Join-Path $configDir ".capiko-verified.json")
exit 0
`
}

// RenderSessionProbe renders the v1 session-probe hook file. It fires on
// EventSessionEnd (once when a Copilot CLI session terminates) and captures
// the full stdin payload and environment variables to a timestamped JSON
// file under ~/.capiko/session-probes/. This is an exploratory/diagnostic
// hook: its output reveals what session metrics Copilot CLI exposes at
// session close, informing the design of a proper reporting feature.
func RenderSessionProbe() HookFile {
	return HookFile{
		Version: SchemaVersion,
		Hooks: map[string][]Hook{
			EventSessionEnd: {{
				Type:       "command",
				Matcher:    "",
				Bash:       renderSessionProbeBashScript(),
				PowerShell: renderSessionProbePowerShellScript(),
				TimeoutSec: TimeoutSeconds,
			}},
		},
	}
}

func renderSessionProbeBashScript() string {
	return `probe_dir="$HOME/.capiko/session-probes"
mkdir -p "$probe_dir"
ts="$(date -u +%Y%m%dT%H%M%SZ)"
stdin_payload="$(cat)"
env_json="{"
first=true
while IFS='=' read -r key value; do
  case "$key" in
    ''|*[!A-Za-z0-9_]*) continue ;;
  esac
  escaped_val="$(printf '%s' "$value" | sed 's/\\/\\\\/g; s/"/\\"/g; s/	/\\t/g')"
  if [ "$first" = true ]; then
    first=false
  else
    env_json="$env_json,"
  fi
  env_json="$env_json\"$key\":\"$escaped_val\""
done <<EOF
$(env)
EOF
env_json="$env_json}"
escaped_stdin="$(printf '%s' "$stdin_payload" | sed 's/\\/\\\\/g; s/"/\\"/g; s/	/\\t/g')"
printf '{"timestamp":"%s","cwd":"%s","stdin_payload":"%s","env":%s}\n' "$ts" "$PWD" "$escaped_stdin" "$env_json" > "$probe_dir/session-$ts.json"
exit 0
`
}

func renderSessionProbePowerShellScript() string {
	return `$probeDir = Join-Path $env:USERPROFILE ".capiko" "session-probes"
if (-not (Test-Path $probeDir)) { New-Item -ItemType Directory -Path $probeDir -Force | Out-Null }
$ts = (Get-Date).ToUniversalTime().ToString("yyyyMMddTHHmmssZ")
$stdinPayload = [Console]::In.ReadToEnd()
$envVars = @{}
Get-ChildItem Env: | ForEach-Object { $envVars[$_.Name] = $_.Value }
$report = [ordered]@{
  timestamp = $ts
  cwd = (Get-Location).Path
  stdin_payload = $stdinPayload
  env = $envVars
}
$report | ConvertTo-Json -Depth 3 | Set-Content -Path (Join-Path $probeDir "session-$ts.json")
exit 0
`
}

// renderPowerShellScript is the PowerShell-syntax equivalent of
// renderBashScript, implementing the same four patterns and the same
// permissionDecision contract (REQ-5.2). It is always rendered alongside the
// bash script (ADR-5); its runtime behavior is not exercised on CI (it only
// executes on a Windows host) — verified here by rendering/structural
// coverage rather than execution (see design "Risks").
//
// The payload variable is named $payload rather than $input: $input is a
// PowerShell AUTOMATIC variable (the pipeline enumerator), and assigning to
// it shadows a reserved name.
func renderPowerShellScript(decision string) string {
	script := "$payload = [Console]::In.ReadToEnd()\n"
	for _, pat := range patterns {
		conds := make([]string, len(pat.powershellConds))
		for i, c := range pat.powershellConds {
			conds[i] = fmt.Sprintf(`$payload -imatch '%s'`, c)
		}
		script += fmt.Sprintf(
			"if (%s) {\n"+
				"  Write-Output '{\"permissionDecision\":\"%s\",\"permissionDecisionReason\":\"%s\"}'\n"+
				"  exit 0\n"+
				"}\n",
			strings.Join(conds, " -and "), decision, pat.reason,
		)
	}
	script += "exit 0\n"
	return script
}
