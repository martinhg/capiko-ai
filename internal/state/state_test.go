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

func TestChecksumStable(t *testing.T) {
	if Checksum("hello") != Checksum("hello") {
		t.Error("checksum not deterministic")
	}
	if Checksum("a") == Checksum("b") {
		t.Error("different content produced same checksum")
	}
}
