package reviewstore

import (
	"errors"
	"testing"

	"github.com/martinhg/capiko-ai/internal/rdd"
)

// stubDefaultIdentitySeams stubs every seam BuildIdentity calls with a
// non-error, "happy path" behavior and restores the originals on cleanup.
// Individual tests override just the seam(s) they need to exercise.
func stubDefaultIdentitySeams(t *testing.T) {
	t.Helper()

	origIsBare := gitIsBareRepo
	origIsShallow := gitIsShallowRepo
	origRemote := gitRemoteOriginURL
	origRevParseTree := gitRevParseTree
	origWriteTree := gitWriteTree
	origDiffTree := gitDiffTree
	t.Cleanup(func() {
		gitIsBareRepo = origIsBare
		gitIsShallowRepo = origIsShallow
		gitRemoteOriginURL = origRemote
		gitRevParseTree = origRevParseTree
		gitWriteTree = origWriteTree
		gitDiffTree = origDiffTree
	})

	gitIsBareRepo = func(string) (bool, error) { return false, nil }
	gitIsShallowRepo = func(string) (bool, error) { return false, nil }
	gitRemoteOriginURL = func(string) (string, error) { return "git@example.com:org/repo.git", nil }
	gitRevParseTree = func(_, ref string) (string, error) {
		if ref != "HEAD" {
			t.Fatalf("gitRevParseTree() ref = %q, want HEAD", ref)
		}
		return "base-tree-hash", nil
	}
	gitWriteTree = func(string) (string, error) { return "candidate-tree-hash", nil }
	gitDiffTree = func(_, base, candidate string) ([]rdd.PathMode, error) {
		if base != "base-tree-hash" || candidate != "candidate-tree-hash" {
			t.Fatalf("gitDiffTree() base=%q candidate=%q, want base-tree-hash/candidate-tree-hash", base, candidate)
		}
		return []rdd.PathMode{
			{Path: "internal/reviewstore/identity.go", Mode: "100644"},
			{Path: "internal/rdd/identity.go", Mode: "100644"},
		}, nil
	}
}

func TestBuildIdentity_HappyPath(t *testing.T) {
	stubDefaultIdentitySeams(t)

	got, err := BuildIdentity("/workspace/repo")
	if err != nil {
		t.Fatalf("BuildIdentity() error = %v, want nil", err)
	}

	want := rdd.CandidateIdentity{
		RepositoryID:  "git@example.com:org/repo.git",
		BaseTree:      "base-tree-hash",
		CandidateTree: "candidate-tree-hash",
		ChangedPathsModesDigest: rdd.ComputeDigest([]rdd.PathMode{
			{Path: "internal/reviewstore/identity.go", Mode: "100644"},
			{Path: "internal/rdd/identity.go", Mode: "100644"},
		}),
		PolicyHash: "",
	}
	if got != want {
		t.Errorf("BuildIdentity() = %+v, want %+v", got, want)
	}
}

func TestBuildIdentity_BareRepo(t *testing.T) {
	stubDefaultIdentitySeams(t)
	gitIsBareRepo = func(string) (bool, error) { return true, nil }

	got, err := BuildIdentity("/workspace/repo")
	if !errors.Is(err, ErrBareRepo) {
		t.Fatalf("BuildIdentity() error = %v, want errors.Is(err, ErrBareRepo)", err)
	}
	if got != (rdd.CandidateIdentity{}) {
		t.Errorf("BuildIdentity() = %+v, want zero value on error", got)
	}
}

func TestBuildIdentity_IsBareRepoCheckFails(t *testing.T) {
	stubDefaultIdentitySeams(t)
	wantErr := errors.New("git rev-parse --is-bare-repository failed")
	gitIsBareRepo = func(string) (bool, error) { return false, wantErr }

	_, err := BuildIdentity("/workspace/repo")
	if !errors.Is(err, wantErr) {
		t.Fatalf("BuildIdentity() error = %v, want errors.Is(err, wantErr)", err)
	}
}

func TestBuildIdentity_ShallowRepo(t *testing.T) {
	stubDefaultIdentitySeams(t)
	gitIsShallowRepo = func(string) (bool, error) { return true, nil }

	got, err := BuildIdentity("/workspace/repo")
	if !errors.Is(err, ErrShallowRepo) {
		t.Fatalf("BuildIdentity() error = %v, want errors.Is(err, ErrShallowRepo)", err)
	}
	if got != (rdd.CandidateIdentity{}) {
		t.Errorf("BuildIdentity() = %+v, want zero value on error", got)
	}
}

func TestBuildIdentity_IsShallowRepoCheckFails(t *testing.T) {
	stubDefaultIdentitySeams(t)
	wantErr := errors.New("git rev-parse --is-shallow-repository failed")
	gitIsShallowRepo = func(string) (bool, error) { return false, wantErr }

	_, err := BuildIdentity("/workspace/repo")
	if !errors.Is(err, wantErr) {
		t.Fatalf("BuildIdentity() error = %v, want errors.Is(err, wantErr)", err)
	}
}

func TestBuildIdentity_RemoteOriginFallsBackToWorkspaceBasename(t *testing.T) {
	stubDefaultIdentitySeams(t)
	gitRemoteOriginURL = func(string) (string, error) {
		return "", errors.New("no such remote 'origin'")
	}

	got, err := BuildIdentity("/workspace/my-repo")
	if err != nil {
		t.Fatalf("BuildIdentity() error = %v, want nil", err)
	}
	if got.RepositoryID != "my-repo" {
		t.Errorf("BuildIdentity() RepositoryID = %q, want %q (workspace basename fallback)", got.RepositoryID, "my-repo")
	}
}

func TestBuildIdentity_RevParseTreeFails(t *testing.T) {
	stubDefaultIdentitySeams(t)
	wantErr := errors.New("git rev-parse HEAD^{tree} failed")
	gitRevParseTree = func(string, string) (string, error) { return "", wantErr }

	_, err := BuildIdentity("/workspace/repo")
	if !errors.Is(err, wantErr) {
		t.Fatalf("BuildIdentity() error = %v, want errors.Is(err, wantErr)", err)
	}
}

func TestBuildIdentity_WriteTreeFails(t *testing.T) {
	stubDefaultIdentitySeams(t)
	wantErr := errors.New("git write-tree failed")
	gitWriteTree = func(string) (string, error) { return "", wantErr }

	_, err := BuildIdentity("/workspace/repo")
	if !errors.Is(err, wantErr) {
		t.Fatalf("BuildIdentity() error = %v, want errors.Is(err, wantErr)", err)
	}
}

func TestBuildIdentity_DiffTreeFails(t *testing.T) {
	stubDefaultIdentitySeams(t)
	wantErr := errors.New("git diff-tree failed")
	gitDiffTree = func(string, string, string) ([]rdd.PathMode, error) { return nil, wantErr }

	_, err := BuildIdentity("/workspace/repo")
	if !errors.Is(err, wantErr) {
		t.Fatalf("BuildIdentity() error = %v, want errors.Is(err, wantErr)", err)
	}
}

func TestBuildIdentity_NoChangedPaths(t *testing.T) {
	stubDefaultIdentitySeams(t)
	gitDiffTree = func(string, string, string) ([]rdd.PathMode, error) { return nil, nil }

	got, err := BuildIdentity("/workspace/repo")
	if err != nil {
		t.Fatalf("BuildIdentity() error = %v, want nil", err)
	}
	wantDigest := rdd.ComputeDigest(nil)
	if got.ChangedPathsModesDigest != wantDigest {
		t.Errorf("BuildIdentity() ChangedPathsModesDigest = %q, want %q", got.ChangedPathsModesDigest, wantDigest)
	}
}
