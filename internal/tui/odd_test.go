package tui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/martinhg/capiko-ai/internal/backup"
	"github.com/martinhg/capiko-ai/internal/copilot"
	"github.com/martinhg/capiko-ai/internal/state"
)

func TestApplyODD_ModularWritesModulesAndCleansOnMonolithic(t *testing.T) {
	host := &copilot.Host{SkillsDir: t.TempDir(), AgentsDir: t.TempDir(), ConfigDir: t.TempDir()}
	store := state.NewStore(t.TempDir())
	bkp := backup.NewStore(t.TempDir())
	modDir := filepath.Join(host.ConfigDir, "instructions")

	if err := ApplyODD(host, store, bkp, false, true); err != nil {
		t.Fatalf("modular apply: %v", err)
	}
	for _, name := range []string{"capiko-delegation", "capiko-odd-detail"} {
		if _, err := os.Stat(filepath.Join(modDir, moduleFileName(name))); err != nil {
			t.Errorf("module %s not written: %v", name, err)
		}
	}
	data, _ := os.ReadFile(filepath.Join(host.ConfigDir, "copilot-instructions.md"))
	if strings.Contains(string(data), "### Mandatory Delegation Triggers") {
		t.Error("core block should not contain delegation triggers")
	}
	if st, _ := store.Load(); !st.ModularODD {
		t.Error("ModularODD should be true")
	}

	if err := ApplyODD(host, store, bkp, false, false); err != nil {
		t.Fatalf("monolithic apply: %v", err)
	}
	if _, err := os.Stat(filepath.Join(modDir, moduleFileName("capiko-delegation"))); !os.IsNotExist(err) {
		t.Errorf("module should be removed, stat err = %v", err)
	}
	data, _ = os.ReadFile(filepath.Join(host.ConfigDir, "copilot-instructions.md"))
	if !strings.Contains(string(data), "### Mandatory Delegation Triggers") {
		t.Error("monolithic block should contain delegation triggers")
	}
	if st, _ := store.Load(); st.ModularODD {
		t.Error("ModularODD should be false")
	}
}
