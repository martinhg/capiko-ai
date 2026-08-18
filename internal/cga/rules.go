package cga

import "strings"

// LearnedRule is an approved rule with provenance metadata, persisted per
// engram-project once a user approves a drafted pattern (spec F3.3/F3.4).
type LearnedRule struct {
	ID            string   `json:"id"`             // deterministic hash of severity+normalized description
	Severity      Severity `json:"severity"`
	Text          string   `json:"text"`           // the rule prose line, e.g. "REQUIRE: missing test coverage"
	EvidenceCount int      `json:"evidence_count"`
	ApprovedAt    string   `json:"approved_at"` // RFC 3339
}

// ComposeRules appends a "## Learned Rules" section to staticRules when
// learned is non-empty, one bullet per rule in the order given. When learned
// is empty, staticRules is returned unchanged so existing callers (and the
// no-learned-rules scenario from spec F3.5) see byte-identical output. It is
// a pure function: no I/O.
func ComposeRules(staticRules string, learned []LearnedRule) string {
	if len(learned) == 0 {
		return staticRules
	}

	var b strings.Builder
	b.WriteString(staticRules)
	b.WriteString("\n## Learned Rules\n\n")
	for _, r := range learned {
		b.WriteString("- ")
		b.WriteString(r.Text)
		b.WriteString("\n")
	}
	return b.String()
}
