package tui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/martinhg/capiko-ai/internal/copilot"
	"github.com/martinhg/capiko-ai/internal/state"
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

func TestRunSyncWritesCatalogAndRecordsState(t *testing.T) {
	dir := t.TempDir()
	store := state.NewStore(t.TempDir())

	n, err := RunSync(&copilot.Host{SkillsDir: dir}, testCatalog(), store, nil)
	if err != nil {
		t.Fatalf("RunSync: %v", err)
	}
	if n != len(testCatalog()) {
		t.Errorf("count = %d, want %d", n, len(testCatalog()))
	}

	st, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	for _, sk := range testCatalog() {
		if _, err := os.Stat(filepath.Join(dir, sk.Name, "SKILL.md")); err != nil {
			t.Errorf("%s not written: %v", sk.Name, err)
		}
		rec, ok := st.Skills[sk.Name]
		if !ok {
			t.Errorf("%s not recorded in state", sk.Name)
			continue
		}
		if rec.Checksum != state.Checksum(sk.Content) {
			t.Errorf("%s checksum = %q, want content checksum", sk.Name, rec.Checksum)
		}
	}
}

func TestRunSyncReappliesPersona(t *testing.T) {
	cfgDir := t.TempDir()
	host := &copilot.Host{ConfigDir: cfgDir, SkillsDir: filepath.Join(cfgDir, "skills")}
	store := state.NewStore(t.TempDir())
	if err := store.SetPersona("capiko"); err != nil {
		t.Fatal(err)
	}

	if _, err := RunSync(host, testCatalog(), store, nil); err != nil {
		t.Fatalf("RunSync: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(cfgDir, "copilot-instructions.md"))
	if err != nil {
		t.Fatalf("persona instructions not written by sync: %v", err)
	}
	if !strings.Contains(string(data), "capiko:persona:start") {
		t.Errorf("sync did not re-apply the persona block: %q", data)
	}
}

func TestSyncQuitGoesBack(t *testing.T) {
	s, _ := newSync(services{host: &copilot.Host{SkillsDir: t.TempDir()}}, testCatalog()).(*syncScreen)
	_, cmd := s.Update(key("q"))
	if _, ok := cmd().(backMsg); !ok {
		t.Error("q should emit backMsg")
	}
}
