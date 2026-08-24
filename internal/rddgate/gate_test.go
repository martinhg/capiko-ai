package rddgate

import (
	"testing"

	"github.com/martinhg/capiko-ai/internal/rdd"
)

func TestEvaluate(t *testing.T) {
	allRelations := []rdd.Relation{
		rdd.RelationExact,
		rdd.RelationCompatibleBaseAdvance,
		rdd.RelationProvableContraction,
		rdd.RelationChanged,
		rdd.RelationUnrelated,
		rdd.RelationAmbiguous,
		rdd.RelationUnknown,
	}

	accepted := map[rdd.Relation]bool{
		rdd.RelationExact:                 true,
		rdd.RelationCompatibleBaseAdvance: true,
	}

	tests := []struct {
		name   string
		gate   Gate
		wantOK map[rdd.Relation]bool
	}{
		{name: "pre-commit gate", gate: GatePreCommit, wantOK: accepted},
		{name: "pre-push gate", gate: GatePrePush, wantOK: accepted},
	}

	for _, tt := range tests {
		for _, relation := range allRelations {
			relation := relation
			wantAllowed := tt.wantOK[relation]
			t.Run(string(tt.gate)+"/"+string(relation), func(t *testing.T) {
				result := Evaluate(tt.gate, relation)

				if result.Allowed != wantAllowed {
					t.Fatalf("Evaluate(%q, %q).Allowed = %v, want %v", tt.gate, relation, result.Allowed, wantAllowed)
				}
				if result.Relation != relation {
					t.Fatalf("Evaluate(%q, %q).Relation = %q, want %q", tt.gate, relation, result.Relation, relation)
				}
				if !wantAllowed && result.Reason == "" {
					t.Fatalf("Evaluate(%q, %q) rejected but Reason is empty", tt.gate, relation)
				}
				if wantAllowed && result.Reason != "" {
					t.Fatalf("Evaluate(%q, %q) allowed but Reason is non-empty: %q", tt.gate, relation, result.Reason)
				}
			})
		}
	}
}

func TestEvaluate_UnrecognizedGateRejects(t *testing.T) {
	result := Evaluate(Gate("bogus-gate"), rdd.RelationExact)

	if result.Allowed {
		t.Fatalf("Evaluate with unrecognized gate: Allowed = true, want false (fail-closed)")
	}
	if result.Reason == "" {
		t.Fatalf("Evaluate with unrecognized gate: Reason is empty, want explanation")
	}
	if result.Relation != rdd.RelationExact {
		t.Fatalf("Evaluate with unrecognized gate: Relation = %q, want %q (still reported for diagnostics)", result.Relation, rdd.RelationExact)
	}
}
