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
