package tui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/martinhg/capiko-ai/internal/backup"
	"github.com/martinhg/capiko-ai/internal/copilot"
	"github.com/martinhg/capiko-ai/internal/odd"
	"github.com/martinhg/capiko-ai/internal/sdd"
	"github.com/martinhg/capiko-ai/internal/state"
)

func TestSwitchToSDD_WritesSDDBlock(t *testing.T) {
	host := &copilot.Host{
		SkillsDir: t.TempDir(),
		AgentsDir: t.TempDir(),
		ConfigDir: t.TempDir(),
	}
	store := state.NewStore(t.TempDir())
	bkp := backup.NewStore(t.TempDir())
	svc := services{host: host, state: store, backup: bkp}

	// Seed an ODD block first.
	if err := applyODD(host, store, bkp, false); err != nil {
		t.Fatalf("seed ODD: %v", err)
	}

	if err := switchToSDD(svc); err != nil {
		t.Fatalf("switchToSDD: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(host.ConfigDir, "copilot-instructions.md"))
	if err != nil {
		t.Fatalf("read instructions: %v", err)
	}
	content := string(data)
	if !strings.Contains(content, sdd.MarkerStart) {
		t.Error("SDD marker not found after switching to SDD")
	}
	if strings.Contains(content, odd.MarkerStart) {
		t.Error("ODD marker should be removed after switching to SDD")
	}

	st, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	if !st.IsSDDWorkflow() {
		t.Error("workflow should be sdd")
	}
}

func TestSwitchToODD_WritesODDBlock(t *testing.T) {
	host := &copilot.Host{
		SkillsDir: t.TempDir(),
		AgentsDir: t.TempDir(),
		ConfigDir: t.TempDir(),
	}
	store := state.NewStore(t.TempDir())
	bkp := backup.NewStore(t.TempDir())
	svc := services{host: host, state: store, backup: bkp}

	// Seed an SDD block first.
	if err := applySDD(host, store, bkp, sdd.DefaultAssignments(), sdd.DefaultEfforts(), false, nil); err != nil {
		t.Fatalf("seed SDD: %v", err)
	}

	if err := switchToODD(svc); err != nil {
		t.Fatalf("switchToODD: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(host.ConfigDir, "copilot-instructions.md"))
	if err != nil {
		t.Fatalf("read instructions: %v", err)
	}
	content := string(data)
	if !strings.Contains(content, odd.MarkerStart) {
		t.Error("ODD marker not found after switching to ODD")
	}
	if strings.Contains(content, sdd.MarkerStart) {
		t.Error("SDD marker should be removed after switching to ODD")
	}

	st, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	if st.IsSDDWorkflow() {
		t.Error("workflow should not be sdd after switching to ODD")
	}
	if !st.ODD {
		t.Error("state.ODD should be true after switching to ODD")
	}
}

func TestNewWorkflow_DefaultsToODD(t *testing.T) {
	store := state.NewStore(t.TempDir())
	svc := services{state: store}
	scr := newWorkflow(svc).(*workflowScreen)
	if scr.current != workflowODD {
		t.Errorf("default workflow = %d, want ODD (0)", scr.current)
	}
}

func TestNewWorkflow_DetectsSDD(t *testing.T) {
	store := state.NewStore(t.TempDir())
	if err := store.SetWorkflow("sdd"); err != nil {
		t.Fatal(err)
	}
	svc := services{state: store}
	scr := newWorkflow(svc).(*workflowScreen)
	if scr.current != workflowSDD {
		t.Errorf("detected workflow = %d, want SDD (1)", scr.current)
	}
}
