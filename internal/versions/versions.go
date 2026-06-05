// Package versions centralizes pinned external tool versions so Renovate can
// auto-PR bumps. The marker comments are machine-readable directives consumed by
// the customManager defined in renovate.json — keep them in the exact form
// `// renovate: datasource=<ds> depName=<name>` immediately above each const.
package versions

// CopilotCLI is the GitHub Copilot CLI version capiko-ai targets and is tested
// against. capiko configures Copilot; this records the version it expects.
// renovate: datasource=npm depName=@github/copilot
const CopilotCLI = "1.0.59"
