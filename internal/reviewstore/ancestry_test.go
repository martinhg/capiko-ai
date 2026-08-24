package reviewstore

import (
	"errors"
	"reflect"
	"testing"

	"github.com/martinhg/capiko-ai/internal/rdd"
)

// --- BuildAncestryEvidence tests: stub gitMergeBase + gitPatchID seams, no
// real git binary invoked. ---

func TestBuildAncestryEvidence_Success(t *testing.T) {
	originalMergeBase := gitMergeBase
	originalPatchID := gitPatchID
	t.Cleanup(func() {
		gitMergeBase = originalMergeBase
		gitPatchID = originalPatchID
	})

	gitMergeBase = func(workspace, baseA, baseB string) (string, error) {
		if workspace != "ws" || baseA != "base-a" || baseB != "base-b" {
			t.Fatalf("gitMergeBase called with unexpected args: %q %q %q", workspace, baseA, baseB)
		}
		return "merge-base-hash", nil
	}
	gitPatchID = func(workspace, base, candidate string) (string, error) {
		if base != "merge-base-hash" {
			t.Fatalf("gitPatchID called with unexpected base: %q, want %q", base, "merge-base-hash")
		}
		switch candidate {
		case "commit-a":
			return "patch-id-a", nil
		case "commit-b":
			return "patch-id-b", nil
		default:
			t.Fatalf("gitPatchID called with unexpected candidate: %q", candidate)
			return "", nil
		}
	}

	got, err := BuildAncestryEvidence("ws", "base-a", "commit-a", "base-b", "commit-b")
	if err != nil {
		t.Fatalf("BuildAncestryEvidence() error = %v, want nil", err)
	}
	if got == nil {
		t.Fatal("BuildAncestryEvidence() = nil, want populated evidence")
	}
	want := rdd.AncestryEvidence{
		MergeBaseHash: "merge-base-hash",
		APatchID:      "patch-id-a",
		BPatchID:      "patch-id-b",
	}
	if *got != want {
		t.Errorf("BuildAncestryEvidence() = %+v, want %+v", *got, want)
	}
}

func TestBuildAncestryEvidence_MergeBaseFails_ReturnsNilEvidence(t *testing.T) {
	originalMergeBase := gitMergeBase
	t.Cleanup(func() { gitMergeBase = originalMergeBase })

	gitMergeBase = func(string, string, string) (string, error) {
		return "", errors.New("merge-base lookup failed")
	}

	got, err := BuildAncestryEvidence("ws", "base-a", "commit-a", "base-b", "commit-b")
	if err != nil {
		t.Fatalf("BuildAncestryEvidence() error = %v, want nil (graceful degradation, not a hard error)", err)
	}
	if got != nil {
		t.Errorf("BuildAncestryEvidence() = %+v, want nil evidence when merge-base fails so rdd.Compare degrades to unknown", got)
	}
}

func TestBuildAncestryEvidence_PatchIDFails_ReturnsEvidenceWithEmptyPatchIDs(t *testing.T) {
	originalMergeBase := gitMergeBase
	originalPatchID := gitPatchID
	t.Cleanup(func() {
		gitMergeBase = originalMergeBase
		gitPatchID = originalPatchID
	})

	gitMergeBase = func(string, string, string) (string, error) {
		return "merge-base-hash", nil
	}
	gitPatchID = func(string, string, string) (string, error) {
		return "", errors.New("patch-id failed")
	}

	got, err := BuildAncestryEvidence("ws", "base-a", "commit-a", "base-b", "commit-b")
	if err != nil {
		t.Fatalf("BuildAncestryEvidence() error = %v, want nil", err)
	}
	if got == nil {
		t.Fatal("BuildAncestryEvidence() = nil, want evidence with MergeBaseHash still populated")
	}
	if got.MergeBaseHash != "merge-base-hash" {
		t.Errorf("MergeBaseHash = %q, want %q", got.MergeBaseHash, "merge-base-hash")
	}
	if got.APatchID != "" || got.BPatchID != "" {
		t.Errorf("APatchID/BPatchID = %q/%q, want both empty — rdd.Compare treats this as unknown, never a fabricated relation", got.APatchID, got.BPatchID)
	}
}

func TestBuildAncestryEvidence_OnePatchIDFails_KeepsTheOtherPatchID(t *testing.T) {
	originalMergeBase := gitMergeBase
	originalPatchID := gitPatchID
	t.Cleanup(func() {
		gitMergeBase = originalMergeBase
		gitPatchID = originalPatchID
	})

	gitMergeBase = func(string, string, string) (string, error) {
		return "merge-base-hash", nil
	}
	gitPatchID = func(workspace, base, candidate string) (string, error) {
		if candidate == "commit-a" {
			return "patch-id-a", nil
		}
		return "", errors.New("patch-id failed for candidate b")
	}

	got, err := BuildAncestryEvidence("ws", "base-a", "commit-a", "base-b", "commit-b")
	if err != nil {
		t.Fatalf("BuildAncestryEvidence() error = %v, want nil", err)
	}
	if got.APatchID != "patch-id-a" {
		t.Errorf("APatchID = %q, want %q", got.APatchID, "patch-id-a")
	}
	if got.BPatchID != "" {
		t.Errorf("BPatchID = %q, want empty after gitPatchID failure for candidate B", got.BPatchID)
	}
}

// --- BuildGateAncestryEvidence tests: stub gitPatchID seam, no real git
// binary invoked (R-3 PR2b). Unlike BuildAncestryEvidence, this function
// takes two full rdd.CandidateIdentity values (tree hashes only — no
// commit SHAs, no gitMergeBase call) and diffs each one's own base/
// candidate tree pair directly. ---

func TestBuildGateAncestryEvidence_Success(t *testing.T) {
	originalPatchID := gitPatchID
	t.Cleanup(func() { gitPatchID = originalPatchID })

	a := rdd.CandidateIdentity{BaseTree: "a-base-tree", CandidateTree: "a-candidate-tree"}
	b := rdd.CandidateIdentity{BaseTree: "b-base-tree", CandidateTree: "b-candidate-tree"}

	gitPatchID = func(workspace, base, candidate string) (string, error) {
		if workspace != "ws" {
			t.Fatalf("gitPatchID() workspace = %q, want ws", workspace)
		}
		switch {
		case base == "a-base-tree" && candidate == "a-candidate-tree":
			return "patch-id-a", nil
		case base == "b-base-tree" && candidate == "b-candidate-tree":
			return "patch-id-b", nil
		default:
			t.Fatalf("gitPatchID() called with unexpected base/candidate: %q/%q", base, candidate)
			return "", nil
		}
	}

	got, err := BuildGateAncestryEvidence("ws", a, b)
	if err != nil {
		t.Fatalf("BuildGateAncestryEvidence() error = %v, want nil", err)
	}
	if got == nil {
		t.Fatal("BuildGateAncestryEvidence() = nil, want populated evidence")
	}
	if got.MergeBaseHash == "" {
		t.Error("BuildGateAncestryEvidence() MergeBaseHash is empty, want a non-empty sentinel — rdd.Compare short-circuits to Unknown on empty MergeBaseHash")
	}
	if got.APatchID != "patch-id-a" {
		t.Errorf("APatchID = %q, want %q", got.APatchID, "patch-id-a")
	}
	if got.BPatchID != "patch-id-b" {
		t.Errorf("BPatchID = %q, want %q", got.BPatchID, "patch-id-b")
	}
}

func TestBuildGateAncestryEvidence_PassesTreeHashesNotCommitSHAs(t *testing.T) {
	// Regression guard: BuildGateAncestryEvidence must diff BaseTree/
	// CandidateTree (tree hashes stored in the receipt), never any commit
	// SHA — the receipt only persists tree hashes (design "Gate Ancestry
	// Evidence").
	originalPatchID := gitPatchID
	t.Cleanup(func() { gitPatchID = originalPatchID })

	a := rdd.CandidateIdentity{
		RepositoryID:  "repo",
		BaseTree:      "tree-base-a",
		CandidateTree: "tree-candidate-a",
	}
	b := rdd.CandidateIdentity{
		RepositoryID:  "repo",
		BaseTree:      "tree-base-b",
		CandidateTree: "tree-candidate-b",
	}

	var gotBases, gotCandidates []string
	gitPatchID = func(_, base, candidate string) (string, error) {
		gotBases = append(gotBases, base)
		gotCandidates = append(gotCandidates, candidate)
		return "patch-id", nil
	}

	if _, err := BuildGateAncestryEvidence("ws", a, b); err != nil {
		t.Fatalf("BuildGateAncestryEvidence() error = %v, want nil", err)
	}

	wantBases := []string{"tree-base-a", "tree-base-b"}
	wantCandidates := []string{"tree-candidate-a", "tree-candidate-b"}
	if !reflect.DeepEqual(gotBases, wantBases) {
		t.Errorf("gitPatchID bases = %v, want %v (tree hashes, not commit SHAs)", gotBases, wantBases)
	}
	if !reflect.DeepEqual(gotCandidates, wantCandidates) {
		t.Errorf("gitPatchID candidates = %v, want %v (tree hashes, not commit SHAs)", gotCandidates, wantCandidates)
	}
}

func TestBuildGateAncestryEvidence_APatchIDFails_LeavesItEmpty(t *testing.T) {
	originalPatchID := gitPatchID
	t.Cleanup(func() { gitPatchID = originalPatchID })

	a := rdd.CandidateIdentity{BaseTree: "a-base", CandidateTree: "a-candidate"}
	b := rdd.CandidateIdentity{BaseTree: "b-base", CandidateTree: "b-candidate"}

	gitPatchID = func(_, base, candidate string) (string, error) {
		if base == "a-base" {
			return "", errors.New("patch-id failed for a")
		}
		return "patch-id-b", nil
	}

	got, err := BuildGateAncestryEvidence("ws", a, b)
	if err != nil {
		t.Fatalf("BuildGateAncestryEvidence() error = %v, want nil", err)
	}
	if got.APatchID != "" {
		t.Errorf("APatchID = %q, want empty after gitPatchID failure", got.APatchID)
	}
	if got.BPatchID != "patch-id-b" {
		t.Errorf("BPatchID = %q, want %q", got.BPatchID, "patch-id-b")
	}
	if got.MergeBaseHash == "" {
		t.Error("MergeBaseHash is empty even though one patch-id failed; sentinel must still be set so rdd.Compare doesn't short-circuit on emptiness alone")
	}
}

func TestBuildGateAncestryEvidence_BPatchIDFails_LeavesItEmpty(t *testing.T) {
	originalPatchID := gitPatchID
	t.Cleanup(func() { gitPatchID = originalPatchID })

	a := rdd.CandidateIdentity{BaseTree: "a-base", CandidateTree: "a-candidate"}
	b := rdd.CandidateIdentity{BaseTree: "b-base", CandidateTree: "b-candidate"}

	gitPatchID = func(_, base, candidate string) (string, error) {
		if base == "b-base" {
			return "", errors.New("patch-id failed for b")
		}
		return "patch-id-a", nil
	}

	got, err := BuildGateAncestryEvidence("ws", a, b)
	if err != nil {
		t.Fatalf("BuildGateAncestryEvidence() error = %v, want nil", err)
	}
	if got.APatchID != "patch-id-a" {
		t.Errorf("APatchID = %q, want %q", got.APatchID, "patch-id-a")
	}
	if got.BPatchID != "" {
		t.Errorf("BPatchID = %q, want empty after gitPatchID failure", got.BPatchID)
	}
}
