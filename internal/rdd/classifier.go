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

// Classify assigns a RiskTier to a set of changed paths using only
// hardcoded path-pattern evidence (spec rdd-risk-classifier) — never file
// count, diff size, or any other size-based signal. Unmatched paths default
// to RiskTierStandard. Every RiskTierElevated result carries at least one
// named reason identifying the matched category and the path that
// triggered it. Reasons are sorted by category for deterministic output
// regardless of input path order. It is a pure function: no I/O.
func Classify(changedPaths []string) RiskClassification {
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

// matchesAny reports whether path contains any of substrings.
func matchesAny(path string, substrings []string) bool {
	for _, s := range substrings {
		if strings.Contains(path, s) {
			return true
		}
	}
	return false
}
