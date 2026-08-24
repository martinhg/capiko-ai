package rdd

import (
	"fmt"
	"sort"
)

// LifecycleState is a review candidate's position in the R-2 review
// lifecycle (spec review-lifecycle: "typed 12-state machine replacing
// ReviewState.State string"). It is string-based so it stays
// source-compatible with the pre-R-2 ReviewState.State field and requires
// no format migration on read (design #780: "State Field Type").
type LifecycleState string

const (
	// StateUnreviewed is the initial state before `review start` runs.
	StateUnreviewed LifecycleState = "unreviewed"
	// StateReviewing means the candidate identity and risk tier/lenses are
	// frozen and lens results are being captured.
	StateReviewing LifecycleState = "reviewing"
	// StateFindingsFrozen means every mandated lens has submitted a result
	// and findings can no longer be added or changed.
	StateFindingsFrozen LifecycleState = "findings_frozen"
	// StateEvidenceClassified means frozen findings have been evaluated and
	// classified into a fix decision.
	StateEvidenceClassified LifecycleState = "evidence_classified"
	// StateFixRequired means evidence classification determined a fix is
	// needed before final verification.
	StateFixRequired LifecycleState = "fix_required"
	// StateFixing means a fix is in progress.
	StateFixing LifecycleState = "fixing"
	// StateFixValidating means a completed fix is being validated. Exactly
	// one fix cycle is permitted: this state cannot return to fixing or
	// fix_required (spec review-lifecycle: "One-way correction").
	StateFixValidating LifecycleState = "fix_validating"
	// StateReadyFinalVerification means the candidate is ready for final
	// verification, either directly from evidence_classified (no fix
	// needed) or after a validated fix.
	StateReadyFinalVerification LifecycleState = "ready_final_verification"
	// StateFinalVerifying means final verification is in progress.
	StateFinalVerifying LifecycleState = "final_verifying"
	// StateApproved is a terminal state: the candidate passed review.
	StateApproved LifecycleState = "approved"
	// StateEscalated is a terminal state: the candidate requires human
	// escalation and exits the automated lifecycle.
	StateEscalated LifecycleState = "escalated"
	// StateInvalidated is a terminal state reachable from any non-terminal
	// state: the candidate identity is no longer valid (e.g. superseded by
	// a new candidate) and review must restart from scratch.
	StateInvalidated LifecycleState = "invalidated"
)

// validTransitions is the fail-closed transition table (spec
// review-lifecycle: "Explicit transition table"; design #780 "State Machine
// Location": pure transition table in rdd, store validates before write).
// Terminal states are present with an empty target set — spec
// review-lifecycle: "Terminal finality": terminal states MUST have no
// outbound transitions.
var validTransitions = map[LifecycleState]map[LifecycleState]bool{
	StateUnreviewed: {
		StateReviewing:   true,
		StateInvalidated: true,
	},
	StateReviewing: {
		StateFindingsFrozen: true,
		StateInvalidated:    true,
	},
	StateFindingsFrozen: {
		StateEvidenceClassified: true,
		StateInvalidated:        true,
	},
	StateEvidenceClassified: {
		StateFixRequired:            true,
		StateReadyFinalVerification: true,
		StateInvalidated:            true,
	},
	StateFixRequired: {
		StateFixing:      true,
		StateInvalidated: true,
	},
	StateFixing: {
		StateFixValidating: true,
		StateInvalidated:   true,
	},
	StateFixValidating: {
		StateReadyFinalVerification: true,
		StateEscalated:              true,
		StateInvalidated:            true,
	},
	StateReadyFinalVerification: {
		StateFinalVerifying: true,
		StateInvalidated:    true,
	},
	StateFinalVerifying: {
		StateApproved:    true,
		StateEscalated:   true,
		StateInvalidated: true,
	},
	StateApproved:    {},
	StateEscalated:   {},
	StateInvalidated: {},
}

// ValidateTransition reports whether moving from `from` to `to` is allowed
// by the R-2 lifecycle transition table. It returns nil for an allowed
// transition and a non-nil, actionable error for anything else — including
// unknown states, self-transitions, and transitions out of a terminal state
// (spec review-lifecycle: "Fail-closed": any transition not in the table
// MUST be rejected with an actionable error and MUST NOT mutate state).
//
// This function only validates the transition itself; it performs no I/O
// and mutates nothing. Freezing tier/lens immutability once a candidate
// leaves unreviewed is enforced by the store (PR4), not here.
func ValidateTransition(from, to LifecycleState) error {
	targets, known := validTransitions[from]
	if !known {
		return fmt.Errorf("rdd: unknown lifecycle state %q", from)
	}
	if _, validTarget := validTransitions[to]; !validTarget {
		return fmt.Errorf("rdd: unknown lifecycle state %q", to)
	}
	if targets[to] {
		return nil
	}
	if IsTerminal(from) {
		return fmt.Errorf("rdd: invalid transition %q -> %q: %q is a terminal state with no outbound transitions", from, to, from)
	}
	return fmt.Errorf("rdd: invalid transition %q -> %q: not in the lifecycle transition table", from, to)
}

// AvailableTransitions returns the sorted set of valid target states from
// state, per the same fail-closed transition table ValidateTransition uses
// (spec bounded-correction / review-status-cli "Transition lookup"; design
// "AvailableTransitions"). Terminal states return an empty slice (design:
// "no new states"; validTransitions maps every terminal to an empty set).
// An unrecognized state returns nil rather than a fabricated transition
// set — the exported function preserves ValidateTransition's encapsulation
// of validTransitions rather than exposing the map directly (design
// "AvailableTransitions": "Function preserves encapsulation"). Sorting
// gives deterministic CLI/JSON rendering (design: "sorted output gives
// deterministic CLI/JSON rendering").
func AvailableTransitions(state LifecycleState) []LifecycleState {
	targets, known := validTransitions[state]
	if !known {
		return nil
	}

	var transitions []LifecycleState
	for to, ok := range targets {
		if ok {
			transitions = append(transitions, to)
		}
	}
	sort.Slice(transitions, func(i, j int) bool { return transitions[i] < transitions[j] })
	return transitions
}

// IsTerminal reports whether s is a terminal lifecycle state: approved,
// escalated, or invalidated. Terminal states have zero outbound transitions
// (spec review-lifecycle: "Terminal finality").
func IsTerminal(s LifecycleState) bool {
	switch s {
	case StateApproved, StateEscalated, StateInvalidated:
		return true
	default:
		return false
	}
}
