package sddstatus

import (
	"os"
	"path/filepath"
	"testing"
)

// writeFile creates a file (and parent dirs) with trivial content.
func writeFile(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestListActiveOpenSpecChanges(t *testing.T) {
	cwd := t.TempDir()
	changes := filepath.Join(cwd, "openspec", "changes")
	for _, name := range []string{"fix-bug", "add-auth"} {
		writeFile(t, filepath.Join(changes, name, "proposal.md"))
	}
	// An archive/ entry must be excluded — it holds completed changes, not active.
	writeFile(t, filepath.Join(changes, "archive", "old-change", "proposal.md"))

	got, err := ListActiveOpenSpecChanges(cwd)
	if err != nil {
		t.Fatalf("ListActiveOpenSpecChanges: %v", err)
	}
	if len(got) != 2 || got[0] != "add-auth" || got[1] != "fix-bug" {
		t.Errorf("changes = %v, want [add-auth fix-bug] sorted, archive excluded", got)
	}
}

func TestListActiveOpenSpecChangesMissingDir(t *testing.T) {
	got, err := ListActiveOpenSpecChanges(t.TempDir())
	if err != nil {
		t.Fatalf("missing openspec dir should not error: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("got %v, want no changes", got)
	}
}

func TestResolveArtifactPaths(t *testing.T) {
	cwd := t.TempDir()
	root := filepath.Join(cwd, "openspec", "changes", "add-auth")
	writeFile(t, filepath.Join(root, "proposal.md"))
	writeFile(t, filepath.Join(root, "spec.md"))
	writeFile(t, filepath.Join(root, "design.md"))
	writeFile(t, filepath.Join(root, "tasks.md"))
	// design-only fields left absent: apply-progress.md, verify-report.md.

	p := ResolveArtifactPaths(cwd, "add-auth")
	if len(p.Proposal) != 1 || filepath.Base(p.Proposal[0]) != "proposal.md" {
		t.Errorf("Proposal = %v", p.Proposal)
	}
	if len(p.Specs) != 1 || filepath.Base(p.Specs[0]) != "spec.md" {
		t.Errorf("Specs = %v, want [spec.md]", p.Specs)
	}
	if len(p.Design) != 1 || len(p.Tasks) != 1 {
		t.Errorf("Design=%v Tasks=%v", p.Design, p.Tasks)
	}
	if len(p.ApplyProgress) != 0 || len(p.VerifyReport) != 0 {
		t.Errorf("absent artifacts should be empty: apply=%v verify=%v", p.ApplyProgress, p.VerifyReport)
	}
}

func TestResolveArtifactPathsSpecIsDeltaFileOnly(t *testing.T) {
	// capiko's per-change spec delta is a single spec.md. A specs/ directory under
	// the change is NOT capiko's layout (that is gentle-ai's per-capability deltas,
	// and the top-level openspec/specs/ holds the canonical specs) — it must be
	// ignored, not mistaken for the change's spec.
	cwd := t.TempDir()
	root := filepath.Join(cwd, "openspec", "changes", "add-auth")
	writeFile(t, filepath.Join(root, "spec.md"))
	writeFile(t, filepath.Join(root, "specs", "stray.md"))

	p := ResolveArtifactPaths(cwd, "add-auth")
	if len(p.Specs) != 1 || filepath.Base(p.Specs[0]) != "spec.md" {
		t.Errorf("Specs = %v, want only the spec.md delta (specs/ under a change ignored)", p.Specs)
	}
}
