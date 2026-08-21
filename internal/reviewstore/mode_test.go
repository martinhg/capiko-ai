package reviewstore

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/martinhg/capiko-ai/internal/rdd"
)

func TestSaveAndLoadModeRecord_RoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "review-mode.json")

	want := &rdd.ModeRecord{
		SchemaVersion: 1,
		Mode:          rdd.ModeDisabled,
		UpdatedAt:     "2026-08-21T00:00:00Z",
	}

	if err := SaveModeRecord(path, want); err != nil {
		t.Fatalf("SaveModeRecord() error = %v, want nil", err)
	}

	got, err := LoadModeRecord(path)
	if err != nil {
		t.Fatalf("LoadModeRecord() error = %v, want nil", err)
	}
	if got == nil {
		t.Fatal("LoadModeRecord() returned nil record, want round-tripped record")
	}
	if *got != *want {
		t.Errorf("LoadModeRecord() = %+v, want %+v", *got, *want)
	}
}

func TestSaveModeRecord_CreatesParentDir(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "nested", "scope", "review-mode.json")

	rec := &rdd.ModeRecord{SchemaVersion: 1, Mode: rdd.ModeManaged, UpdatedAt: "2026-08-21T00:00:00Z"}
	if err := SaveModeRecord(path, rec); err != nil {
		t.Fatalf("SaveModeRecord() error = %v, want nil", err)
	}

	if _, err := os.Stat(path); err != nil {
		t.Fatalf("expected mode record at %s, stat error = %v", path, err)
	}
}

func TestSaveModeRecord_AtomicWrite_NoTempFileLeftBehind(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "review-mode.json")

	rec := &rdd.ModeRecord{SchemaVersion: 1, Mode: rdd.ModeManaged, UpdatedAt: "2026-08-21T00:00:00Z"}
	if err := SaveModeRecord(path, rec); err != nil {
		t.Fatalf("SaveModeRecord() error = %v, want nil", err)
	}

	if _, err := os.Stat(path + ".tmp"); !os.IsNotExist(err) {
		t.Errorf("expected tmp file to be renamed away, stat error = %v", err)
	}
}

func TestLoadModeRecord_MissingFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "does-not-exist.json")

	got, err := LoadModeRecord(path)
	if err != nil {
		t.Fatalf("LoadModeRecord() error = %v, want nil for missing file", err)
	}
	if got != nil {
		t.Errorf("LoadModeRecord() = %+v, want nil for missing file", got)
	}
}

func TestLoadModeRecord_CorruptJSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "review-mode.json")

	if err := os.WriteFile(path, []byte("{not valid json"), 0o644); err != nil {
		t.Fatalf("failed to seed corrupt file: %v", err)
	}

	got, err := LoadModeRecord(path)
	if got != nil {
		t.Errorf("LoadModeRecord() record = %+v, want nil on corrupt input", got)
	}
	if !errors.Is(err, ErrCorruptMode) {
		t.Errorf("LoadModeRecord() error = %v, want errors.Is(err, ErrCorruptMode)", err)
	}
}

func TestSaveModeRecord_OverwritesExisting(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "review-mode.json")

	first := &rdd.ModeRecord{SchemaVersion: 1, Mode: rdd.ModeManaged, UpdatedAt: "2026-08-21T00:00:00Z"}
	second := &rdd.ModeRecord{SchemaVersion: 1, Mode: rdd.ModeDisabled, UpdatedAt: "2026-08-21T01:00:00Z"}

	if err := SaveModeRecord(path, first); err != nil {
		t.Fatalf("first SaveModeRecord() error = %v", err)
	}
	if err := SaveModeRecord(path, second); err != nil {
		t.Fatalf("second SaveModeRecord() error = %v", err)
	}

	got, err := LoadModeRecord(path)
	if err != nil {
		t.Fatalf("LoadModeRecord() error = %v", err)
	}
	if *got != *second {
		t.Errorf("LoadModeRecord() = %+v, want %+v (overwritten by second save)", *got, *second)
	}
}
