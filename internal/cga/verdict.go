package cga

import (
	"encoding/json"
	"fmt"
	"strings"
)

// VerdictResult is the outcome of parsing Copilot's review response.
type VerdictResult int

const (
	// VerdictPass means the reviewed diff has no rule violations.
	VerdictPass VerdictResult = iota
	// VerdictFail means the reviewed diff violates a REJECT or REQUIRE rule.
	VerdictFail
	// VerdictAmbiguous means the response could not be parsed into a
	// recognizable PASS/FAIL verdict. The caller resolves this via the
	// StrictMode policy — it must never silently default to Pass or Fail.
	VerdictAmbiguous
)

// Severity is the impact level of a per-file finding, as emitted by Copilot
// alongside an optional "findings" array. It mirrors the REJECT/REQUIRE/
// PREFER vocabulary from the review rules (see prompt.go).
type Severity string

const (
	// SeverityCritical marks a finding that violates a REJECT rule.
	SeverityCritical Severity = "CRITICAL"
	// SeverityWarning marks a finding that violates a REQUIRE rule.
	SeverityWarning Severity = "WARNING"
	// SeveritySuggestion marks a finding that violates a PREFER rule.
	SeveritySuggestion Severity = "SUGGESTION"
)

// Finding is a single per-file (optionally per-line) review comment.
// Findings are diagnostic only — they never gate the commit; the top-level
// Verdict.Result/Reason remain the sole PASS/FAIL authority.
type Finding struct {
	File        string
	Line        int // 0 means file-level (no specific line)
	Severity    Severity
	Description string
}

// Verdict is the parsed outcome of a Copilot review response.
type Verdict struct {
	Result   VerdictResult
	Reason   string    // non-empty only for VerdictFail, when Copilot provided one
	Findings []Finding // nil when absent or unparseable — never gating
}

// verdictJSON mirrors the schema BuildPrompt asks Copilot to emit.
type verdictJSON struct {
	Verdict  string          `json:"verdict"`
	Reason   string          `json:"reason"`
	Findings json.RawMessage `json:"findings"`
}

// findingJSON is the wire shape of a single entry in verdictJSON.Findings.
type findingJSON struct {
	File        string `json:"file"`
	Line        int    `json:"line"`
	Severity    string `json:"severity"`
	Description string `json:"description"`
}

// ParseVerdict parses Copilot's --output-format json response into a
// PASS/FAIL/Ambiguous verdict. It is a pure function: no I/O.
//
// Copilot is instructed (via BuildPrompt) to emit a single JSON object, but
// models can wrap it in surrounding prose or a code fence, so ParseVerdict
// first tries the whole input as JSON, then falls back to locating the
// outermost {...} substring. Malformed JSON, a missing/unrecognized verdict
// field, or empty input all resolve to VerdictAmbiguous — never a silent
// PASS or FAIL.
func ParseVerdict(jsonOutput string) Verdict {
	v, ok := decodeVerdictJSON(jsonOutput)
	if !ok {
		return Verdict{Result: VerdictAmbiguous}
	}

	switch strings.ToUpper(strings.TrimSpace(v.Verdict)) {
	case "PASS":
		return Verdict{Result: VerdictPass, Findings: parseFindings(v.Findings)}
	case "FAIL":
		return Verdict{Result: VerdictFail, Reason: v.Reason, Findings: parseFindings(v.Findings)}
	default:
		return Verdict{Result: VerdictAmbiguous}
	}
}

// parseFindings converts the raw "findings" JSON value into []Finding,
// permissively. Any failure (wrong type, malformed entries, absent field)
// returns nil — findings parsing never turns a valid verdict into ambiguous
// and never affects Verdict.Result/Reason.
func parseFindings(raw json.RawMessage) []Finding {
	if len(raw) == 0 {
		return nil
	}

	var entries []findingJSON
	if err := json.Unmarshal(raw, &entries); err != nil {
		return nil
	}

	if len(entries) == 0 {
		return nil
	}

	findings := make([]Finding, len(entries))
	for i, e := range entries {
		findings[i] = Finding{
			File:        e.File,
			Line:        e.Line,
			Severity:    Severity(strings.ToUpper(strings.TrimSpace(e.Severity))),
			Description: e.Description,
		}
	}
	return findings
}

// FormatFindings renders findings for terminal display, one per line, in
// the form "[SEVERITY] file:line - description" (":line" omitted when
// Line is 0). Returns "" when findings is nil or empty.
func FormatFindings(findings []Finding) string {
	if len(findings) == 0 {
		return ""
	}

	lines := make([]string, len(findings))
	for i, f := range findings {
		location := f.File
		if f.Line > 0 {
			location = fmt.Sprintf("%s:%d", f.File, f.Line)
		}
		lines[i] = fmt.Sprintf("[%s] %s - %s", f.Severity, location, f.Description)
	}
	return strings.Join(lines, "\n")
}

func decodeVerdictJSON(s string) (verdictJSON, bool) {
	var v verdictJSON
	if err := json.Unmarshal([]byte(strings.TrimSpace(s)), &v); err == nil {
		return v, true
	}

	start := strings.Index(s, "{")
	end := strings.LastIndex(s, "}")
	if start < 0 || end <= start {
		return verdictJSON{}, false
	}
	if err := json.Unmarshal([]byte(s[start:end+1]), &v); err == nil {
		return v, true
	}

	return verdictJSON{}, false
}
