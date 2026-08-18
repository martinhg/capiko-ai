package cga

import (
	"encoding/json"
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

// Verdict is the parsed outcome of a Copilot review response.
type Verdict struct {
	Result VerdictResult
	Reason string // non-empty only for VerdictFail, when Copilot provided one
}

// verdictJSON mirrors the schema BuildPrompt asks Copilot to emit.
type verdictJSON struct {
	Verdict string `json:"verdict"`
	Reason  string `json:"reason"`
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
		return Verdict{Result: VerdictPass}
	case "FAIL":
		return Verdict{Result: VerdictFail, Reason: v.Reason}
	default:
		return Verdict{Result: VerdictAmbiguous}
	}
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
