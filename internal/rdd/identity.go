// Package rdd holds the pure-logic core of Review Driven Development: no I/O,
// no git shell-outs, no file access. Persistence and git access live in
// internal/reviewstore, which imports this package (never the reverse).
package rdd

import (
	"crypto/sha256"
	"encoding/hex"
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
	// RelationChanged means both identities share a RepositoryID but differ
	// in tree hashes or changed-paths digest: the candidate has moved on.
	RelationChanged Relation = "changed"
	// RelationDistinct means the identities belong to different
	// repositories entirely and are not comparable.
	RelationDistinct Relation = "distinct"
)

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
// replay"). Different RepositoryID values are always RelationDistinct,
// regardless of any other field. Within the same repository, byte-identical
// identities are RelationExact; anything else is RelationChanged. It is a
// pure function: no I/O.
func Compare(a, b CandidateIdentity) Relation {
	if a.RepositoryID != b.RepositoryID {
		return RelationDistinct
	}
	if a == b {
		return RelationExact
	}
	return RelationChanged
}
