package state

import (
	"os"
	"path/filepath"
	"testing"
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

func TestChecksumStable(t *testing.T) {
	if Checksum("hello") != Checksum("hello") {
		t.Error("checksum not deterministic")
	}
	if Checksum("a") == Checksum("b") {
		t.Error("different content produced same checksum")
	}
}
