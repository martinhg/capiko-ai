package reviewstore

import "github.com/martinhg/capiko-ai/internal/rdd"

// BuildAncestryEvidence resolves the git-derived facts rdd.Compare needs to
// distinguish a genuine change from a compatible base advance or a provable
// contraction (spec relation-algebra; design "Seam Catalog"). baseA and
// baseB are the base commits the two candidates diverged from — used only to
// compute their merge-base; commitA and commitB are the candidates' own
// commits, each diffed against that merge-base to produce a patch-id.
//
// BuildAncestryEvidence never guesses: any seam failure degrades toward
// rdd.RelationUnknown rather than fabricating evidence (spec
// relation-algebra: "Compare MUST NOT guess").
//
//   - If gitMergeBase fails, there is no common ancestor to anchor any patch
//     comparison on, so BuildAncestryEvidence returns (nil, nil). Callers
//     pass this nil ancestry into rdd.Compare, which degrades to
//     RelationUnknown (spec relation-algebra scenario: "GIVEN merge-base
//     lookup fails, WHEN Compare runs, THEN result is unknown, not
//     changed").
//   - If a gitPatchID call fails, BuildAncestryEvidence still returns
//     evidence with MergeBaseHash populated and the failed side's patch-id
//     left as the empty string. rdd.Compare treats a fully empty patch-id
//     pair as RelationUnknown, and a partial pair per its own ambiguous/
//     contraction rules — it never fabricates a stronger relation than the
//     available evidence supports.
func BuildAncestryEvidence(workspace, baseA, commitA, baseB, commitB string) (*rdd.AncestryEvidence, error) {
	mergeBase, err := gitMergeBase(workspace, baseA, baseB)
	if err != nil {
		return nil, nil
	}

	evidence := &rdd.AncestryEvidence{MergeBaseHash: mergeBase}

	if aPatchID, err := gitPatchID(workspace, mergeBase, commitA); err == nil {
		evidence.APatchID = aPatchID
	}
	if bPatchID, err := gitPatchID(workspace, mergeBase, commitB); err == nil {
		evidence.BPatchID = bPatchID
	}

	return evidence, nil
}

// gateAncestrySentinel is the value BuildGateAncestryEvidence sets
// AncestryEvidence.MergeBaseHash to. Gate validation has no commit SHAs to
// resolve a real merge-base from — the receipt only persists tree hashes —
// so there is no genuine merge-base to report. rdd.Compare only ever checks
// MergeBaseHash for emptiness (to short-circuit to RelationUnknown when
// ancestry evidence is entirely absent); it never reads the hash's value.
// A non-empty sentinel therefore signals "ancestry evidence is present"
// without claiming a specific commit as the merge-base.
const gateAncestrySentinel = "gate-evidence"

// BuildGateAncestryEvidence computes ancestry evidence for gate validation
// (pre-commit/pre-push) directly from two rdd.CandidateIdentity values,
// without any commit SHAs or a gitMergeBase call (design "Gate Ancestry
// Evidence"). BuildAncestryEvidence (above) needs commit SHAs to resolve a
// real merge-base via gitMergeBase, but a receipt's stored Candidate and a
// gate's freshly built CandidateIdentity only carry tree hashes — there is
// no commit to look up a merge-base from.
//
// Instead, BuildGateAncestryEvidence computes each side's patch-id directly
// from its own BaseTree -> CandidateTree diff (git diff accepts tree
// hashes directly, so no merge-base is needed to anchor the comparison),
// and sets MergeBaseHash to a non-empty sentinel so rdd.Compare's
// emptiness check treats this as present evidence.
//
// Like BuildAncestryEvidence, a failed gitPatchID call for one side leaves
// that side's patch-id as the empty string rather than fabricating a
// relation the evidence doesn't support (spec relation-algebra: "Compare
// MUST NOT guess").
func BuildGateAncestryEvidence(workspace string, a, b rdd.CandidateIdentity) (*rdd.AncestryEvidence, error) {
	evidence := &rdd.AncestryEvidence{MergeBaseHash: gateAncestrySentinel}

	if aPatchID, err := gitPatchID(workspace, a.BaseTree, a.CandidateTree); err == nil {
		evidence.APatchID = aPatchID
	}
	if bPatchID, err := gitPatchID(workspace, b.BaseTree, b.CandidateTree); err == nil {
		evidence.BPatchID = bPatchID
	}

	return evidence, nil
}
