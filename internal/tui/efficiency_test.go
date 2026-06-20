package tui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/martinhg/capiko-ai/internal/copilot"
	"github.com/martinhg/capiko-ai/internal/efficiency"
	"github.com/martinhg/capiko-ai/internal/persona"
	"github.com/martinhg/capiko-ai/internal/state"
)

func TestApplyOutputEfficiencyWritesBlock(t *testing.T) {
	cfgDir := t.TempDir()
	host := &copilot.Host{ConfigDir: cfgDir}
	store := state.NewStore(t.TempDir())

	if err := applyOutputEfficiency(host, store, nil, true); err != nil {
		t.Fatalf("applyOutputEfficiency: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(cfgDir, "copilot-instructions.md"))
	if err != nil {
		t.Fatalf("instructions not written: %v", err)
	}
	content := string(data)
	if !strings.Contains(content, efficiency.MarkerStart) {
		t.Error("efficiency marker not found in instructions file")
	}
	if !strings.Contains(content, "Output efficiency") {
		t.Error("efficiency heading not found")
	}

	st, _ := store.Load()
	if !st.OutputEfficiency {
		t.Error("OutputEfficiency not recorded in state")
	}
}

func TestApplyOutputEfficiencyDisabledRemovesBlock(t *testing.T) {
	cfgDir := t.TempDir()
	host := &copilot.Host{ConfigDir: cfgDir}
	store := state.NewStore(t.TempDir())

	if err := applyOutputEfficiency(host, store, nil, true); err != nil {
		t.Fatal(err)
	}
	if err := applyOutputEfficiency(host, store, nil, false); err != nil {
		t.Fatal(err)
	}

	data, _ := os.ReadFile(filepath.Join(cfgDir, "copilot-instructions.md"))
	if strings.Contains(string(data), efficiency.MarkerStart) {
		t.Error("efficiency block should be removed when disabled")
	}

	st, _ := store.Load()
	if st.OutputEfficiency {
		t.Error("OutputEfficiency should be false after disabling")
	}
}

func TestPersonaScreenTogglesEfficiency(t *testing.T) {
	s, ok := newPersona(services{host: &copilot.Host{SkillsDir: t.TempDir()}}, testCatalog(), map[string]bool{}).(*personaScreen)
	if !ok {
		t.Fatal("newPersona did not return *personaScreen")
	}
	if s.efficiency {
		t.Fatal("efficiency should default to off")
	}
	if !strings.Contains(s.View(), "[ ] Output efficiency") {
		t.Errorf("persona view should show the efficiency toggle unchecked:\n%s", s.View())
	}

	s.Update(key("e"))
	if !s.efficiency {
		t.Error("'e' should toggle efficiency on")
	}
	if !strings.Contains(s.View(), "[x] Output efficiency") {
		t.Errorf("persona view should show the efficiency toggle checked after 'e':\n%s", s.View())
	}

	s.Update(key("e"))
	if s.efficiency {
		t.Error("'e' again should toggle efficiency back off")
	}
}

func TestRunSyncReappliesOutputEfficiency(t *testing.T) {
	cfgDir := t.TempDir()
	host := &copilot.Host{ConfigDir: cfgDir, SkillsDir: filepath.Join(cfgDir, "skills")}
	store := state.NewStore(t.TempDir())
	if err := store.SetOutputEfficiency(true); err != nil {
		t.Fatal(err)
	}

	if _, err := RunSync(host, testCatalog(), nil, store, nil); err != nil {
		t.Fatalf("RunSync: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(cfgDir, "copilot-instructions.md"))
	if err != nil {
		t.Fatalf("instructions not written by sync: %v", err)
	}
	if !strings.Contains(string(data), efficiency.MarkerStart) {
		t.Errorf("sync did not re-apply the output-efficiency block:\n%s", data)
	}
}

// TestApplyOutputEfficiencyPreservesPersona asserts the efficiency block coexists
// with the persona block — capiko only ever touches its own marker section.
func TestApplyOutputEfficiencyPreservesPersona(t *testing.T) {
	cfgDir := t.TempDir()
	host := &copilot.Host{ConfigDir: cfgDir, SkillsDir: filepath.Join(cfgDir, "skills")}
	store := state.NewStore(t.TempDir())
	p, ok := persona.ByID("capiko")
	if !ok {
		t.Fatal("persona capiko not found")
	}
	if err := applyPersona(host, store, nil, p); err != nil {
		t.Fatal(err)
	}
	if err := applyOutputEfficiency(host, store, nil, true); err != nil {
		t.Fatal(err)
	}

	data, _ := os.ReadFile(filepath.Join(cfgDir, "copilot-instructions.md"))
	content := string(data)
	if !strings.Contains(content, "capiko:persona:start") {
		t.Error("persona block clobbered by efficiency apply")
	}
	if !strings.Contains(content, efficiency.MarkerStart) {
		t.Error("efficiency block missing")
	}
}
