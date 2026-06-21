// Package headroom integrates the headroom context-compression layer
// (github.com/chopratejas/headroom, Apache-2.0) into the Copilot environment
// capiko manages. capiko *configures* headroom — it never installs, provisions, or
// runs it — exactly as it configures (never provisions) engram. The compression
// value is created by headroom; capiko is the integrator.
//
// It detects the headroom CLI, builds the MCP server entry capiko wires into
// ~/.copilot/mcp-config.json, and supplies the agent guidance block that tells
// Copilot to actually use the compression tools.
package headroom

import "os/exec"

// ServerName is the key capiko uses for headroom's entry in mcp-config.json.
const ServerName = "headroom"

// GuidanceMarkerStart and GuidanceMarkerEnd delimit the capiko-managed block,
// injected into copilot-instructions.md, that tells the agent to use headroom's
// MCP tools. Without it the wired server sits unused — this block is what turns the
// wiring into actual token savings.
const (
	GuidanceMarkerStart = "<!-- capiko:headroom:start -->"
	GuidanceMarkerEnd   = "<!-- capiko:headroom:end -->"
)

// Guidance returns the agent instruction block that pairs with the wired MCP
// server: route bulky, low-signal content through headroom before it floods the
// context window, and prefer compressing over truncating.
func Guidance() string { return guidance }

const guidance = `## Context compression (headroom)

The headroom MCP server is wired into this environment. Use it to keep large,
low-signal content from flooding the context window:

- Before relying on bulky tool output, logs, files, or RAG chunks, route them
  through headroom_compress — it keeps the substance at a fraction of the tokens.
- Use headroom_retrieve to pull back detail you compressed earlier when a task needs it.
- headroom_stats reports what has been saved.

Prefer compressing over truncating: truncation drops information, headroom keeps it.
Skip it for short content, where the round-trip is not worth it.`

// command is the headroom CLI binary. Its MCP server is launched over stdio with
// `headroom mcp serve` — confirmed against headroom v0.26.0: `headroom mcp install`
// writes exactly {command: "headroom", args: ["mcp", "serve"]} (headroom's own
// `type` is "stdio" for Claude Code; capiko uses Copilot CLI's "local", matching
// how it wires engram).
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
