package rdd

import "testing"

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

	tests := []struct {
		name string
		a    CandidateIdentity
		b    CandidateIdentity
		want Relation
	}{
		{
			name: "byte-identical identities are exact",
			a:    base,
			b:    base,
			want: RelationExact,
		},
		{
			name: "same repository with different candidate tree is changed",
			a:    base,
			b: CandidateIdentity{
				RepositoryID:            "repo-a",
				BaseTree:                "tree-1",
				CandidateTree:           "tree-3",
				ChangedPathsModesDigest: "digest-2",
				PolicyHash:              "policy-1",
			},
			want: RelationChanged,
		},
		{
			name: "same repository with different base tree is changed",
			a:    base,
			b: CandidateIdentity{
				RepositoryID:            "repo-a",
				BaseTree:                "tree-9",
				CandidateTree:           "tree-2",
				ChangedPathsModesDigest: "digest-1",
				PolicyHash:              "policy-1",
			},
			want: RelationChanged,
		},
		{
			name: "different repository is distinct regardless of other fields",
			a:    base,
			b: CandidateIdentity{
				RepositoryID:            "repo-b",
				BaseTree:                "tree-1",
				CandidateTree:           "tree-2",
				ChangedPathsModesDigest: "digest-1",
				PolicyHash:              "policy-1",
			},
			want: RelationDistinct,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Compare(tt.a, tt.b)
			if got != tt.want {
				t.Errorf("Compare(%+v, %+v) = %v, want %v", tt.a, tt.b, got, tt.want)
			}
		})
	}
}
