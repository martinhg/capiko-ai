// Package sddstatus is capiko's native SDD engine. It computes the state of an
// SDD change deterministically from the OpenSpec store (openspec/changes/<change>),
// so the agent can read authoritative status instead of inferring it from prose.
//
// This file covers OpenSpec discovery: locating active changes and resolving the
// on-disk paths of each artifact. The state machine that interprets them lives in
// the resolver (a later slice).
package sddstatus

import (
	"os"
	"path/filepath"
	"sort"
)

// ArtifactPaths holds the existing on-disk paths of each SDD artifact for a
// change. Each field lists only files that are actually present; a missing
// artifact is an empty slice, never a path that does not exist.
//
// Specs is the per-change spec *delta* — capiko writes a single spec.md under the
// change. It is kept as a slice for shape-compatibility with the status schema
// (which allows multiple spec files) and to stay forward-compatible. It is
// distinct from the top-level openspec/specs/ canonical specs, which are change
// context, not a change artifact.
type ArtifactPaths struct {
	Proposal      []string `json:"proposal"`
	Specs         []string `json:"specs"` // the change's spec.md delta
	Design        []string `json:"design"`
	Tasks         []string `json:"tasks"`
	ApplyProgress []string `json:"applyProgress"`
	VerifyReport  []string `json:"verifyReport"`
}

// withArrays returns a copy in which every nil slice is an empty slice, so JSON
// serializes missing artifacts as [] rather than null (the status contract
// requires arrays).
func (a ArtifactPaths) withArrays() ArtifactPaths {
	arr := func(s []string) []string {
		if s == nil {
			return []string{}
		}
		return s
	}
	return ArtifactPaths{
		Proposal:      arr(a.Proposal),
		Specs:         arr(a.Specs),
		Design:        arr(a.Design),
		Tasks:         arr(a.Tasks),
		ApplyProgress: arr(a.ApplyProgress),
		VerifyReport:  arr(a.VerifyReport),
	}
}

// OpenSpecDir returns the openspec directory under a workspace root.
func OpenSpecDir(cwd string) string { return filepath.Join(cwd, "openspec") }

// changesDir returns the openspec/changes directory under a workspace root.
func changesDir(cwd string) string { return filepath.Join(OpenSpecDir(cwd), "changes") }

// changeRoot returns the directory holding a change's artifacts.
func changeRoot(cwd, change string) string { return filepath.Join(changesDir(cwd), change) }

// ListActiveOpenSpecChanges returns the names of active changes — the immediate
// subdirectories of openspec/changes/, excluding the archive/ folder of completed
// changes — sorted. A missing openspec/changes directory yields no changes and no
// error, so callers can report "no active change" cleanly.
func ListActiveOpenSpecChanges(cwd string) ([]string, error) {
	entries, err := os.ReadDir(changesDir(cwd))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var changes []string
	for _, e := range entries {
		if !e.IsDir() || e.Name() == "archive" {
			continue
		}
		changes = append(changes, e.Name())
	}
	sort.Strings(changes)
	return changes, nil
}

// ResolveArtifactPaths returns the existing artifact paths for a change. The spec
// is the change's spec.md delta; a specs/ directory under the change is not
// capiko's layout (the canonical openspec/specs/ is separate) and is ignored.
func ResolveArtifactPaths(cwd, change string) ArtifactPaths {
	root := changeRoot(cwd, change)
	return ArtifactPaths{
		Proposal:      existing(filepath.Join(root, "proposal.md")),
		Specs:         existing(filepath.Join(root, "spec.md")),
		Design:        existing(filepath.Join(root, "design.md")),
		Tasks:         existing(filepath.Join(root, "tasks.md")),
		ApplyProgress: existing(filepath.Join(root, "apply-progress.md")),
		VerifyReport:  existing(filepath.Join(root, "verify-report.md")),
	}
}

// existing returns a one-element slice with path when it is a regular file, else
// an empty slice.
func existing(path string) []string {
	if info, err := os.Stat(path); err == nil && !info.IsDir() {
		return []string{path}
	}
	return nil
}
