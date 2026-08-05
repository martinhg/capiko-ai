package integrity_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/martinhg/capiko-ai/internal/integrity"
	"github.com/martinhg/capiko-ai/internal/state"
)

func writeFile(t *testing.T, dir, name, content string) string {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile(%s): %v", name, err)
	}
	return p
}

func TestManifest_Deterministic(t *testing.T) {
	dir := t.TempDir()
	a := writeFile(t, dir, "a.txt", "hello")
	b := writeFile(t, dir, "b.txt", "world")

	checksums1, combined1 := integrity.Manifest([]string{a, b})
	checksums2, combined2 := integrity.Manifest([]string{a, b})

	if combined1 != combined2 {
		t.Errorf("combined checksum not deterministic: got %q then %q", combined1, combined2)
	}
	if checksums1[a] != checksums2[a] || checksums1[b] != checksums2[b] {
		t.Errorf("per-file checksums not deterministic: got %v then %v", checksums1, checksums2)
	}
	if checksums1[a] == "" || checksums1[b] == "" {
		t.Errorf("expected non-empty checksums, got %v", checksums1)
	}
}

func TestManifest_ByteSensitive(t *testing.T) {
	dir := t.TempDir()
	a := writeFile(t, dir, "a.txt", "hello")

	_, combinedBefore := integrity.Manifest([]string{a})
	checksumsBefore, _ := integrity.Manifest([]string{a})

	writeFile(t, dir, "a.txt", "hellp") // change last byte

	checksumsAfter, combinedAfter := integrity.Manifest([]string{a})

	if checksumsBefore[a] == checksumsAfter[a] {
		t.Errorf("per-file checksum should change when content changes, both = %q", checksumsBefore[a])
	}
	if combinedBefore == combinedAfter {
		t.Errorf("combined checksum should change when content changes, both = %q", combinedBefore)
	}
}

func TestManifest_AbsentFile(t *testing.T) {
	dir := t.TempDir()
	missing := filepath.Join(dir, "does-not-exist.txt")

	checksums, combined := integrity.Manifest([]string{missing})

	if got, want := checksums[missing], ""; got != want {
		t.Errorf("checksums[missing] = %q, want %q", got, want)
	}
	if combined == "" {
		t.Error("combined checksum should still be computed for an absent file")
	}
}

func TestManifest_OrderIndependent(t *testing.T) {
	dir := t.TempDir()
	a := writeFile(t, dir, "a.txt", "hello")
	b := writeFile(t, dir, "b.txt", "world")

	_, combinedAB := integrity.Manifest([]string{a, b})
	_, combinedBA := integrity.Manifest([]string{b, a})

	if combinedAB != combinedBA {
		t.Errorf("combined checksum should be order independent: %q vs %q", combinedAB, combinedBA)
	}
}

func TestCombinedChecksum_MatchesManifest(t *testing.T) {
	dir := t.TempDir()
	a := writeFile(t, dir, "a.txt", "hello")
	b := writeFile(t, dir, "b.txt", "world")

	_, combined := integrity.Manifest([]string{a, b})
	got := integrity.CombinedChecksum([]string{a, b})

	if got != combined {
		t.Errorf("CombinedChecksum = %q, want %q (Manifest's combined)", got, combined)
	}
}

func TestDrifted_NilRecord(t *testing.T) {
	if got := integrity.Drifted(nil); got != nil {
		t.Errorf("Drifted(nil) = %v, want nil", got)
	}
}

func TestDrifted_Disabled(t *testing.T) {
	rec := &state.IntegrityRecord{Enabled: false, Files: []string{"anything"}}
	if got := integrity.Drifted(rec); got != nil {
		t.Errorf("Drifted(disabled) = %v, want nil", got)
	}
}

func TestDrifted_NoFiles(t *testing.T) {
	rec := &state.IntegrityRecord{Enabled: true, Files: nil}
	if got := integrity.Drifted(rec); got != nil {
		t.Errorf("Drifted(no files) = %v, want nil", got)
	}
}

func TestDrifted_NoDrift(t *testing.T) {
	dir := t.TempDir()
	a := writeFile(t, dir, "a.txt", "hello")
	b := writeFile(t, dir, "b.txt", "world")

	checksums, combined := integrity.Manifest([]string{a, b})
	rec := &state.IntegrityRecord{
		Enabled:   true,
		Files:     []string{a, b},
		Checksums: checksums,
		Checksum:  combined,
	}

	if got := integrity.Drifted(rec); got != nil {
		t.Errorf("Drifted(no drift) = %v, want nil", got)
	}
}

func TestDrifted_FileChanged(t *testing.T) {
	dir := t.TempDir()
	a := writeFile(t, dir, "a.txt", "hello")
	b := writeFile(t, dir, "b.txt", "world")

	checksums, combined := integrity.Manifest([]string{a, b})
	rec := &state.IntegrityRecord{
		Enabled:   true,
		Files:     []string{a, b},
		Checksums: checksums,
		Checksum:  combined,
	}

	writeFile(t, dir, "a.txt", "changed")

	got := integrity.Drifted(rec)
	if len(got) != 1 || got[0] != a {
		t.Errorf("Drifted(file changed) = %v, want [%s]", got, a)
	}
}

func TestDrifted_FileDeleted(t *testing.T) {
	dir := t.TempDir()
	a := writeFile(t, dir, "a.txt", "hello")
	b := writeFile(t, dir, "b.txt", "world")

	checksums, combined := integrity.Manifest([]string{a, b})
	rec := &state.IntegrityRecord{
		Enabled:   true,
		Files:     []string{a, b},
		Checksums: checksums,
		Checksum:  combined,
	}

	if err := os.Remove(a); err != nil {
		t.Fatalf("Remove: %v", err)
	}

	got := integrity.Drifted(rec)
	if len(got) != 1 || got[0] != a {
		t.Errorf("Drifted(file deleted) = %v, want [%s]", got, a)
	}
}

func TestDrifted_FileCreated(t *testing.T) {
	dir := t.TempDir()
	a := filepath.Join(dir, "a.txt") // not created yet
	b := writeFile(t, dir, "b.txt", "world")

	checksums, combined := integrity.Manifest([]string{a, b})
	rec := &state.IntegrityRecord{
		Enabled:   true,
		Files:     []string{a, b},
		Checksums: checksums,
		Checksum:  combined,
	}

	writeFile(t, dir, "a.txt", "now exists")

	got := integrity.Drifted(rec)
	if len(got) != 1 || got[0] != a {
		t.Errorf("Drifted(file created) = %v, want [%s]", got, a)
	}
}
