package tui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/martinhg/capiko-ai/internal/copilot"
	"github.com/martinhg/capiko-ai/internal/engram"
	"github.com/martinhg/capiko-ai/internal/headroom"
	"github.com/martinhg/capiko-ai/internal/persona"
	"github.com/martinhg/capiko-ai/internal/state"
)

func TestApplyHeadroomWiresEntry(t *testing.T) {
	mcp := filepath.Join(t.TempDir(), "mcp-config.json")
	host := &copilot.Host{MCPConfigPath: mcp}
	store := state.NewStore(t.TempDir())

	if err := applyHeadroom(host, store, nil, true); err != nil {
		t.Fatalf("applyHeadroom: %v", err)
	}

	data, err := os.ReadFile(mcp)
	if err != nil {
		t.Fatalf("mcp-config not written: %v", err)
	}
	content := string(data)
	if !strings.Contains(content, `"headroom"`) || !strings.Contains(content, `"serve"`) {
		t.Errorf("headroom entry not written:\n%s", content)
	}

	st, _ := store.Load()
	if st.Headroom == nil || !st.Headroom.Enabled || st.Headroom.Checksum == "" {
		t.Errorf("headroom not recorded enabled with checksum: %+v", st.Headroom)
	}
	// Checksum must match the rendered entry, so drift comparisons are meaningful.
	if want := engram.EntryChecksum(headroom.CopilotCLIEntry()); st.Headroom.Checksum != want {
		t.Errorf("recorded checksum %q, want %q", st.Headroom.Checksum, want)
	}
}

func TestApplyHeadroomDisabledRemovesEntry(t *testing.T) {
	mcp := filepath.Join(t.TempDir(), "mcp-config.json")
	host := &copilot.Host{MCPConfigPath: mcp}
	store := state.NewStore(t.TempDir())

	if err := applyHeadroom(host, store, nil, true); err != nil {
		t.Fatal(err)
	}
	if err := applyHeadroom(host, store, nil, false); err != nil {
		t.Fatal(err)
	}

	data, _ := os.ReadFile(mcp)
	if strings.Contains(string(data), `"headroom"`) {
		t.Errorf("headroom entry should be removed when disabled:\n%s", data)
	}
	st, _ := store.Load()
	if st.Headroom == nil || st.Headroom.Enabled {
		t.Errorf("headroom should be recorded disabled, got %+v", st.Headroom)
	}
}

func TestApplyHeadroomPreservesOtherServers(t *testing.T) {
	mcp := filepath.Join(t.TempDir(), "mcp-config.json")
	host := &copilot.Host{MCPConfigPath: mcp}
	// Seed an existing engram entry; wiring headroom must not clobber it.
	if err := engram.MergeMCPEntry(mcp, "mcpServers", "engram", engram.CopilotCLIEntry("")); err != nil {
		t.Fatal(err)
	}
	if err := applyHeadroom(host, state.NewStore(t.TempDir()), nil, true); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(mcp)
	content := string(data)
	if !strings.Contains(content, `"engram"`) || !strings.Contains(content, `"headroom"`) {
		t.Errorf("both servers must coexist:\n%s", content)
	}
}

func TestApplyHeadroomWritesAgentGuidance(t *testing.T) {
	cfg := t.TempDir()
	host := &copilot.Host{ConfigDir: cfg, MCPConfigPath: filepath.Join(cfg, "mcp-config.json")}

	if err := applyHeadroom(host, state.NewStore(t.TempDir()), nil, true); err != nil {
		t.Fatalf("applyHeadroom: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(cfg, "copilot-instructions.md"))
	if err != nil {
		t.Fatalf("instructions not written: %v", err)
	}
	content := string(data)
	if !strings.Contains(content, headroom.GuidanceMarkerStart) {
		t.Error("headroom guidance marker not injected")
	}
	if !strings.Contains(content, "headroom_compress") {
		t.Errorf("guidance should name the compression tools:\n%s", content)
	}
}

func TestDisableHeadroomRemovesAgentGuidance(t *testing.T) {
	cfg := t.TempDir()
	host := &copilot.Host{ConfigDir: cfg, MCPConfigPath: filepath.Join(cfg, "mcp-config.json")}
	store := state.NewStore(t.TempDir())

	if err := applyHeadroom(host, store, nil, true); err != nil {
		t.Fatal(err)
	}
	if err := applyHeadroom(host, store, nil, false); err != nil {
		t.Fatal(err)
	}

	data, _ := os.ReadFile(filepath.Join(cfg, "copilot-instructions.md"))
	if strings.Contains(string(data), headroom.GuidanceMarkerStart) {
		t.Errorf("guidance should be removed when headroom is disabled:\n%s", data)
	}
}

func TestApplyHeadroomGuidanceCoexistsWithPersona(t *testing.T) {
	cfg := t.TempDir()
	host := &copilot.Host{ConfigDir: cfg, MCPConfigPath: filepath.Join(cfg, "mcp-config.json"), SkillsDir: filepath.Join(cfg, "skills")}
	store := state.NewStore(t.TempDir())
	p, ok := persona.ByID("capiko")
	if !ok {
		t.Fatal("persona capiko not found")
	}
	if err := applyPersona(host, store, nil, p); err != nil {
		t.Fatal(err)
	}
	if err := applyHeadroom(host, store, nil, true); err != nil {
		t.Fatal(err)
	}

	data, _ := os.ReadFile(filepath.Join(cfg, "copilot-instructions.md"))
	content := string(data)
	if !strings.Contains(content, "capiko:persona:start") {
		t.Error("persona block clobbered by headroom guidance")
	}
	if !strings.Contains(content, headroom.GuidanceMarkerStart) {
		t.Error("headroom guidance block missing")
	}
}

// withStubHeadroomDetected swaps the detection seam for the screen tests.
func withStubHeadroomDetected(t *testing.T, detected bool) {
	t.Helper()
	prev := headroomDetected
	headroomDetected = func() bool { return detected }
	t.Cleanup(func() { headroomDetected = prev })
}

func TestHeadroomScreenTogglesAndApplies(t *testing.T) {
	withStubHeadroomDetected(t, true)
	mcp := filepath.Join(t.TempDir(), "mcp-config.json")
	svc := services{host: &copilot.Host{MCPConfigPath: mcp}, state: state.NewStore(t.TempDir())}
	s, ok := newHeadroom(svc).(*headroomScreen)
	if !ok {
		t.Fatal("newHeadroom did not return *headroomScreen")
	}
	if s.enabled {
		t.Fatal("headroom should default to off")
	}

	s.Update(key(" ")) // toggle Enabled on (cursor starts at 0)
	if !s.enabled {
		t.Error("space should toggle Enabled on")
	}

	s.Update(key("down")) // move to Apply
	_, cmd := s.Update(key("enter"))
	if cmd == nil {
		t.Fatal("enter on Apply should return a command")
	}
	if _, ok := cmd().(headroomAppliedMsg); !ok {
		t.Fatal("Apply should emit headroomAppliedMsg")
	}
	if _, err := os.Stat(mcp); err != nil {
		t.Errorf("Apply should have written mcp-config: %v", err)
	}
}

func TestHeadroomScreenShowsInstallHintWhenAbsent(t *testing.T) {
	withStubHeadroomDetected(t, false)
	s := newHeadroom(services{host: &copilot.Host{MCPConfigPath: filepath.Join(t.TempDir(), "m.json")}})
	view := s.View()
	if !strings.Contains(view, "not on PATH") || !strings.Contains(view, "headroom-ai") {
		t.Errorf("absent headroom should show an install hint:\n%s", view)
	}
}

func TestHeadroomScreenNoInstallHintWhenPresent(t *testing.T) {
	withStubHeadroomDetected(t, true)
	s := newHeadroom(services{host: &copilot.Host{MCPConfigPath: filepath.Join(t.TempDir(), "m.json")}})
	if strings.Contains(s.View(), "not on PATH") {
		t.Errorf("present headroom should not show the install hint:\n%s", s.View())
	}
}

func TestRunSyncReappliesHeadroom(t *testing.T) {
	mcp := filepath.Join(t.TempDir(), "mcp-config.json")
	host := &copilot.Host{MCPConfigPath: mcp, SkillsDir: t.TempDir()}
	store := state.NewStore(t.TempDir())
	if err := store.SetHeadroom(&state.HeadroomRecord{Enabled: true}); err != nil {
		t.Fatal(err)
	}

	if _, err := RunSync(host, testCatalog(), nil, store, nil); err != nil {
		t.Fatalf("RunSync: %v", err)
	}
	data, _ := os.ReadFile(mcp)
	if !strings.Contains(string(data), `"headroom"`) {
		t.Errorf("sync did not re-apply the headroom wiring:\n%s", data)
	}
}
