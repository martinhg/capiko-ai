package copilothooks_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/martinhg/capiko-ai/internal/copilothooks"
	"github.com/martinhg/capiko-ai/internal/state"
)

func TestWriteHookFile_CreatesDirAndWritesAtomically(t *testing.T) {
	base := t.TempDir()
	hooksDir := filepath.Join(base, "hooks") // does not exist yet

	data := []byte(`{"version":1}` + "\n")
	if err := copilothooks.WriteHookFile(hooksDir, copilothooks.GuardrailsFile, data); err != nil {
		t.Fatalf("WriteHookFile: %v", err)
	}

	target := filepath.Join(hooksDir, copilothooks.GuardrailsFile)
	info, err := os.Stat(target)
	if err != nil {
		t.Fatalf("Stat target: %v", err)
	}
	if info.Mode().Perm() != 0o644 {
		t.Errorf("mode = %v, want 0o644", info.Mode().Perm())
	}

	got, err := os.ReadFile(target)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if string(got) != string(data) {
		t.Errorf("content = %q, want %q", got, data)
	}

	// No leftover tmp file after a successful atomic write.
	entries, err := os.ReadDir(hooksDir)
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("hooksDir entries = %v, want exactly one file (no leftover tmp)", entries)
	}
}

func TestWriteHookFile_OverwriteReplacesCleanly(t *testing.T) {
	hooksDir := t.TempDir()

	if err := copilothooks.WriteHookFile(hooksDir, copilothooks.GuardrailsFile, []byte("first")); err != nil {
		t.Fatalf("first WriteHookFile: %v", err)
	}
	if err := copilothooks.WriteHookFile(hooksDir, copilothooks.GuardrailsFile, []byte("second")); err != nil {
		t.Fatalf("second WriteHookFile: %v", err)
	}

	got, err := os.ReadFile(filepath.Join(hooksDir, copilothooks.GuardrailsFile))
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if string(got) != "second" {
		t.Errorf("content = %q, want %q (overwrite)", got, "second")
	}
}

func TestWriteHookFile_PathGuardRejectsEscape(t *testing.T) {
	hooksDir := t.TempDir()

	cases := []string{
		"../escape.json",
		"sub/escape.json",
		"..",
	}
	for _, name := range cases {
		if err := copilothooks.WriteHookFile(hooksDir, name, []byte("x")); err == nil {
			t.Errorf("WriteHookFile(%q): want error (path guard), got nil", name)
		}
	}

	// Confirm nothing escaped hooksDir's parent.
	parent := filepath.Dir(hooksDir)
	entries, err := os.ReadDir(parent)
	if err != nil {
		t.Fatalf("ReadDir(parent): %v", err)
	}
	for _, e := range entries {
		if e.Name() == "escape.json" {
			t.Fatalf("path guard escaped: found %q in parent dir", e.Name())
		}
	}
}

func TestRemoveHookFile_IdempotentOnMissing(t *testing.T) {
	hooksDir := t.TempDir()

	if err := copilothooks.RemoveHookFile(hooksDir, copilothooks.GuardrailsFile); err != nil {
		t.Errorf("RemoveHookFile on missing file: want nil, got %v", err)
	}
}

func TestRemoveHookFile_RemovesExisting(t *testing.T) {
	hooksDir := t.TempDir()
	if err := copilothooks.WriteHookFile(hooksDir, copilothooks.GuardrailsFile, []byte("x")); err != nil {
		t.Fatalf("WriteHookFile: %v", err)
	}

	if err := copilothooks.RemoveHookFile(hooksDir, copilothooks.GuardrailsFile); err != nil {
		t.Fatalf("RemoveHookFile: %v", err)
	}

	if _, err := os.Stat(filepath.Join(hooksDir, copilothooks.GuardrailsFile)); !os.IsNotExist(err) {
		t.Errorf("want file removed, Stat err = %v", err)
	}
}

func TestRemoveHookFile_PathGuardRejectsEscape(t *testing.T) {
	hooksDir := t.TempDir()
	if err := copilothooks.RemoveHookFile(hooksDir, "../escape.json"); err == nil {
		t.Error("RemoveHookFile(\"../escape.json\"): want error (path guard), got nil")
	}
}

func TestHookFileChecksum_AbsentIsEmpty(t *testing.T) {
	if got := copilothooks.HookFileChecksum(filepath.Join(t.TempDir(), "nope.json")); got != "" {
		t.Errorf("HookFileChecksum(absent) = %q, want empty", got)
	}
}

func TestHookFileChecksum_MatchesStateChecksum(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "capiko-guardrails.json")
	content := []byte(`{"version":1}` + "\n")
	if err := os.WriteFile(path, content, 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	got := copilothooks.HookFileChecksum(path)
	want := state.Checksum(string(content))
	if got != want {
		t.Errorf("HookFileChecksum = %q, want %q", got, want)
	}
}

func TestHookFileChecksum_ChangesOnByteChange(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "capiko-guardrails.json")

	if err := os.WriteFile(path, []byte("a"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	first := copilothooks.HookFileChecksum(path)

	if err := os.WriteFile(path, []byte("b"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	second := copilothooks.HookFileChecksum(path)

	if first == second {
		t.Error("HookFileChecksum: want different checksum after byte change, got same")
	}
}

func TestCombinedChecksum_AbsentDirIsEmpty(t *testing.T) {
	if got := copilothooks.CombinedChecksum(filepath.Join(t.TempDir(), "missing")); got != "" {
		t.Errorf("CombinedChecksum(missing dir) = %q, want empty", got)
	}
}

func TestCombinedChecksum_NoCapikoFilesIsEmpty(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "other.json"), []byte("x"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	if got := copilothooks.CombinedChecksum(dir); got != "" {
		t.Errorf("CombinedChecksum(no capiko-*.json) = %q, want empty", got)
	}
}

func TestCombinedChecksum_SingleFile(t *testing.T) {
	dir := t.TempDir()
	name := "capiko-guardrails.json"
	content := []byte(`{"version":1}`)
	if err := os.WriteFile(filepath.Join(dir, name), content, 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	got := copilothooks.CombinedChecksum(dir)
	want := state.Checksum(name + "\n" + string(content))
	if got != want {
		t.Errorf("CombinedChecksum = %q, want %q", got, want)
	}
}

func TestCombinedChecksum_IgnoresNonCapikoFiles(t *testing.T) {
	dir := t.TempDir()
	name := "capiko-guardrails.json"
	content := []byte(`{"version":1}`)
	if err := os.WriteFile(filepath.Join(dir, name), content, 0o644); err != nil {
		t.Fatalf("WriteFile capiko file: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "other.json"), []byte("ignored"), 0o644); err != nil {
		t.Fatalf("WriteFile other file: %v", err)
	}

	got := copilothooks.CombinedChecksum(dir)
	want := state.Checksum(name + "\n" + string(content))
	if got != want {
		t.Errorf("CombinedChecksum = %q, want %q (non-capiko file must not contribute)", got, want)
	}
}

func TestCombinedChecksum_OrderIndependentAcrossCreationOrder(t *testing.T) {
	dirA := t.TempDir()
	dirB := t.TempDir()

	// dirA: create "capiko-b" then "capiko-a".
	if err := os.WriteFile(filepath.Join(dirA, "capiko-b.json"), []byte("B"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dirA, "capiko-a.json"), []byte("A"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	// dirB: create "capiko-a" then "capiko-b" (reverse order).
	if err := os.WriteFile(filepath.Join(dirB, "capiko-a.json"), []byte("A"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dirB, "capiko-b.json"), []byte("B"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	got := copilothooks.CombinedChecksum(dirA)
	want := copilothooks.CombinedChecksum(dirB)
	if got != want {
		t.Errorf("CombinedChecksum order-independence: dirA = %q, dirB = %q, want equal", got, want)
	}
}

func TestCombinedChecksum_ChangesOnByteChange(t *testing.T) {
	dir := t.TempDir()
	name := filepath.Join(dir, "capiko-guardrails.json")

	if err := os.WriteFile(name, []byte("first"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	first := copilothooks.CombinedChecksum(dir)

	if err := os.WriteFile(name, []byte("second"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	second := copilothooks.CombinedChecksum(dir)

	if first == second {
		t.Error("CombinedChecksum: want different checksum after byte change, got same")
	}
}
