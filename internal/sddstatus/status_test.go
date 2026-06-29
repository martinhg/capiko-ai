package sddstatus

import (
	"os"
	"path/filepath"
	"strings"
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

// ---------------------------------------------------------------------------
// Phase 3: Engram export seam + observation helpers
// ---------------------------------------------------------------------------

// withEngram swaps the engramExport seam to return canned observations and
// restores the original via t.Cleanup. Tests that exercise the Engram fallback
// path call this helper instead of touching the real engram binary.
func withEngram(t *testing.T, obs []engramObservation) {
	t.Helper()
	prev := engramExport
	engramExport = func() ([]engramObservation, error) { return obs, nil }
	t.Cleanup(func() { engramExport = prev })
}

// SC-11: The export seam must never call the real engram binary in tests.
func TestEngramSeamIsolation(t *testing.T) {
	prev := engramExport
	engramExport = func() ([]engramObservation, error) {
		t.Fatal("the real engram binary must never be invoked during tests")
		return nil, nil
	}
	t.Cleanup(func() { engramExport = prev })

	// Resolve with all gating OFF — the seam must not be consulted.
	cwd := t.TempDir()
	t.Setenv("CAPIKO_SDD_STATUS_ENGRAM", "")
	st, err := Resolve(ResolveOptions{Cwd: cwd})
	if err != nil {
		t.Fatal(err)
	}
	if st.NextRecommended != "sdd-new" {
		t.Errorf("expected sdd-new with no active changes and no gating, got %q", st.NextRecommended)
	}
}

// ---------- inferEngramProject / projectFromGitConfig ----------

func TestInferEngramProject_EnvVar(t *testing.T) {
	t.Setenv("ENGRAM_PROJECT", "acme/my-service")
	cwd := t.TempDir()
	if got := inferEngramProject(cwd); got != "acme/my-service" {
		t.Errorf("got %q, want acme/my-service", got)
	}
}

func TestInferEngramProject_GitConfigHTTPS(t *testing.T) {
	t.Setenv("ENGRAM_PROJECT", "")
	cwd := t.TempDir()
	gitDir := filepath.Join(cwd, ".git")
	if err := os.MkdirAll(gitDir, 0o755); err != nil {
		t.Fatal(err)
	}
	cfg := "[core]\n\trepositoryformatversion = 0\n[remote \"origin\"]\n\turl = https://github.com/myorg/myrepo.git\n\tfetch = +refs/heads/*:refs/remotes/origin/*\n"
	if err := os.WriteFile(filepath.Join(gitDir, "config"), []byte(cfg), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := inferEngramProject(cwd); got != "myorg/myrepo" {
		t.Errorf("got %q, want myorg/myrepo", got)
	}
}

func TestInferEngramProject_GitConfigSSH(t *testing.T) {
	t.Setenv("ENGRAM_PROJECT", "")
	cwd := t.TempDir()
	gitDir := filepath.Join(cwd, ".git")
	if err := os.MkdirAll(gitDir, 0o755); err != nil {
		t.Fatal(err)
	}
	cfg := "[remote \"origin\"]\n\turl = git@github.com:myorg/myrepo.git\n"
	if err := os.WriteFile(filepath.Join(gitDir, "config"), []byte(cfg), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := inferEngramProject(cwd); got != "myorg/myrepo" {
		t.Errorf("got %q, want myorg/myrepo", got)
	}
}

func TestInferEngramProject_DirBasename(t *testing.T) {
	t.Setenv("ENGRAM_PROJECT", "")
	// No .git/config — falls back to lowercased basename.
	cwd := t.TempDir()
	got := inferEngramProject(cwd)
	want := strings.ToLower(filepath.Base(cwd))
	if got != want {
		t.Errorf("got %q, want %q (lowercased basename)", got, want)
	}
}

func TestProjectFromGitConfig_NoGitDir(t *testing.T) {
	cwd := t.TempDir()
	if got := projectFromGitConfig(cwd); got != "" {
		t.Errorf("got %q, want empty when .git/config is absent", got)
	}
}

func TestProjectFromGitConfig_NoOriginSection(t *testing.T) {
	cwd := t.TempDir()
	gitDir := filepath.Join(cwd, ".git")
	if err := os.MkdirAll(gitDir, 0o755); err != nil {
		t.Fatal(err)
	}
	cfg := "[core]\n\trepositoryformatversion = 0\n"
	if err := os.WriteFile(filepath.Join(gitDir, "config"), []byte(cfg), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := projectFromGitConfig(cwd); got != "" {
		t.Errorf("got %q, want empty when no origin section", got)
	}
}

// ---------- collectEngramChanges ----------

func TestCollectEngramChanges_OneChange(t *testing.T) {
	obs := []engramObservation{
		{Title: "sdd/my-feature/proposal", Content: "...", Project: "myorg/myrepo", Scope: "project"},
		{Title: "sdd/my-feature/spec", Content: "...", Project: "myorg/myrepo", Scope: "project"},
	}
	got := collectEngramChanges(obs, "myorg/myrepo")
	if len(got) != 1 || got[0] != "my-feature" {
		t.Errorf("got %v, want [my-feature]", got)
	}
}

func TestCollectEngramChanges_TwoChanges(t *testing.T) {
	obs := []engramObservation{
		{Title: "sdd/feat-a/proposal", Project: "myorg/myrepo", Scope: "project"},
		{Title: "sdd/feat-b/proposal", Project: "myorg/myrepo", Scope: "project"},
	}
	got := collectEngramChanges(obs, "myorg/myrepo")
	if len(got) != 2 {
		t.Errorf("got %v, want [feat-a feat-b]", got)
	}
	if got[0] != "feat-a" || got[1] != "feat-b" {
		t.Errorf("got %v, want sorted [feat-a feat-b]", got)
	}
}

func TestCollectEngramChanges_ExcludesPersonalScope(t *testing.T) {
	obs := []engramObservation{
		{Title: "sdd/my-feature/proposal", Project: "myorg/myrepo", Scope: "personal"},
	}
	got := collectEngramChanges(obs, "myorg/myrepo")
	if len(got) != 0 {
		t.Errorf("personal-scope observations must be excluded, got %v", got)
	}
}

func TestCollectEngramChanges_ExcludesProjectMismatch(t *testing.T) {
	obs := []engramObservation{
		{Title: "sdd/my-feature/proposal", Project: "other/repo", Scope: "project"},
	}
	got := collectEngramChanges(obs, "myorg/myrepo")
	if len(got) != 0 {
		t.Errorf("project-mismatch observations must be excluded, got %v", got)
	}
}

func TestCollectEngramChanges_Empty(t *testing.T) {
	got := collectEngramChanges(nil, "myorg/myrepo")
	if len(got) != 0 {
		t.Errorf("got %v, want empty slice for nil observations", got)
	}
}

// ---------- selectEngramChange ----------

func TestSelectEngramChange_SingleNoRequest(t *testing.T) {
	name, ok := selectEngramChange([]string{"my-feature"}, "")
	if !ok || name != "my-feature" {
		t.Errorf("got (%q, %v), want (my-feature, true)", name, ok)
	}
}

func TestSelectEngramChange_TwoChangesNoRequest(t *testing.T) {
	_, ok := selectEngramChange([]string{"feat-a", "feat-b"}, "")
	if ok {
		t.Error("two changes with no request should return (_, false)")
	}
}

func TestSelectEngramChange_ZeroChangesNoRequest(t *testing.T) {
	_, ok := selectEngramChange([]string{}, "")
	if ok {
		t.Error("zero changes should return (_, false)")
	}
}

func TestSelectEngramChange_RequestedFound(t *testing.T) {
	name, ok := selectEngramChange([]string{"feat-a", "feat-b"}, "feat-b")
	if !ok || name != "feat-b" {
		t.Errorf("got (%q, %v), want (feat-b, true)", name, ok)
	}
}

func TestSelectEngramChange_RequestedNotFound(t *testing.T) {
	_, ok := selectEngramChange([]string{"feat-a"}, "ghost")
	if ok {
		t.Error("requested change not in list should return (_, false)")
	}
}

// ---------- engramArtifactState ----------

func TestEngramArtifactState_PresentNonEmpty(t *testing.T) {
	if got := engramArtifactState("some content", true); got != ArtifactDone {
		t.Errorf("got %q, want done", got)
	}
}

func TestEngramArtifactState_Absent(t *testing.T) {
	if got := engramArtifactState("", false); got != ArtifactMissing {
		t.Errorf("got %q, want missing", got)
	}
}

func TestEngramArtifactState_PresentButEmpty(t *testing.T) {
	if got := engramArtifactState("   ", true); got != ArtifactPartial {
		t.Errorf("got %q, want partial", got)
	}
}

// ---------- engramArtifactContent ----------

func TestEngramArtifactContent_Found(t *testing.T) {
	obs := []engramObservation{
		{Title: "sdd/my-feature/tasks", Content: "- [ ] do it", Project: "p", Scope: "project"},
	}
	got := engramArtifactContent(obs, "my-feature", "p", "tasks")
	if got != "- [ ] do it" {
		t.Errorf("got %q, want %q", got, "- [ ] do it")
	}
}

func TestEngramArtifactContent_NotFound(t *testing.T) {
	got := engramArtifactContent(nil, "my-feature", "p", "tasks")
	if got != "" {
		t.Errorf("got %q, want empty for missing observation", got)
	}
}

func TestEngramArtifactContent_ProjectMismatch(t *testing.T) {
	obs := []engramObservation{
		{Title: "sdd/my-feature/tasks", Content: "- [ ] do it", Project: "other/repo", Scope: "project"},
	}
	got := engramArtifactContent(obs, "my-feature", "myorg/myrepo", "tasks")
	if got != "" {
		t.Errorf("got %q, want empty for project mismatch", got)
	}
}

// ---------- engramArtifactsForChange ----------

func TestEngramArtifactsForChange_SixKeyMap(t *testing.T) {
	obs := []engramObservation{
		{Title: "sdd/my-feature/proposal", Content: "# Proposal", Project: "p", Scope: "project"},
		{Title: "sdd/my-feature/spec", Content: "# Spec", Project: "p", Scope: "project"},
		{Title: "sdd/my-feature/design", Content: "# Design", Project: "p", Scope: "project"},
		{Title: "sdd/my-feature/tasks", Content: "- [ ] do it", Project: "p", Scope: "project"},
		// applyProgress and verifyReport absent
	}
	got := engramArtifactsForChange(obs, "my-feature", "p")
	required := []string{"proposal", "specs", "design", "tasks", "applyProgress", "verifyReport"}
	for _, k := range required {
		if _, ok := got[k]; !ok {
			t.Errorf("artifact map is missing key %q", k)
		}
	}
	if got["proposal"] != ArtifactDone {
		t.Errorf("proposal = %q, want done", got["proposal"])
	}
	if got["specs"] != ArtifactDone {
		t.Errorf("specs = %q, want done", got["specs"])
	}
	if got["design"] != ArtifactDone {
		t.Errorf("design = %q, want done", got["design"])
	}
	if got["tasks"] != ArtifactDone {
		t.Errorf("tasks = %q, want done", got["tasks"])
	}
	if got["applyProgress"] != ArtifactMissing {
		t.Errorf("applyProgress = %q, want missing", got["applyProgress"])
	}
	if got["verifyReport"] != ArtifactMissing {
		t.Errorf("verifyReport = %q, want missing", got["verifyReport"])
	}
}

// ---------- engramArtifactPaths ----------

func TestEngramArtifactPaths_SentinelForNonMissing(t *testing.T) {
	artifacts := map[string]ArtifactState{
		"proposal":      ArtifactDone,
		"specs":         ArtifactDone,
		"design":        ArtifactMissing,
		"tasks":         ArtifactMissing,
		"applyProgress": ArtifactMissing,
		"verifyReport":  ArtifactMissing,
	}
	paths := engramArtifactPaths("my-feature", artifacts)
	if len(paths.Proposal) != 1 || paths.Proposal[0] != "engram:sdd/my-feature/proposal" {
		t.Errorf("Proposal = %v, want [engram:sdd/my-feature/proposal]", paths.Proposal)
	}
	if len(paths.Specs) != 1 || paths.Specs[0] != "engram:sdd/my-feature/spec" {
		t.Errorf("Specs = %v, want [engram:sdd/my-feature/spec]", paths.Specs)
	}
	if len(paths.Design) != 0 {
		t.Errorf("Design = %v, want empty slice for missing artifact", paths.Design)
	}
	if len(paths.Tasks) != 0 {
		t.Errorf("Tasks = %v, want empty slice for missing artifact", paths.Tasks)
	}
	if len(paths.ApplyProgress) != 0 {
		t.Errorf("ApplyProgress = %v, want empty slice for missing artifact", paths.ApplyProgress)
	}
	if len(paths.VerifyReport) != 0 {
		t.Errorf("VerifyReport = %v, want empty slice for missing artifact", paths.VerifyReport)
	}
}

func TestEngramArtifactPaths_AllMissing(t *testing.T) {
	artifacts := map[string]ArtifactState{
		"proposal": ArtifactMissing, "specs": ArtifactMissing, "design": ArtifactMissing,
		"tasks": ArtifactMissing, "applyProgress": ArtifactMissing, "verifyReport": ArtifactMissing,
	}
	paths := engramArtifactPaths("my-feature", artifacts)
	if len(paths.Proposal)+len(paths.Specs)+len(paths.Design)+len(paths.Tasks)+len(paths.ApplyProgress)+len(paths.VerifyReport) != 0 {
		t.Errorf("expected all empty slices for all-missing artifacts, got %+v", paths)
	}
}

func TestEngramArtifactPaths_AllPresent(t *testing.T) {
	artifacts := map[string]ArtifactState{
		"proposal": ArtifactDone, "specs": ArtifactDone, "design": ArtifactDone,
		"tasks": ArtifactDone, "applyProgress": ArtifactDone, "verifyReport": ArtifactDone,
	}
	paths := engramArtifactPaths("my-feature", artifacts)
	cases := []struct {
		got  []string
		want string
	}{
		{paths.Proposal, "engram:sdd/my-feature/proposal"},
		{paths.Specs, "engram:sdd/my-feature/spec"},
		{paths.Design, "engram:sdd/my-feature/design"},
		{paths.Tasks, "engram:sdd/my-feature/tasks"},
		{paths.ApplyProgress, "engram:sdd/my-feature/apply-progress"},
		{paths.VerifyReport, "engram:sdd/my-feature/verify-report"},
	}
	for _, tc := range cases {
		if len(tc.got) != 1 || tc.got[0] != tc.want {
			t.Errorf("got %v, want [%s]", tc.got, tc.want)
		}
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
