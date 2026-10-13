package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/martinhg/capiko-ai/internal/copilot"
	"github.com/martinhg/capiko-ai/internal/state"
	"github.com/martinhg/capiko-ai/internal/tui"
)

func TestPurgeCommand_NotPurge(t *testing.T) {
	handled, _, _ := purgeCommand("other", nil, &bytes.Buffer{})
	if handled {
		t.Error("purgeCommand must not handle non-purge commands")
	}
}

func TestPurgeCommand_WithoutConfirm_ShowsPreview(t *testing.T) {
	orig := gatherPurgeInputs
	gatherPurgeInputs = func() (purgeInputs, error) {
		return purgeInputs{
			host:         &copilot.Host{ConfigDir: t.TempDir()},
			hostExitCode: 0,
		}, nil
	}
	t.Cleanup(func() { gatherPurgeInputs = orig })

	var buf bytes.Buffer
	handled, exitCode, err := purgeCommand("purge", nil, &buf)
	if !handled {
		t.Fatal("purgeCommand must handle 'purge'")
	}
	if exitCode != 0 {
		t.Errorf("exit code = %d, want 0", exitCode)
	}
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if !strings.Contains(buf.String(), "--confirm") {
		t.Error("preview must mention --confirm")
	}
}

func TestPurgeCommand_WithConfirm_ExecutesPurge(t *testing.T) {
	host := &copilot.Host{
		ConfigDir:     t.TempDir(),
		SkillsDir:     t.TempDir(),
		AgentsDir:     t.TempDir(),
		MCPConfigPath: filepath.Join(t.TempDir(), "mcp-config.json"),
		HooksDir:      filepath.Join(t.TempDir(), "hooks"),
	}
	store := state.NewStore(t.TempDir())
	workspace := t.TempDir()

	orig := gatherPurgeInputs
	gatherPurgeInputs = func() (purgeInputs, error) {
		return purgeInputs{
			host:         host,
			hostExitCode: 0,
			store:        store,
			workspace:    workspace,
		}, nil
	}
	t.Cleanup(func() { gatherPurgeInputs = orig })

	// Stub the tui seams so PurgeAll doesn't hit real git/home
	origGitDir := tui.ExportPurgeGitDir()
	origGitCommonDir := tui.ExportPurgeGitCommonDir()
	origVSCode := tui.ExportPurgeVSCodeUserPath()
	origHome := tui.ExportPurgeHomeDir()

	capikoHome := filepath.Join(t.TempDir(), ".capiko")
	if err := os.MkdirAll(capikoHome, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(capikoHome, "state.json"), []byte(`{}`), 0o644); err != nil {
		t.Fatal(err)
	}

	gitDir := filepath.Join(workspace, ".git")
	if err := os.MkdirAll(gitDir, 0o755); err != nil {
		t.Fatal(err)
	}

	tui.SetPurgeGitDir(func(string) (string, error) { return gitDir, nil })
	tui.SetPurgeGitCommonDir(func(string) (string, error) { return gitDir, nil })
	tui.SetPurgeVSCodeUserPath(func() (string, error) { return filepath.Join(t.TempDir(), "mcp.json"), nil })
	tui.SetPurgeHomeDir(func() (string, error) { return filepath.Dir(capikoHome), nil })
	t.Cleanup(func() {
		tui.SetPurgeGitDir(origGitDir)
		tui.SetPurgeGitCommonDir(origGitCommonDir)
		tui.SetPurgeVSCodeUserPath(origVSCode)
		tui.SetPurgeHomeDir(origHome)
	})

	var buf bytes.Buffer
	handled, exitCode, err := purgeCommand("purge", []string{"--confirm"}, &buf)
	if !handled {
		t.Fatal("purgeCommand must handle 'purge'")
	}
	if exitCode != 0 {
		t.Errorf("exit code = %d, want 0", exitCode)
	}
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if !strings.Contains(buf.String(), "removed") || !strings.Contains(buf.String(), "purge") {
		t.Errorf("output must show purge result, got: %s", buf.String())
	}
}

func TestPurgeCommand_JSON_Output(t *testing.T) {
	host := &copilot.Host{
		ConfigDir:     t.TempDir(),
		SkillsDir:     t.TempDir(),
		AgentsDir:     t.TempDir(),
		MCPConfigPath: filepath.Join(t.TempDir(), "mcp-config.json"),
		HooksDir:      filepath.Join(t.TempDir(), "hooks"),
	}
	store := state.NewStore(t.TempDir())
	workspace := t.TempDir()

	orig := gatherPurgeInputs
	gatherPurgeInputs = func() (purgeInputs, error) {
		return purgeInputs{
			host:         host,
			hostExitCode: 0,
			store:        store,
			workspace:    workspace,
		}, nil
	}
	t.Cleanup(func() { gatherPurgeInputs = orig })

	origGitDir := tui.ExportPurgeGitDir()
	origGitCommonDir := tui.ExportPurgeGitCommonDir()
	origVSCode := tui.ExportPurgeVSCodeUserPath()
	origHome := tui.ExportPurgeHomeDir()

	capikoHome := filepath.Join(t.TempDir(), ".capiko")
	if err := os.MkdirAll(capikoHome, 0o755); err != nil {
		t.Fatal(err)
	}

	gitDir := filepath.Join(workspace, ".git")
	if err := os.MkdirAll(gitDir, 0o755); err != nil {
		t.Fatal(err)
	}

	tui.SetPurgeGitDir(func(string) (string, error) { return gitDir, nil })
	tui.SetPurgeGitCommonDir(func(string) (string, error) { return gitDir, nil })
	tui.SetPurgeVSCodeUserPath(func() (string, error) { return filepath.Join(t.TempDir(), "mcp.json"), nil })
	tui.SetPurgeHomeDir(func() (string, error) { return filepath.Dir(capikoHome), nil })
	t.Cleanup(func() {
		tui.SetPurgeGitDir(origGitDir)
		tui.SetPurgeGitCommonDir(origGitCommonDir)
		tui.SetPurgeVSCodeUserPath(origVSCode)
		tui.SetPurgeHomeDir(origHome)
	})

	var buf bytes.Buffer
	_, exitCode, _ := purgeCommand("purge", []string{"--json", "--confirm"}, &buf)
	if exitCode != 0 {
		t.Errorf("exit code = %d, want 0", exitCode)
	}

	var result PurgeResult
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("JSON output must be valid: %v\noutput: %s", err, buf.String())
	}
	if !result.OK {
		t.Errorf("OK = false, want true")
	}
	if result.Command != "purge" {
		t.Errorf("Command = %q, want purge", result.Command)
	}
}

func TestPurgeCommand_CopilotNotFound_Exit2(t *testing.T) {
	orig := gatherPurgeInputs
	gatherPurgeInputs = func() (purgeInputs, error) {
		return purgeInputs{hostExitCode: 2}, nil
	}
	t.Cleanup(func() { gatherPurgeInputs = orig })

	var buf bytes.Buffer
	_, exitCode, _ := purgeCommand("purge", []string{"--confirm"}, &buf)
	if exitCode != 2 {
		t.Errorf("exit code = %d, want 2", exitCode)
	}
}

func TestPurgeCommand_UnknownArg_Error(t *testing.T) {
	var buf bytes.Buffer
	_, exitCode, err := purgeCommand("purge", []string{"--oops"}, &buf)
	if exitCode != 1 {
		t.Errorf("exit code = %d, want 1", exitCode)
	}
	if err == nil || !strings.Contains(err.Error(), "oops") {
		t.Errorf("expected error about unknown arg, got: %v", err)
	}
}

func TestPurgeCommand_PreviewJSON(t *testing.T) {
	orig := gatherPurgeInputs
	gatherPurgeInputs = func() (purgeInputs, error) {
		return purgeInputs{
			host:         &copilot.Host{ConfigDir: t.TempDir()},
			hostExitCode: 0,
		}, nil
	}
	t.Cleanup(func() { gatherPurgeInputs = orig })

	var buf bytes.Buffer
	_, exitCode, _ := purgeCommand("purge", []string{"--json"}, &buf)
	if exitCode != 0 {
		t.Errorf("exit code = %d, want 0", exitCode)
	}

	var result PurgeResult
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("preview JSON must be valid: %v\noutput: %s", err, buf.String())
	}
	if !strings.Contains(result.Error, "--confirm") {
		t.Error("preview JSON must mention --confirm")
	}
}
