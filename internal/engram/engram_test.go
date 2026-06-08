package engram

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestCloudCommandsShellOutWithExactArgs(t *testing.T) {
	orig := run
	t.Cleanup(func() { run = orig })
	var got []string
	run = func(args ...string) error { got = args; return nil }

	if err := CloudConfig("https://engram.example.com"); err != nil {
		t.Fatal(err)
	}
	if want := []string{"cloud", "config", "--server", "https://engram.example.com"}; !reflect.DeepEqual(got, want) {
		t.Errorf("CloudConfig args = %v, want %v", got, want)
	}

	if err := CloudEnroll("repo-core"); err != nil {
		t.Fatal(err)
	}
	if want := []string{"cloud", "enroll", "repo-core"}; !reflect.DeepEqual(got, want) {
		t.Errorf("CloudEnroll args = %v, want %v", got, want)
	}

	if err := CloudStatus(); err != nil {
		t.Fatal(err)
	}
	if want := []string{"cloud", "status"}; !reflect.DeepEqual(got, want) {
		t.Errorf("CloudStatus args = %v, want %v", got, want)
	}
}

func TestWriteServerScaffoldIsHardened(t *testing.T) {
	dir := t.TempDir()
	path, err := WriteServerScaffold(dir)
	if err != nil {
		t.Fatal(err)
	}
	if want := filepath.Join(dir, "docker-compose.cloud.yml"); path != want {
		t.Errorf("path = %q, want %q", path, want)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	s := string(data)
	if strings.Contains(s, `ENGRAM_CLOUD_INSECURE_NO_AUTH: "1"`) {
		t.Error("scaffold must not enable insecure no-auth mode")
	}
	if !strings.Contains(s, "ENGRAM_JWT_SECRET") {
		t.Error("scaffold should require a JWT secret")
	}
	if !strings.Contains(s, "cloud") {
		t.Error("scaffold should run engram cloud serve")
	}
}

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

func TestRemoveMCPEntryPreservesOthers(t *testing.T) {
	path := filepath.Join(t.TempDir(), "mcp-config.json")
	seed := `{"mcpServers":{"engram":{"command":"engram"},"github":{"command":"gh"}},"experimental":{"x":1}}`
	if err := os.WriteFile(path, []byte(seed), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := RemoveMCPEntry(path, "mcpServers", "engram"); err != nil {
		t.Fatal(err)
	}
	root := readJSON(t, path)
	servers := root["mcpServers"].(map[string]any)
	if _, ok := servers["engram"]; ok {
		t.Error("engram entry should be removed")
	}
	if _, ok := servers["github"]; !ok {
		t.Error("other servers must be preserved")
	}
	if _, ok := root["experimental"]; !ok {
		t.Error("unknown top-level keys must be preserved")
	}
}

func TestRemoveMCPEntryNoOpWhenAbsent(t *testing.T) {
	if err := RemoveMCPEntry(filepath.Join(t.TempDir(), "nope.json"), "mcpServers", "engram"); err != nil {
		t.Errorf("missing file should be a no-op, got %v", err)
	}
	path := filepath.Join(t.TempDir(), "mcp-config.json")
	if err := os.WriteFile(path, []byte(`{"mcpServers":{"github":{"command":"gh"}}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := RemoveMCPEntry(path, "mcpServers", "engram"); err != nil {
		t.Fatal(err)
	}
	if _, ok := readJSON(t, path)["mcpServers"].(map[string]any)["github"]; !ok {
		t.Error("github must survive a no-op remove")
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

func TestVSCodeEntryLeanShapeAndTokenReference(t *testing.T) {
	local := VSCodeEntry("")
	if local["command"] != "engram" {
		t.Errorf("command = %v, want engram", local["command"])
	}
	if _, ok := local["type"]; ok {
		t.Error("VS Code entry should not carry the Copilot-CLI 'type' field")
	}
	if _, ok := local["env"]; ok {
		t.Error("local-only VS Code entry should not carry cloud env")
	}

	t.Setenv("ENGRAM_CLOUD_TOKEN", "super-secret-value-123")
	cloud := VSCodeEntry("https://engram.example.com")
	env := cloud["env"].(map[string]string)
	if env["ENGRAM_CLOUD_TOKEN"] != "${ENGRAM_CLOUD_TOKEN}" {
		t.Errorf("token = %q, want the ${ENGRAM_CLOUD_TOKEN} reference", env["ENGRAM_CLOUD_TOKEN"])
	}
	if blob, _ := json.Marshal(cloud); bytes.Contains(blob, []byte("super-secret-value-123")) {
		t.Fatal("the literal token leaked into the VS Code entry")
	}
}

func TestMergeMCPEntryServersKeyPreservesOthers(t *testing.T) {
	path := filepath.Join(t.TempDir(), "mcp.json")
	if err := os.WriteFile(path, []byte(`{"servers":{"other":{"command":"x"}}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := MergeMCPEntry(path, "servers", "engram", VSCodeEntry("")); err != nil {
		t.Fatal(err)
	}
	servers := readJSON(t, path)["servers"].(map[string]any)
	if _, ok := servers["other"]; !ok {
		t.Error("an existing server under the servers key must be preserved")
	}
	if _, ok := servers["engram"]; !ok {
		t.Error("engram must be added under the servers key")
	}
}

func TestVSCodeUserMCPPath(t *testing.T) {
	origHome, origGOOS := userHomeDir, goos
	t.Cleanup(func() { userHomeDir, goos = origHome, origGOOS })
	userHomeDir = func() (string, error) { return "/home/u", nil }

	goos = "darwin"
	if p, err := VSCodeUserMCPPath(); err != nil || p != filepath.Join("/home/u", "Library", "Application Support", "Code", "User", "mcp.json") {
		t.Errorf("darwin path = %q (err %v)", p, err)
	}

	goos = "linux"
	if p, err := VSCodeUserMCPPath(); err != nil || p != filepath.Join("/home/u", ".config", "Code", "User", "mcp.json") {
		t.Errorf("linux path = %q (err %v)", p, err)
	}

	goos = "windows"
	t.Setenv("APPDATA", "/appdata")
	if p, err := VSCodeUserMCPPath(); err != nil || p != filepath.Join("/appdata", "Code", "User", "mcp.json") {
		t.Errorf("windows path = %q (err %v)", p, err)
	}
}

func TestEntryChecksumStableAndDistinct(t *testing.T) {
	local := EntryChecksum(CopilotCLIEntry(""))
	if local != EntryChecksum(CopilotCLIEntry("")) {
		t.Error("checksum should be deterministic")
	}
	if local == EntryChecksum(CopilotCLIEntry("https://engram.example.com")) {
		t.Error("cloud and local entries should checksum differently")
	}
}

func TestCLIEntryChecksumMatchesWritten(t *testing.T) {
	path := filepath.Join(t.TempDir(), "mcp-config.json")
	entry := CopilotCLIEntry("https://engram.example.com")
	if err := MergeMCPEntry(path, "mcpServers", "engram", entry); err != nil {
		t.Fatal(err)
	}
	got, ok := CLIEntryChecksum(path)
	if !ok {
		t.Fatal("should read the engram entry back")
	}
	if got != EntryChecksum(entry) {
		t.Errorf("on-disk checksum %q != expected %q", got, EntryChecksum(entry))
	}
}

func TestCLIEntryChecksumAbsent(t *testing.T) {
	if _, ok := CLIEntryChecksum(filepath.Join(t.TempDir(), "nope.json")); ok {
		t.Error("missing file should report not ok")
	}
	path := filepath.Join(t.TempDir(), "mcp-config.json")
	if err := os.WriteFile(path, []byte(`{"mcpServers":{"github":{"command":"gh"}}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, ok := CLIEntryChecksum(path); ok {
		t.Error("a config without an engram entry should report not ok")
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
