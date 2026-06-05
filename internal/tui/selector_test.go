package tui

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/martinhg/capiko-ai/internal/backup"
	"github.com/martinhg/capiko-ai/internal/copilot"
	"github.com/martinhg/capiko-ai/internal/state"
)

func installScreen(t *testing.T, skillsDir string, installed ...string) *selector {
	t.Helper()
	inst := map[string]bool{}
	for _, n := range installed {
		inst[n] = true
	}
	s, ok := newInstall(services{host: &copilot.Host{SkillsDir: skillsDir}}, testCatalog(), inst).(*selector)
	if !ok {
		t.Fatal("newInstall did not return *selector")
	}
	return s
}

func TestInstallSeedsFromDisk(t *testing.T) {
	s := installScreen(t, t.TempDir(), "capiko-hello")
	if !s.desired[0] {
		t.Error("installed skill should be seeded as desired")
	}
	if s.hasChanges() {
		t.Error("fresh install screen should have no changes")
	}
}

func TestInstallAppliesNewlyMarked(t *testing.T) {
	dir := t.TempDir()
	s := installScreen(t, dir) // nothing installed

	s.Update(key("space")) // mark capiko-hello (cursor 0)

	_, cmd := s.Update(key("enter"))
	if s.state != selApplying {
		t.Fatalf("state = %d, want selApplying", s.state)
	}
	rm, ok := cmd().(reconciledMsg)
	if !ok || rm.err != nil {
		t.Fatalf("reconcile failed: %+v", rm)
	}
	if len(rm.result.installed) != 1 || rm.result.installed[0] != "capiko-hello" {
		t.Errorf("installed = %v, want [capiko-hello]", rm.result.installed)
	}
	if _, err := os.Stat(filepath.Join(dir, "capiko-hello", "SKILL.md")); err != nil {
		t.Errorf("file not written: %v", err)
	}

	s.Update(rm)
	if s.state != selDone {
		t.Errorf("state = %d, want selDone", s.state)
	}
}

func TestInstallRecordsState(t *testing.T) {
	dir := t.TempDir()
	store := state.NewStore(t.TempDir())

	s, _ := newInstall(services{host: &copilot.Host{SkillsDir: dir}, state: store}, testCatalog(), map[string]bool{}).(*selector)
	s.Update(key("space")) // mark capiko-hello (cursor 0)

	_, cmd := s.Update(key("enter"))
	rm, ok := cmd().(reconciledMsg)
	if !ok || rm.err != nil {
		t.Fatalf("reconcile failed: %+v", rm)
	}

	st, err := store.Load()
	if err != nil {
		t.Fatalf("load state: %v", err)
	}
	rec, ok := st.Skills["capiko-hello"]
	if !ok {
		t.Fatalf("state did not record capiko-hello: %+v", st.Skills)
	}
	if rec.Checksum == "" {
		t.Error("recorded skill has no checksum")
	}
}

func TestInstallCreatesBackupBeforeMutating(t *testing.T) {
	dir := t.TempDir()
	bkp := backup.NewStore(t.TempDir())

	s, _ := newInstall(services{host: &copilot.Host{SkillsDir: dir}, backup: bkp}, testCatalog(), map[string]bool{}).(*selector)
	s.Update(key("space")) // mark capiko-hello

	_, cmd := s.Update(key("enter"))
	if rm := cmd().(reconciledMsg); rm.err != nil {
		t.Fatalf("reconcile failed: %v", rm.err)
	}

	list, err := bkp.List()
	if err != nil {
		t.Fatalf("list backups: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("expected 1 backup created, got %d", len(list))
	}
	if list[0].Reason != "reconcile" {
		t.Errorf("backup reason = %q, want reconcile", list[0].Reason)
	}
}

func TestUninstallRemovesUnmarked(t *testing.T) {
	dir := t.TempDir()
	writeSkillFile(t, dir, "capiko-hello")

	s, ok := newUninstall(services{host: &copilot.Host{SkillsDir: dir}}, testCatalog(), map[string]bool{"capiko-hello": true}).(*selector)
	if !ok {
		t.Fatal("newUninstall did not return *selector")
	}
	if len(s.items) != 1 {
		t.Fatalf("uninstall should list only installed skills, got %d", len(s.items))
	}

	s.Update(key("space")) // unmark capiko-hello

	_, cmd := s.Update(key("enter"))
	rm, ok := cmd().(reconciledMsg)
	if !ok || rm.err != nil {
		t.Fatalf("reconcile failed: %+v", rm)
	}
	if len(rm.result.removed) != 1 || rm.result.removed[0] != "capiko-hello" {
		t.Errorf("removed = %v, want [capiko-hello]", rm.result.removed)
	}
	if _, err := os.Stat(filepath.Join(dir, "capiko-hello")); !os.IsNotExist(err) {
		t.Errorf("skill not removed: %v", err)
	}
}

func TestUninstallEmptyGoesBack(t *testing.T) {
	s, _ := newUninstall(services{host: &copilot.Host{SkillsDir: t.TempDir()}}, testCatalog(), map[string]bool{}).(*selector)
	if len(s.items) != 0 {
		t.Fatalf("expected no installed skills, got %d", len(s.items))
	}

	_, cmd := s.Update(key("q"))
	if _, ok := cmd().(backMsg); !ok {
		t.Error("q on empty uninstall should go back")
	}
}

func TestSelectorQuitGoesBack(t *testing.T) {
	s := installScreen(t, t.TempDir())
	_, cmd := s.Update(key("q"))
	if _, ok := cmd().(backMsg); !ok {
		t.Error("q should emit backMsg")
	}
}
