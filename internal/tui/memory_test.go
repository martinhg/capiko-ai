package tui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/martinhg/capiko-ai/internal/copilot"
	"github.com/martinhg/capiko-ai/internal/efficiency"
	"github.com/martinhg/capiko-ai/internal/memory"
	"github.com/martinhg/capiko-ai/internal/persona"
	"github.com/martinhg/capiko-ai/internal/state"
)

// ---- Work Unit 2: applyMemoryProtocol applier ----

func TestApplyMemoryProtocolWritesBlock(t *testing.T) {
	cfgDir := t.TempDir()
	host := &copilot.Host{ConfigDir: cfgDir}

	if err := applyMemoryProtocol(host, nil, nil, true); err != nil {
		t.Fatalf("applyMemoryProtocol: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(cfgDir, "copilot-instructions.md"))
	if err != nil {
		t.Fatalf("instructions not written: %v", err)
	}
	content := string(data)
	if !strings.Contains(content, memory.MarkerStart) {
		t.Error("memory marker not found in instructions file")
	}
	if !strings.Contains(content, "Memory protocol") {
		t.Error("memory heading not found")
	}
}

func TestApplyMemoryProtocolDisabledRemovesBlock(t *testing.T) {
	cfgDir := t.TempDir()
	host := &copilot.Host{ConfigDir: cfgDir}

	if err := applyMemoryProtocol(host, nil, nil, true); err != nil {
		t.Fatal(err)
	}
	if err := applyMemoryProtocol(host, nil, nil, false); err != nil {
		t.Fatal(err)
	}

	data, _ := os.ReadFile(filepath.Join(cfgDir, "copilot-instructions.md"))
	if strings.Contains(string(data), memory.MarkerStart) {
		t.Error("memory block should be removed when disabled")
	}
}

func TestApplyMemoryProtocolNilHostIsNoop(t *testing.T) {
	if err := applyMemoryProtocol(nil, nil, nil, true); err != nil {
		t.Errorf("nil host should be a no-op, got error: %v", err)
	}
}

func TestApplyMemoryProtocolIdempotent(t *testing.T) {
	cfgDir := t.TempDir()
	host := &copilot.Host{ConfigDir: cfgDir}

	if err := applyMemoryProtocol(host, nil, nil, true); err != nil {
		t.Fatal(err)
	}
	if err := applyMemoryProtocol(host, nil, nil, true); err != nil {
		t.Fatal(err)
	}

	data, _ := os.ReadFile(filepath.Join(cfgDir, "copilot-instructions.md"))
	content := string(data)
	count := strings.Count(content, memory.MarkerStart)
	if count != 1 {
		t.Errorf("memory.MarkerStart appears %d times, want 1", count)
	}
}

func TestApplyMemoryProtocolPreservesPersonaAndEfficiency(t *testing.T) {
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
	if err := applyMemoryProtocol(host, store, nil, true); err != nil {
		t.Fatal(err)
	}

	data, _ := os.ReadFile(filepath.Join(cfgDir, "copilot-instructions.md"))
	content := string(data)
	if !strings.Contains(content, "capiko:persona:start") {
		t.Error("persona block clobbered by memory apply")
	}
	if !strings.Contains(content, efficiency.MarkerStart) {
		t.Error("efficiency block clobbered by memory apply")
	}
	if !strings.Contains(content, memory.MarkerStart) {
		t.Error("memory block missing after apply")
	}
}

// ---- Work Unit 3: wiring in applyEngramConfig / disableEngram ----

func TestApplyEngramConfigInjectsMemory(t *testing.T) {
	origC, origE, origU := cloudConfig, cloudEnroll, vscodeUserMCPath
	t.Cleanup(func() { cloudConfig, cloudEnroll, vscodeUserMCPath = origC, origE, origU })
	cloudConfig = func(string) error { return nil }
	cloudEnroll = func(string) error { return nil }
	vscodeUserMCPath = func() (string, error) { return filepath.Join(t.TempDir(), "user-mcp.json"), nil }

	cfgDir := t.TempDir()
	host := &copilot.Host{ConfigDir: cfgDir, MCPConfigPath: filepath.Join(cfgDir, "mcp-config.json")}
	store := state.NewStore(t.TempDir())
	workspace := t.TempDir()
	rec := &state.EngramRecord{Enabled: true, ArtifactMode: "hybrid"}

	if err := applyEngramConfig(services{host: host, state: store}, workspace, rec); err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(filepath.Join(cfgDir, "copilot-instructions.md"))
	if err != nil {
		t.Fatalf("instructions not written: %v", err)
	}
	if !strings.Contains(string(data), memory.MarkerStart) {
		t.Error("applyEngramConfig did not inject memory block")
	}
}

func TestDisableEngramRemovesMemory(t *testing.T) {
	origC, origE, origU := cloudConfig, cloudEnroll, vscodeUserMCPath
	t.Cleanup(func() { cloudConfig, cloudEnroll, vscodeUserMCPath = origC, origE, origU })
	cloudConfig = func(string) error { return nil }
	cloudEnroll = func(string) error { return nil }
	vscodeUserMCPath = func() (string, error) { return filepath.Join(t.TempDir(), "user-mcp.json"), nil }

	cfgDir := t.TempDir()
	host := &copilot.Host{ConfigDir: cfgDir, MCPConfigPath: filepath.Join(cfgDir, "mcp-config.json")}
	store := state.NewStore(t.TempDir())
	workspace := t.TempDir()

	// Enable first.
	if err := applyEngramConfig(services{host: host, state: store}, workspace, &state.EngramRecord{Enabled: true, ArtifactMode: "hybrid"}); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(filepath.Join(cfgDir, "copilot-instructions.md"))
	if !strings.Contains(string(data), memory.MarkerStart) {
		t.Fatal("precondition: memory block must be present after enable")
	}

	// Disable.
	if err := applyEngramConfig(services{host: host, state: store}, workspace, &state.EngramRecord{Enabled: false}); err != nil {
		t.Fatal(err)
	}

	data, _ = os.ReadFile(filepath.Join(cfgDir, "copilot-instructions.md"))
	if strings.Contains(string(data), memory.MarkerStart) {
		t.Error("disableEngram should remove memory block")
	}
}

// ---- Work Unit 4: RunSync re-apply ----

func TestRunSyncReappliesMemoryProtocol(t *testing.T) {
	cfgDir := t.TempDir()
	host := &copilot.Host{
		ConfigDir:     cfgDir,
		SkillsDir:     filepath.Join(cfgDir, "skills"),
		MCPConfigPath: filepath.Join(cfgDir, "mcp-config.json"),
	}
	store := state.NewStore(t.TempDir())
	if err := store.SetEngram(&state.EngramRecord{Enabled: true}); err != nil {
		t.Fatal(err)
	}

	if _, err := RunSync(host, testCatalog(), nil, store, nil); err != nil {
		t.Fatalf("RunSync: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(cfgDir, "copilot-instructions.md"))
	if err != nil {
		t.Fatalf("instructions not written by sync: %v", err)
	}
	if !strings.Contains(string(data), memory.MarkerStart) {
		t.Errorf("sync did not re-apply the memory protocol block:\n%s", data)
	}
}

func TestRunSyncSkipsMemoryWhenEngramDisabled(t *testing.T) {
	cfgDir := t.TempDir()
	host := &copilot.Host{
		ConfigDir:     cfgDir,
		SkillsDir:     filepath.Join(cfgDir, "skills"),
		MCPConfigPath: filepath.Join(cfgDir, "mcp-config.json"),
	}
	store := state.NewStore(t.TempDir())
	// No engram record → st.Engram is nil.

	if _, err := RunSync(host, testCatalog(), nil, store, nil); err != nil {
		t.Fatalf("RunSync: %v", err)
	}

	data, _ := os.ReadFile(filepath.Join(cfgDir, "copilot-instructions.md"))
	if strings.Contains(string(data), memory.MarkerStart) {
		t.Error("sync should not write memory block when engram is not enabled")
	}
}
