package reviewstore

import (
	"errors"
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
