package tui

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/martinhg/capiko-ai/internal/copilot"
	"github.com/martinhg/capiko-ai/internal/copilothooks"
	"github.com/martinhg/capiko-ai/internal/efficiency"
	"github.com/martinhg/capiko-ai/internal/headroom"
	"github.com/martinhg/capiko-ai/internal/memory"
	"github.com/martinhg/capiko-ai/internal/persona"
	"github.com/martinhg/capiko-ai/internal/scoped"
	"github.com/martinhg/capiko-ai/internal/sdd"
	"github.com/martinhg/capiko-ai/internal/state"
	"github.com/martinhg/capiko-ai/internal/trigger"
)

func purgeHost(t *testing.T) *copilot.Host {
	t.Helper()
	configDir := t.TempDir()
	return &copilot.Host{
		ConfigDir:     configDir,
		SkillsDir:     filepath.Join(configDir, "skills"),
		AgentsDir:     filepath.Join(configDir, "agents"),
		MCPConfigPath: filepath.Join(configDir, "mcp-config.json"),
		HooksDir:      filepath.Join(configDir, "hooks"),
	}
}

func seedPurgeEnv(t *testing.T, host *copilot.Host, store *state.Store, workspace, capikoHome string) {
	t.Helper()
	seedInstalled(t, host, store)

	// copilot-instructions.md with all marker blocks
	instrPath := filepath.Join(host.ConfigDir, "copilot-instructions.md")
	content := persona.MarkerStart + "\npersona block\n" + persona.MarkerEnd + "\n\n" +
		sdd.MarkerStart + "\nsdd block\n" + sdd.MarkerEnd + "\n\n" +
		memory.MarkerStart + "\nmemory block\n" + memory.MarkerEnd + "\n\n" +
		trigger.MarkerStart + "\ntrigger block\n" + trigger.MarkerEnd + "\n\n" +
		headroom.GuidanceMarkerStart + "\nheadroom block\n" + headroom.GuidanceMarkerEnd + "\n\n" +
		efficiency.MarkerStart + "\nefficiency block\n" + efficiency.MarkerEnd + "\n"
	if err := os.WriteFile(instrPath, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	// MCP config with engram and headroom
	mcpData := map[string]any{
		"mcpServers": map[string]any{
			"engram":   map[string]any{"command": "engram"},
			"headroom": map[string]any{"command": "headroom"},
		},
	}
	mcpJSON, _ := json.MarshalIndent(mcpData, "", "  ")
	if err := os.WriteFile(host.MCPConfigPath, mcpJSON, 0o644); err != nil {
		t.Fatal(err)
	}

	// Copilot hooks
	if err := os.MkdirAll(host.HooksDir, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, f := range []string{copilothooks.GuardrailsFile, copilothooks.SessionCheckFile, copilothooks.SessionProbeFile} {
		if err := os.WriteFile(filepath.Join(host.HooksDir, f), []byte(`{}`), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	// .capiko-verified.json
	if err := os.WriteFile(filepath.Join(host.ConfigDir, ".capiko-verified.json"), []byte(`{}`), 0o644); err != nil {
		t.Fatal(err)
	}

	// Scoped instructions
	scopedDir := scoped.Dir(host.ConfigDir)
	if err := os.MkdirAll(scopedDir, 0o755); err != nil {
		t.Fatal(err)
	}
	ins, _ := scoped.Load()
	for _, in := range ins {
		if err := os.WriteFile(scoped.Path(scopedDir, in), []byte(in.Content), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	// Workspace git hooks
	hooksDir := filepath.Join(workspace, ".git", "hooks")
	if err := os.MkdirAll(hooksDir, 0o755); err != nil {
		t.Fatal(err)
	}
	hookContent := "#!/bin/sh\n\n" +
		"# >>> capiko:cga:pre-commit >>>\necho cga\n# <<< capiko:cga:pre-commit <<<\n"
	if err := os.WriteFile(filepath.Join(hooksDir, "pre-commit"), []byte(hookContent), 0o755); err != nil {
		t.Fatal(err)
	}

	// .git/capiko directory
	gitCapiko := filepath.Join(workspace, ".git", "capiko")
	if err := os.MkdirAll(gitCapiko, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(gitCapiko, "cga-findings.jsonl"), []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}

	// .engram/config.json
	engramDir := filepath.Join(workspace, ".engram")
	if err := os.MkdirAll(engramDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(engramDir, "config.json"), []byte(`{"project_name":"test"}`), 0o644); err != nil {
		t.Fatal(err)
	}

	// VS Code workspace MCP
	vscodeDir := filepath.Join(workspace, ".vscode")
	if err := os.MkdirAll(vscodeDir, 0o755); err != nil {
		t.Fatal(err)
	}
	vscodeMCP := map[string]any{
		"servers": map[string]any{
			"engram": map[string]any{"command": "engram"},
		},
	}
	vscodeJSON, _ := json.MarshalIndent(vscodeMCP, "", "  ")
	if err := os.WriteFile(filepath.Join(vscodeDir, "mcp.json"), vscodeJSON, 0o644); err != nil {
		t.Fatal(err)
	}

	// ~/.capiko with state and backups
	if err := os.MkdirAll(filepath.Join(capikoHome, "backups"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(capikoHome, "state.json"), []byte(`{}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(capikoHome, "review-mode.json"), []byte(`{}`), 0o644); err != nil {
		t.Fatal(err)
	}
}

func stubPurgeSeams(t *testing.T, workspace, capikoHome, vsCodeUserPath string) {
	t.Helper()
	origGitDir := purgeGitDir
	origGitCommonDir := purgeGitCommonDir
	origVSCode := purgeVSCodeUserPath
	origHome := purgeHomeDir
	origRemoveAll := purgeRemoveAll
	origRemove := purgeRemove
	origScopedLoad := purgeScopedLoad
	origCustomDirs := purgeCustomInstrDirs

	gitDir := filepath.Join(workspace, ".git")

	purgeGitDir = func(string) (string, error) { return gitDir, nil }
	purgeGitCommonDir = func(string) (string, error) { return gitDir, nil }
	purgeVSCodeUserPath = func() (string, error) { return vsCodeUserPath, nil }
	purgeHomeDir = func() (string, error) { return filepath.Dir(capikoHome), nil }
	purgeRemoveAll = os.RemoveAll
	purgeRemove = os.Remove
	purgeScopedLoad = scoped.Load
	purgeCustomInstrDirs = func() []string { return nil }

	t.Cleanup(func() {
		purgeGitDir = origGitDir
		purgeGitCommonDir = origGitCommonDir
		purgeVSCodeUserPath = origVSCode
		purgeHomeDir = origHome
		purgeRemoveAll = origRemoveAll
		purgeRemove = origRemove
		purgeScopedLoad = origScopedLoad
		purgeCustomInstrDirs = origCustomDirs
	})
}

func TestPurgeAll_FullPurge(t *testing.T) {
	host := purgeHost(t)
	store := state.NewStore(t.TempDir())
	workspace := t.TempDir()
	capikoHome := filepath.Join(t.TempDir(), ".capiko")
	vsCodeUserPath := filepath.Join(t.TempDir(), "Code", "User", "mcp.json")

	// Write VS Code user MCP
	if err := os.MkdirAll(filepath.Dir(vsCodeUserPath), 0o755); err != nil {
		t.Fatal(err)
	}
	vscodeMCP := map[string]any{"servers": map[string]any{"engram": map[string]any{"command": "engram"}}}
	vscodeJSON, _ := json.MarshalIndent(vscodeMCP, "", "  ")
	if err := os.WriteFile(vsCodeUserPath, vscodeJSON, 0o644); err != nil {
		t.Fatal(err)
	}

	// Seed everything
	if err := os.MkdirAll(filepath.Join(workspace, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	seedPurgeEnv(t, host, store, workspace, capikoHome)
	stubPurgeSeams(t, workspace, capikoHome, vsCodeUserPath)

	rpt, err := PurgeAll(PurgeConfig{
		Host:      host,
		Store:     store,
		Workspace: workspace,
	})
	if err != nil {
		t.Fatalf("PurgeAll: %v", err)
	}

	// Skills and agents removed
	if len(rpt.Skills) != len(testCatalog()) {
		t.Errorf("Skills removed = %d, want %d", len(rpt.Skills), len(testCatalog()))
	}
	if len(rpt.Agents) != len(testAgentCatalog()) {
		t.Errorf("Agents removed = %d, want %d", len(rpt.Agents), len(testAgentCatalog()))
	}

	// Instruction blocks removed
	if len(rpt.InstructionBlocks) != 6 {
		t.Errorf("InstructionBlocks removed = %d, want 6, got %v", len(rpt.InstructionBlocks), rpt.InstructionBlocks)
	}

	// MCP entries removed
	if len(rpt.MCPEntries) != 2 {
		t.Errorf("MCPEntries removed = %d, want 2", len(rpt.MCPEntries))
	}

	// Copilot hooks removed
	if len(rpt.CopilotHooks) != 3 {
		t.Errorf("CopilotHooks removed = %d, want 3", len(rpt.CopilotHooks))
	}

	// Git hooks cleaned
	if len(rpt.GitHooks) == 0 {
		t.Error("expected at least one git hook removal")
	}

	// Home artifacts removed
	if len(rpt.HomeArtifacts) == 0 {
		t.Error("expected home artifacts to be removed")
	}

	// VS Code entries cleaned
	if len(rpt.VSCodeEntries) != 2 {
		t.Errorf("VSCodeEntries = %d, want 2 (workspace + user)", len(rpt.VSCodeEntries))
	}

	// No warnings in a clean purge
	if len(rpt.Warnings) > 0 {
		t.Errorf("unexpected warnings: %v", rpt.Warnings)
	}

	// Verify ~/.capiko is gone
	if _, err := os.Stat(capikoHome); !os.IsNotExist(err) {
		t.Error("~/.capiko must be removed")
	}

	// Verify copilot-instructions.md is empty or contains no markers
	instrPath := filepath.Join(host.ConfigDir, "copilot-instructions.md")
	data, err := os.ReadFile(instrPath)
	if err != nil && !os.IsNotExist(err) {
		t.Fatal(err)
	}
	for _, m := range allInstructionMarkers {
		if strings.Contains(string(data), m.start) {
			t.Errorf("copilot-instructions.md still contains %s marker", m.name)
		}
	}

	// Verify .engram/config.json removed
	if _, err := os.Stat(filepath.Join(workspace, ".engram", "config.json")); !os.IsNotExist(err) {
		t.Error(".engram/config.json must be removed")
	}

	// Verify .git/capiko removed
	if _, err := os.Stat(filepath.Join(workspace, ".git", "capiko")); !os.IsNotExist(err) {
		t.Error(".git/capiko must be removed")
	}

	// TotalRemoved makes sense
	if rpt.TotalRemoved() == 0 {
		t.Error("TotalRemoved() must be >0 after full purge")
	}
}

func TestPurgeAll_NilStore_WarnsButContinues(t *testing.T) {
	host := purgeHost(t)
	workspace := t.TempDir()
	capikoHome := filepath.Join(t.TempDir(), ".capiko")
	if err := os.MkdirAll(capikoHome, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(capikoHome, "state.json"), []byte(`{}`), 0o644); err != nil {
		t.Fatal(err)
	}
	vsCodeUserPath := filepath.Join(t.TempDir(), "mcp.json")

	if err := os.MkdirAll(filepath.Join(workspace, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}

	// Write copilot-instructions.md with a persona block
	instrPath := filepath.Join(host.ConfigDir, "copilot-instructions.md")
	content := persona.MarkerStart + "\npersona block\n" + persona.MarkerEnd + "\n"
	if err := os.WriteFile(instrPath, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	stubPurgeSeams(t, workspace, capikoHome, vsCodeUserPath)

	rpt, err := PurgeAll(PurgeConfig{
		Host:      host,
		Store:     nil,
		Workspace: workspace,
	})
	if err != nil {
		t.Fatalf("PurgeAll with nil store: %v", err)
	}

	// Should have a warning about store
	hasStoreWarning := false
	for _, w := range rpt.Warnings {
		if strings.Contains(w, "state store unavailable") {
			hasStoreWarning = true
		}
	}
	if !hasStoreWarning {
		t.Error("expected warning about unavailable state store")
	}

	// Other categories should still work
	if len(rpt.InstructionBlocks) == 0 {
		t.Error("instruction blocks should still be cleaned with nil store")
	}

	// Home should still be cleaned
	if _, err := os.Stat(capikoHome); !os.IsNotExist(err) {
		t.Error("~/.capiko must be removed even with nil store")
	}
}

func TestPurgeAll_EmptyState_NoSkillAgentRemoval(t *testing.T) {
	host := purgeHost(t)
	store := state.NewStore(t.TempDir())
	workspace := t.TempDir()
	capikoHome := filepath.Join(t.TempDir(), ".capiko")
	if err := os.MkdirAll(capikoHome, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(capikoHome, "state.json"), []byte(`{}`), 0o644); err != nil {
		t.Fatal(err)
	}
	vsCodeUserPath := filepath.Join(t.TempDir(), "mcp.json")

	if err := os.MkdirAll(filepath.Join(workspace, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	stubPurgeSeams(t, workspace, capikoHome, vsCodeUserPath)

	rpt, err := PurgeAll(PurgeConfig{
		Host:      host,
		Store:     store,
		Workspace: workspace,
	})
	if err != nil {
		t.Fatalf("PurgeAll with empty state: %v", err)
	}

	if len(rpt.Skills) != 0 {
		t.Errorf("Skills = %v, want none with empty state", rpt.Skills)
	}
	if len(rpt.Agents) != 0 {
		t.Errorf("Agents = %v, want none with empty state", rpt.Agents)
	}
}

func TestPurgeAll_MCPEntriesRemoved(t *testing.T) {
	host := purgeHost(t)
	store := state.NewStore(t.TempDir())
	workspace := t.TempDir()
	capikoHome := filepath.Join(t.TempDir(), ".capiko")
	if err := os.MkdirAll(capikoHome, 0o755); err != nil {
		t.Fatal(err)
	}
	vsCodeUserPath := filepath.Join(t.TempDir(), "mcp.json")

	// Write MCP config with engram and headroom entries
	mcpData := map[string]any{
		"mcpServers": map[string]any{
			"engram":   map[string]any{"command": "engram"},
			"headroom": map[string]any{"command": "headroom"},
			"other":    map[string]any{"command": "keep-me"},
		},
	}
	mcpJSON, _ := json.MarshalIndent(mcpData, "", "  ")
	if err := os.WriteFile(host.MCPConfigPath, mcpJSON, 0o644); err != nil {
		t.Fatal(err)
	}

	if err := os.MkdirAll(filepath.Join(workspace, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	stubPurgeSeams(t, workspace, capikoHome, vsCodeUserPath)

	rpt, err := PurgeAll(PurgeConfig{
		Host:      host,
		Store:     store,
		Workspace: workspace,
	})
	if err != nil {
		t.Fatalf("PurgeAll: %v", err)
	}

	if len(rpt.MCPEntries) != 2 {
		t.Errorf("MCPEntries = %v, want [engram headroom]", rpt.MCPEntries)
	}

	// Verify the "other" server is preserved
	data, err := os.ReadFile(host.MCPConfigPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "keep-me") {
		t.Error("non-capiko MCP entry must be preserved")
	}
	if strings.Contains(string(data), `"engram"`) {
		t.Error("engram MCP entry must be removed")
	}
}

func TestPurgeAll_InstructionBlocksCleaned(t *testing.T) {
	host := purgeHost(t)
	store := state.NewStore(t.TempDir())
	workspace := t.TempDir()
	capikoHome := filepath.Join(t.TempDir(), ".capiko")
	if err := os.MkdirAll(capikoHome, 0o755); err != nil {
		t.Fatal(err)
	}
	vsCodeUserPath := filepath.Join(t.TempDir(), "mcp.json")

	// Write instructions file with user content + capiko blocks
	instrPath := filepath.Join(host.ConfigDir, "copilot-instructions.md")
	content := "# My own instructions\n\nKeep this.\n\n" +
		persona.MarkerStart + "\npersona block\n" + persona.MarkerEnd + "\n\n" +
		sdd.MarkerStart + "\nsdd block\n" + sdd.MarkerEnd + "\n"
	if err := os.WriteFile(instrPath, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := os.MkdirAll(filepath.Join(workspace, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	stubPurgeSeams(t, workspace, capikoHome, vsCodeUserPath)

	_, err := PurgeAll(PurgeConfig{
		Host:      host,
		Store:     store,
		Workspace: workspace,
	})
	if err != nil {
		t.Fatalf("PurgeAll: %v", err)
	}

	data, err := os.ReadFile(instrPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "Keep this.") {
		t.Error("user content must be preserved in copilot-instructions.md")
	}
	if strings.Contains(string(data), persona.MarkerStart) {
		t.Error("persona markers must be removed")
	}
	if strings.Contains(string(data), sdd.MarkerStart) {
		t.Error("sdd markers must be removed")
	}
}

func TestPurgeAll_HomeDirRemoved(t *testing.T) {
	host := purgeHost(t)
	store := state.NewStore(t.TempDir())
	workspace := t.TempDir()
	capikoHome := filepath.Join(t.TempDir(), ".capiko")
	vsCodeUserPath := filepath.Join(t.TempDir(), "mcp.json")

	if err := os.MkdirAll(filepath.Join(capikoHome, "backups", "bk1"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(capikoHome, "state.json"), []byte(`{}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(capikoHome, "review-mode.json"), []byte(`{}`), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := os.MkdirAll(filepath.Join(workspace, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	stubPurgeSeams(t, workspace, capikoHome, vsCodeUserPath)

	rpt, err := PurgeAll(PurgeConfig{
		Host:      host,
		Store:     store,
		Workspace: workspace,
	})
	if err != nil {
		t.Fatalf("PurgeAll: %v", err)
	}

	if _, err := os.Stat(capikoHome); !os.IsNotExist(err) {
		t.Error("~/.capiko must be removed entirely")
	}

	// Reports what was inside
	found := map[string]bool{}
	for _, n := range rpt.HomeArtifacts {
		found[n] = true
	}
	if !found["state.json"] {
		t.Error("HomeArtifacts should list state.json")
	}
	if !found["review-mode.json"] {
		t.Error("HomeArtifacts should list review-mode.json")
	}
	if !found["backups"] {
		t.Error("HomeArtifacts should list backups")
	}
}

func TestPurgeAll_GitHooksCleaned(t *testing.T) {
	host := purgeHost(t)
	store := state.NewStore(t.TempDir())
	workspace := t.TempDir()
	capikoHome := filepath.Join(t.TempDir(), ".capiko")
	if err := os.MkdirAll(capikoHome, 0o755); err != nil {
		t.Fatal(err)
	}
	vsCodeUserPath := filepath.Join(t.TempDir(), "mcp.json")

	// Write git hooks with both capiko and non-capiko content
	hooksDir := filepath.Join(workspace, ".git", "hooks")
	if err := os.MkdirAll(hooksDir, 0o755); err != nil {
		t.Fatal(err)
	}
	hookContent := "#!/bin/sh\n\necho user-hook\n\n" +
		"# >>> capiko:cga:pre-commit >>>\necho cga\n# <<< capiko:cga:pre-commit <<<\n"
	if err := os.WriteFile(filepath.Join(hooksDir, "pre-commit"), []byte(hookContent), 0o755); err != nil {
		t.Fatal(err)
	}

	stubPurgeSeams(t, workspace, capikoHome, vsCodeUserPath)

	rpt, err := PurgeAll(PurgeConfig{
		Host:      host,
		Store:     store,
		Workspace: workspace,
	})
	if err != nil {
		t.Fatalf("PurgeAll: %v", err)
	}

	if len(rpt.GitHooks) == 0 {
		t.Error("expected git hooks to be cleaned")
	}

	// User hook content preserved
	data, err := os.ReadFile(filepath.Join(hooksDir, "pre-commit"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "echo user-hook") {
		t.Error("user hook content must be preserved")
	}
	if strings.Contains(string(data), "capiko:cga") {
		t.Error("capiko hook block must be removed")
	}
}

func TestPurgeAll_CategoryFailureProducesWarning(t *testing.T) {
	host := purgeHost(t)
	store := state.NewStore(t.TempDir())
	workspace := t.TempDir()
	capikoHome := filepath.Join(t.TempDir(), ".capiko")
	if err := os.MkdirAll(capikoHome, 0o755); err != nil {
		t.Fatal(err)
	}
	vsCodeUserPath := filepath.Join(t.TempDir(), "mcp.json")

	if err := os.MkdirAll(filepath.Join(workspace, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	stubPurgeSeams(t, workspace, capikoHome, vsCodeUserPath)

	// Make MCP config unreadable
	if err := os.WriteFile(host.MCPConfigPath, []byte("not json"), 0o644); err != nil {
		t.Fatal(err)
	}

	rpt, err := PurgeAll(PurgeConfig{
		Host:      host,
		Store:     store,
		Workspace: workspace,
	})
	if err != nil {
		t.Fatalf("PurgeAll must not return error for category failures: %v", err)
	}

	hasWarning := false
	for _, w := range rpt.Warnings {
		if strings.Contains(w, "MCP") || strings.Contains(w, "parse") {
			hasWarning = true
		}
	}
	if !hasWarning {
		t.Errorf("expected a warning about invalid MCP config, got warnings: %v", rpt.Warnings)
	}
}

func TestPurgeAll_NoWorkspace_SkipsWorkspaceArtifacts(t *testing.T) {
	host := purgeHost(t)
	store := state.NewStore(t.TempDir())
	capikoHome := filepath.Join(t.TempDir(), ".capiko")
	if err := os.MkdirAll(capikoHome, 0o755); err != nil {
		t.Fatal(err)
	}
	vsCodeUserPath := filepath.Join(t.TempDir(), "mcp.json")

	stubPurgeSeams(t, "", capikoHome, vsCodeUserPath)

	rpt, err := PurgeAll(PurgeConfig{
		Host:      host,
		Store:     store,
		Workspace: "",
	})
	if err != nil {
		t.Fatalf("PurgeAll: %v", err)
	}

	if len(rpt.GitHooks) != 0 {
		t.Error("no workspace = no git hooks to clean")
	}
	if len(rpt.VSCodeEntries) > 1 {
		t.Errorf("no workspace = at most user-level VS Code entry, got %v", rpt.VSCodeEntries)
	}
}

func TestPurgeReport_TotalRemoved(t *testing.T) {
	rpt := PurgeReport{
		Skills:            []string{"a", "b"},
		Agents:            []string{"c"},
		InstructionBlocks: []string{"persona"},
		MCPEntries:        []string{"engram"},
		CopilotHooks:      []string{"guardrails"},
	}
	if got := rpt.TotalRemoved(); got != 6 {
		t.Errorf("TotalRemoved() = %d, want 6", got)
	}
}

// seedInstalled writes the test skill and agent catalogs to host and records
// them in state, so purge sees managed items to remove.
func seedInstalled(t *testing.T, host *copilot.Host, store *state.Store) {
	t.Helper()
	cat := testCatalog()
	agents := testAgentCatalog()

	for _, sk := range cat {
		dir := filepath.Join(host.SkillsDir, sk.Name)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte(sk.Content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.MkdirAll(host.AgentsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, a := range agents {
		if err := os.WriteFile(filepath.Join(host.AgentsDir, a.Name+".agent.md"), []byte(a.Content), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	recs := make([]state.Installed, len(cat))
	for i, sk := range cat {
		recs[i] = state.Installed{Name: sk.Name, Checksum: state.Checksum(sk.CanonicalContent())}
	}
	if err := store.Apply(Version, recs, nil); err != nil {
		t.Fatal(err)
	}
	agentRecs := make([]state.Installed, len(agents))
	for i, a := range agents {
		agentRecs[i] = state.Installed{Name: a.Name, Checksum: state.Checksum(a.CanonicalContent())}
	}
	if err := store.ApplyAgents(Version, agentRecs, nil); err != nil {
		t.Fatal(err)
	}
}
