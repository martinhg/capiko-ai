package tui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/martinhg/capiko-ai/internal/backup"
	"github.com/martinhg/capiko-ai/internal/copilot"
	"github.com/martinhg/capiko-ai/internal/sdd"
	"github.com/martinhg/capiko-ai/internal/state"
)

func newSDDT(t *testing.T, inFlow bool) (*sddScreen, services, string) {
	t.Helper()
	cfgDir := t.TempDir()
	svc := services{
		host:   &copilot.Host{ConfigDir: cfgDir, SkillsDir: filepath.Join(cfgDir, "skills")},
		state:  state.NewStore(t.TempDir()),
		backup: backup.NewStore(t.TempDir()),
	}
	s := newSDD(svc, testCatalog(), map[string]bool{}, inFlow).(*sddScreen)
	return s, svc, filepath.Join(cfgDir, "copilot-instructions.md")
}

func TestSDDCycleModel(t *testing.T) {
	s, _, _ := newSDDT(t, false)
	// cursor 0 = orchestrator, starts at "default"
	if s.models["orchestrator"] != sdd.DefaultModel {
		t.Fatalf("start = %q, want default", s.models["orchestrator"])
	}
	s.Update(key("right"))
	if s.models["orchestrator"] == sdd.DefaultModel {
		t.Error("right should advance the model off default")
	}
	s.Update(key("left"))
	if s.models["orchestrator"] != sdd.DefaultModel {
		t.Errorf("left should return to default, got %q", s.models["orchestrator"])
	}
}

func TestSDDCustomEntry(t *testing.T) {
	s, _, _ := newSDDT(t, false)
	s.Update(key("c")) // enter custom edit on orchestrator
	if !s.editing {
		t.Fatal("c should start editing")
	}
	for _, r := range "my-model" {
		s.Update(key(string(r)))
	}
	s.Update(key("enter"))
	if s.editing {
		t.Error("enter should confirm editing")
	}
	if s.models["orchestrator"] != "my-model" {
		t.Errorf("custom model = %q, want my-model", s.models["orchestrator"])
	}
}

func TestSDDApplyWritesAndRecords(t *testing.T) {
	s, svc, path := newSDDT(t, false)
	s.models["orchestrator"] = "claude-opus-4.8"
	s.cursor = len(sdd.Phases) // Apply row

	_, cmd := s.Update(key("enter"))
	if s.state != sddApplying {
		t.Fatalf("state = %d, want sddApplying", s.state)
	}
	applied, ok := cmd().(sddAppliedMsg)
	if !ok || applied.err != nil {
		t.Fatalf("apply failed: %+v", applied)
	}

	data, err := os.ReadFile(path)
	if err != nil || !strings.Contains(string(data), "capiko:sdd:start") {
		t.Fatalf("orchestrator block not written: %v / %q", err, data)
	}
	if !strings.Contains(string(data), "claude-opus-4.8") {
		t.Error("assigned model not in the block")
	}

	st, _ := svc.state.Load()
	if st.SDDModels["orchestrator"] != "claude-opus-4.8" {
		t.Errorf("state SDD models = %v", st.SDDModels)
	}

	// applied msg returns to the menu when not in the install flow
	_, cmd2 := s.Update(applied)
	if _, ok := cmd2().(backMsg); !ok {
		t.Error("non-flow apply should return to the menu")
	}
}

func TestSDDInFlowApplyOpensInstall(t *testing.T) {
	s, _, _ := newSDDT(t, true)
	next, _ := s.Update(sddAppliedMsg{})
	if _, ok := next.(*selector); !ok {
		t.Errorf("in-flow apply should open the selector, got %T", next)
	}
}

func TestSDDStrictToggleAppliesAndPersists(t *testing.T) {
	s, svc, path := newSDDT(t, false)
	if s.strict {
		t.Fatal("strict TDD should default off")
	}
	s.Update(key("t"))
	if !s.strict {
		t.Fatal("t should toggle strict TDD on")
	}

	s.cursor = len(sdd.Phases) // Apply
	_, cmd := s.Update(key("enter"))
	applied, ok := cmd().(sddAppliedMsg)
	if !ok || applied.err != nil {
		t.Fatalf("apply failed: %+v", applied)
	}

	st, _ := svc.state.Load()
	if !st.StrictTDD {
		t.Error("strict TDD not persisted in state")
	}
	data, _ := os.ReadFile(path)
	if !strings.Contains(string(data), "Strict TDD") {
		t.Errorf("strict TDD section not in the orchestrator block: %q", data)
	}
}

func TestSDDCycleEffort(t *testing.T) {
	s, _, _ := newSDDT(t, false)
	// orchestrator defaults to "high"
	if s.efforts["orchestrator"] != "high" {
		t.Fatalf("start effort = %q, want high", s.efforts["orchestrator"])
	}
	s.Update(key("e"))
	if s.efforts["orchestrator"] != "low" {
		t.Errorf("e should cycle effort to low, got %q", s.efforts["orchestrator"])
	}
	s.Update(key("e"))
	if s.efforts["orchestrator"] != "medium" {
		t.Errorf("e should cycle effort to medium, got %q", s.efforts["orchestrator"])
	}
	s.Update(key("e"))
	if s.efforts["orchestrator"] != "high" {
		t.Errorf("e should cycle effort back to high, got %q", s.efforts["orchestrator"])
	}
}

func TestSDDEffortPersistsOnApply(t *testing.T) {
	s, svc, _ := newSDDT(t, false)
	s.efforts["orchestrator"] = "low"
	s.cursor = len(sdd.Phases) // Apply
	_, cmd := s.Update(key("enter"))
	applied, ok := cmd().(sddAppliedMsg)
	if !ok || applied.err != nil {
		t.Fatalf("apply failed: %+v", applied)
	}
	st, _ := svc.state.Load()
	if st.SDDEfforts["orchestrator"] != "low" {
		t.Errorf("state SDD efforts = %v, want orchestrator=low", st.SDDEfforts)
	}
}

func TestSDDBackGoesToMenu(t *testing.T) {
	s, _, _ := newSDDT(t, false)
	_, cmd := s.Update(key("esc"))
	if _, ok := cmd().(backMsg); !ok {
		t.Error("esc should emit backMsg")
	}
}
