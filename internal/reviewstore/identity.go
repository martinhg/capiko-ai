package reviewstore

import (
	"errors"
	"fmt"
	"path/filepath"

	"github.com/martinhg/capiko-ai/internal/rdd"
)

// ErrBareRepo is returned by BuildIdentity when workspace is a bare
// repository: RDD requires a working tree to diff against, which a bare
// repository does not have (spec rdd-candidate-identity "Fail-closed on
// unsupported repo shape", design Error Handling table).
var ErrBareRepo = errors.New("reviewstore: bare repositories are not supported; RDD requires a working tree")

// ErrShallowRepo is returned by BuildIdentity when workspace is a shallow
// clone: a shallow clone's history is incomplete, which can make tree
// comparisons unreliable (spec rdd-candidate-identity "Fail-closed on
// unsupported repo shape", design Error Handling table).
var ErrShallowRepo = errors.New("reviewstore: shallow clones are not supported; run `git fetch --unshallow` first")

// BuildIdentity orchestrates the git command seams to construct a complete
// rdd.CandidateIdentity for workspace (design "Data Flow", spec
// rdd-candidate-identity). It fails closed on bare or shallow repositories,
// falls back to the workspace's base name as RepositoryID when no "origin"
// remote is configured, and leaves PolicyHash empty since R-1 ships no
// policy system yet.
func BuildIdentity(workspace string) (rdd.CandidateIdentity, error) {
	isBare, err := gitIsBareRepo(workspace)
	if err != nil {
		return rdd.CandidateIdentity{}, fmt.Errorf("reviewstore: checking bare repository: %w", err)
	}
	if isBare {
		return rdd.CandidateIdentity{}, ErrBareRepo
	}

	isShallow, err := gitIsShallowRepo(workspace)
	if err != nil {
		return rdd.CandidateIdentity{}, fmt.Errorf("reviewstore: checking shallow repository: %w", err)
	}
	if isShallow {
		return rdd.CandidateIdentity{}, ErrShallowRepo
	}

	repositoryID, err := gitRemoteOriginURL(workspace)
	if err != nil {
		// No "origin" remote configured (or another git failure resolving
		// it) — degrade gracefully to the workspace's base name rather than
		// failing the whole identity computation (design Error Handling
		// "No remote origin").
		repositoryID = filepath.Base(workspace)
	}

	baseTree, err := gitRevParseTree(workspace, "HEAD")
	if err != nil {
		return rdd.CandidateIdentity{}, fmt.Errorf("reviewstore: resolving base tree: %w", err)
	}

	candidateTree, err := gitWriteTree(workspace)
	if err != nil {
		return rdd.CandidateIdentity{}, fmt.Errorf("reviewstore: writing candidate tree: %w", err)
	}

	changedPaths, err := gitDiffTree(workspace, baseTree, candidateTree)
	if err != nil {
		return rdd.CandidateIdentity{}, fmt.Errorf("reviewstore: diffing base and candidate trees: %w", err)
	}

	return rdd.CandidateIdentity{
		RepositoryID:            repositoryID,
		BaseTree:                baseTree,
		CandidateTree:           candidateTree,
		ChangedPathsModesDigest: rdd.ComputeDigest(changedPaths),
		PolicyHash:              "",
	}, nil
}
