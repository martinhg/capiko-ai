package tui

import (
	"os"
	"path/filepath"
	"testing"

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
