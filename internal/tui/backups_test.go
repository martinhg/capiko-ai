package tui

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/martinhg/capiko-ai/internal/backup"
	"github.com/martinhg/capiko-ai/internal/copilot"
)

// newStore returns a backup store rooted at a throwaway temp dir.
func newStore(t *testing.T) *backup.Store {
	t.Helper()
	return backup.NewStore(t.TempDir())
}

// newBackupsScreenT builds the backups screen wired to the given store and host.
func newBackupsScreenT(t *testing.T, store *backup.Store, host *copilot.Host) *backupsScreen {
	t.Helper()
	s, ok := newBackups(services{host: host, backup: store}).(*backupsScreen)
	if !ok {
		t.Fatal("newBackups did not return *backupsScreen")
	}
	return s
}

// seedBackups writes n snapshots into the store and returns their ids.
func seedBackups(t *testing.T, store *backup.Store, skillsDir string, n int) []string {
	t.Helper()
	ids := make([]string, 0, n)
	for i := 0; i < n; i++ {
		id, err := store.Create(skillsDir, "sync", "0.1.0", nil)
		if err != nil {
			t.Fatalf("seed backup %d: %v", i, err)
		}
		ids = append(ids, id)
	}
	return ids
}

func TestBackupsQuitGoesBack(t *testing.T) {
	for _, k := range []string{"q", "esc"} {
		t.Run(k, func(t *testing.T) {
			s := newBackupsScreenT(t, newStore(t), &copilot.Host{SkillsDir: t.TempDir()})
			_, cmd := s.Update(key(k))
			if cmd == nil {
				t.Fatalf("%s should emit a command", k)
			}
			if _, ok := cmd().(backMsg); !ok {
				t.Errorf("%s should emit backMsg", k)
			}
		})
	}
}

func TestBackupsNavigationClamps(t *testing.T) {
	store := newStore(t)
	seedBackups(t, store, t.TempDir(), 3)
	s := newBackupsScreenT(t, store, &copilot.Host{SkillsDir: t.TempDir()})
	if len(s.items) != 3 {
		t.Fatalf("items = %d, want 3", len(s.items))
	}

	// up at the top stays pinned at 0
	s.Update(key("up"))
	if s.cursor != 0 {
		t.Errorf("up at top: cursor = %d, want 0", s.cursor)
	}

	// down walks to the last item
	s.Update(key("down"))
	s.Update(key("down"))
	if s.cursor != 2 {
		t.Errorf("after two downs: cursor = %d, want 2", s.cursor)
	}

	// down at the bottom clamps
	s.Update(key("down"))
	if s.cursor != 2 {
		t.Errorf("down at bottom: cursor = %d, want 2", s.cursor)
	}

	// up walks back
	s.Update(key("up"))
	if s.cursor != 1 {
		t.Errorf("after up: cursor = %d, want 1", s.cursor)
	}
}

func TestBackupsDeleteRemovesSelected(t *testing.T) {
	store := newStore(t)
	seedBackups(t, store, t.TempDir(), 2)
	s := newBackupsScreenT(t, store, &copilot.Host{SkillsDir: t.TempDir()})

	target := s.items[s.cursor].ID
	s.Update(key("d"))

	if s.err != nil {
		t.Fatalf("delete error: %v", s.err)
	}
	if s.status != "Deleted "+target {
		t.Errorf("status = %q, want %q", s.status, "Deleted "+target)
	}
	if len(s.items) != 1 {
		t.Fatalf("items after delete = %d, want 1", len(s.items))
	}
	if s.items[0].ID == target {
		t.Errorf("deleted backup %s still listed", target)
	}

	// the reload should also drop it from disk
	left, err := store.List()
	if err != nil {
		t.Fatal(err)
	}
	if len(left) != 1 {
		t.Errorf("store still holds %d backups, want 1", len(left))
	}
}

func TestBackupsRestoreRevertsSkill(t *testing.T) {
	skillsDir := t.TempDir()
	writeSkillFile(t, skillsDir, "capiko-hello")

	store := newStore(t)
	id, err := store.Create(skillsDir, "install", "0.1.0", []string{"capiko-hello"})
	if err != nil {
		t.Fatal(err)
	}

	// simulate a later mutation: the captured skill is removed
	if err := os.RemoveAll(filepath.Join(skillsDir, "capiko-hello")); err != nil {
		t.Fatal(err)
	}

	s := newBackupsScreenT(t, store, &copilot.Host{SkillsDir: skillsDir})
	s.Update(key("r"))

	if s.err != nil {
		t.Fatalf("restore error: %v", s.err)
	}
	if s.status != "Restored "+id {
		t.Errorf("status = %q, want %q", s.status, "Restored "+id)
	}
	if _, err := os.Stat(filepath.Join(skillsDir, "capiko-hello", "SKILL.md")); err != nil {
		t.Errorf("skill was not restored: %v", err)
	}
}

func TestBackupsEmptyActionsAreNoops(t *testing.T) {
	s := newBackupsScreenT(t, newStore(t), &copilot.Host{SkillsDir: t.TempDir()})
	if len(s.items) != 0 {
		t.Fatalf("expected an empty list, got %d items", len(s.items))
	}
	for _, k := range []string{"r", "d"} {
		s.Update(key(k))
		if s.status != "" || s.err != nil {
			t.Errorf("%s on empty list mutated state: status=%q err=%v", k, s.status, s.err)
		}
	}
}

func TestBackupsIgnoresNonKeyMsg(t *testing.T) {
	store := newStore(t)
	seedBackups(t, store, t.TempDir(), 2)
	s := newBackupsScreenT(t, store, &copilot.Host{SkillsDir: t.TempDir()})

	next, cmd := s.Update(struct{}{})
	if next != screen(s) {
		t.Error("non-key msg should return the same screen")
	}
	if cmd != nil {
		t.Error("non-key msg should not emit a command")
	}
	if s.cursor != 0 || s.status != "" {
		t.Errorf("non-key msg mutated state: cursor=%d status=%q", s.cursor, s.status)
	}
}
