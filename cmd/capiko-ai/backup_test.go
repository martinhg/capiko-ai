package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/martinhg/capiko-ai/internal/backup"
	"github.com/martinhg/capiko-ai/internal/copilot"
)

func withStubBackupInputs(t *testing.T, in backupInputs) {
	t.Helper()
	prev := gatherBackupInputs
	gatherBackupInputs = func() (backupInputs, error) { return in, nil }
	t.Cleanup(func() { gatherBackupInputs = prev })
}

func readyBackupInputs(t *testing.T) backupInputs {
	t.Helper()
	return backupInputs{
		host: &copilot.Host{SkillsDir: t.TempDir(), AgentsDir: t.TempDir()},
		bkp:  backup.NewStore(t.TempDir()),
	}
}

func TestBackupCommandNotHandledForOtherName(t *testing.T) {
	var buf bytes.Buffer
	handled, exitCode, err := backupCommand("install", nil, &buf)
	if handled || exitCode != 0 || err != nil {
		t.Fatalf("backup should not handle %q", "install")
	}
}

func TestBackupCommandRequiresSubcommand(t *testing.T) {
	withStubBackupInputs(t, readyBackupInputs(t))
	var buf bytes.Buffer
	handled, exitCode, _ := backupCommand("backup", nil, &buf)
	if !handled || exitCode != 1 {
		t.Fatalf("handled=%v exitCode=%d, want handled=true exitCode=1", handled, exitCode)
	}
}

func TestBackupListTextEmpty(t *testing.T) {
	withStubBackupInputs(t, readyBackupInputs(t))
	var buf bytes.Buffer
	handled, exitCode, err := backupCommand("backup", []string{"list"}, &buf)
	if !handled || exitCode != 0 || err != nil {
		t.Fatalf("handled=%v exitCode=%d err=%v", handled, exitCode, err)
	}
	if !strings.Contains(buf.String(), "No backups") {
		t.Errorf("expected 'No backups' message, got:\n%s", buf.String())
	}
}

func TestBackupListTextWithEntries(t *testing.T) {
	in := readyBackupInputs(t)
	skillDir := in.host.SkillsDir
	if err := os.MkdirAll(filepath.Join(skillDir, "test-skill"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(skillDir, "test-skill", "SKILL.md"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	id, err := in.bkp.Create(skillDir, "install", "1.0.0", []string{"test-skill"})
	if err != nil {
		t.Fatal(err)
	}

	withStubBackupInputs(t, in)
	var buf bytes.Buffer
	handled, exitCode, berr := backupCommand("backup", []string{"list"}, &buf)
	if !handled || exitCode != 0 || berr != nil {
		t.Fatalf("handled=%v exitCode=%d err=%v", handled, exitCode, berr)
	}
	out := buf.String()
	if !strings.Contains(out, id) {
		t.Errorf("expected backup id %q in output:\n%s", id, out)
	}
	if !strings.Contains(out, "install") {
		t.Errorf("expected reason 'install' in output:\n%s", out)
	}
}

func TestBackupListJSON(t *testing.T) {
	in := readyBackupInputs(t)
	skillDir := in.host.SkillsDir
	if err := os.MkdirAll(filepath.Join(skillDir, "s"), 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := in.bkp.Create(skillDir, "sync", "1.0.0", []string{"s"}); err != nil {
		t.Fatal(err)
	}

	withStubBackupInputs(t, in)
	var buf bytes.Buffer
	handled, exitCode, err := backupCommand("backup", []string{"list", "--json"}, &buf)
	if !handled || exitCode != 0 || err != nil {
		t.Fatalf("handled=%v exitCode=%d err=%v", handled, exitCode, err)
	}
	out := strings.TrimSpace(buf.String())
	if !strings.HasPrefix(out, "[") {
		t.Errorf("--json should emit a JSON array, got:\n%s", out)
	}
	var parsed []backup.Manifest
	if err := json.Unmarshal([]byte(out), &parsed); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, out)
	}
	if len(parsed) != 1 {
		t.Errorf("expected 1 backup, got %d", len(parsed))
	}
}

func TestBackupRestoreRequiresID(t *testing.T) {
	withStubBackupInputs(t, readyBackupInputs(t))
	var buf bytes.Buffer
	handled, exitCode, _ := backupCommand("backup", []string{"restore"}, &buf)
	if !handled || exitCode != 1 {
		t.Fatalf("handled=%v exitCode=%d, want handled=true exitCode=1", handled, exitCode)
	}
	if !strings.Contains(buf.String(), "--id") {
		t.Errorf("expected --id hint in error:\n%s", buf.String())
	}
}

func TestBackupRestoreSuccess(t *testing.T) {
	in := readyBackupInputs(t)
	skillDir := in.host.SkillsDir
	skillPath := filepath.Join(skillDir, "test-skill")
	if err := os.MkdirAll(skillPath, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(skillPath, "SKILL.md"), []byte("original"), 0o644); err != nil {
		t.Fatal(err)
	}

	id, err := in.bkp.Create(skillDir, "sync", "1.0.0", []string{"test-skill"})
	if err != nil {
		t.Fatal(err)
	}

	// Mutate the skill.
	if err := os.WriteFile(filepath.Join(skillPath, "SKILL.md"), []byte("mutated"), 0o644); err != nil {
		t.Fatal(err)
	}

	withStubBackupInputs(t, in)
	var buf bytes.Buffer
	handled, exitCode, berr := backupCommand("backup", []string{"restore", "--id", id}, &buf)
	if !handled || exitCode != 0 || berr != nil {
		t.Fatalf("handled=%v exitCode=%d err=%v", handled, exitCode, berr)
	}
	if !strings.Contains(buf.String(), "Restored") {
		t.Errorf("expected 'Restored' in output:\n%s", buf.String())
	}

	// Verify the file was restored.
	data, err := os.ReadFile(filepath.Join(skillPath, "SKILL.md"))
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "original" {
		t.Errorf("skill content = %q, want %q", data, "original")
	}
}

func TestBackupRestoreBadID(t *testing.T) {
	withStubBackupInputs(t, readyBackupInputs(t))
	var buf bytes.Buffer
	handled, exitCode, _ := backupCommand("backup", []string{"restore", "--id", "nonexistent"}, &buf)
	if !handled || exitCode != 1 {
		t.Fatalf("handled=%v exitCode=%d, want exitCode=1", handled, exitCode)
	}
}

func TestBackupUnknownSubcommand(t *testing.T) {
	withStubBackupInputs(t, readyBackupInputs(t))
	var buf bytes.Buffer
	handled, exitCode, _ := backupCommand("backup", []string{"nope"}, &buf)
	if !handled || exitCode != 1 {
		t.Fatalf("handled=%v exitCode=%d, want exitCode=1", handled, exitCode)
	}
	if !strings.Contains(buf.String(), "unknown") {
		t.Errorf("expected 'unknown' in error:\n%s", buf.String())
	}
}

func TestBackupCopilotNotFound(t *testing.T) {
	withStubBackupInputs(t, backupInputs{hostExitCode: 2})
	var buf bytes.Buffer
	handled, exitCode, _ := backupCommand("backup", []string{"list"}, &buf)
	if !handled || exitCode != 2 {
		t.Fatalf("handled=%v exitCode=%d, want exitCode=2", handled, exitCode)
	}
}

func TestGatherBackupInputsIsAFunctionVar(t *testing.T) {
	if gatherBackupInputs == nil {
		t.Fatal("gatherBackupInputs must be set")
	}
}
