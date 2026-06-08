package tui

import (
	"os"
	"path/filepath"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/martinhg/capiko-ai/internal/copilot"
	"github.com/martinhg/capiko-ai/internal/engram"
	"github.com/martinhg/capiko-ai/internal/state"
)

func TestApplyEngramWritesEntryAndRecordsChecksum(t *testing.T) {
	cfgDir := t.TempDir()
	host := &copilot.Host{ConfigDir: cfgDir, MCPConfigPath: filepath.Join(cfgDir, "mcp-config.json")}
	store := state.NewStore(t.TempDir())
	rec := &state.EngramRecord{Enabled: true, CloudServer: "https://engram.example.com"}

	if err := applyEngram(host, store, nil, rec); err != nil {
		t.Fatal(err)
	}
	got, ok := engram.CLIEntryChecksum(host.MCPConfigPath)
	if !ok {
		t.Fatal("engram entry should be written to mcp-config.json")
	}
	st, _ := store.Load()
	if st.Engram == nil || st.Engram.Checksum != got {
		t.Errorf("state checksum = %+v, want the on-disk checksum %q", st.Engram, got)
	}
}

func TestRunSyncReappliesEngram(t *testing.T) {
	cfgDir := t.TempDir()
	host := &copilot.Host{ConfigDir: cfgDir, SkillsDir: filepath.Join(cfgDir, "skills"), MCPConfigPath: filepath.Join(cfgDir, "mcp-config.json")}
	store := state.NewStore(t.TempDir())
	if err := store.SetEngram(&state.EngramRecord{Enabled: true, CloudServer: "https://engram.example.com"}); err != nil {
		t.Fatal(err)
	}

	if _, err := RunSync(host, testCatalog(), nil, store, nil); err != nil {
		t.Fatalf("RunSync: %v", err)
	}
	if _, ok := engram.CLIEntryChecksum(host.MCPConfigPath); !ok {
		t.Error("sync did not re-apply the engram MCP entry")
	}
}

func TestApplyEngramConfigWithCloud(t *testing.T) {
	origC, origE := cloudConfig, cloudEnroll
	t.Cleanup(func() { cloudConfig, cloudEnroll = origC, origE })
	var gotServer, gotProject string
	cloudConfig = func(s string) error { gotServer = s; return nil }
	cloudEnroll = func(p string) error { gotProject = p; return nil }

	cfgDir := t.TempDir()
	host := &copilot.Host{ConfigDir: cfgDir, MCPConfigPath: filepath.Join(cfgDir, "mcp-config.json")}
	store := state.NewStore(t.TempDir())
	workspace := t.TempDir()
	rec := &state.EngramRecord{Enabled: true, ArtifactMode: "hybrid", CloudServer: "https://engram.example.com"}

	if err := applyEngramConfig(services{host: host, state: store}, workspace, rec); err != nil {
		t.Fatal(err)
	}
	if gotServer != "https://engram.example.com" {
		t.Errorf("cloud config server = %q", gotServer)
	}
	if want := filepath.Base(workspace); gotProject != want {
		t.Errorf("enrolled project = %q, want %q", gotProject, want)
	}
	if _, err := os.Stat(filepath.Join(workspace, ".engram", "config.json")); err != nil {
		t.Errorf("project config not written: %v", err)
	}
	if _, ok := engram.CLIEntryChecksum(host.MCPConfigPath); !ok {
		t.Error("MCP entry not written")
	}
	st, _ := store.Load()
	if st.Engram == nil || !st.Engram.Enabled || st.Engram.CloudServer != "https://engram.example.com" {
		t.Errorf("state = %+v", st.Engram)
	}
}

func TestApplyEngramConfigLocalOnlySkipsCloud(t *testing.T) {
	origC, origE := cloudConfig, cloudEnroll
	t.Cleanup(func() { cloudConfig, cloudEnroll = origC, origE })
	called := false
	cloudConfig = func(string) error { called = true; return nil }
	cloudEnroll = func(string) error { called = true; return nil }

	cfgDir := t.TempDir()
	host := &copilot.Host{ConfigDir: cfgDir, MCPConfigPath: filepath.Join(cfgDir, "mcp-config.json")}
	store := state.NewStore(t.TempDir())
	workspace := t.TempDir()
	rec := &state.EngramRecord{Enabled: true, ArtifactMode: "hybrid"}

	if err := applyEngramConfig(services{host: host, state: store}, workspace, rec); err != nil {
		t.Fatal(err)
	}
	if called {
		t.Error("cloud ops must not run without a server")
	}
	if _, ok := engram.CLIEntryChecksum(host.MCPConfigPath); !ok {
		t.Error("MCP entry should still be written locally")
	}
}

func TestApplyEngramConfigWritesVSCodeSurface(t *testing.T) {
	cfgDir := t.TempDir()
	host := &copilot.Host{ConfigDir: cfgDir, MCPConfigPath: filepath.Join(cfgDir, "mcp-config.json")}
	store := state.NewStore(t.TempDir())
	workspace := t.TempDir()
	rec := &state.EngramRecord{Enabled: true, ArtifactMode: "hybrid", Surfaces: []string{"cli", "vscode"}}

	if err := applyEngramConfig(services{host: host, state: store}, workspace, rec); err != nil {
		t.Fatal(err)
	}
	if _, ok := engram.CLIEntryChecksum(host.MCPConfigPath); !ok {
		t.Error("CLI MCP entry not written")
	}
	if _, err := os.Stat(filepath.Join(workspace, ".vscode", "mcp.json")); err != nil {
		t.Errorf("VS Code mcp.json not written: %v", err)
	}
}

func TestApplyEngramConfigSkipsVSCodeWhenNotSelected(t *testing.T) {
	cfgDir := t.TempDir()
	host := &copilot.Host{ConfigDir: cfgDir, MCPConfigPath: filepath.Join(cfgDir, "mcp-config.json")}
	workspace := t.TempDir()
	rec := &state.EngramRecord{Enabled: true, Surfaces: []string{"cli"}}

	if err := applyEngramConfig(services{host: host, state: state.NewStore(t.TempDir())}, workspace, rec); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(workspace, ".vscode", "mcp.json")); err == nil {
		t.Error("VS Code mcp.json should not be written when the vscode surface is off")
	}
}

func TestApplyEngramConfigDisableRemovesEntry(t *testing.T) {
	cfgDir := t.TempDir()
	host := &copilot.Host{ConfigDir: cfgDir, MCPConfigPath: filepath.Join(cfgDir, "mcp-config.json")}
	store := state.NewStore(t.TempDir())
	workspace := t.TempDir()

	if err := applyEngramConfig(services{host: host, state: store}, workspace, &state.EngramRecord{Enabled: true, ArtifactMode: "hybrid"}); err != nil {
		t.Fatal(err)
	}
	if _, ok := engram.CLIEntryChecksum(host.MCPConfigPath); !ok {
		t.Fatal("precondition: entry should exist after enable")
	}

	if err := applyEngramConfig(services{host: host, state: store}, workspace, &state.EngramRecord{Enabled: false}); err != nil {
		t.Fatal(err)
	}
	if _, ok := engram.CLIEntryChecksum(host.MCPConfigPath); ok {
		t.Error("disable should remove the engram MCP entry")
	}
	st, _ := store.Load()
	if st.Engram == nil || st.Engram.Enabled {
		t.Errorf("state should record disabled engram, got %+v", st.Engram)
	}
}

func TestEngramScreenToggleVSCode(t *testing.T) {
	s := newEngram(services{}).(*engramScreen)
	s.cursor = 3 // VS Code row
	if s.vscode {
		t.Fatal("vscode should start off")
	}
	s.Update(key("space"))
	if !s.vscode {
		t.Error("space should toggle the vscode surface on")
	}
}

func TestEngramScreenToggleAndCycleMode(t *testing.T) {
	s := newEngram(services{}).(*engramScreen)
	if s.enabled {
		t.Fatal("engram should start disabled")
	}
	s.Update(key("space")) // cursor 0 = Enabled
	if !s.enabled {
		t.Error("space should toggle enabled on")
	}
	s.Update(key("down")) // cursor 1 = Mode
	start := s.mode
	s.Update(key("right"))
	if s.mode == start {
		t.Errorf("right should cycle the mode away from %q", start)
	}
}

func TestEngramScreenEditServer(t *testing.T) {
	s := newEngram(services{}).(*engramScreen)
	s.cursor = 2 // Cloud server
	s.Update(key("c"))
	if !s.editing {
		t.Fatal("c should start editing the server")
	}
	s.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("https://e.example.com")})
	s.Update(key("enter"))
	if s.server != "https://e.example.com" {
		t.Errorf("server = %q after edit", s.server)
	}
}

func TestEngramScreenApplyTransitionsAndDone(t *testing.T) {
	s := newEngram(services{}).(*engramScreen)
	s.cursor = engramRows // Apply
	_, cmd := s.Update(key("enter"))
	if s.state != engramApplying {
		t.Fatalf("state = %d, want applying", s.state)
	}
	if cmd == nil {
		t.Fatal("Apply should return a command")
	}
	// Drive the result message directly (avoid running the real apply / Getwd).
	next, _ := s.Update(engramAppliedMsg{})
	if next.(*engramScreen).state != engramDone {
		t.Error("applied message should move to done")
	}
}

func TestEngramScreenBack(t *testing.T) {
	s := newEngram(services{}).(*engramScreen)
	_, cmd := s.Update(key("esc"))
	if _, ok := cmd().(backMsg); !ok {
		t.Error("esc should emit backMsg")
	}
}

func TestRunSyncSkipsEngramWhenUnmanaged(t *testing.T) {
	cfgDir := t.TempDir()
	host := &copilot.Host{ConfigDir: cfgDir, SkillsDir: filepath.Join(cfgDir, "skills"), MCPConfigPath: filepath.Join(cfgDir, "mcp-config.json")}
	store := state.NewStore(t.TempDir())

	if _, err := RunSync(host, testCatalog(), nil, store, nil); err != nil {
		t.Fatalf("RunSync: %v", err)
	}
	if _, err := os.Stat(host.MCPConfigPath); err == nil {
		t.Error("sync wrote an engram entry for an unmanaged user")
	}
}
