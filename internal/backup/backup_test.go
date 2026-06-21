package backup

import (
	"os"
	"path/filepath"
	"testing"
)

func writeSkill(t *testing.T, skillsDir, name, content string) {
	t.Helper()
	dir := filepath.Join(skillsDir, name)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func readSkill(t *testing.T, skillsDir, name string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(skillsDir, name, "SKILL.md"))
	if err != nil {
		t.Fatalf("read %s: %v", name, err)
	}
	return string(data)
}

// A standalone file is snapshotted and restored to its original path.
func TestCreateFilesAndRestore(t *testing.T) {
	store := NewStore(t.TempDir())
	target := filepath.Join(t.TempDir(), "copilot-instructions.md")
	if err := os.WriteFile(target, []byte("v1"), 0o644); err != nil {
		t.Fatal(err)
	}

	id, err := store.CreateFiles("persona", "1.0.0", []string{target})
	if err != nil {
		t.Fatalf("CreateFiles: %v", err)
	}

	mans, _ := store.List()
	if len(mans) != 1 || len(mans[0].Files) != 1 || mans[0].Reason != "persona" {
		t.Fatalf("manifest = %+v, want one persona file backup", mans)
	}

	if err := os.WriteFile(target, []byte("v2"), 0o644); err != nil { // mutate
		t.Fatal(err)
	}
	if err := store.Restore(t.TempDir(), id); err != nil {
		t.Fatalf("Restore: %v", err)
	}
	if data, _ := os.ReadFile(target); string(data) != "v1" {
		t.Errorf("content = %q, want v1 after restore", data)
	}
}

// Restoring a files backup taken before the file existed removes it.
func TestRestoreRemovesNewlyCreatedFile(t *testing.T) {
	store := NewStore(t.TempDir())
	target := filepath.Join(t.TempDir(), "copilot-instructions.md") // does not exist yet

	id, err := store.CreateFiles("persona", "1.0.0", []string{target})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(target, []byte("created later"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := store.Restore(t.TempDir(), id); err != nil {
		t.Fatalf("Restore: %v", err)
	}
	if _, err := os.Stat(target); !os.IsNotExist(err) {
		t.Errorf("file should be removed on restore, stat err = %v", err)
	}
}

// Restoring an overwritten skill brings back the original content.
func TestRestoreRecoversOverwrittenSkill(t *testing.T) {
	skillsDir := t.TempDir()
	store := NewStore(t.TempDir())
	writeSkill(t, skillsDir, "capiko-hello", "v1")

	id, err := store.Create(skillsDir, "sync", "0.1.0", []string{"capiko-hello"})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	writeSkill(t, skillsDir, "capiko-hello", "v2") // mutate

	if err := store.Restore(skillsDir, id); err != nil {
		t.Fatalf("Restore: %v", err)
	}
	if got := readSkill(t, skillsDir, "capiko-hello"); got != "v1" {
		t.Errorf("content = %q, want v1 after restore", got)
	}
}

// Restoring an install (skill did not exist before) removes it.
func TestRestoreRemovesNewlyInstalledSkill(t *testing.T) {
	skillsDir := t.TempDir()
	store := NewStore(t.TempDir())

	id, err := store.Create(skillsDir, "install", "0.1.0", []string{"capiko-conventions"})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	writeSkill(t, skillsDir, "capiko-conventions", "new") // the mutation installs it

	if err := store.Restore(skillsDir, id); err != nil {
		t.Fatalf("Restore: %v", err)
	}
	if _, err := os.Stat(filepath.Join(skillsDir, "capiko-conventions")); !os.IsNotExist(err) {
		t.Errorf("skill should be gone after restoring a pre-install snapshot: %v", err)
	}
}

func writeAgent(t *testing.T, agentsDir, name, content string) {
	t.Helper()
	if err := os.MkdirAll(agentsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(agentsDir, name+".agent.md"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func readAgent(t *testing.T, agentsDir, name string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(agentsDir, name+".agent.md"))
	if err != nil {
		t.Fatalf("read agent %s: %v", name, err)
	}
	return string(data)
}

// CreateWithAgents captures skills and agent files in a single backup; restoring
// it reinstates an overwritten agent (the uninstall/overwrite case).
func TestCreateWithAgents_RestoresOverwrittenAgent(t *testing.T) {
	skillsDir, agentsDir := t.TempDir(), t.TempDir()
	store := NewStore(t.TempDir())
	writeSkill(t, skillsDir, "capiko-hello", "skill-v1")
	writeAgent(t, agentsDir, "sdd-spec", "agent-v1")

	id, err := store.CreateWithAgents(skillsDir, agentsDir, "uninstall", "1.0.0",
		[]string{"capiko-hello"}, []string{"sdd-spec"})
	if err != nil {
		t.Fatalf("CreateWithAgents: %v", err)
	}

	mans, _ := store.List()
	if len(mans) != 1 || len(mans[0].Entries) != 1 || len(mans[0].Files) != 1 {
		t.Fatalf("manifest = %+v, want one backup with one skill and one agent", mans)
	}

	writeAgent(t, agentsDir, "sdd-spec", "agent-v2") // mutate
	writeSkill(t, skillsDir, "capiko-hello", "skill-v2")

	if err := store.Restore(skillsDir, id); err != nil {
		t.Fatalf("Restore: %v", err)
	}
	if got := readAgent(t, agentsDir, "sdd-spec"); got != "agent-v1" {
		t.Errorf("agent = %q, want agent-v1 after restore", got)
	}
	if got := readSkill(t, skillsDir, "capiko-hello"); got != "skill-v1" {
		t.Errorf("skill = %q, want skill-v1 after restore", got)
	}
}

// Restoring a CreateWithAgents backup taken before the agent existed removes it
// (the install case: install snapshots, then writes the new agent).
func TestCreateWithAgents_RemovesNewlyInstalledAgent(t *testing.T) {
	skillsDir, agentsDir := t.TempDir(), t.TempDir()
	store := NewStore(t.TempDir())

	id, err := store.CreateWithAgents(skillsDir, agentsDir, "install", "1.0.0",
		nil, []string{"sdd-spec"})
	if err != nil {
		t.Fatalf("CreateWithAgents: %v", err)
	}

	writeAgent(t, agentsDir, "sdd-spec", "installed later") // the mutation installs it

	if err := store.Restore(skillsDir, id); err != nil {
		t.Fatalf("Restore: %v", err)
	}
	if _, err := os.Stat(filepath.Join(agentsDir, "sdd-spec.agent.md")); !os.IsNotExist(err) {
		t.Errorf("agent should be gone after restoring a pre-install snapshot: %v", err)
	}
}

func TestListNewestFirstAndMissingDir(t *testing.T) {
	store := NewStore(filepath.Join(t.TempDir(), "nope"))
	if got, err := store.List(); err != nil || len(got) != 0 {
		t.Fatalf("missing dir: got %v err %v, want empty", got, err)
	}

	skillsDir := t.TempDir()
	store = NewStore(t.TempDir())
	writeSkill(t, skillsDir, "a", "x")
	id1, _ := store.Create(skillsDir, "install", "0.1.0", []string{"a"})
	id2, _ := store.Create(skillsDir, "sync", "0.1.0", []string{"a"})

	list, err := store.List()
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 2 {
		t.Fatalf("want 2 backups, got %d", len(list))
	}
	if list[0].ID != id2 || list[1].ID != id1 {
		t.Errorf("not newest-first: %s then %s (created %s, %s)", list[0].ID, list[1].ID, id1, id2)
	}
}

func TestDeleteRemovesBackupButRefusesNonBackup(t *testing.T) {
	skillsDir := t.TempDir()
	store := NewStore(t.TempDir())
	writeSkill(t, skillsDir, "a", "x")
	id, _ := store.Create(skillsDir, "install", "0.1.0", []string{"a"})

	if err := store.Delete(id); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if list, _ := store.List(); len(list) != 0 {
		t.Errorf("backup not deleted: %v", list)
	}

	if err := store.Delete("does-not-exist"); err == nil {
		t.Error("Delete should refuse a non-backup id")
	}
}
