// Package rdd holds the pure-logic core of Review Driven Development: no I/O,
// no git shell-outs, no file access. Persistence and git access live in
// internal/reviewstore, which imports this package (never the reverse).
package rdd

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
)

// CandidateIdentity binds a review candidate to exact repository content: a
// repository identifier, the base and candidate git tree hashes, a digest
// over changed paths and their file modes, and the policy version the
// candidate was evaluated against. Two identities are comparable only within
// the same RepositoryID (see Compare).
type CandidateIdentity struct {
	RepositoryID            string `json:"repository_id"`
	BaseTree                string `json:"base_tree"`
	CandidateTree           string `json:"candidate_tree"`
	ChangedPathsModesDigest string `json:"changed_paths_modes_digest"`
	PolicyHash              string `json:"policy_hash"`
}

// PathMode is a single changed file path and its git file mode (e.g.
// "100644"), the unit ComputeDigest hashes over.
type PathMode struct {
	Path string `json:"path"`
	Mode string `json:"mode"`
}

// Relation is the outcome of comparing two CandidateIdentity values.
type Relation string

const (
	// RelationExact means both identities are byte-identical: the candidate
	// is an exact replay of a previously seen request.
	RelationExact Relation = "exact"
	// RelationCompatibleBaseAdvance means the candidate rebased onto a newer
	// base (fast-forward merge-base) while carrying an identical patch: the
	// net change is unchanged even though the tree hashes differ.
	RelationCompatibleBaseAdvance Relation = "compatible_base_advance"
	// RelationProvableContraction means the ancestry evidence proves the
	// candidate removes a subset of the prior candidate's changes (e.g. the
	// candidate's patch is empty relative to the merge-base).
	RelationProvableContraction Relation = "provable_contraction"
	// RelationChanged means both identities share a RepositoryID but differ
	// in tree hashes or changed-paths digest, and available ancestry
	// evidence confirms the candidate has moved on rather than merely
	// rebased or contracted.
	RelationChanged Relation = "changed"
	// RelationUnrelated means the identities belong to different
	// repositories entirely and are not comparable.
	RelationUnrelated Relation = "unrelated"
	// RelationAmbiguous means the ancestry evidence is present but
	// contradictory or otherwise cannot support a confident relation.
	RelationAmbiguous Relation = "ambiguous"
	// RelationUnknown means there is insufficient (or no) ancestry evidence
	// to determine the relation. Compare never guesses: absent or
	// incomplete evidence always degrades to RelationUnknown, never to a
	// stronger relation.
	RelationUnknown Relation = "unknown"
)

// validRelations is the fail-closed allow-list of every Relation value this
// package recognizes. It intentionally excludes the removed "distinct"
// value (renamed to RelationUnrelated) so any state persisted under the old
// name is rejected rather than silently reinterpreted.
var validRelations = map[Relation]struct{}{
	RelationExact:                 {},
	RelationCompatibleBaseAdvance: {},
	RelationProvableContraction:   {},
	RelationChanged:               {},
	RelationUnrelated:             {},
	RelationAmbiguous:             {},
	RelationUnknown:               {},
}

// UnmarshalJSON fails closed on any Relation value outside the 7 recognized
// constants, including the removed "distinct" value (spec rdd-candidate-identity
// migration: unknown or retired relation values must be rejected, not
// silently accepted or coerced).
func (r *Relation) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return err
	}
	candidate := Relation(s)
	if _, ok := validRelations[candidate]; !ok {
		return fmt.Errorf("rdd: invalid Relation value %q", s)
	}
	*r = candidate
	return nil
}

// AncestryEvidence carries the git-derived facts Compare uses to distinguish
// a genuine change from a compatible base advance or a provable contraction.
// It is supplied by internal/reviewstore, which resolves it via git
// merge-base and git patch-id; internal/rdd never computes it itself.
type AncestryEvidence struct {
	// MergeBaseHash is the merge-base of the two candidates' base commits.
	MergeBaseHash string
	// APatchID is the patch-id of the prior candidate's diff against the
	// merge-base.
	APatchID string
	// BPatchID is the patch-id of the new candidate's diff against the
	// merge-base.
	BPatchID string
}

// ComputeDigest computes a stable SHA-256 digest over entries, sorted by
// Path (then Mode, to stay deterministic even for duplicate paths) before
// hashing so digest equality is independent of input order. It is a pure
// function: no I/O.
func ComputeDigest(entries []PathMode) string {
	sorted := make([]PathMode, len(entries))
	copy(sorted, entries)
	sort.Slice(sorted, func(i, j int) bool {
		if sorted[i].Path != sorted[j].Path {
			return sorted[i].Path < sorted[j].Path
		}
		return sorted[i].Mode < sorted[j].Mode
	})

	h := sha256.New()
	for _, e := range sorted {
		h.Write([]byte(e.Path))
		h.Write([]byte{0})
		h.Write([]byte(e.Mode))
		h.Write([]byte{0})
	}
	return hex.EncodeToString(h.Sum(nil))
}

// Compare classifies the relation between two CandidateIdentity values
// (spec rdd-candidate-identity; design security kernel mapping "Exact
// replay"). Different RepositoryID values are always RelationUnrelated,
// regardless of any other field or ancestry evidence. Within the same
// repository, byte-identical identities are RelationExact.
//
// For any other same-repository pair, Compare consults ancestry: a nil (or
// incomplete) AncestryEvidence never yields more than RelationUnknown — this
// function never guesses a stronger relation than the evidence supports.
// With complete evidence, a matching patch-id across the merge-base proves
// RelationCompatibleBaseAdvance; a candidate with no diff against the
// merge-base proves RelationProvableContraction (the empty set is always a
// subset of the prior candidate's changes); evidence that contradicts the
// expected shape (a prior patch-id of "" paired with a non-empty candidate
// patch-id) is RelationAmbiguous; anything else with two distinct,
// non-empty patch-ids is RelationChanged.
//
// It is a pure function: no I/O. ancestry is supplied by the caller
// (internal/reviewstore resolves it via git); this package never computes
// it.
func Compare(a, b CandidateIdentity, ancestry *AncestryEvidence) Relation {
	if a.RepositoryID != b.RepositoryID {
		return RelationUnrelated
	}
	if a == b {
		return RelationExact
	}
	if ancestry == nil || ancestry.MergeBaseHash == "" {
		return RelationUnknown
	}
	switch {
	case ancestry.APatchID == "" && ancestry.BPatchID == "":
		return RelationUnknown
	case ancestry.APatchID != "" && ancestry.APatchID == ancestry.BPatchID:
		return RelationCompatibleBaseAdvance
	case ancestry.APatchID != "" && ancestry.BPatchID == "":
		return RelationProvableContraction
	case ancestry.APatchID == "" && ancestry.BPatchID != "":
		return RelationAmbiguous
	default:
		return RelationChanged
	}
}
