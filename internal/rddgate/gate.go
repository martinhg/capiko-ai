// Package rddgate holds the pure gate-policy logic for RDD delivery gates:
// no I/O, no git shell-outs, no file access. It imports internal/rdd for the
// Relation vocabulary but never the reverse — rdd stays gate-agnostic.
package rddgate

import (
	"fmt"

	"github.com/martinhg/capiko-ai/internal/rdd"
)

// Gate identifies which git hook boundary is being validated.
type Gate string

const (
	// GatePreCommit validates the staged candidate against the terminal
	// review receipt before a commit is created.
	GatePreCommit Gate = "pre-commit"
	// GatePrePush validates each pushed ref's candidate against the
	// terminal review receipt before a push is accepted.
	GatePrePush Gate = "pre-push"
)

// acceptedRelations maps each recognized Gate to the set of rdd.Relation
// values that pass validation. Every relation not present in a gate's set
// — including any relation for an unrecognized Gate — is rejected
// (fail-closed). Both gates currently share the same accept policy
// (RelationExact, RelationCompatibleBaseAdvance): a base advance after the
// receipt was captured does not invalidate previously reviewed code.
var acceptedRelations = map[Gate]map[rdd.Relation]bool{
	GatePreCommit: {
		rdd.RelationExact:                 true,
		rdd.RelationCompatibleBaseAdvance: true,
	},
	GatePrePush: {
		rdd.RelationExact:                 true,
		rdd.RelationCompatibleBaseAdvance: true,
	},
}

// GateResult is the outcome of evaluating a relation against a gate's
// accept policy.
type GateResult struct {
	// Allowed is true when relation passes the gate's accept policy.
	Allowed bool
	// Reason is a human-readable explanation, set only when Allowed is
	// false.
	Reason string
	// Relation is the evaluated relation, echoed back for diagnostics.
	Relation rdd.Relation
}

// Evaluate classifies whether relation passes gate's accept policy. An
// unrecognized gate is rejected (fail-closed) rather than treated as
// permissive. It is a pure function: no I/O.
func Evaluate(gate Gate, relation rdd.Relation) GateResult {
	accepted, knownGate := acceptedRelations[gate]
	if !knownGate {
		return GateResult{
			Allowed:  false,
			Reason:   fmt.Sprintf("unknown gate: %q", gate),
			Relation: relation,
		}
	}

	if accepted[relation] {
		return GateResult{Allowed: true, Relation: relation}
	}

	return GateResult{
		Allowed:  false,
		Reason:   fmt.Sprintf("receipt does not cover current candidate (relation: %s)", relation),
		Relation: relation,
	}
}
