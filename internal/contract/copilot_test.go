// Package contract validates that every artifact capiko writes to the Copilot
// CLI host conforms to the Copilot CLI v1 schema contract. It is the single
// source of truth for "what fields does Copilot CLI recognise" — individual
// packages test rendering correctness; this package tests schema compliance.
//
// Pinned against: GitHub Copilot CLI 1.0.60 (versions.CopilotCLI).
package contract_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/martinhg/capiko-ai/internal/agent"
	"github.com/martinhg/capiko-ai/internal/copilothooks"
	"github.com/martinhg/capiko-ai/internal/engram"
	"github.com/martinhg/capiko-ai/internal/headroom"
	"github.com/martinhg/capiko-ai/internal/skill"
	"gopkg.in/yaml.v3"
)

// ---------------------------------------------------------------------------
// Copilot CLI v1 SKILL.md frontmatter contract
// ---------------------------------------------------------------------------

// knownSkillFrontmatterKeys is the exhaustive set of YAML keys the Copilot CLI
// v1 SKILL.md parser recognises. An unknown key is silently ignored by Copilot
// but is a capiko authoring bug — it signals the author intended a field that
// has no effect, or the field name drifted from the schema.
var knownSkillFrontmatterKeys = map[string]bool{
	"name":                     true,
	"description":              true,
	"license":                  true,
	"metadata":                 true,
	"depends_on":               true,
	"disable-model-invocation": true,
	"user-invocable":           true,
}

// knownSkillMetadataKeys lists valid subfields under the "metadata" key.
var knownSkillMetadataKeys = map[string]bool{
	"author":  true,
	"version": true,
}

func TestSkillCatalog_CopilotCLIContract(t *testing.T) {
	catalog := loadSkillCatalog(t)
	if len(catalog) == 0 {
		t.Fatal("embedded skill catalog is empty")
	}

	for _, sk := range catalog {
		sk := sk
		t.Run(sk.Name, func(t *testing.T) {
			fm := parseRawFrontmatter(t, sk.Content)

			// Only known keys.
			for key := range fm {
				if !knownSkillFrontmatterKeys[key] {
					t.Errorf("unknown frontmatter key %q — Copilot CLI ignores it; remove or fix the field name", key)
				}
			}

			// Required: description.
			desc, ok := fm["description"]
			if !ok {
				t.Fatal("missing required key: description")
			}
			if s, _ := desc.(string); strings.TrimSpace(s) == "" {
				t.Error("description must be non-empty")
			}

			// Metadata subfields must be known.
			if meta, ok := fm["metadata"]; ok {
				metaMap, ok := meta.(map[string]any)
				if !ok {
					t.Fatalf("metadata must be a map, got %T", meta)
				}
				for key := range metaMap {
					if !knownSkillMetadataKeys[key] {
						t.Errorf("unknown metadata key %q", key)
					}
				}
			}

			// depends_on must be a list of strings.
			if deps, ok := fm["depends_on"]; ok {
				depList, ok := deps.([]any)
				if !ok {
					t.Fatalf("depends_on must be a list, got %T", deps)
				}
				for _, d := range depList {
					if _, ok := d.(string); !ok {
						t.Errorf("depends_on entry must be a string, got %T", d)
					}
				}
			}

			// Body after frontmatter must be non-empty.
			body := extractBody(sk.Content)
			if strings.TrimSpace(body) == "" {
				t.Error("SKILL.md body after frontmatter is empty — Copilot loads it as the skill instruction")
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Copilot CLI v1 .agent.md frontmatter contract
// ---------------------------------------------------------------------------

// knownAgentFrontmatterKeys is the exhaustive set of YAML keys the Copilot CLI
// v1 .agent.md parser recognises. An unknown key is a capiko authoring bug.
var knownAgentFrontmatterKeys = map[string]bool{
	"description":    true,
	"tools":          true,
	"user-invocable": true,
	"agents":         true,
	"target":         true,
	"model":          true,
}

// allowedAgentTools is the set of tool aliases Copilot CLI v1 recognises in the
// "tools" frontmatter field.
var allowedAgentTools = map[string]bool{
	"read": true, "edit": true, "search": true, "execute": true, "agent": true,
}

func TestAgentCatalog_CopilotCLIContract(t *testing.T) {
	catalog := loadAgentCatalog(t)
	if len(catalog) == 0 {
		t.Fatal("embedded agent catalog is empty")
	}

	agentNames := make(map[string]bool, len(catalog))
	for _, a := range catalog {
		agentNames[a.Name] = true
	}

	for _, a := range catalog {
		a := a
		t.Run(a.Name, func(t *testing.T) {
			fm := parseRawFrontmatter(t, a.Content)

			// Only known keys.
			for key := range fm {
				if !knownAgentFrontmatterKeys[key] {
					t.Errorf("unknown frontmatter key %q — Copilot CLI ignores it; remove or fix the field name", key)
				}
			}

			// Required: description.
			desc, ok := fm["description"]
			if !ok {
				t.Fatal("missing required key: description")
			}
			if s, _ := desc.(string); strings.TrimSpace(s) == "" {
				t.Error("description must be non-empty")
			}

			// tools must be a list of known aliases.
			if rawTools, ok := fm["tools"]; ok {
				toolList, ok := rawTools.([]any)
				if !ok {
					t.Fatalf("tools must be a list, got %T", rawTools)
				}
				for _, tool := range toolList {
					ts, ok := tool.(string)
					if !ok {
						t.Errorf("tools entry must be a string, got %T", tool)
						continue
					}
					if !allowedAgentTools[ts] {
						t.Errorf("tools entry %q is not a valid Copilot CLI tool alias", ts)
					}
				}
			}

			// agents must be a list of strings referencing real catalog agents.
			if rawAgents, ok := fm["agents"]; ok {
				agentList, ok := rawAgents.([]any)
				if !ok {
					t.Fatalf("agents must be a list, got %T", rawAgents)
				}
				for _, ref := range agentList {
					name, ok := ref.(string)
					if !ok {
						t.Errorf("agents entry must be a string, got %T", ref)
						continue
					}
					if !agentNames[name] {
						t.Errorf("agents entry %q references a non-existent agent", name)
					}
				}
			}

			// user-invocable must be a boolean when present.
			if rawUI, ok := fm["user-invocable"]; ok {
				if _, ok := rawUI.(bool); !ok {
					t.Errorf("user-invocable must be a boolean, got %T (%v)", rawUI, rawUI)
				}
			}

			// model must be a string when present.
			if rawModel, ok := fm["model"]; ok {
				if _, ok := rawModel.(string); !ok {
					t.Errorf("model must be a string, got %T (%v)", rawModel, rawModel)
				}
			}

			// Body after frontmatter must be non-empty.
			body := extractBody(a.Content)
			if strings.TrimSpace(body) == "" {
				t.Error(".agent.md body after frontmatter is empty — Copilot loads it as the agent instruction")
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Copilot CLI v1 mcp-config.json entry contract
// ---------------------------------------------------------------------------

// knownMCPEntryKeys is the exhaustive set of JSON keys the Copilot CLI v1
// MCP server entry shape recognises under "mcpServers".
var knownMCPEntryKeys = map[string]bool{
	"type":    true,
	"command": true,
	"args":    true,
	"tools":   true,
	"env":     true,
}

func TestEngramMCPEntry_CopilotCLIContract(t *testing.T) {
	for _, tc := range []struct {
		name        string
		entry       map[string]any
		expectCloud bool
	}{
		{"local-only", engram.CopilotCLIEntry(""), false},
		{"with-cloud", engram.CopilotCLIEntry("https://engram.example.com"), true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			validateMCPEntry(t, tc.entry, tc.expectCloud)
		})
	}
}

func TestHeadroomMCPEntry_CopilotCLIContract(t *testing.T) {
	validateMCPEntry(t, headroom.CopilotCLIEntry(), false)
}

func validateMCPEntry(t *testing.T, entry map[string]any, expectEnv bool) {
	t.Helper()

	// Marshal and unmarshal to normalise types (map[string]any keys).
	blob, err := json.Marshal(entry)
	if err != nil {
		t.Fatalf("marshal entry: %v", err)
	}
	var normalised map[string]any
	if err := json.Unmarshal(blob, &normalised); err != nil {
		t.Fatalf("unmarshal entry: %v", err)
	}

	// Only known keys.
	for key := range normalised {
		if !knownMCPEntryKeys[key] {
			t.Errorf("unknown MCP entry key %q — Copilot CLI ignores it", key)
		}
	}

	// Required: type must be "local".
	if typ, ok := normalised["type"]; !ok {
		t.Error("missing required key: type")
	} else if typ != "local" {
		t.Errorf("type = %v, want \"local\"", typ)
	}

	// Required: command must be a non-empty string.
	if cmd, ok := normalised["command"]; !ok {
		t.Error("missing required key: command")
	} else if s, _ := cmd.(string); s == "" {
		t.Error("command must be non-empty")
	}

	// Required: args must be a non-empty list of strings.
	rawArgs, ok := normalised["args"]
	if !ok {
		t.Error("missing required key: args")
	} else {
		argList, ok := rawArgs.([]any)
		if !ok {
			t.Fatalf("args must be a list, got %T", rawArgs)
		}
		if len(argList) == 0 {
			t.Error("args must be non-empty")
		}
		for _, a := range argList {
			if _, ok := a.(string); !ok {
				t.Errorf("args entry must be a string, got %T", a)
			}
		}
	}

	// tools, when present, must be a list of strings.
	if rawTools, ok := normalised["tools"]; ok {
		toolList, ok := rawTools.([]any)
		if !ok {
			t.Fatalf("tools must be a list, got %T", rawTools)
		}
		for _, tool := range toolList {
			if _, ok := tool.(string); !ok {
				t.Errorf("tools entry must be a string, got %T", tool)
			}
		}
	}

	// env, when present, must be a map of string to string.
	rawEnv, hasEnv := normalised["env"]
	if expectEnv && !hasEnv {
		t.Error("expected env to be present (cloud configured)")
	}
	if hasEnv {
		envMap, ok := rawEnv.(map[string]any)
		if !ok {
			t.Fatalf("env must be a map, got %T", rawEnv)
		}
		for k, v := range envMap {
			if _, ok := v.(string); !ok {
				t.Errorf("env[%q] must be a string, got %T", k, v)
			}
		}
	}
}

// ---------------------------------------------------------------------------
// Copilot CLI v1 hook file contract
// ---------------------------------------------------------------------------

// knownHookFileKeys is the exhaustive set of JSON keys the Copilot CLI v1 hook
// file recognises at the top level.
var knownHookFileKeys = map[string]bool{
	"version": true,
	"hooks":   true,
}

// knownHookEntryKeys is the exhaustive set of JSON keys per hook entry.
var knownHookEntryKeys = map[string]bool{
	"type":       true,
	"matcher":    true,
	"bash":       true,
	"powershell": true,
	"timeoutSec": true,
}

// knownHookEvents is the set of hook event names Copilot CLI v1 recognises.
var knownHookEvents = map[string]bool{
	"preToolUse":   true,
	"postToolUse":  true,
	"sessionStart": true,
	"sessionEnd":   true,
}

func TestHookFiles_CopilotCLIContract(t *testing.T) {
	hookFiles := []struct {
		name string
		hf   copilothooks.HookFile
	}{
		{"guardrails-strict", mustRenderGuardrails(t, copilothooks.PostureStrict)},
		{"guardrails-warn", mustRenderGuardrails(t, copilothooks.PostureWarn)},
		{"session-check", copilothooks.RenderSessionCheck()},
		{"session-probe", copilothooks.RenderSessionProbe()},
	}

	for _, tc := range hookFiles {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			data, err := copilothooks.Marshal(tc.hf)
			if err != nil {
				t.Fatalf("Marshal: %v", err)
			}

			var raw map[string]json.RawMessage
			if err := json.Unmarshal(data, &raw); err != nil {
				t.Fatalf("unmarshal hook file: %v", err)
			}

			// Only known top-level keys.
			for key := range raw {
				if !knownHookFileKeys[key] {
					t.Errorf("unknown top-level key %q in hook file", key)
				}
			}

			// version must be present.
			if _, ok := raw["version"]; !ok {
				t.Error("missing required key: version")
			}

			// hooks must be present and contain only known events.
			hooksRaw, ok := raw["hooks"]
			if !ok {
				t.Fatal("missing required key: hooks")
			}
			var hooks map[string]json.RawMessage
			if err := json.Unmarshal(hooksRaw, &hooks); err != nil {
				t.Fatalf("unmarshal hooks: %v", err)
			}
			for event, entriesRaw := range hooks {
				if !knownHookEvents[event] {
					t.Errorf("unknown hook event %q", event)
				}

				var entries []map[string]json.RawMessage
				if err := json.Unmarshal(entriesRaw, &entries); err != nil {
					t.Fatalf("unmarshal hooks[%s]: %v", event, err)
				}
				for i, entry := range entries {
					for key := range entry {
						if !knownHookEntryKeys[key] {
							t.Errorf("hooks[%s][%d]: unknown entry key %q", event, i, key)
						}
					}
				}
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Cross-artifact: agent → skill reference integrity
// ---------------------------------------------------------------------------

func TestAgentSkillReferences_Resolvable(t *testing.T) {
	agents := loadAgentCatalog(t)
	skills := loadSkillCatalog(t)

	skillNames := make(map[string]bool, len(skills))
	for _, s := range skills {
		skillNames[s.Name] = true
	}

	for _, a := range agents {
		body := extractBody(a.Content)
		refs := extractSkillPaths(body)
		for _, ref := range refs {
			if !skillNames[ref] {
				t.Errorf("agent %q references skill %q via ~/.copilot/skills/%s/SKILL.md, but it is not in the catalog", a.Name, ref, ref)
			}
		}
	}
}

// ---------------------------------------------------------------------------
// Cross-artifact: mcp-config round-trip through MergeMCPEntry
// ---------------------------------------------------------------------------

func TestMCPEntry_MergeRoundTrip_PreservesContract(t *testing.T) {
	path := filepath.Join(t.TempDir(), "mcp-config.json")

	entries := []struct {
		name  string
		entry map[string]any
	}{
		{"engram", engram.CopilotCLIEntry("")},
		{"headroom", headroom.CopilotCLIEntry()},
	}

	for _, e := range entries {
		if err := engram.MergeMCPEntry(path, "mcpServers", e.name, e.entry); err != nil {
			t.Fatalf("MergeMCPEntry(%s): %v", e.name, err)
		}
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read mcp-config: %v", err)
	}

	var root map[string]json.RawMessage
	if err := json.Unmarshal(data, &root); err != nil {
		t.Fatalf("unmarshal mcp-config: %v", err)
	}

	// The file must have exactly mcpServers at the top level.
	for key := range root {
		if key != "mcpServers" {
			t.Errorf("unexpected top-level key %q in mcp-config.json (capiko should only write mcpServers)", key)
		}
	}

	var servers map[string]json.RawMessage
	if err := json.Unmarshal(root["mcpServers"], &servers); err != nil {
		t.Fatalf("unmarshal mcpServers: %v", err)
	}

	for name, entryRaw := range servers {
		var entryMap map[string]any
		if err := json.Unmarshal(entryRaw, &entryMap); err != nil {
			t.Fatalf("unmarshal entry %q: %v", name, err)
		}
		for key := range entryMap {
			if !knownMCPEntryKeys[key] {
				t.Errorf("mcpServers[%s]: unknown key %q after round-trip through MergeMCPEntry", name, key)
			}
		}
	}
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func loadSkillCatalog(t *testing.T) []skill.Skill {
	t.Helper()
	dir := filepath.Join("..", "catalog", "skills")
	fsys := os.DirFS(dir)
	catalog, err := skill.LoadCatalog(fsys)
	if err != nil {
		t.Fatalf("LoadCatalog: %v", err)
	}
	return catalog
}

func loadAgentCatalog(t *testing.T) []agent.Agent {
	t.Helper()
	dir := filepath.Join("..", "catalog", "agents")
	fsys := os.DirFS(dir)
	agents, err := agent.LoadCatalog(fsys)
	if err != nil {
		t.Fatalf("LoadCatalog (agents): %v", err)
	}
	return agents
}

// parseRawFrontmatter extracts the YAML frontmatter block and unmarshals it
// into map[string]any to detect unknown keys that a typed struct would silently
// discard.
func parseRawFrontmatter(t *testing.T, content string) map[string]any {
	t.Helper()
	lines := strings.Split(content, "\n")
	if len(lines) == 0 || strings.TrimSpace(lines[0]) != "---" {
		t.Fatal("missing frontmatter opening ---")
	}
	var body []string
	closed := false
	for _, l := range lines[1:] {
		if strings.TrimSpace(l) == "---" {
			closed = true
			break
		}
		body = append(body, l)
	}
	if !closed {
		t.Fatal("unterminated frontmatter")
	}
	var m map[string]any
	if err := yaml.Unmarshal([]byte(strings.Join(body, "\n")), &m); err != nil {
		t.Fatalf("parse frontmatter: %v", err)
	}
	return m
}

// extractBody returns the content after the closing --- frontmatter delimiter.
func extractBody(content string) string {
	lines := strings.Split(content, "\n")
	inFM := false
	for i, l := range lines {
		trimmed := strings.TrimSpace(l)
		if trimmed == "---" {
			if !inFM {
				inFM = true
				continue
			}
			return strings.Join(lines[i+1:], "\n")
		}
	}
	return ""
}

// extractSkillPaths finds ~/.copilot/skills/<name>/SKILL.md references in body
// and returns the <name> parts.
func extractSkillPaths(body string) []string {
	const prefix = "~/.copilot/skills/"
	const suffix = "/SKILL.md"
	var names []string
	seen := map[string]bool{}
	for {
		idx := strings.Index(body, prefix)
		if idx < 0 {
			break
		}
		rest := body[idx+len(prefix):]
		end := strings.Index(rest, suffix)
		if end < 0 {
			break
		}
		name := rest[:end]
		if name != "" && !seen[name] {
			names = append(names, name)
			seen[name] = true
		}
		body = rest[end+len(suffix):]
	}
	sort.Strings(names)
	return names
}

func mustRenderGuardrails(t *testing.T, posture copilothooks.Posture) copilothooks.HookFile {
	t.Helper()
	hf, err := copilothooks.RenderGuardrails(posture)
	if err != nil {
		t.Fatalf("RenderGuardrails(%s): %v", posture, err)
	}
	return hf
}
