package rdd

import (
	"reflect"
	"sort"
	"testing"
)

// allLifecycleStates lists every LifecycleState in a fixed order so tests can
// build the exhaustive 12x11 transition matrix deterministically.
var allLifecycleStates = []LifecycleState{
	StateUnreviewed,
	StateReviewing,
	StateFindingsFrozen,
	StateEvidenceClassified,
	StateFixRequired,
	StateFixing,
	StateFixValidating,
	StateReadyFinalVerification,
	StateFinalVerifying,
	StateApproved,
	StateEscalated,
	StateInvalidated,
}

// validTransitionSet mirrors design #780's transition table (spec
// review-lifecycle: "Explicit transition table"). It is intentionally
// duplicated here (not imported from lifecycle.go) so the test asserts the
// public contract of ValidateTransition, not the internal table's identity.
var validTransitionSet = map[LifecycleState]map[LifecycleState]bool{
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
	// Terminal states: no outbound transitions.
	StateApproved:    {},
	StateEscalated:   {},
	StateInvalidated: {},
}

// TestValidateTransition_ExhaustiveMatrix exercises every ordered pair of
// distinct states (12x11 = 132 pairs). Every pair present (with true) in
// validTransitionSet must return nil; every other pair must return a
// non-nil, actionable error, and must never be silently accepted (spec
// review-lifecycle: "Fail-closed").
func TestValidateTransition_ExhaustiveMatrix(t *testing.T) {
	total := 0
	for _, from := range allLifecycleStates {
		for _, to := range allLifecycleStates {
			if from == to {
				continue
			}
			total++
			wantValid := validTransitionSet[from][to]

			t.Run(string(from)+"->"+string(to), func(t *testing.T) {
				err := ValidateTransition(from, to)
				if wantValid {
					if err != nil {
						t.Fatalf("ValidateTransition(%q, %q) = %v, want nil (valid transition)", from, to, err)
					}
					return
				}
				if err == nil {
					t.Fatalf("ValidateTransition(%q, %q) = nil, want non-nil error (invalid transition)", from, to)
				}
				if err.Error() == "" {
					t.Fatalf("ValidateTransition(%q, %q) returned an error with empty message, want actionable error text", from, to)
				}
			})
		}
	}

	// 12 states x 11 possible targets each (excluding self) = 132 pairs.
	const wantPairs = 132
	if total != wantPairs {
		t.Fatalf("exhaustive matrix covered %d pairs, want %d — allLifecycleStates is missing or has extra entries", total, wantPairs)
	}
}

// TestValidateTransition_InvalidatedReachableFromAnyNonTerminal covers the
// spec invariant that `invalidated` is reachable from any non-terminal
// state, independent of the exhaustive matrix above.
func TestValidateTransition_InvalidatedReachableFromAnyNonTerminal(t *testing.T) {
	for _, from := range allLifecycleStates {
		if IsTerminal(from) {
			continue
		}
		t.Run(string(from)+"->invalidated", func(t *testing.T) {
			if err := ValidateTransition(from, StateInvalidated); err != nil {
				t.Fatalf("ValidateTransition(%q, invalidated) = %v, want nil", from, err)
			}
		})
	}
}

// TestValidateTransition_TerminalStatesHaveNoOutboundTransitions covers the
// spec invariant "Terminal finality": terminal states MUST have zero
// outbound transitions, including to themselves and to invalidated.
func TestValidateTransition_TerminalStatesHaveNoOutboundTransitions(t *testing.T) {
	terminals := []LifecycleState{StateApproved, StateEscalated, StateInvalidated}

	for _, from := range terminals {
		for _, to := range allLifecycleStates {
			if from == to {
				continue
			}
			t.Run(string(from)+"->"+string(to), func(t *testing.T) {
				if err := ValidateTransition(from, to); err == nil {
					t.Fatalf("ValidateTransition(%q, %q) = nil, want error: terminal states must have no outbound transitions", from, to)
				}
			})
		}
	}
}

// TestValidateTransition_OneWayCorrection covers the spec invariant that
// fix_validating MUST NOT transition back to fixing or fix_required: exactly
// one fix cycle is permitted.
func TestValidateTransition_OneWayCorrection(t *testing.T) {
	cases := []struct {
		from LifecycleState
		to   LifecycleState
	}{
		{StateFixValidating, StateFixing},
		{StateFixValidating, StateFixRequired},
		{StateFixing, StateFixRequired},
	}

	for _, tt := range cases {
		t.Run(string(tt.from)+"->"+string(tt.to), func(t *testing.T) {
			if err := ValidateTransition(tt.from, tt.to); err == nil {
				t.Fatalf("ValidateTransition(%q, %q) = nil, want error: one-way correction forbids going back", tt.from, tt.to)
			}
		})
	}
}

// TestValidateTransition_SelfTransitionRejected ensures a state cannot
// transition to itself (not present in the transition table for any state,
// including terminals).
func TestValidateTransition_SelfTransitionRejected(t *testing.T) {
	for _, s := range allLifecycleStates {
		t.Run(string(s)+"->"+string(s), func(t *testing.T) {
			if err := ValidateTransition(s, s); err == nil {
				t.Fatalf("ValidateTransition(%q, %q) = nil, want error: self-transitions are not in the table", s, s)
			}
		})
	}
}

// TestAvailableTransitions_MatchesValidTransitionSet covers every state
// (spec bounded-correction / review-status-cli "Transition lookup"; design
// "AvailableTransitions": exported, sorted `[]LifecycleState`). Output must
// be sorted and must exactly match the set of true entries in
// validTransitionSet for that state.
func TestAvailableTransitions_MatchesValidTransitionSet(t *testing.T) {
	for _, state := range allLifecycleStates {
		t.Run(string(state), func(t *testing.T) {
			var want []LifecycleState
			for to, ok := range validTransitionSet[state] {
				if ok {
					want = append(want, to)
				}
			}
			sort.Slice(want, func(i, j int) bool { return want[i] < want[j] })

			got := AvailableTransitions(state)
			if !reflect.DeepEqual(got, want) {
				t.Errorf("AvailableTransitions(%q) = %v, want %v", state, got, want)
			}
		})
	}
}

// TestAvailableTransitions_TerminalStatesReturnEmpty covers terminal states
// explicitly: they have zero outbound transitions, so AvailableTransitions
// must return an empty (nil or zero-length) slice, not error.
func TestAvailableTransitions_TerminalStatesReturnEmpty(t *testing.T) {
	for _, state := range []LifecycleState{StateApproved, StateEscalated, StateInvalidated} {
		t.Run(string(state), func(t *testing.T) {
			got := AvailableTransitions(state)
			if len(got) != 0 {
				t.Errorf("AvailableTransitions(%q) = %v, want empty", state, got)
			}
		})
	}
}

// TestAvailableTransitions_UnknownStateReturnsNil covers the fail-closed
// contract for an unrecognized state: nil, not a panic or a fabricated
// transition set.
func TestAvailableTransitions_UnknownStateReturnsNil(t *testing.T) {
	got := AvailableTransitions(LifecycleState("bogus"))
	if got != nil {
		t.Errorf("AvailableTransitions(%q) = %v, want nil for unknown state", "bogus", got)
	}
}

// TestAvailableTransitions_Sorted pins the deterministic-ordering contract
// directly on a state with multiple outbound transitions (design
// "AvailableTransitions": "sorted output gives deterministic CLI/JSON
// rendering").
func TestAvailableTransitions_Sorted(t *testing.T) {
	got := AvailableTransitions(StateFinalVerifying)
	want := []LifecycleState{StateApproved, StateEscalated, StateInvalidated}
	sort.Slice(want, func(i, j int) bool { return want[i] < want[j] })
	if !reflect.DeepEqual(got, want) {
		t.Errorf("AvailableTransitions(%q) = %v, want sorted %v", StateFinalVerifying, got, want)
	}
}

// TestIsTerminal covers all 12 states: approved, escalated, invalidated must
// report true; every other state must report false.
func TestIsTerminal(t *testing.T) {
	tests := []struct {
		state LifecycleState
		want  bool
	}{
		{StateUnreviewed, false},
		{StateReviewing, false},
		{StateFindingsFrozen, false},
		{StateEvidenceClassified, false},
		{StateFixRequired, false},
		{StateFixing, false},
		{StateFixValidating, false},
		{StateReadyFinalVerification, false},
		{StateFinalVerifying, false},
		{StateApproved, true},
		{StateEscalated, true},
		{StateInvalidated, true},
	}

	if len(tests) != len(allLifecycleStates) {
		t.Fatalf("IsTerminal test table covers %d states, want %d (one per LifecycleState)", len(tests), len(allLifecycleStates))
	}

	for _, tt := range tests {
		t.Run(string(tt.state), func(t *testing.T) {
			if got := IsTerminal(tt.state); got != tt.want {
				t.Fatalf("IsTerminal(%q) = %v, want %v", tt.state, got, tt.want)
			}
		})
	}
}
