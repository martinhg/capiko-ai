// Package engram configures the engram cross-session memory backend for the
// Copilot host: it writes engram's MCP server entry into the host's MCP config
// (merging, never clobbering other servers) and the per-repo project config that
// scopes memories correctly in multi-repo workspaces.
//
// It never persists secrets: the cloud token is written only as the
// ${ENGRAM_CLOUD_TOKEN} reference, resolved from the environment at runtime.
package engram

import (
	"bytes"
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
)

// Test seams for OS-specific path resolution.
var (
	userHomeDir = os.UserHomeDir
	goos        = runtime.GOOS
)

// VSCodeUserMCPPath returns the OS-specific user-level VS Code MCP config path
// (Code/User/mcp.json), which applies to every VS Code window — unlike the
// workspace-level .vscode/mcp.json.
func VSCodeUserMCPPath() (string, error) {
	home, err := userHomeDir()
	if err != nil {
		return "", err
	}
	switch goos {
	case "darwin":
		return filepath.Join(home, "Library", "Application Support", "Code", "User", "mcp.json"), nil
	case "windows":
		appData := os.Getenv("APPDATA")
		if appData == "" {
			appData = filepath.Join(home, "AppData", "Roaming")
		}
		return filepath.Join(appData, "Code", "User", "mcp.json"), nil
	default: // linux and other unix
		return filepath.Join(home, ".config", "Code", "User", "mcp.json"), nil
	}
}

// tokenRef is the reference capiko writes for the cloud token. The real value is
// resolved from the environment by the engram process, never stored on disk.
const tokenRef = "${ENGRAM_CLOUD_TOKEN}"

// DefaultMode is the team default artifact-store mode: canonical specs in git,
// memory and artifacts replicated through Engram Cloud.
const DefaultMode = "hybrid"

// Modes are the per-change artifact-store modes, in display order.
var Modes = []string{"hybrid", "engram", "openspec", "none"}

// run executes an engram subcommand. It is a test seam.
var run = func(args ...string) error {
	return exec.Command("engram", args...).Run()
}

// CloudConfig points the local engram at the team's cloud server. The URL is
// persisted by engram to ~/.engram/cloud.json.
func CloudConfig(server string) error { return run("cloud", "config", "--server", server) }

// CloudEnroll enrolls a project for cloud replication.
func CloudEnroll(project string) error { return run("cloud", "enroll", project) }

// CloudStatus prints the cloud configuration and daemon health.
func CloudStatus() error { return run("cloud", "status") }

//go:embed templates/docker-compose.cloud.yml
var serverScaffold string

// WriteServerScaffold writes a hardened Engram Cloud docker-compose template into
// dir for the team's devops to adapt. capiko configures the client side and ships
// this scaffold; it never runs the server itself.
func WriteServerScaffold(dir string) (string, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	path := filepath.Join(dir, "docker-compose.cloud.yml")
	if err := os.WriteFile(path, []byte(serverScaffold), 0o644); err != nil {
		return "", err
	}
	return path, nil
}

// cloudEnv is the engram MCP process environment for cloud replication: the
// server URL, autosync on, and the token as a reference (never the literal value).
func cloudEnv(server string) map[string]string {
	return map[string]string{
		"ENGRAM_CLOUD_SERVER":   server,
		"ENGRAM_CLOUD_AUTOSYNC": "1",
		"ENGRAM_CLOUD_TOKEN":    tokenRef,
	}
}

// CopilotCLIEntry builds the engram MCP server entry for Copilot CLI's
// mcp-config.json (top-level key "mcpServers"). When cloudServer is non-empty the
// entry carries the cloud env.
func CopilotCLIEntry(cloudServer string) map[string]any {
	entry := map[string]any{
		"type":    "local",
		"command": "engram",
		"args":    []string{"mcp"},
		"tools":   []string{"*"},
	}
	if cloudServer != "" {
		entry["env"] = cloudEnv(cloudServer)
	}
	return entry
}

// VSCodeEntry builds the engram MCP server entry for VS Code's mcp.json
// (top-level key "servers"), which uses a leaner shape than Copilot CLI — no
// "type"/"tools" fields. When cloudServer is non-empty it carries the cloud env.
func VSCodeEntry(cloudServer string) map[string]any {
	entry := map[string]any{
		"command": "engram",
		"args":    []string{"mcp"},
	}
	if cloudServer != "" {
		entry["env"] = cloudEnv(cloudServer)
	}
	return entry
}

// MergeMCPEntry sets or updates the named server entry under topKey in the JSON
// config at path, preserving every other top-level key and every other server. A
// missing file is created; the write is atomic.
func MergeMCPEntry(path, topKey, name string, entry any) error {
	root := map[string]json.RawMessage{}
	switch data, err := os.ReadFile(path); {
	case err == nil:
		if len(bytes.TrimSpace(data)) > 0 {
			if err := json.Unmarshal(data, &root); err != nil {
				return fmt.Errorf("parse %s: %w", path, err)
			}
		}
	case !os.IsNotExist(err):
		return err
	}

	servers := map[string]json.RawMessage{}
	if raw, ok := root[topKey]; ok {
		if err := json.Unmarshal(raw, &servers); err != nil {
			return fmt.Errorf("parse %q in %s: %w", topKey, path, err)
		}
	}
	entryJSON, err := json.Marshal(entry)
	if err != nil {
		return err
	}
	servers[name] = entryJSON

	serversJSON, err := json.Marshal(servers)
	if err != nil {
		return err
	}
	root[topKey] = serversJSON

	out, err := json.MarshalIndent(root, "", "  ")
	if err != nil {
		return err
	}
	return atomicWrite(path, out)
}

// EntryChecksum returns a canonical SHA-256 of an MCP entry. json.Marshal sorts
// map keys, so the same logical entry always yields the same checksum regardless
// of how it was built.
func EntryChecksum(entry any) string {
	b, _ := json.Marshal(entry)
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

// CLIEntryChecksum reads the engram entry from a Copilot CLI mcp-config.json and
// returns its canonical checksum. ok is false when the file, the mcpServers key,
// or the engram entry is absent or unreadable.
func CLIEntryChecksum(mcpConfigPath string) (checksum string, ok bool) {
	return MCPEntryChecksum(mcpConfigPath, "engram")
}

// MCPEntryChecksum reads the named server entry under "mcpServers" from a Copilot
// CLI mcp-config.json and returns its canonical checksum. ok is false when the
// file, the mcpServers key, or the named entry is absent or unreadable. It is the
// generic primitive other managed MCP servers (engram, headroom) compare against.
func MCPEntryChecksum(mcpConfigPath, name string) (checksum string, ok bool) {
	data, err := os.ReadFile(mcpConfigPath)
	if err != nil {
		return "", false
	}
	var root map[string]json.RawMessage
	if json.Unmarshal(data, &root) != nil {
		return "", false
	}
	raw, ok := root["mcpServers"]
	if !ok {
		return "", false
	}
	var servers map[string]json.RawMessage
	if json.Unmarshal(raw, &servers) != nil {
		return "", false
	}
	entryRaw, ok := servers[name]
	if !ok {
		return "", false
	}
	var entry any
	if json.Unmarshal(entryRaw, &entry) != nil {
		return "", false
	}
	return EntryChecksum(entry), true
}

// RemoveMCPEntry deletes the named server entry under topKey from the JSON config
// at path, preserving every other top-level key and every other server. A missing
// file, key, or entry is a no-op. The write is atomic.
func RemoveMCPEntry(path, topKey, name string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	root := map[string]json.RawMessage{}
	if len(bytes.TrimSpace(data)) > 0 {
		if err := json.Unmarshal(data, &root); err != nil {
			return fmt.Errorf("parse %s: %w", path, err)
		}
	}
	raw, ok := root[topKey]
	if !ok {
		return nil
	}
	servers := map[string]json.RawMessage{}
	if err := json.Unmarshal(raw, &servers); err != nil {
		return fmt.Errorf("parse %q in %s: %w", topKey, path, err)
	}
	if _, ok := servers[name]; !ok {
		return nil
	}
	delete(servers, name)
	serversJSON, err := json.Marshal(servers)
	if err != nil {
		return err
	}
	root[topKey] = serversJSON
	out, err := json.MarshalIndent(root, "", "  ")
	if err != nil {
		return err
	}
	return atomicWrite(path, out)
}

type projectConfig struct {
	ProjectName string `json:"project_name"`
}

// WriteProjectConfig writes <repoRoot>/.engram/config.json = {"project_name": name}
// so engram attributes memories to the right project in parent-folder multi-repo
// workspaces. It is idempotent: a config already naming this project is left
// untouched (no spurious rewrite).
func WriteProjectConfig(repoRoot, name string) error {
	path := filepath.Join(repoRoot, ".engram", "config.json")
	if data, err := os.ReadFile(path); err == nil {
		var got projectConfig
		if json.Unmarshal(data, &got) == nil && got.ProjectName == name {
			return nil
		}
	}
	out, err := json.MarshalIndent(projectConfig{ProjectName: name}, "", "  ")
	if err != nil {
		return err
	}
	return atomicWrite(path, out)
}

// ReadProjectName returns the project name recorded in
// <repoRoot>/.engram/config.json ("project_name" key). Falls back to
// filepath.Base(repoRoot) when the file is absent, unreadable, malformed, or
// contains an empty project_name. It shares the projectConfig struct with
// WriteProjectConfig so schema changes stay in one place.
func ReadProjectName(repoRoot string) string {
	path := filepath.Join(repoRoot, ".engram", "config.json")
	if data, err := os.ReadFile(path); err == nil {
		var cfg projectConfig
		if json.Unmarshal(data, &cfg) == nil && cfg.ProjectName != "" {
			return cfg.ProjectName
		}
	}
	return filepath.Base(repoRoot)
}

// atomicWrite creates the parent directory and writes data via a temp file +
// rename, so a crash mid-write cannot leave a partial config.
func atomicWrite(path string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}
