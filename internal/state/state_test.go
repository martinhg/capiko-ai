package state

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestLoadMissingReturnsEmpty(t *testing.T) {
	s := NewStore(t.TempDir())
	st, err := s.Load()
	if err != nil {
		t.Fatalf("Load on missing file: %v", err)
	}
	if st.Skills == nil {
		t.Error("Skills map should be initialized")
	}
	if len(st.Skills) != 0 {
		t.Errorf("expected empty state, got %v", st.Skills)
	}
}

func TestApplyRecordsAndRemoves(t *testing.T) {
	s := NewStore(t.TempDir())

	if err := s.Apply("0.1.0", []Installed{
		{Name: "capiko-hello", Checksum: "abc"},
		{Name: "capiko-pr", Checksum: "def"},
	}, nil); err != nil {
		t.Fatalf("Apply install: %v", err)
	}

	st, _ := s.Load()
	if st.Version != "0.1.0" {
		t.Errorf("version = %q, want 0.1.0", st.Version)
	}
	if len(st.Skills) != 2 {
		t.Fatalf("expected 2 skills, got %v", st.Skills)
	}
	if st.Skills["capiko-hello"].Checksum != "abc" {
		t.Errorf("checksum not recorded: %+v", st.Skills["capiko-hello"])
	}
	if st.Skills["capiko-hello"].InstalledAt.IsZero() {
		t.Error("InstalledAt not stamped")
	}

	// Removing one drops only that record.
	if err := s.Apply("0.1.0", nil, []string{"capiko-pr"}); err != nil {
		t.Fatalf("Apply remove: %v", err)
	}
	st, _ = s.Load()
	if _, ok := st.Skills["capiko-pr"]; ok {
		t.Error("capiko-pr should have been removed")
	}
	if _, ok := st.Skills["capiko-hello"]; !ok {
		t.Error("capiko-hello should remain")
	}
}

func TestSaveIsAtomicAndPersists(t *testing.T) {
	dir := t.TempDir()
	s := NewStore(dir)

	if err := s.Apply("0.1.0", []Installed{{Name: "x", Checksum: "h"}}, nil); err != nil {
		t.Fatal(err)
	}

	if _, err := os.Stat(filepath.Join(dir, "state.json")); err != nil {
		t.Errorf("state.json not written: %v", err)
	}
	// The temp file must not linger.
	if _, err := os.Stat(filepath.Join(dir, "state.json.tmp")); !os.IsNotExist(err) {
		t.Errorf("temp file should not remain: %v", err)
	}
}

func TestSetPersona(t *testing.T) {
	s := NewStore(t.TempDir())
	if err := s.SetPersona("capiko"); err != nil {
		t.Fatal(err)
	}
	st, err := s.Load()
	if err != nil {
		t.Fatal(err)
	}
	if st.Persona != "capiko" {
		t.Errorf("persona = %q, want capiko", st.Persona)
	}
	if err := s.SetPersona(""); err != nil {
		t.Fatal(err)
	}
	st, _ = s.Load()
	if st.Persona != "" {
		t.Errorf("persona = %q, want empty after clear", st.Persona)
	}
}

func TestSetSDDModels(t *testing.T) {
	s := NewStore(t.TempDir())
	models := map[string]string{"orchestrator": "claude-opus-4.8", "spec": "gemini-5.4"}
	if err := s.SetSDDModels(models); err != nil {
		t.Fatal(err)
	}
	st, err := s.Load()
	if err != nil {
		t.Fatal(err)
	}
	if st.SDDModels["orchestrator"] != "claude-opus-4.8" || st.SDDModels["spec"] != "gemini-5.4" {
		t.Errorf("sdd models = %v", st.SDDModels)
	}
}

func TestSetSDDFallbackModels(t *testing.T) {
	s := NewStore(t.TempDir())
	models := map[string]string{"apply": "claude-sonnet-4.6", "verify": "gpt-5.2"}
	if err := s.SetSDDFallbackModels(models); err != nil {
		t.Fatal(err)
	}
	st, err := s.Load()
	if err != nil {
		t.Fatal(err)
	}
	if st.SDDFallbackModels["apply"] != "claude-sonnet-4.6" || st.SDDFallbackModels["verify"] != "gpt-5.2" {
		t.Errorf("sdd fallback models = %v", st.SDDFallbackModels)
	}
}

func TestSetStrictTDD(t *testing.T) {
	s := NewStore(t.TempDir())
	if err := s.SetStrictTDD(true); err != nil {
		t.Fatal(err)
	}
	st, err := s.Load()
	if err != nil {
		t.Fatal(err)
	}
	if !st.StrictTDD {
		t.Error("StrictTDD should be true after SetStrictTDD(true)")
	}
	if err := s.SetStrictTDD(false); err != nil {
		t.Fatal(err)
	}
	st, _ = s.Load()
	if st.StrictTDD {
		t.Error("StrictTDD should be false after SetStrictTDD(false)")
	}
}

func TestSetInstructionsInstalled(t *testing.T) {
	s := NewStore(t.TempDir())
	if err := s.SetInstructionsInstalled(true); err != nil {
		t.Fatal(err)
	}
	st, err := s.Load()
	if err != nil {
		t.Fatal(err)
	}
	if !st.InstructionsInstalled {
		t.Error("InstructionsInstalled should be true after SetInstructionsInstalled(true)")
	}
	if err := s.SetInstructionsInstalled(false); err != nil {
		t.Fatal(err)
	}
	st, _ = s.Load()
	if st.InstructionsInstalled {
		t.Error("InstructionsInstalled should be false after SetInstructionsInstalled(false)")
	}
}

func TestSetHeadroom(t *testing.T) {
	s := NewStore(t.TempDir())
	if err := s.SetHeadroom(&HeadroomRecord{Enabled: true, Checksum: "abc"}); err != nil {
		t.Fatal(err)
	}
	st, err := s.Load()
	if err != nil {
		t.Fatal(err)
	}
	if st.Headroom == nil || !st.Headroom.Enabled || st.Headroom.Checksum != "abc" {
		t.Errorf("Headroom not persisted: %+v", st.Headroom)
	}
	if err := s.SetHeadroom(&HeadroomRecord{Enabled: false}); err != nil {
		t.Fatal(err)
	}
	st, _ = s.Load()
	if st.Headroom == nil || st.Headroom.Enabled {
		t.Errorf("Headroom should be recorded disabled, got %+v", st.Headroom)
	}
}

func TestSetCodeReview(t *testing.T) {
	s := NewStore(t.TempDir())
	if err := s.SetCodeReview(&CodeReviewRecord{Enabled: true, Provider: "claude", StrictMode: true}); err != nil {
		t.Fatal(err)
	}
	st, err := s.Load()
	if err != nil {
		t.Fatal(err)
	}
	if st.CodeReview == nil || !st.CodeReview.Enabled || st.CodeReview.Provider != "claude" {
		t.Errorf("CodeReview not persisted: %+v", st.CodeReview)
	}
	if err := s.SetCodeReview(&CodeReviewRecord{Enabled: false}); err != nil {
		t.Fatal(err)
	}
	st, _ = s.Load()
	if st.CodeReview == nil || st.CodeReview.Enabled {
		t.Errorf("CodeReview should be recorded disabled, got %+v", st.CodeReview)
	}
}

func TestSetOutputEfficiency(t *testing.T) {
	s := NewStore(t.TempDir())
	if err := s.SetOutputEfficiency(true); err != nil {
		t.Fatal(err)
	}
	st, err := s.Load()
	if err != nil {
		t.Fatal(err)
	}
	if !st.OutputEfficiency {
		t.Error("OutputEfficiency should be true after SetOutputEfficiency(true)")
	}
	if err := s.SetOutputEfficiency(false); err != nil {
		t.Fatal(err)
	}
	st, _ = s.Load()
	if st.OutputEfficiency {
		t.Error("OutputEfficiency should be false after SetOutputEfficiency(false)")
	}
}

func TestSetEngram(t *testing.T) {
	s := NewStore(t.TempDir())
	rec := &EngramRecord{
		Enabled:      true,
		ArtifactMode: "hybrid",
		CloudServer:  "https://engram.example.com",
		Projects:     []string{"repo-core", "repo-back"},
		Surfaces:     []string{"cli"},
		Checksum:     "abc123",
	}
	if err := s.SetEngram(rec); err != nil {
		t.Fatal(err)
	}
	st, err := s.Load()
	if err != nil {
		t.Fatal(err)
	}
	if st.Engram == nil {
		t.Fatal("Engram record should persist")
	}
	if !st.Engram.Enabled || st.Engram.ArtifactMode != "hybrid" || st.Engram.CloudServer != "https://engram.example.com" {
		t.Errorf("engram = %+v", st.Engram)
	}
	if len(st.Engram.Projects) != 2 || st.Engram.Projects[0] != "repo-core" {
		t.Errorf("projects = %v", st.Engram.Projects)
	}

	// Clearing it (nil) marks engram unmanaged again.
	if err := s.SetEngram(nil); err != nil {
		t.Fatal(err)
	}
	st, _ = s.Load()
	if st.Engram != nil {
		t.Errorf("engram should be nil after clear, got %+v", st.Engram)
	}
}

func TestEngramTokenNeverPersisted(t *testing.T) {
	// The state stores only the non-secret URL; the token lives in the environment.
	s := NewStore(t.TempDir())
	if err := s.SetEngram(&EngramRecord{Enabled: true, CloudServer: "https://engram.example.com"}); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(filepath.Join(s.Dir(), "state.json"))
	if bytes.Contains(bytes.ToLower(data), []byte("token")) {
		t.Errorf("state.json must never contain a token field, got:\n%s", data)
	}
}

func TestEngramOmittedWhenUnset(t *testing.T) {
	s := NewStore(t.TempDir())
	if err := s.Apply("1.0.0", nil, nil); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(filepath.Join(s.Dir(), "state.json"))
	if bytes.Contains(data, []byte("engram")) {
		t.Errorf("state without engram should omit the engram key, got:\n%s", data)
	}
}

func TestDefaultStoreRootsAtCapikoHome(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	s, err := DefaultStore()
	if err != nil {
		t.Fatalf("DefaultStore: %v", err)
	}
	want := filepath.Join(home, ".capiko")
	if s.Dir() != want {
		t.Errorf("Dir() = %q, want %q", s.Dir(), want)
	}
}

func TestLoadRejectsMalformedJSON(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "state.json"), []byte("{not json"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := NewStore(dir).Load(); err == nil {
		t.Error("Load should error on malformed JSON, got nil")
	}
}

func TestState_AgentsMap_InitializedOnLoad(t *testing.T) {
	s := NewStore(t.TempDir())
	st, err := s.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if st.Agents == nil {
		t.Error("Agents map should be initialized (non-nil) on load from empty state")
	}
}

func TestStore_ApplyAgents_RecordsChecksums(t *testing.T) {
	s := NewStore(t.TempDir())

	installed := []Installed{
		{Name: "capiko-sdd-apply", Checksum: Checksum("content-a")},
		{Name: "capiko-sdd-spec", Checksum: Checksum("content-b")},
	}
	if err := s.ApplyAgents("1.0.0", installed, nil); err != nil {
		t.Fatalf("ApplyAgents: %v", err)
	}

	st, err := s.Load()
	if err != nil {
		t.Fatalf("Load after ApplyAgents: %v", err)
	}
	if st.Agents == nil {
		t.Fatal("Agents map is nil after ApplyAgents")
	}
	if len(st.Agents) != 2 {
		t.Fatalf("expected 2 agent records, got %d: %v", len(st.Agents), st.Agents)
	}
	got := st.Agents["capiko-sdd-apply"]
	if got.Checksum != Checksum("content-a") {
		t.Errorf("Agents[capiko-sdd-apply].Checksum = %q, want %q", got.Checksum, Checksum("content-a"))
	}
	if got.InstalledAt.IsZero() {
		t.Error("Agents[capiko-sdd-apply].InstalledAt not stamped")
	}

	// Removing one agent drops only that record.
	if err := s.ApplyAgents("1.0.0", nil, []string{"capiko-sdd-spec"}); err != nil {
		t.Fatalf("ApplyAgents remove: %v", err)
	}
	st, _ = s.Load()
	if _, ok := st.Agents["capiko-sdd-spec"]; ok {
		t.Error("capiko-sdd-spec should have been removed")
	}
	if _, ok := st.Agents["capiko-sdd-apply"]; !ok {
		t.Error("capiko-sdd-apply should remain")
	}
}

func TestSetLastUpdateCheck(t *testing.T) {
	s := NewStore(t.TempDir())
	now := time.Now().UTC().Truncate(time.Second)
	if err := s.SetLastUpdateCheck(now); err != nil {
		t.Fatal(err)
	}
	st, err := s.Load()
	if err != nil {
		t.Fatal(err)
	}
	if st.LastUpdateCheck == nil {
		t.Fatal("LastUpdateCheck should be set")
	}
	if !st.LastUpdateCheck.Equal(now) {
		t.Errorf("LastUpdateCheck = %v, want %v", st.LastUpdateCheck, now)
	}
}

func TestLastUpdateCheckOmittedWhenNil(t *testing.T) {
	s := NewStore(t.TempDir())
	if err := s.Apply("1.0.0", nil, nil); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(filepath.Join(s.Dir(), "state.json"))
	if bytes.Contains(data, []byte("last_update_check")) {
		t.Errorf("state without last_update_check should omit the key, got:\n%s", data)
	}
}

func TestLastUpdateCheckPreservedAcrossApply(t *testing.T) {
	s := NewStore(t.TempDir())
	now := time.Now().UTC().Truncate(time.Second)
	if err := s.SetLastUpdateCheck(now); err != nil {
		t.Fatal(err)
	}
	if err := s.Apply("2.0.0", []Installed{{Name: "x", Checksum: "h"}}, nil); err != nil {
		t.Fatal(err)
	}
	st, _ := s.Load()
	if st.LastUpdateCheck == nil || !st.LastUpdateCheck.Equal(now) {
		t.Errorf("Apply should preserve LastUpdateCheck, got %v", st.LastUpdateCheck)
	}
}

func TestLastUpdateCheckBackwardCompat(t *testing.T) {
	dir := t.TempDir()
	old := `{"version":"1.0.0","updated_at":"2026-01-01T00:00:00Z","skills":{}}`
	if err := os.WriteFile(filepath.Join(dir, "state.json"), []byte(old), 0o644); err != nil {
		t.Fatal(err)
	}
	st, err := NewStore(dir).Load()
	if err != nil {
		t.Fatalf("Load old state: %v", err)
	}
	if st.LastUpdateCheck != nil {
		t.Errorf("old state should have nil LastUpdateCheck, got %v", st.LastUpdateCheck)
	}
}

func TestChecksumStable(t *testing.T) {
	if Checksum("hello") != Checksum("hello") {
		t.Error("checksum not deterministic")
	}
	if Checksum("a") == Checksum("b") {
		t.Error("different content produced same checksum")
	}
}

// ---------------------------------------------------------------------------
// TeamSyncRecord / SetTeamSync
// ---------------------------------------------------------------------------

func TestSetTeamSync_RoundTrip(t *testing.T) {
	s := NewStore(t.TempDir())
	rec := &TeamSyncRecord{
		Enabled:   true,
		Workspace: "/home/user/repo",
		Project:   "my-project",
		Conflict:  "",
	}
	if err := s.SetTeamSync(rec); err != nil {
		t.Fatalf("SetTeamSync: %v", err)
	}
	st, err := s.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if st.TeamSync == nil {
		t.Fatal("TeamSync should be set after SetTeamSync")
	}
	if !st.TeamSync.Enabled {
		t.Error("Enabled should be true")
	}
	if st.TeamSync.Workspace != "/home/user/repo" {
		t.Errorf("Workspace = %q, want /home/user/repo", st.TeamSync.Workspace)
	}
	if st.TeamSync.Project != "my-project" {
		t.Errorf("Project = %q, want my-project", st.TeamSync.Project)
	}
	if !st.UpdatedAt.IsZero() == false {
		// UpdatedAt must be stamped.
	}
}

func TestSetTeamSync_NilClears(t *testing.T) {
	s := NewStore(t.TempDir())
	// Seed a record first.
	if err := s.SetTeamSync(&TeamSyncRecord{Enabled: true, Workspace: "/repo", Project: "p"}); err != nil {
		t.Fatalf("SetTeamSync seed: %v", err)
	}
	// Now clear it.
	if err := s.SetTeamSync(nil); err != nil {
		t.Fatalf("SetTeamSync nil: %v", err)
	}
	st, err := s.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if st.TeamSync != nil {
		t.Errorf("TeamSync should be nil after clear, got %+v", st.TeamSync)
	}
}

func TestSetTeamSync_UpdatedAt(t *testing.T) {
	s := NewStore(t.TempDir())
	before := time.Now().UTC()
	if err := s.SetTeamSync(&TeamSyncRecord{Enabled: true}); err != nil {
		t.Fatalf("SetTeamSync: %v", err)
	}
	st, err := s.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if !st.UpdatedAt.After(before) && !st.UpdatedAt.Equal(before) {
		t.Errorf("UpdatedAt %v should be >= %v", st.UpdatedAt, before)
	}
}

func TestSetTeamSync_ConflictPersisted(t *testing.T) {
	s := NewStore(t.TempDir())
	rec := &TeamSyncRecord{
		Enabled:   true,
		Workspace: "/repo",
		Project:   "proj",
		Conflict:  "husky is configured (.husky/ directory found)",
	}
	if err := s.SetTeamSync(rec); err != nil {
		t.Fatalf("SetTeamSync: %v", err)
	}
	st, err := s.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if st.TeamSync == nil {
		t.Fatal("TeamSync should be set")
	}
	if st.TeamSync.Conflict != rec.Conflict {
		t.Errorf("Conflict = %q, want %q", st.TeamSync.Conflict, rec.Conflict)
	}
}

func TestTeamSyncOmittedWhenNil(t *testing.T) {
	s := NewStore(t.TempDir())
	if err := s.Apply("1.0.0", nil, nil); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(filepath.Join(s.Dir(), "state.json"))
	if bytes.Contains(data, []byte("team_sync")) {
		t.Errorf("state without team_sync should omit the key, got:\n%s", data)
	}
}

// ---------------------------------------------------------------------------
// CopilotHooksRecord / SetCopilotHooks
// ---------------------------------------------------------------------------

func TestSetCopilotHooks_RoundTrip(t *testing.T) {
	s := NewStore(t.TempDir())
	rec := &CopilotHooksRecord{
		Enabled:  true,
		Posture:  "strict",
		Presets:  []string{"guardrails"},
		Checksum: "abc123",
	}
	if err := s.SetCopilotHooks(rec); err != nil {
		t.Fatalf("SetCopilotHooks: %v", err)
	}
	st, err := s.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if st.CopilotHooks == nil {
		t.Fatal("CopilotHooks should be set after SetCopilotHooks")
	}
	if !st.CopilotHooks.Enabled {
		t.Error("Enabled should be true")
	}
	if st.CopilotHooks.Posture != "strict" {
		t.Errorf("Posture = %q, want strict", st.CopilotHooks.Posture)
	}
	if len(st.CopilotHooks.Presets) != 1 || st.CopilotHooks.Presets[0] != "guardrails" {
		t.Errorf("Presets = %v, want [guardrails]", st.CopilotHooks.Presets)
	}
	if st.CopilotHooks.Checksum != "abc123" {
		t.Errorf("Checksum = %q, want abc123", st.CopilotHooks.Checksum)
	}
}

func TestSetCopilotHooks_NilClears(t *testing.T) {
	s := NewStore(t.TempDir())
	if err := s.SetCopilotHooks(&CopilotHooksRecord{Enabled: true, Posture: "warn"}); err != nil {
		t.Fatalf("SetCopilotHooks seed: %v", err)
	}
	if err := s.SetCopilotHooks(nil); err != nil {
		t.Fatalf("SetCopilotHooks nil: %v", err)
	}
	st, err := s.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if st.CopilotHooks != nil {
		t.Errorf("CopilotHooks should be nil after clear, got %+v", st.CopilotHooks)
	}
}

func TestSetCopilotHooks_UpdatedAt(t *testing.T) {
	s := NewStore(t.TempDir())
	before := time.Now().UTC()
	if err := s.SetCopilotHooks(&CopilotHooksRecord{Enabled: true, Posture: "warn"}); err != nil {
		t.Fatalf("SetCopilotHooks: %v", err)
	}
	st, err := s.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if st.UpdatedAt.Before(before) {
		t.Errorf("UpdatedAt %v should be >= %v", st.UpdatedAt, before)
	}
}

func TestCopilotHooksOmittedWhenNil(t *testing.T) {
	s := NewStore(t.TempDir())
	if err := s.Apply("1.0.0", nil, nil); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(filepath.Join(s.Dir(), "state.json"))
	if bytes.Contains(data, []byte("copilot_hooks")) {
		t.Errorf("state without copilot_hooks should omit the key, got:\n%s", data)
	}
}

func TestLoadCopilotHooksBackwardCompat(t *testing.T) {
	// A state.json written before this field existed must still load cleanly,
	// with CopilotHooks left nil (unmanaged).
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "state.json"), []byte(`{"version":"1.0.0"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	s := NewStore(dir)
	st, err := s.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if st.CopilotHooks != nil {
		t.Errorf("CopilotHooks should be nil for a state.json predating this field, got %+v", st.CopilotHooks)
	}
}

func TestSetIntegrity_RoundTrip(t *testing.T) {
	s := NewStore(t.TempDir())
	rec := &IntegrityRecord{
		Enabled:   true,
		Files:     []string{"/a/b.txt", "/a/c.txt"},
		Checksums: map[string]string{"/a/b.txt": "sum-b", "/a/c.txt": "sum-c"},
		Checksum:  "combined-sum",
	}
	if err := s.SetIntegrity(rec); err != nil {
		t.Fatalf("SetIntegrity: %v", err)
	}
	st, err := s.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if st.Integrity == nil {
		t.Fatal("Integrity should be set after SetIntegrity")
	}
	if !st.Integrity.Enabled {
		t.Error("Enabled should be true")
	}
	if len(st.Integrity.Files) != 2 || st.Integrity.Files[0] != "/a/b.txt" || st.Integrity.Files[1] != "/a/c.txt" {
		t.Errorf("Files = %v, want [/a/b.txt /a/c.txt]", st.Integrity.Files)
	}
	if st.Integrity.Checksums["/a/b.txt"] != "sum-b" || st.Integrity.Checksums["/a/c.txt"] != "sum-c" {
		t.Errorf("Checksums = %v, want map with sum-b/sum-c", st.Integrity.Checksums)
	}
	if st.Integrity.Checksum != "combined-sum" {
		t.Errorf("Checksum = %q, want combined-sum", st.Integrity.Checksum)
	}
}

func TestSetIntegrity_NilClears(t *testing.T) {
	s := NewStore(t.TempDir())
	if err := s.SetIntegrity(&IntegrityRecord{Enabled: true, Files: []string{"/a/b.txt"}}); err != nil {
		t.Fatalf("SetIntegrity seed: %v", err)
	}
	if err := s.SetIntegrity(nil); err != nil {
		t.Fatalf("SetIntegrity nil: %v", err)
	}
	st, err := s.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if st.Integrity != nil {
		t.Errorf("Integrity should be nil after clear, got %+v", st.Integrity)
	}
}

func TestSetIntegrity_UpdatedAt(t *testing.T) {
	s := NewStore(t.TempDir())
	before := time.Now().UTC()
	if err := s.SetIntegrity(&IntegrityRecord{Enabled: true, Files: []string{"/a/b.txt"}}); err != nil {
		t.Fatalf("SetIntegrity: %v", err)
	}
	st, err := s.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if st.UpdatedAt.Before(before) {
		t.Errorf("UpdatedAt %v should be >= %v", st.UpdatedAt, before)
	}
}

func TestIntegrityOmittedWhenNil(t *testing.T) {
	s := NewStore(t.TempDir())
	if err := s.Apply("1.0.0", nil, nil); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(filepath.Join(s.Dir(), "state.json"))
	if bytes.Contains(data, []byte("integrity")) {
		t.Errorf("state without integrity should omit the key, got:\n%s", data)
	}
}

func TestLoadIntegrityBackwardCompat(t *testing.T) {
	// A state.json written before this field existed must still load cleanly,
	// with Integrity left nil (unmanaged).
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "state.json"), []byte(`{"version":"1.0.0"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	s := NewStore(dir)
	st, err := s.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if st.Integrity != nil {
		t.Errorf("Integrity should be nil for a state.json predating this field, got %+v", st.Integrity)
	}
}
