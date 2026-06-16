package tui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/martinhg/capiko-ai/internal/copilot"
	"github.com/martinhg/capiko-ai/internal/sdd"
	"github.com/martinhg/capiko-ai/internal/state"
	"github.com/martinhg/capiko-ai/internal/trigger"
)

func TestApplyTriggerRulesWritesBlock(t *testing.T) {
	cfgDir := t.TempDir()
	host := &copilot.Host{ConfigDir: cfgDir}
	store := state.NewStore(t.TempDir())

	if err := applyTriggerRules(host, store, nil, true); err != nil {
		t.Fatalf("applyTriggerRules: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(cfgDir, "copilot-instructions.md"))
	if err != nil {
		t.Fatalf("instructions not written: %v", err)
	}
	content := string(data)
	if !strings.Contains(content, trigger.MarkerStart) {
		t.Error("trigger marker not found in instructions file")
	}
	if !strings.Contains(content, "Trigger Rules") {
		t.Error("trigger rules heading not found")
	}
	if !strings.Contains(content, "review-risk") {
		t.Error("review-risk rule not rendered")
	}

	st, _ := store.Load()
	if !st.TriggerRules {
		t.Error("TriggerRules not recorded in state")
	}
}

func TestApplyTriggerRulesDisabledRemovesBlock(t *testing.T) {
	cfgDir := t.TempDir()
	host := &copilot.Host{ConfigDir: cfgDir}
	store := state.NewStore(t.TempDir())

	// Enable first.
	if err := applyTriggerRules(host, store, nil, true); err != nil {
		t.Fatal(err)
	}

	// Disable.
	if err := applyTriggerRules(host, store, nil, false); err != nil {
		t.Fatal(err)
	}

	data, _ := os.ReadFile(filepath.Join(cfgDir, "copilot-instructions.md"))
	if strings.Contains(string(data), trigger.MarkerStart) {
		t.Error("trigger block should be removed when disabled")
	}

	st, _ := store.Load()
	if st.TriggerRules {
		t.Error("TriggerRules should be false after disabling")
	}
}

func TestSDDApplyAlsoEnablesTriggerRules(t *testing.T) {
	s, svc, path := newSDDT(t, false)
	s.cursor = len(sdd.Phases) // Apply

	_, cmd := s.Update(key("enter"))
	applied, ok := cmd().(sddAppliedMsg)
	if !ok || applied.err != nil {
		t.Fatalf("apply failed: %+v", applied)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), trigger.MarkerStart) {
		t.Error("SDD apply did not inject trigger rules block")
	}

	st, _ := svc.state.Load()
	if !st.TriggerRules {
		t.Error("TriggerRules not enabled after SDD apply")
	}
}

func TestRunSyncReappliesTriggerRules(t *testing.T) {
	cfgDir := t.TempDir()
	host := &copilot.Host{ConfigDir: cfgDir, SkillsDir: filepath.Join(cfgDir, "skills")}
	store := state.NewStore(t.TempDir())
	if err := store.SetTriggerRules(true); err != nil {
		t.Fatal(err)
	}

	if _, err := RunSync(host, testCatalog(), nil, store, nil); err != nil {
		t.Fatalf("RunSync: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(cfgDir, "copilot-instructions.md"))
	if err != nil {
		t.Fatalf("instructions not written by sync: %v", err)
	}
	if !strings.Contains(string(data), trigger.MarkerStart) {
		t.Error("sync did not re-apply trigger rules block")
	}
}

func TestRunSyncSkipsTriggerRulesWhenUnmanaged(t *testing.T) {
	cfgDir := t.TempDir()
	host := &copilot.Host{ConfigDir: cfgDir, SkillsDir: filepath.Join(cfgDir, "skills")}
	store := state.NewStore(t.TempDir())

	if _, err := RunSync(host, testCatalog(), nil, store, nil); err != nil {
		t.Fatalf("RunSync: %v", err)
	}

	path := filepath.Join(cfgDir, "copilot-instructions.md")
	if data, err := os.ReadFile(path); err == nil {
		if strings.Contains(string(data), trigger.MarkerStart) {
			t.Error("sync should not write trigger rules for an unmanaged user")
		}
	}
}
