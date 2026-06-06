package sddstatus

import (
	"os"
	"path/filepath"
	"testing"
)

// change writes a set of artifact files for a change and returns the workspace
// root. Each map entry is a relative path under the change dir → file content.
func change(t *testing.T, name string, files map[string]string) string {
	t.Helper()
	cwd := t.TempDir()
	root := filepath.Join(cwd, "openspec", "changes", name)
	for rel, content := range files {
		p := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return cwd
}

// coreArtifacts is a planning-complete change with one unchecked task.
func coreArtifacts() map[string]string {
	return map[string]string{
		"proposal.md": "# Proposal\nwhy",
		"spec.md":     "# Spec\nreqs",
		"design.md":   "# Design\napproach",
		"tasks.md":    "- [ ] 1. do the thing",
	}
}

func TestResolveNoActiveChanges(t *testing.T) {
	st, err := Resolve(ResolveOptions{Cwd: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	if st.NextRecommended != "sdd-new" {
		t.Errorf("nextRecommended = %q, want sdd-new", st.NextRecommended)
	}
	if len(st.BlockedReasons) == 0 {
		t.Error("expected a blocked reason for no active changes")
	}
}

func TestResolveAmbiguousChange(t *testing.T) {
	cwd := t.TempDir()
	for _, n := range []string{"a", "b"} {
		change := filepath.Join(cwd, "openspec", "changes", n)
		_ = os.MkdirAll(change, 0o755)
		_ = os.WriteFile(filepath.Join(change, "proposal.md"), []byte("x"), 0o644)
	}
	st, err := Resolve(ResolveOptions{Cwd: cwd})
	if err != nil {
		t.Fatal(err)
	}
	if st.NextRecommended != "select-change" {
		t.Errorf("nextRecommended = %q, want select-change", st.NextRecommended)
	}
}

func TestResolveNamedChangeNotFound(t *testing.T) {
	st, err := Resolve(ResolveOptions{Cwd: t.TempDir(), ChangeName: "ghost"})
	if err != nil {
		t.Fatal(err)
	}
	if st.NextRecommended != "sdd-new" || len(st.BlockedReasons) == 0 {
		t.Errorf("missing named change should block: next=%q reasons=%v", st.NextRecommended, st.BlockedReasons)
	}
}

func TestResolveApplyReady(t *testing.T) {
	cwd := change(t, "add-auth", coreArtifacts())
	st, err := Resolve(ResolveOptions{Cwd: cwd})
	if err != nil {
		t.Fatal(err)
	}
	if st.ApplyState != ApplyReady {
		t.Errorf("applyState = %q, want ready", st.ApplyState)
	}
	if st.Dependencies.Apply != DependencyReady {
		t.Errorf("apply dep = %q, want ready", st.Dependencies.Apply)
	}
	if st.NextRecommended != string(PhaseApply) {
		t.Errorf("next = %q, want apply", st.NextRecommended)
	}
	if st.TaskProgress.Total != 1 || st.TaskProgress.Pending != 1 || st.TaskProgress.AllComplete {
		t.Errorf("task progress = %+v", st.TaskProgress)
	}
}

func TestResolveVerifyReadyWhenTasksComplete(t *testing.T) {
	files := coreArtifacts()
	files["tasks.md"] = "- [x] 1. done"
	cwd := change(t, "add-auth", files)
	st, _ := Resolve(ResolveOptions{Cwd: cwd})
	if st.ApplyState != ApplyAllDone {
		t.Errorf("applyState = %q, want all_done", st.ApplyState)
	}
	if st.Dependencies.Verify != DependencyReady {
		t.Errorf("verify dep = %q, want ready", st.Dependencies.Verify)
	}
	if st.NextRecommended != string(PhaseVerify) {
		t.Errorf("next = %q, want verify", st.NextRecommended)
	}
}

func TestResolveArchiveReadyWhenVerifyPasses(t *testing.T) {
	files := coreArtifacts()
	files["tasks.md"] = "- [x] 1. done"
	files["verify-report.md"] = "# Verify\nVerdict: PASS\nAll checks passed."
	cwd := change(t, "add-auth", files)
	st, _ := Resolve(ResolveOptions{Cwd: cwd})
	if st.Dependencies.Verify != DependencyAllDone {
		t.Errorf("verify dep = %q, want all_done", st.Dependencies.Verify)
	}
	if st.Dependencies.Archive != DependencyReady {
		t.Errorf("archive dep = %q, want ready", st.Dependencies.Archive)
	}
	if st.NextRecommended != string(PhaseArchive) {
		t.Errorf("next = %q, want archive", st.NextRecommended)
	}
}

func TestResolveVerifyReportFailingBlocksArchive(t *testing.T) {
	files := coreArtifacts()
	files["tasks.md"] = "- [x] 1. done"
	files["verify-report.md"] = "# Verify\nVerdict: FAIL\nCRITICAL: null deref"
	cwd := change(t, "add-auth", files)
	st, _ := Resolve(ResolveOptions{Cwd: cwd})
	if st.Dependencies.Archive == DependencyReady {
		t.Error("a failing verify-report must not make archive ready")
	}
	blocked := false
	for _, r := range st.BlockedReasons {
		if r == "verify-report.md is not clearly passing." {
			blocked = true
		}
	}
	if !blocked {
		t.Errorf("expected a not-clearly-passing blocker, got %v", st.BlockedReasons)
	}
}

func TestResolvePartialArtifactBlocks(t *testing.T) {
	files := coreArtifacts()
	files["design.md"] = "   " // present but empty → partial
	cwd := change(t, "add-auth", files)
	st, _ := Resolve(ResolveOptions{Cwd: cwd})
	if st.Artifacts["design"] != ArtifactPartial {
		t.Errorf("design state = %q, want partial", st.Artifacts["design"])
	}
	if st.ApplyState != ApplyBlocked {
		t.Errorf("applyState = %q, want blocked when a core artifact is partial", st.ApplyState)
	}
}

func TestResolveMissingTasksCheckboxesBlocks(t *testing.T) {
	files := coreArtifacts()
	files["tasks.md"] = "# Tasks\njust prose, no checkboxes"
	cwd := change(t, "add-auth", files)
	st, _ := Resolve(ResolveOptions{Cwd: cwd})
	if st.ApplyState != ApplyBlocked {
		t.Errorf("applyState = %q, want blocked when tasks has no checkboxes", st.ApplyState)
	}
}
