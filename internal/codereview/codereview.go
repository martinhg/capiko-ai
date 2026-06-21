// Package codereview generates the configuration capiko writes to wire Gentleman
// Guardian Angel (gga) into a project: a managed AGENTS.md rules block (curated,
// gga-optimized REJECT/REQUIRE/PREFER standards plus a pointer to the active
// persona) and the .gga config file. capiko configures gga; gga performs the
// review on commit. capiko owns only its marker-delimited block, so user-authored
// rules in the same file survive a re-sync.
package codereview

import (
	"fmt"
	"strings"
)

// Marker delimiters for capiko's managed block inside the AGENTS.md rules file.
const (
	MarkerStart = "<!-- capiko:review:start -->"
	MarkerEnd   = "<!-- capiko:review:end -->"
)

// Config is the subset of .gga settings capiko manages on the user's behalf.
type Config struct {
	Provider        string // gga provider string, e.g. "claude", "ollama:llama3.2"
	RulesFile       string // rules file gga reads; capiko manages a block within it
	FilePatterns    string // comma-separated globs to review
	ExcludePatterns string // comma-separated globs to skip
	StrictMode      bool   // fail the commit on an ambiguous AI response
	Timeout         int    // provider timeout in seconds
}

// DefaultConfig returns capiko's recommended gga configuration. The provider
// defaults to claude because gga has no Copilot provider — capiko manages Copilot
// for authoring, while the review runs through a separate AI the user can change.
func DefaultConfig() Config {
	return Config{
		Provider:        "claude",
		RulesFile:       "AGENTS.md",
		FilePatterns:    "*",
		ExcludePatterns: "",
		StrictMode:      true,
		Timeout:         300,
	}
}

// Rules returns capiko's curated review-rules block body (without markers), as
// gga-optimized REJECT/REQUIRE/PREFER standards. When personaName is non-empty it
// appends a pointer so the reviewer honors the active persona's standards too.
func Rules(personaName string) string {
	var b strings.Builder

	b.WriteString("# Code Review Rules (capiko-managed)\n\n")
	b.WriteString("These standards are enforced by Gentleman Guardian Angel on every commit.\n")
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

// RenderConfig produces the .gga config file content for the given settings,
// mirroring gga's own config format so it loads as plain shell variables.
func RenderConfig(c Config) string {
	strict := "false"
	if c.StrictMode {
		strict = "true"
	}
	var b strings.Builder
	b.WriteString("# Gentleman Guardian Angel configuration (capiko-managed)\n")
	b.WriteString("# Edit freely; capiko re-applies these values on sync.\n\n")
	fmt.Fprintf(&b, "PROVIDER=%q\n", c.Provider)
	fmt.Fprintf(&b, "FILE_PATTERNS=%q\n", c.FilePatterns)
	fmt.Fprintf(&b, "EXCLUDE_PATTERNS=%q\n", c.ExcludePatterns)
	fmt.Fprintf(&b, "RULES_FILE=%q\n", c.RulesFile)
	fmt.Fprintf(&b, "STRICT_MODE=%q\n", strict)
	fmt.Fprintf(&b, "TIMEOUT=%q\n", fmt.Sprintf("%d", c.Timeout))
	return b.String()
}
