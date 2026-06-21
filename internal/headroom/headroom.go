// Package headroom integrates the headroom context-compression layer
// (github.com/chopratejas/headroom, Apache-2.0) into the Copilot environment
// capiko manages. capiko *configures* headroom — it never installs, provisions, or
// runs it — exactly as it configures (never provisions) engram. The compression
// value is created by headroom; capiko is the integrator.
//
// This package is the foundation slice: it detects the headroom CLI and builds the
// MCP server entry. Wiring that entry into ~/.copilot/mcp-config.json (state-
// tracked, drift-detectable, uninstallable) lands in a later slice.
package headroom

import "os/exec"

// ServerName is the key capiko uses for headroom's entry in mcp-config.json.
const ServerName = "headroom"

// command is the headroom CLI binary. Its MCP server is launched over stdio with
// `headroom mcp serve` — headroom's documented MCP invocation. This is not a
// guaranteed-stable public contract, so it must be confirmed against the installed
// tool (e.g. `headroom mcp install`) before capiko writes it into a user's config.
const command = "headroom"

// lookPath is a test seam over exec.LookPath.
var lookPath = exec.LookPath

// Detected reports whether the headroom CLI is on PATH. capiko configures headroom
// only when present; absence is a clean no-op, never an error.
func Detected() bool {
	_, err := lookPath(command)
	return err == nil
}

// CopilotCLIEntry builds headroom's MCP server entry for Copilot CLI's
// mcp-config.json (top-level key "mcpServers"). The shape mirrors how capiko wires
// engram: a local stdio server exposing all of headroom's tools
// (headroom_compress, headroom_retrieve, headroom_stats).
func CopilotCLIEntry() map[string]any {
	return map[string]any{
		"type":    "local",
		"command": command,
		"args":    []string{"mcp", "serve"},
		"tools":   []string{"*"},
	}
}
