package engram

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func readJSON(t *testing.T, path string) map[string]any {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatalf("unmarshal %s: %v", path, err)
	}
	return m
}

func TestMergeMCPEntryCreatesFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "mcp-config.json")
	if err := MergeMCPEntry(path, "mcpServers", "engram", CopilotCLIEntry("")); err != nil {
		t.Fatal(err)
	}
	servers := readJSON(t, path)["mcpServers"].(map[string]any)
	entry := servers["engram"].(map[string]any)
	if entry["type"] != "local" || entry["command"] != "engram" {
		t.Errorf("engram entry = %+v, want type local / command engram", entry)
	}
}

func TestMergeMCPEntryPreservesOthers(t *testing.T) {
	path := filepath.Join(t.TempDir(), "mcp-config.json")
	seed := `{
  "mcpServers": { "github": { "type": "local", "command": "gh" } },
  "experimental": { "foo": true }
}`
	if err := os.WriteFile(path, []byte(seed), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := MergeMCPEntry(path, "mcpServers", "engram", CopilotCLIEntry("")); err != nil {
		t.Fatal(err)
	}
	root := readJSON(t, path)
	servers := root["mcpServers"].(map[string]any)
	if _, ok := servers["github"]; !ok {
		t.Error("existing github server must be preserved")
	}
	if _, ok := servers["engram"]; !ok {
		t.Error("engram server must be added")
	}
	if _, ok := root["experimental"]; !ok {
		t.Error("unknown top-level keys must be preserved")
	}
}

func TestCopilotCLIEntryNeverWritesLiteralToken(t *testing.T) {
	// Even with a real-looking secret in the environment, capiko writes only the
	// ${ENGRAM_CLOUD_TOKEN} reference, never the value.
	t.Setenv("ENGRAM_CLOUD_TOKEN", "super-secret-value-123")
	entry := CopilotCLIEntry("https://engram.example.com")
	env := entry["env"].(map[string]string)
	if env["ENGRAM_CLOUD_TOKEN"] != "${ENGRAM_CLOUD_TOKEN}" {
		t.Errorf("token = %q, want the ${ENGRAM_CLOUD_TOKEN} reference", env["ENGRAM_CLOUD_TOKEN"])
	}
	if env["ENGRAM_CLOUD_SERVER"] != "https://engram.example.com" {
		t.Errorf("server = %q, want the cloud URL", env["ENGRAM_CLOUD_SERVER"])
	}
	if env["ENGRAM_CLOUD_AUTOSYNC"] != "1" {
		t.Error("autosync should be enabled when cloud is configured")
	}
	if blob, _ := json.Marshal(entry); bytes.Contains(blob, []byte("super-secret-value-123")) {
		t.Fatal("the literal token leaked into the MCP entry")
	}
}

func TestCopilotCLIEntryLocalOnlyHasNoCloudEnv(t *testing.T) {
	if _, ok := CopilotCLIEntry("")["env"]; ok {
		t.Error("a local-only entry should not carry cloud env")
	}
}

func TestWriteProjectConfig(t *testing.T) {
	root := t.TempDir()
	if err := WriteProjectConfig(root, "repo-core"); err != nil {
		t.Fatal(err)
	}
	got := readJSON(t, filepath.Join(root, ".engram", "config.json"))
	if got["project_name"] != "repo-core" {
		t.Errorf("project_name = %v, want repo-core", got["project_name"])
	}
}

func TestWriteProjectConfigIdempotent(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, ".engram", "config.json")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	// Already correct, plus a marker the writer would drop if it rewrote.
	if err := os.WriteFile(path, []byte(`{"project_name":"repo-core","_keep":true}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := WriteProjectConfig(root, "repo-core"); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(path)
	if !bytes.Contains(data, []byte("_keep")) {
		t.Error("an already-correct config must not be rewritten")
	}
}

func TestWriteProjectConfigUpdatesWrongName(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, ".engram", "config.json")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(`{"project_name":"old"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := WriteProjectConfig(root, "new"); err != nil {
		t.Fatal(err)
	}
	if got := readJSON(t, path)["project_name"]; got != "new" {
		t.Errorf("project_name = %v, want new (a wrong name should be corrected)", got)
	}
}
