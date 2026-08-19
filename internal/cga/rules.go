package cga

import (
	"fmt"
	"sort"
	"strings"
)

// LearnedRulesCap is the fixed maximum number of active learned rules
// EnforceCap allows (spec F3.6, design D6): ~500 tokens at 25 rules, well
// within the ~2000 token composition budget.
const LearnedRulesCap = 25

// Scope values for LearnedRule (spec CGA Phase 4 Slice A, design D1):
// "project" rules represent team-grade quality gates, "personal" rules are
// individual style preferences that stay visible to their author but do not
// present as project-wide guidance.
const (
	ScopeProject  = "project"
	ScopePersonal = "personal"
)

// LearnedRule is an approved rule with provenance metadata, persisted per
// engram-project once a user approves a drafted pattern (spec F3.3/F3.4).
type LearnedRule struct {
	ID            string   `json:"id"` // deterministic hash of severity+normalized description
	Severity      Severity `json:"severity"`
	Text          string   `json:"text"` // the rule prose line, e.g. "REQUIRE: missing test coverage"
	EvidenceCount int      `json:"evidence_count"`
	ApprovedAt    string   `json:"approved_at"`     // RFC 3339
	Scope         string   `json:"scope,omitempty"` // "project" or "personal" (design D1); empty means legacy/unset
}

// ResolveScope normalizes a scope value, mapping "" (unset — legacy JSON
// written before this field existed) to ScopeProject so every rule has a
// well-defined scope with zero migration (design D1).
func ResolveScope(s string) string {
	if s == "" {
		return ScopeProject
	}
	return s
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

// EnforceCap trims rules to at most cap entries (spec F3.6, design D6): when
// already within cap it returns rules unchanged (no-op); otherwise it
// removes the lowest-EvidenceCount rules first, breaking ties by oldest
// ApprovedAt, until exactly cap rules remain. It is a pure function: no I/O.
func EnforceCap(rules []LearnedRule, cap int) []LearnedRule {
	if len(rules) <= cap {
		return rules
	}

	// Sort ascending by removal priority: lowest EvidenceCount first, then
	// oldest ApprovedAt first — these are trimmed off the front.
	trimmed := make([]LearnedRule, len(rules))
	copy(trimmed, rules)
	sort.SliceStable(trimmed, func(i, j int) bool {
		if trimmed[i].EvidenceCount != trimmed[j].EvidenceCount {
			return trimmed[i].EvidenceCount < trimmed[j].EvidenceCount
		}
		return trimmed[i].ApprovedAt < trimmed[j].ApprovedAt
	})

	return trimmed[len(trimmed)-cap:]
}

// CheckRetirement partitions rules into active and retired by re-checking
// each rule's evidence against the current detection pass (spec F3.6, design
// D7): a rule stays active only when a pattern with the same severity and
// drafted text exists in patterns with EvidenceCount >= threshold; otherwise
// (no matching pattern, or its evidence fell below threshold) the rule is
// retired. Matching uses DraftRuleText so this stays independent of any rule
// ID scheme. It is a pure function: no I/O.
func CheckRetirement(rules []LearnedRule, patterns []Pattern, threshold int) (active, retired []LearnedRule) {
	current := make(map[string]int, len(patterns))
	for _, p := range patterns {
		current[DraftRuleText(p)] = p.EvidenceCount
	}

	for _, r := range rules {
		if count, ok := current[r.Text]; ok && count >= threshold {
			active = append(active, r)
			continue
		}
		retired = append(retired, r)
	}
	return active, retired
}

// FilterByScope returns the subset of rules matching scope (design D4). A
// scope of "all" or "" returns rules unchanged — no filtering. Otherwise each
// rule's Scope is resolved via ResolveScope (so legacy rules with an empty
// Scope are treated as project-scoped) before comparing against scope.
// Relative order is preserved. It is a pure function: no I/O.
func FilterByScope(rules []LearnedRule, scope string) []LearnedRule {
	if scope == "" || scope == "all" {
		return rules
	}

	var filtered []LearnedRule
	for _, r := range rules {
		if ResolveScope(r.Scope) == scope {
			filtered = append(filtered, r)
		}
	}
	return filtered
}

// FormatLearnedRules renders learned rules for terminal display (used by
// `cga rules`), one line per rule as
// "[SEVERITY] text (evidence: N, scope: project|personal)". A rule's scope
// is resolved via ResolveScope so legacy rules with no stored scope display
// as "project". Returns "" when rules is nil or empty. It is a pure
// function: no I/O.
func FormatLearnedRules(rules []LearnedRule) string {
	if len(rules) == 0 {
		return ""
	}

	lines := make([]string, len(rules))
	for i, r := range rules {
		lines[i] = fmt.Sprintf("[%s] %s (evidence: %d, scope: %s)", r.Severity, r.Text, r.EvidenceCount, ResolveScope(r.Scope))
	}
	return strings.Join(lines, "\n")
}
