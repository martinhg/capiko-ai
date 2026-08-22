package rdd

import "fmt"

// Lens identifies one of the canonical review lenses (spec lens-mandates).
type Lens string

const (
	// LensRisk covers security, privilege boundaries, data exposure, and
	// dependency risks.
	LensRisk Lens = "R1"
	// LensReadability covers naming, complexity, intention clarity, and
	// maintainability.
	LensReadability Lens = "R2"
	// LensReliability covers behavior-first tests, coverage, edge cases,
	// and determinism.
	LensReliability Lens = "R3"
	// LensResilience covers fallbacks, retry/backoff, graceful
	// degradation, and observability.
	LensResilience Lens = "R4"
)

// LensMandate describes a canonical review lens: its identity, display
// title, and the inspection focus a reviewer applying it must cover.
type LensMandate struct {
	ID              Lens   `json:"id"`
	Title           string `json:"title"`
	InspectionFocus string `json:"inspection_focus"`
}

// lensMandates is the fixed, exhaustive set of canonical review lenses
// (spec lens-mandates). Exactly these four lenses are selectable; there is
// no runtime-configurable lens set.
var lensMandates = map[Lens]LensMandate{
	LensRisk: {
		ID:              LensRisk,
		Title:           "Risk",
		InspectionFocus: "security, privilege boundaries, data exposure, dependency risks",
	},
	LensReadability: {
		ID:              LensReadability,
		Title:           "Readability",
		InspectionFocus: "naming, complexity, intention clarity, maintainability",
	},
	LensReliability: {
		ID:              LensReliability,
		Title:           "Reliability",
		InspectionFocus: "behavior-first tests, coverage, edge cases, determinism",
	},
	LensResilience: {
		ID:              LensResilience,
		Title:           "Resilience",
		InspectionFocus: "fallbacks, retry/backoff, graceful degradation, observability",
	},
}

// SelectLenses returns the mandated lenses for a given risk tier (spec
// lens-mandates): tier 0 selects none, tier 1 selects only LensRisk, and
// tier 4 selects all four canonical lenses. Any tier value outside this
// known set fails closed to all four lenses rather than guessing a smaller,
// potentially insufficient set.
func SelectLenses(tier RiskTier) []Lens {
	switch tier {
	case RiskTierStandard:
		return nil
	case RiskTierElevated:
		return []Lens{LensRisk}
	case RiskTierCritical:
		return allLenses()
	default:
		return allLenses()
	}
}

// allLenses returns the four canonical lenses in fixed, deterministic
// order: Risk, Readability, Reliability, Resilience.
func allLenses() []Lens {
	return []Lens{LensRisk, LensReadability, LensReliability, LensResilience}
}

// LookupMandate returns the LensMandate for l. It fails closed: an
// unrecognized lens ID returns an error rather than a fabricated mandate
// (spec lens-mandates).
func LookupMandate(l Lens) (LensMandate, error) {
	mandate, ok := lensMandates[l]
	if !ok {
		return LensMandate{}, fmt.Errorf("rdd: unrecognized lens %q", l)
	}
	return mandate, nil
}
