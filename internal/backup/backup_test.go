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
