package tui

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/martinhg/capiko-ai/internal/copilot"
)

func TestSyncWritesWholeCatalog(t *testing.T) {
	dir := t.TempDir()
	s, ok := newSync(services{host: &copilot.Host{SkillsDir: dir}}, testCatalog()).(*syncScreen)
	if !ok {
		t.Fatal("newSync did not return *syncScreen")
	}

	_, cmd := s.Update(key("y"))
	if s.state != syncApplying {
		t.Fatalf("state = %d, want syncApplying", s.state)
	}

	sm, ok := cmd().(syncedMsg)
	if !ok || sm.err != nil {
		t.Fatalf("sync failed: %+v", sm)
	}
	if sm.count != len(testCatalog()) {
		t.Errorf("count = %d, want %d", sm.count, len(testCatalog()))
	}
	for _, sk := range testCatalog() {
		if _, err := os.Stat(filepath.Join(dir, sk.Name, "SKILL.md")); err != nil {
			t.Errorf("%s not written: %v", sk.Name, err)
		}
	}

	s.Update(sm)
	if s.state != syncDone {
		t.Errorf("state = %d, want syncDone", s.state)
	}
}

func TestSyncQuitGoesBack(t *testing.T) {
	s, _ := newSync(services{host: &copilot.Host{SkillsDir: t.TempDir()}}, testCatalog()).(*syncScreen)
	_, cmd := s.Update(key("q"))
	if _, ok := cmd().(backMsg); !ok {
		t.Error("q should emit backMsg")
	}
}
