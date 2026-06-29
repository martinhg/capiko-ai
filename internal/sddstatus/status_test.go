package sddstatus

import (
	"os"
	"path/filepath"
	"testing"
)

// ---------------------------------------------------------------------------
// Phase 1: text cores (countTaskProgressText, reportTextIsClearlyPassing)
// ---------------------------------------------------------------------------

func TestCountTaskProgressText_Mixed(t *testing.T) {
	content := "- [x] Step 1\n- [ ] Step 2\n- [X] Step 3"
	got := countTaskProgressText(content)
	if got.Total != 3 {
		t.Errorf("Total = %d, want 3", got.Total)
	}
	if got.Completed != 2 {
		t.Errorf("Completed = %d, want 2", got.Completed)
	}
	if got.Pending != 1 {
		t.Errorf("Pending = %d, want 1", got.Pending)
	}
	if got.AllComplete {
		t.Error("AllComplete = true, want false")
	}
}

func TestCountTaskProgressText_ProseOnly(t *testing.T) {
	content := "# Tasks\nJust some prose.\nNo checkboxes here."
	got := countTaskProgressText(content)
	if got.Total != 0 || got.Completed != 0 || got.Pending != 0 {
		t.Errorf("expected zero counts for prose-only, got %+v", got)
	}
	if got.AllComplete {
		t.Error("AllComplete = true for prose-only, want false")
	}
}

func TestCountTaskProgressText_Empty(t *testing.T) {
	got := countTaskProgressText("")
	if got.Total != 0 || got.Completed != 0 || got.Pending != 0 || got.AllComplete {
		t.Errorf("expected zero TaskProgress for empty string, got %+v", got)
	}
}

// TestCountTaskProgressText_FileParity asserts that countTaskProgressText(content)
// returns the same result as countTaskProgress applied to a temp file with the same content.
func TestCountTaskProgressText_FileParity(t *testing.T) {
	content := "- [x] Step 1\n- [ ] Step 2\n- [X] Step 3"
	dir := t.TempDir()
	p := filepath.Join(dir, "tasks.md")
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	fromText := countTaskProgressText(content)
	fromFile := countTaskProgress(p)
	if fromText != fromFile {
		t.Errorf("parity broken: text=%+v file=%+v", fromText, fromFile)
	}
}

func TestReportTextIsClearlyPassing_PassKeyword(t *testing.T) {
	text := "Verdict: PASS\nAll checks passed."
	if !reportTextIsClearlyPassing(text) {
		t.Error("expected true for text with PASS keyword, got false")
	}
}

func TestReportTextIsClearlyPassing_FailKeyword(t *testing.T) {
	text := "Verdict: FAIL\nSomething went wrong."
	if reportTextIsClearlyPassing(text) {
		t.Error("expected false for text with FAIL keyword, got true")
	}
}

func TestReportTextIsClearlyPassing_CriticalBlocker(t *testing.T) {
	text := "CRITICAL: null deref detected\nVerdict: PASS"
	if reportTextIsClearlyPassing(text) {
		t.Error("expected false for text with non-benign CRITICAL, got true")
	}
}

func TestReportTextIsClearlyPassing_NegationPattern(t *testing.T) {
	text := "Tests are not passing yet."
	if reportTextIsClearlyPassing(text) {
		t.Error("expected false for text with negation of pass, got true")
	}
}

func TestReportTextIsClearlyPassing_Empty(t *testing.T) {
	if reportTextIsClearlyPassing("") {
		t.Error("expected false for empty string, got true")
	}
}

func TestReportTextIsClearlyPassing_Blank(t *testing.T) {
	if reportTextIsClearlyPassing("   \n\t  ") {
		t.Error("expected false for blank string, got true")
	}
}

// TestReportTextIsClearlyPassing_FileParity asserts that reportTextIsClearlyPassing(text)
// returns the same result as reportIsClearlyPassing applied to a temp file with the same content.
func TestReportTextIsClearlyPassing_FileParity(t *testing.T) {
	cases := []struct {
		name    string
		content string
	}{
		{"pass", "Verdict: PASS\nAll checks passed."},
		{"fail", "Verdict: FAIL\nCRITICAL: null deref"},
		{"empty", ""},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			p := filepath.Join(dir, "verify-report.md")
			if err := os.WriteFile(p, []byte(tc.content), 0o644); err != nil {
				t.Fatal(err)
			}
			fromText := reportTextIsClearlyPassing(tc.content)
			fromFile := reportIsClearlyPassing(p)
			if fromText != fromFile {
				t.Errorf("parity broken for %q: text=%v file=%v", tc.name, fromText, fromFile)
			}
		})
	}
}

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

func TestResolveRoutesProposeWhenProposalIncomplete(t *testing.T) {
	cwd := change(t, "add-auth", map[string]string{"proposal.md": ""}) // present but empty → partial
	st, _ := Resolve(ResolveOptions{Cwd: cwd})
	if st.Dependencies.Proposal != DependencyReady {
		t.Errorf("proposal dependency = %q, want ready", st.Dependencies.Proposal)
	}
	if st.NextRecommended != string(PhasePropose) {
		t.Errorf("next = %q, want propose", st.NextRecommended)
	}
}

func TestResolveRoutesSpecAfterProposal(t *testing.T) {
	cwd := change(t, "add-auth", map[string]string{"proposal.md": "# Proposal\nwhy"})
	st, _ := Resolve(ResolveOptions{Cwd: cwd})
	if st.Dependencies.Proposal != DependencyAllDone {
		t.Errorf("proposal dependency = %q, want all_done", st.Dependencies.Proposal)
	}
	if st.Dependencies.Specs != DependencyReady {
		t.Errorf("specs dependency = %q, want ready", st.Dependencies.Specs)
	}
	if st.NextRecommended != string(PhaseSpec) {
		t.Errorf("next = %q, want spec", st.NextRecommended)
	}
}

func TestResolveRoutesDesignAfterSpec(t *testing.T) {
	cwd := change(t, "add-auth", map[string]string{
		"proposal.md": "# Proposal\nwhy",
		"spec.md":     "# Spec\nreqs",
	})
	st, _ := Resolve(ResolveOptions{Cwd: cwd})
	if st.Dependencies.Design != DependencyReady {
		t.Errorf("design dependency = %q, want ready", st.Dependencies.Design)
	}
	// tasks must stay blocked until design completes — the prereq gate must not loosen.
	if st.Dependencies.Tasks != DependencyBlocked {
		t.Errorf("tasks dependency = %q, want blocked while design incomplete", st.Dependencies.Tasks)
	}
	if st.NextRecommended != string(PhaseDesign) {
		t.Errorf("next = %q, want design", st.NextRecommended)
	}
}

func TestResolveRoutesTasksAfterDesign(t *testing.T) {
	cwd := change(t, "add-auth", map[string]string{
		"proposal.md": "# Proposal\nwhy",
		"spec.md":     "# Spec\nreqs",
		"design.md":   "# Design\napproach",
	})
	st, _ := Resolve(ResolveOptions{Cwd: cwd})
	if st.Dependencies.Tasks != DependencyReady {
		t.Errorf("tasks dependency = %q, want ready", st.Dependencies.Tasks)
	}
	if st.NextRecommended != string(PhaseTasks) {
		t.Errorf("next = %q, want tasks", st.NextRecommended)
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
	// A partial mid-DAG artifact re-routes to its own phase, not forward.
	if st.NextRecommended != string(PhaseDesign) {
		t.Errorf("next = %q, want design when design is partial", st.NextRecommended)
	}
}

// ---------------------------------------------------------------------------
// Phase 2: gating infrastructure (shouldTryEngram, configArtifactStoreIsEngram)
// ---------------------------------------------------------------------------

func TestShouldTryEngram_EnvVar(t *testing.T) {
	cwd := t.TempDir()
	t.Setenv("CAPIKO_SDD_STATUS_ENGRAM", "1")
	if !shouldTryEngram(cwd) {
		t.Error("shouldTryEngram = false with env var set, want true")
	}
}

func TestShouldTryEngram_EngramDir(t *testing.T) {
	cwd := t.TempDir()
	if err := os.MkdirAll(filepath.Join(cwd, ".engram"), 0o755); err != nil {
		t.Fatal(err)
	}
	if !shouldTryEngram(cwd) {
		t.Error("shouldTryEngram = false with .engram dir present, want true")
	}
}

func TestShouldTryEngram_ConfigArtifactStoreEngram(t *testing.T) {
	cwd := t.TempDir()
	cfgDir := filepath.Join(cwd, "openspec")
	if err := os.MkdirAll(cfgDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cfgDir, "config.yaml"), []byte("artifact_store: engram\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if !shouldTryEngram(cwd) {
		t.Error("shouldTryEngram = false with artifact_store: engram config, want true")
	}
}

func TestShouldTryEngram_ConfigArtifactStoreHybrid(t *testing.T) {
	cwd := t.TempDir()
	cfgDir := filepath.Join(cwd, "openspec")
	if err := os.MkdirAll(cfgDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cfgDir, "config.yaml"), []byte("artifact_store: hybrid\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if !shouldTryEngram(cwd) {
		t.Error("shouldTryEngram = false with artifact_store: hybrid config, want true")
	}
}

func TestShouldTryEngram_ConfigCamelCaseArtifactStore(t *testing.T) {
	cwd := t.TempDir()
	cfgDir := filepath.Join(cwd, "openspec")
	if err := os.MkdirAll(cfgDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cfgDir, "config.yaml"), []byte("artifactStore: engram\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if !shouldTryEngram(cwd) {
		t.Error("shouldTryEngram = false with camelCase artifactStore: engram, want true")
	}
}

func TestShouldTryEngram_ConfigArtifactStoreOpenspec_NoGate(t *testing.T) {
	cwd := t.TempDir()
	cfgDir := filepath.Join(cwd, "openspec")
	if err := os.MkdirAll(cfgDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cfgDir, "config.yaml"), []byte("artifact_store: openspec\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if shouldTryEngram(cwd) {
		t.Error("shouldTryEngram = true with artifact_store: openspec, want false")
	}
}

func TestShouldTryEngram_YmlExtension(t *testing.T) {
	cwd := t.TempDir()
	cfgDir := filepath.Join(cwd, "openspec")
	if err := os.MkdirAll(cfgDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cfgDir, "config.yml"), []byte("artifact_store: engram\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if !shouldTryEngram(cwd) {
		t.Error("shouldTryEngram = false with .yml extension config, want true")
	}
}

func TestShouldTryEngram_AllOff_ReturnsFalse(t *testing.T) {
	cwd := t.TempDir()
	// Explicitly clear the override so an ambient value never leaks into this case;
	// no .engram dir and no config file are created.
	t.Setenv("CAPIKO_SDD_STATUS_ENGRAM", "")
	if shouldTryEngram(cwd) {
		t.Error("shouldTryEngram = true with no triggers active, want false")
	}
}

func TestShouldTryEngram_CommentedConfig_NoGate(t *testing.T) {
	cwd := t.TempDir()
	cfgDir := filepath.Join(cwd, "openspec")
	if err := os.MkdirAll(cfgDir, 0o755); err != nil {
		t.Fatal(err)
	}
	// A commented-out artifact_store line must not gate on — only a live key counts.
	body := "# artifact_store: engram\nother_key: value\n"
	if err := os.WriteFile(filepath.Join(cfgDir, "config.yaml"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	if shouldTryEngram(cwd) {
		t.Error("shouldTryEngram = true with a commented-out artifact_store, want false")
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
	// A present-but-malformed tasks artifact is "done" (has content), so it must NOT
	// re-route to a clean planning phase; the engine reports a generic blocker instead.
	if st.Dependencies.Tasks != DependencyAllDone {
		t.Errorf("tasks dependency = %q, want all_done (artifact has content)", st.Dependencies.Tasks)
	}
	if st.NextRecommended != "resolve-blockers" {
		t.Errorf("next = %q, want resolve-blockers for malformed tasks", st.NextRecommended)
	}
}
