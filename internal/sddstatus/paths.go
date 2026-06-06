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
type ArtifactPaths struct {
	Proposal      []string
	Specs         []string // spec.md and/or specs/*.md
	Design        []string
	Tasks         []string
	ApplyProgress []string
	VerifyReport  []string
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
// artifact is tolerant of both layouts capiko's skills use: a single spec.md and a
// specs/ directory of capability files (both are included when present).
func ResolveArtifactPaths(cwd, change string) ArtifactPaths {
	root := changeRoot(cwd, change)
	return ArtifactPaths{
		Proposal:      existing(filepath.Join(root, "proposal.md")),
		Specs:         append(existing(filepath.Join(root, "spec.md")), markdownIn(filepath.Join(root, "specs"))...),
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

// markdownIn returns the sorted .md files directly inside dir, or nil when dir is
// absent or empty.
func markdownIn(dir string) []string {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	var out []string
	for _, e := range entries {
		if !e.IsDir() && filepath.Ext(e.Name()) == ".md" {
			out = append(out, filepath.Join(dir, e.Name()))
		}
	}
	sort.Strings(out)
	return out
}
