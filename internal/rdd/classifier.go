package rdd

import (
	"fmt"
	"sort"
	"strings"
)

// RiskTier is the outcome of risk classification. R-1 supports only
// RiskTierStandard and RiskTierElevated; higher tiers (e.g. tier 4) are
// added in R-2 by extending pattern (see riskPatterns below), never by
// changing this type.
type RiskTier int

const (
	// RiskTierStandard is the default tier: no sensitive path evidence.
	RiskTierStandard RiskTier = iota
	// RiskTierElevated means at least one changed path matched a
	// hardcoded sensitive-path pattern.
	RiskTierElevated
)

const (
	// RiskTierCritical is the highest classification tier: at least one
	// changed path matched a hot-path pattern (auth/security, webhook,
	// payments, or review-system code). Values 2 and 3 are reserved for
	// future intermediate tiers and are intentionally unused (spec
	// risk-classifier-extension). RiskTierCritical always sets Lenses to
	// 4 — all canonical review lenses apply.
	RiskTierCritical RiskTier = 4
)

// RiskClassification is the result of Classify: the assigned tier, the
// named reasons that justified it (empty for tier 0), and how many review
// lenses apply (0 for tier 0, 1 for tier 1 in R-1; R-2 extends this for its
// 4-lens canonical set).
type RiskClassification struct {
	Tier    RiskTier `json:"tier"`
	Reasons []string `json:"reasons"`
	Lenses  int      `json:"lenses"`
}

// riskPattern is one hardcoded, named path-evidence rule. category is a
// short human label folded into the reason string; substrings are the
// case-sensitive path fragments that trigger it.
type riskPattern struct {
	category   string
	substrings []string
}

// riskPatterns is the hardcoded evidence table for tier 1 classification
// (spec rdd-risk-classifier: "path-evidence-only classification"). Order is
// fixed and does not affect Classify's output — reasons are always sorted
// by category before being returned, so the declared order here only
// affects readability, not behavior.
var riskPatterns = []riskPattern{
	{category: "authentication-sensitive", substrings: []string{"auth"}},
	{category: "cryptography-sensitive", substrings: []string{"crypto"}},
	{category: "permissions-sensitive", substrings: []string{"permission", "acl"}},
	{category: "secrets-sensitive", substrings: []string{"secret", "credential"}},
}

// tier4Patterns is the hardcoded evidence table for tier 4 (critical)
// classification (spec risk-classifier-extension). It is deliberately
// separate from riskPatterns: a tier-4 hot-path match takes precedence over
// any tier-1 match, and a single tier-4 match is sufficient — there is no
// accumulation. Order is fixed and does not affect Classify's output.
var tier4Patterns = []riskPattern{
	{category: "auth-critical", substrings: []string{"auth"}},
	{category: "security-critical", substrings: []string{"security"}},
	{category: "webhook-critical", substrings: []string{"webhook"}},
	{category: "payments-critical", substrings: []string{"payment"}},
	{category: "review-system-code", substrings: []string{"internal/rdd/", "internal/reviewstore/"}},
}

// Classify assigns a RiskTier to a set of changed paths using only
// hardcoded path-pattern evidence (spec rdd-risk-classifier) — never file
// count, diff size, or any other size-based signal. Unmatched paths default
// to RiskTierStandard. Every RiskTierElevated result carries at least one
// named reason identifying the matched category and the path that
// triggered it. Reasons are sorted by category for deterministic output
// regardless of input path order. It is a pure function: no I/O.
func Classify(changedPaths []string) RiskClassification {
	if classification, matched := classifyTier4(changedPaths); matched {
		return classification
	}

	reasonByCategory := make(map[string]string)

	for _, path := range changedPaths {
		for _, p := range riskPatterns {
			if _, matched := reasonByCategory[p.category]; matched {
				continue
			}
			if matchesAny(path, p.substrings) {
				reasonByCategory[p.category] = fmt.Sprintf(
					"changed path matches %s pattern: %s", p.category, path,
				)
			}
		}
	}

	if len(reasonByCategory) == 0 {
		return RiskClassification{Tier: RiskTierStandard, Reasons: nil, Lenses: 0}
	}

	categories := make([]string, 0, len(reasonByCategory))
	for category := range reasonByCategory {
		categories = append(categories, category)
	}
	sort.Strings(categories)

	reasons := make([]string, len(categories))
	for i, category := range categories {
		reasons[i] = reasonByCategory[category]
	}

	return RiskClassification{Tier: RiskTierElevated, Reasons: reasons, Lenses: 1}
}

// classifyTier4 checks changedPaths against tier4Patterns first, before any
// tier-1 evidence is considered (spec risk-classifier-extension: tier 4
// precedence). A single match against any tier-4 category is sufficient —
// there is no accumulation requirement, unlike tier 1. When matched is
// true, the returned RiskClassification has Tier=RiskTierCritical and
// Lenses=4 (all canonical lenses), with reasons sorted by category for
// deterministic output.
func classifyTier4(changedPaths []string) (classification RiskClassification, matched bool) {
	reasonByCategory := make(map[string]string)

	for _, path := range changedPaths {
		for _, p := range tier4Patterns {
			if _, ok := reasonByCategory[p.category]; ok {
				continue
			}
			if matchesAny(path, p.substrings) {
				reasonByCategory[p.category] = fmt.Sprintf(
					"changed path matches %s pattern: %s", p.category, path,
				)
			}
		}
	}

	if len(reasonByCategory) == 0 {
		return RiskClassification{}, false
	}

	categories := make([]string, 0, len(reasonByCategory))
	for category := range reasonByCategory {
		categories = append(categories, category)
	}
	sort.Strings(categories)

	reasons := make([]string, len(categories))
	for i, category := range categories {
		reasons[i] = reasonByCategory[category]
	}

	return RiskClassification{Tier: RiskTierCritical, Reasons: reasons, Lenses: 4}, true
}

// matchesAny reports whether path contains any of substrings.
func matchesAny(path string, substrings []string) bool {
	for _, s := range substrings {
		if strings.Contains(path, s) {
			return true
		}
	}
	return false
}
