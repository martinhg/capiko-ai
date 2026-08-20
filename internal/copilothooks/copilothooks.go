// Package copilothooks renders and manages the capiko-owned GitHub Copilot
// CLI hook configuration files (v1 schema). It knows the hook file shape and
// the guardrails preset; it does not know about the TUI, state persistence,
// or drift detection — those live in internal/tui, internal/state, and
// internal/drift respectively.
package copilothooks

import "encoding/json"

// Posture selects the guardrails preset's permissionDecision behavior.
type Posture string

// The three user-selectable postures (REQ-4). PostureOff means the guardrail
// preset is fully absent — no hook file is rendered or written for it
// (REQ-4.4); callers must not pass PostureOff to RenderGuardrails.
const (
	PostureOff    Posture = "off"
	PostureWarn   Posture = "warn"   // permissionDecision = "ask" on a match
	PostureStrict Posture = "strict" // permissionDecision = "deny" on a match
)

// v1 hook file constants (REQ-2).
const (
	// GuardrailsFile is the filename of the rendered guardrails hook file
	// under copilot.Host.HooksDir.
	GuardrailsFile = "capiko-guardrails.json"
	// FilePrefix marks capiko ownership: capiko only ever touches
	// <HooksDir>/capiko-*.json files.
	FilePrefix = "capiko-"
	// SchemaVersion is the pinned v1 hooks-file schema version (REQ-2.4).
	SchemaVersion = 1
	// TimeoutSeconds is the pinned per-hook timeout budget (REQ-2.3). Exceeding
	// it is FAIL-OPEN (Copilot proceeds as if the hook were absent), unlike a
	// non-zero exit or crash, which is FAIL-CLOSED (denies).
	TimeoutSeconds = 5
	// MatcherBash is the confirmed Copilot CLI tool name matched by the
	// guardrails preset (REQ-2.2). Copilot compiles the matcher anchored
	// internally; no "^(?:...)$" wrapping is authored here.
	MatcherBash = "bash"
	// EventPreToolUse is the hook event name used as the Hooks map key.
	EventPreToolUse = "preToolUse"
	// EventSessionStart is the hook event name fired once at the start of a
	// Copilot CLI session, used as the Hooks map key for the session
	// verification hook (G-CC2).
	EventSessionStart = "sessionStart"
	// SessionCheckFile is the filename of the rendered session-verification
	// hook file under copilot.Host.HooksDir.
	SessionCheckFile = "capiko-sessioncheck.json"
	// EventSessionEnd is the hook event name fired once when a Copilot CLI
	// session terminates, used for the session-probe hook that captures
	// session metrics (stdin payload, env vars) for reporting.
	EventSessionEnd = "sessionEnd"
	// SessionProbeFile is the filename of the rendered session-probe hook
	// file under copilot.Host.HooksDir.
	SessionProbeFile = "capiko-sessionprobe.json"
)

// HookFile is the v1 Copilot CLI hook configuration file shape. Hooks is an
// object keyed by event name (e.g. EventPreToolUse) — the event is the map
// key, not a field on Hook (REQ-2.1, verified against the real Copilot CLI
// schema, engram #395).
type HookFile struct {
	Version int               `json:"version"`
	Hooks   map[string][]Hook `json:"hooks"`
}

// Hook is a single command-type hook entry. A single entry always carries
// BOTH the bash and the powershell script bodies; Copilot CLI itself selects
// which one to execute based on its host OS at runtime (REQ-5, ADR-5) —
// capiko never chooses an OS at render time.
type Hook struct {
	Type       string `json:"type"`
	Matcher    string `json:"matcher"`
	Bash       string `json:"bash"`
	PowerShell string `json:"powershell"`
	TimeoutSec int    `json:"timeoutSec"`
}

// Marshal renders f as indented JSON plus a trailing newline, for a stable,
// golden-friendly byte sequence.
func Marshal(f HookFile) ([]byte, error) {
	data, err := json.MarshalIndent(f, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(data, '\n'), nil
}
