package rdd

import (
	"encoding/json"
	"testing"
)

func TestComputeDigest_Determinism(t *testing.T) {
	tests := []struct {
		name string
		a    []PathMode
		b    []PathMode
		want bool // true means digests must be equal
	}{
		{
			name: "identical input produces identical digest",
			a: []PathMode{
				{Path: "internal/rdd/identity.go", Mode: "100644"},
				{Path: "internal/rdd/classifier.go", Mode: "100644"},
			},
			b: []PathMode{
				{Path: "internal/rdd/identity.go", Mode: "100644"},
				{Path: "internal/rdd/classifier.go", Mode: "100644"},
			},
			want: true,
		},
		{
			name: "reordered input produces identical digest",
			a: []PathMode{
				{Path: "internal/rdd/identity.go", Mode: "100644"},
				{Path: "internal/rdd/classifier.go", Mode: "100644"},
			},
			b: []PathMode{
				{Path: "internal/rdd/classifier.go", Mode: "100644"},
				{Path: "internal/rdd/identity.go", Mode: "100644"},
			},
			want: true,
		},
		{
			name: "different mode produces different digest",
			a: []PathMode{
				{Path: "internal/rdd/identity.go", Mode: "100644"},
			},
			b: []PathMode{
				{Path: "internal/rdd/identity.go", Mode: "100755"},
			},
			want: false,
		},
		{
			name: "different path produces different digest",
			a: []PathMode{
				{Path: "internal/rdd/identity.go", Mode: "100644"},
			},
			b: []PathMode{
				{Path: "internal/rdd/classifier.go", Mode: "100644"},
			},
			want: false,
		},
		{
			name: "empty input is deterministic",
			a:    nil,
			b:    []PathMode{},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ComputeDigest(tt.a) == ComputeDigest(tt.b)
			if got != tt.want {
				t.Errorf("ComputeDigest(%+v) == ComputeDigest(%+v) = %v, want %v", tt.a, tt.b, got, tt.want)
			}
		})
	}
}

func TestComputeDigest_RepeatedCallsAreByteIdentical(t *testing.T) {
	entries := []PathMode{
		{Path: "cmd/capiko-ai/review.go", Mode: "100644"},
		{Path: "internal/rdd/killswitch.go", Mode: "100644"},
	}

	first := ComputeDigest(entries)
	second := ComputeDigest(entries)

	if first != second {
		t.Errorf("ComputeDigest is not deterministic across repeated calls: %q != %q", first, second)
	}
	if first == "" {
		t.Error("ComputeDigest returned an empty digest for non-empty input")
	}
}

func TestCompare(t *testing.T) {
	base := CandidateIdentity{
		RepositoryID:            "repo-a",
		BaseTree:                "tree-1",
		CandidateTree:           "tree-2",
		ChangedPathsModesDigest: "digest-1",
		PolicyHash:              "policy-1",
	}
	changed := CandidateIdentity{
		RepositoryID:            "repo-a",
		BaseTree:                "tree-1",
		CandidateTree:           "tree-3",
		ChangedPathsModesDigest: "digest-2",
		PolicyHash:              "policy-1",
	}
	otherRepo := CandidateIdentity{
		RepositoryID:            "repo-b",
		BaseTree:                "tree-1",
		CandidateTree:           "tree-2",
		ChangedPathsModesDigest: "digest-1",
		PolicyHash:              "policy-1",
	}

	tests := []struct {
		name     string
		a        CandidateIdentity
		b        CandidateIdentity
		ancestry *AncestryEvidence
		want     Relation
	}{
		{
			name: "byte-identical identities are exact regardless of ancestry",
			a:    base,
			b:    base,
			want: RelationExact,
		},
		{
			name: "different repository with nil ancestry is unrelated",
			a:    base,
			b:    otherRepo,
			want: RelationUnrelated,
		},
		{
			name:     "different repository is unrelated even with full ancestry evidence",
			a:        base,
			b:        otherRepo,
			ancestry: &AncestryEvidence{MergeBaseHash: "merge-1", APatchID: "patch-x", BPatchID: "patch-x"},
			want:     RelationUnrelated,
		},
		{
			name: "same repository, non-exact, nil ancestry is unknown not changed",
			a:    base,
			b:    changed,
			want: RelationUnknown,
		},
		{
			name:     "same repository, ancestry missing merge-base hash is unknown",
			a:        base,
			b:        changed,
			ancestry: &AncestryEvidence{MergeBaseHash: "", APatchID: "patch-x", BPatchID: "patch-x"},
			want:     RelationUnknown,
		},
		{
			name:     "same repository, ancestry with both patch-ids empty is unknown",
			a:        base,
			b:        changed,
			ancestry: &AncestryEvidence{MergeBaseHash: "merge-1", APatchID: "", BPatchID: ""},
			want:     RelationUnknown,
		},
		{
			name:     "same repository, fast-forward merge-base with matching patch-id is compatible base advance",
			a:        base,
			b:        changed,
			ancestry: &AncestryEvidence{MergeBaseHash: "merge-1", APatchID: "patch-x", BPatchID: "patch-x"},
			want:     RelationCompatibleBaseAdvance,
		},
		{
			name:     "same repository, candidate patch-id empty proves contraction of prior changes",
			a:        base,
			b:        changed,
			ancestry: &AncestryEvidence{MergeBaseHash: "merge-1", APatchID: "patch-x", BPatchID: ""},
			want:     RelationProvableContraction,
		},
		{
			name:     "same repository, prior patch-id empty but candidate has a diff is ambiguous",
			a:        base,
			b:        changed,
			ancestry: &AncestryEvidence{MergeBaseHash: "merge-1", APatchID: "", BPatchID: "patch-y"},
			want:     RelationAmbiguous,
		},
		{
			name:     "same repository, differing non-empty patch-ids is changed",
			a:        base,
			b:        changed,
			ancestry: &AncestryEvidence{MergeBaseHash: "merge-1", APatchID: "patch-x", BPatchID: "patch-y"},
			want:     RelationChanged,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Compare(tt.a, tt.b, tt.ancestry)
			if got != tt.want {
				t.Errorf("Compare(%+v, %+v, %+v) = %v, want %v", tt.a, tt.b, tt.ancestry, got, tt.want)
			}
		})
	}
}

func TestRelation_JSONRoundTrip_AllSevenValues(t *testing.T) {
	values := []Relation{
		RelationExact,
		RelationCompatibleBaseAdvance,
		RelationProvableContraction,
		RelationChanged,
		RelationUnrelated,
		RelationAmbiguous,
		RelationUnknown,
	}

	for _, v := range values {
		t.Run(string(v), func(t *testing.T) {
			data, err := json.Marshal(v)
			if err != nil {
				t.Fatalf("Marshal(%v) returned error: %v", v, err)
			}
			wantJSON := `"` + string(v) + `"`
			if string(data) != wantJSON {
				t.Errorf("Marshal(%v) = %s, want %s", v, data, wantJSON)
			}

			var got Relation
			if err := json.Unmarshal(data, &got); err != nil {
				t.Fatalf("Unmarshal(%s) returned error: %v", data, err)
			}
			if got != v {
				t.Errorf("round-trip Unmarshal(%s) = %v, want %v", data, got, v)
			}
		})
	}
}

func TestRelation_UnmarshalJSON_RejectsRemovedDistinctValue(t *testing.T) {
	var r Relation
	err := json.Unmarshal([]byte(`"distinct"`), &r)
	if err == nil {
		t.Fatal(`Unmarshal("distinct") returned nil error, want error: "distinct" was removed and must fail closed, not silently accepted`)
	}
}

func TestRelation_UnmarshalJSON_RejectsUnrecognizedValue(t *testing.T) {
	var r Relation
	err := json.Unmarshal([]byte(`"bogus"`), &r)
	if err == nil {
		t.Fatal(`Unmarshal("bogus") returned nil error, want error: unrecognized Relation values must fail closed`)
	}
}
