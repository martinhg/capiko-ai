package agent_test

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/martinhg/capiko-ai/internal/agent"
)

// --- Phase 1.1: Domain type tests ---

func TestLoadCatalog_ReturnsNineAgents(t *testing.T) {
	fsys := makeValidCatalog(t)
	agents, err := agent.LoadCatalog(fsys)
	if err != nil {
		t.Fatalf("LoadCatalog error: %v", err)
	}
	if len(agents) != 9 {
		t.Fatalf("expected 9 agents, got %d", len(agents))
	}
}

func TestLoadCatalog_MalformedAgent_ReturnsError(t *testing.T) {
	fsys := fstest.MapFS{
		"bad.agent.md": &fstest.MapFile{Data: []byte("no frontmatter at all")},
	}
	_, err := agent.LoadCatalog(fsys)
	if err == nil {
		t.Fatal("expected error for malformed frontmatter, got nil")
	}
}

func TestAgent_Install_WritesFile(t *testing.T) {
	dir := t.TempDir()
	a := agent.Agent{
		Name:    "sdd-spec",
		Content: "---\ndescription: \"spec\"\ntools: ['read']\nuser-invocable: false\n---\nbody\n",
	}
	got, err := a.Install(dir)
	if err != nil {
		t.Fatalf("Install error: %v", err)
	}
	want := filepath.Join(dir, "sdd-spec.agent.md")
	if got != want {
		t.Errorf("returned path: got %q, want %q", got, want)
	}
	data, err := os.ReadFile(got)
	if err != nil {
		t.Fatalf("reading installed file: %v", err)
	}
	if string(data) != a.Content {
		t.Errorf("file content mismatch:\ngot:  %q\nwant: %q", string(data), a.Content)
	}
}

func TestAgent_Install_NoopOnIdenticalContent(t *testing.T) {
	dir := t.TempDir()
	a := agent.Agent{
		Name:    "sdd-spec",
		Content: "---\ndescription: \"spec\"\ntools: ['read']\nuser-invocable: false\n---\nbody\n",
	}
	// First install.
	p, err := a.Install(dir)
	if err != nil {
		t.Fatalf("first Install error: %v", err)
	}
	info1, err := os.Stat(p)
	if err != nil {
		t.Fatalf("stat after first install: %v", err)
	}

	// Second install with identical content — must not write.
	_, err = a.Install(dir)
	if err != nil {
		t.Fatalf("second Install error: %v", err)
	}
	info2, err := os.Stat(p)
	if err != nil {
		t.Fatalf("stat after second install: %v", err)
	}
	if info1.ModTime() != info2.ModTime() {
		t.Error("mtime changed on re-install with identical content (file was re-written unnecessarily)")
	}
}

func TestAgent_Install_OverwritesOnDrift(t *testing.T) {
	dir := t.TempDir()
	a := agent.Agent{
		Name:    "sdd-spec",
		Content: "---\ndescription: \"spec\"\ntools: ['read']\nuser-invocable: false\n---\noriginal\n",
	}
	_, err := a.Install(dir)
	if err != nil {
		t.Fatalf("first Install error: %v", err)
	}

	a.Content = "---\ndescription: \"spec\"\ntools: ['read']\nuser-invocable: false\n---\nupdated\n"
	p, err := a.Install(dir)
	if err != nil {
		t.Fatalf("second Install error: %v", err)
	}
	data, err := os.ReadFile(p)
	if err != nil {
		t.Fatalf("reading file: %v", err)
	}
	if !strings.Contains(string(data), "updated") {
		t.Errorf("file was not overwritten on drift; content: %q", string(data))
	}
}

func TestCanonicalContent_EqualsContent(t *testing.T) {
	a := agent.Agent{
		Name:    "sdd-design",
		Content: "some content",
	}
	if got := a.CanonicalContent(); got != a.Content {
		t.Errorf("CanonicalContent() = %q, want %q", got, a.Content)
	}
}

// --- Phase 1.2: Catalog validation tests ---

// allPhases is the canonical list of 8 worker phase names.
var allPhases = []string{
	"capiko-sdd-explore",
	"capiko-sdd-propose",
	"capiko-sdd-spec",
	"capiko-sdd-design",
	"capiko-sdd-tasks",
	"capiko-sdd-apply",
	"capiko-sdd-verify",
	"capiko-sdd-archive",
}

// allowedTools is the complete set of valid Copilot tool aliases.
var allowedTools = map[string]bool{
	"read": true, "edit": true, "search": true, "execute": true, "agent": true,
}

// anthropicAliases are Anthropic model names that must NOT appear in any model: field.
var anthropicAliases = []string{"opus", "sonnet", "haiku", "claude"}

func TestCatalog_WorkerFrontmatter(t *testing.T) {
	agents := loadRealCatalog(t)
	byName := indexByName(agents)

	for _, phase := range allPhases {
		a, ok := byName[phase]
		if !ok {
			t.Errorf("worker %q not found in catalog", phase)
			continue
		}
		fm, err := agent.ParseFrontmatter(a.Content)
		if err != nil {
			t.Errorf("worker %q: frontmatter parse error: %v", phase, err)
			continue
		}
		if fm.UserInvocable {
			t.Errorf("worker %q: user-invocable must be false", phase)
		}
		for _, tool := range fm.Tools {
			if !allowedTools[tool] {
				t.Errorf("worker %q: disallowed tool %q in tools list", phase, tool)
			}
		}
		for _, alias := range anthropicAliases {
			if strings.EqualFold(fm.Model, alias) {
				t.Errorf("worker %q: model field contains Anthropic alias %q", phase, fm.Model)
			}
		}
	}
}

func TestCatalog_WorkerBodyReferencesSkillPath(t *testing.T) {
	agents := loadRealCatalog(t)
	byName := indexByName(agents)

	for _, phase := range allPhases {
		a, ok := byName[phase]
		if !ok {
			t.Errorf("worker %q not found in catalog", phase)
			continue
		}
		// Each worker name is "capiko-sdd-<phase>"; skill path is "sdd-<phase>/SKILL.md"
		// e.g. capiko-sdd-explore → ~/.copilot/skills/sdd-explore/SKILL.md
		sddPhase := strings.TrimPrefix(phase, "capiko-")
		wantPath := "~/.copilot/skills/" + sddPhase + "/SKILL.md"
		if !strings.Contains(a.Content, wantPath) {
			t.Errorf("worker %q body must contain %q", phase, wantPath)
		}
	}
}

func TestCatalog_CoordinatorAllowlist(t *testing.T) {
	agents := loadRealCatalog(t)
	byName := indexByName(agents)

	coord, ok := byName["capiko-sdd-coordinator"]
	if !ok {
		t.Fatal("capiko-sdd-coordinator not found in catalog")
	}
	fm, err := agent.ParseFrontmatter(coord.Content)
	if err != nil {
		t.Fatalf("coordinator frontmatter parse error: %v", err)
	}

	if len(fm.Agents) != len(allPhases) {
		t.Errorf("coordinator agents allowlist: got %d entries, want %d", len(fm.Agents), len(allPhases))
	}
	allowlist := make(map[string]bool, len(fm.Agents))
	for _, n := range fm.Agents {
		allowlist[n] = true
	}
	for _, phase := range allPhases {
		if !allowlist[phase] {
			t.Errorf("coordinator agents allowlist missing %q", phase)
		}
	}
	// No extras beyond the 8 workers.
	for _, n := range fm.Agents {
		found := false
		for _, phase := range allPhases {
			if n == phase {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("coordinator agents allowlist has unexpected entry %q", n)
		}
	}
	// The coordinator must not carry an Anthropic model alias either (the worker
	// check covers workers; the coordinator escapes it otherwise).
	for _, alias := range anthropicAliases {
		if strings.EqualFold(fm.Model, alias) {
			t.Errorf("coordinator: model field contains Anthropic alias %q", fm.Model)
		}
	}
}

func TestCatalog_CoordinatorBodyCitesNativeEngine(t *testing.T) {
	agents := loadRealCatalog(t)
	byName := indexByName(agents)

	coord, ok := byName["capiko-sdd-coordinator"]
	if !ok {
		t.Fatal("capiko-sdd-coordinator not found in catalog")
	}
	for _, want := range []string{"capiko-ai sdd-status --json", "nextRecommended"} {
		if !strings.Contains(coord.Content, want) {
			t.Errorf("coordinator body must contain %q", want)
		}
	}
}

func TestCatalog_CoordinatorBodyExplicitDelegation(t *testing.T) {
	agents := loadRealCatalog(t)
	byName := indexByName(agents)

	coord, ok := byName["capiko-sdd-coordinator"]
	if !ok {
		t.Fatal("capiko-sdd-coordinator not found in catalog")
	}
	// Pin the backtick form "`agent` tool" — the bare substring "agent" also
	// appears in "agents:" and the worker names, so it would pass even if the body
	// never named the delegation tool.
	if !strings.Contains(coord.Content, "`agent` tool") {
		t.Error("coordinator body must reference the `agent` tool (backtick form) for explicit delegation")
	}
}

func TestCatalog_CoordinatorTriageGate(t *testing.T) {
	agents := loadRealCatalog(t)
	byName := indexByName(agents)

	coord, ok := byName["capiko-sdd-coordinator"]
	if !ok {
		t.Fatal("capiko-sdd-coordinator not found in catalog")
	}
	// Guard the triage gate so a future edit cannot silently drop the rules that
	// keep small changes inline instead of forcing the full SDD phase DAG.
	for _, want := range []string{
		"Triage Gate",
		"4-file rule",
		"Delegate a writer",
		"Fresh review before a PR",
	} {
		if !strings.Contains(coord.Content, want) {
			t.Errorf("coordinator body must contain triage rule %q", want)
		}
	}
}

func TestCatalog_LanguageContract_Coordinator(t *testing.T) {
	agents := loadRealCatalog(t)
	byName := indexByName(agents)

	coord, ok := byName["capiko-sdd-coordinator"]
	if !ok {
		t.Fatal("capiko-sdd-coordinator not found in catalog")
	}
	for _, marker := range []string{"human's language", "English"} {
		if !strings.Contains(coord.Content, marker) {
			t.Errorf("coordinator body must contain language contract marker %q", marker)
		}
	}
}

func TestCatalog_LanguageContract_Workers(t *testing.T) {
	agents := loadRealCatalog(t)
	byName := indexByName(agents)

	for _, phase := range allPhases {
		a, ok := byName[phase]
		if !ok {
			t.Errorf("worker %q not found in catalog", phase)
			continue
		}
		// Workers must carry a language contract line (they reference the coordinator's contract).
		if !strings.Contains(a.Content, "Language:") {
			t.Errorf("worker %q body must contain a Language: line", phase)
		}
	}
}

// --- Phase 1.1/2.1: WithRouting tests ---

func TestWithRouting(t *testing.T) {
	tests := []struct {
		name        string
		agents      []agent.Agent
		models      map[string]string
		wantContent map[string]string // agent Name -> expected Content (only checked when present)
	}{
		{
			name: "injects new model line when none present",
			agents: []agent.Agent{
				{Name: "capiko-sdd-design", Content: "---\ndescription: \"design\"\ntools: ['read']\nuser-invocable: false\n---\nbody\n"},
			},
			models: map[string]string{"design": "gemini-5.4"},
			wantContent: map[string]string{
				"capiko-sdd-design": "---\ndescription: \"design\"\ntools: ['read']\nuser-invocable: false\nmodel: gemini-5.4\n---\nbody\n",
			},
		},
		{
			name: "replaces existing model line idempotently",
			agents: []agent.Agent{
				{Name: "capiko-sdd-spec", Content: "---\ndescription: \"spec\"\nmodel: old-value\ntools: ['read']\n---\nbody\n"},
			},
			models: map[string]string{"spec": "new-value"},
			wantContent: map[string]string{
				"capiko-sdd-spec": "---\ndescription: \"spec\"\nmodel: new-value\ntools: ['read']\n---\nbody\n",
			},
		},
		{
			name: "strips model line on default sentinel",
			agents: []agent.Agent{
				{Name: "capiko-sdd-explore", Content: "---\ndescription: \"explore\"\nmodel: gpt-5.1\ntools: ['read']\n---\nbody\n"},
			},
			models: map[string]string{"explore": "default"},
			wantContent: map[string]string{
				"capiko-sdd-explore": "---\ndescription: \"explore\"\ntools: ['read']\n---\nbody\n",
			},
		},
		{
			name: "strips model line on empty string",
			agents: []agent.Agent{
				{Name: "capiko-sdd-archive", Content: "---\ndescription: \"archive\"\nmodel: something\ntools: ['read']\n---\nbody\n"},
			},
			models: map[string]string{"archive": ""},
			wantContent: map[string]string{
				"capiko-sdd-archive": "---\ndescription: \"archive\"\ntools: ['read']\n---\nbody\n",
			},
		},
		{
			name: "coordinator agent passes through untouched",
			agents: []agent.Agent{
				{Name: "capiko-sdd-coordinator", Content: "---\ndescription: \"coordinator\"\nagents: ['capiko-sdd-explore']\n---\nbody\n"},
			},
			// Convention derives phase key "coordinator" from the name. In real
			// use SDDModels only ever carries sdd.Phases entries, and
			// "coordinator" is not one of them, so the map simply never has a
			// "coordinator" key — no special-casing in WithRouting itself.
			models: map[string]string{"explore": "gemini-5.4"},
			wantContent: map[string]string{
				"capiko-sdd-coordinator": "---\ndescription: \"coordinator\"\nagents: ['capiko-sdd-explore']\n---\nbody\n",
			},
		},
		{
			name: "unmapped phase key is a no-op",
			agents: []agent.Agent{
				{Name: "capiko-sdd-verify", Content: "---\ndescription: \"verify\"\ntools: ['read']\n---\nbody\n"},
			},
			models: map[string]string{"design": "gemini-5.4"}, // no "verify" key
			wantContent: map[string]string{
				"capiko-sdd-verify": "---\ndescription: \"verify\"\ntools: ['read']\n---\nbody\n",
			},
		},
		{
			name: "non-SDD agent name passes through untouched",
			agents: []agent.Agent{
				{Name: "some-other-agent", Content: "---\ndescription: \"other\"\ntools: ['read']\n---\nbody\n"},
			},
			models: map[string]string{"other-agent": "gemini-5.4"}, // TrimPrefix leaves name unchanged, no match
			wantContent: map[string]string{
				"some-other-agent": "---\ndescription: \"other\"\ntools: ['read']\n---\nbody\n",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := agent.WithRouting(tt.agents, tt.models)
			if len(got) != len(tt.agents) {
				t.Fatalf("WithRouting returned %d agents, want %d", len(got), len(tt.agents))
			}
			byName := indexByName(got)
			for name, want := range tt.wantContent {
				a, ok := byName[name]
				if !ok {
					t.Fatalf("agent %q missing from WithRouting output", name)
				}
				if a.Content != want {
					t.Errorf("agent %q Content:\ngot:  %q\nwant: %q", name, a.Content, want)
				}
			}
		})
	}
}

func TestWithRouting_PreservesOtherFrontmatterFields(t *testing.T) {
	original := "---\ndescription: \"apply\"\ntools: ['read', 'edit']\nuser-invocable: false\n---\nbody text\n"
	agents := []agent.Agent{{Name: "capiko-sdd-apply", Content: original}}

	got := agent.WithRouting(agents, map[string]string{"apply": "custom-model"})

	fm, err := agent.ParseFrontmatter(got[0].Content)
	if err != nil {
		t.Fatalf("ParseFrontmatter: %v", err)
	}
	if fm.Description != "apply" {
		t.Errorf("Description = %q, want %q", fm.Description, "apply")
	}
	if len(fm.Tools) != 2 || fm.Tools[0] != "read" || fm.Tools[1] != "edit" {
		t.Errorf("Tools = %v, want [read edit]", fm.Tools)
	}
	if fm.UserInvocable {
		t.Error("UserInvocable = true, want false")
	}
	if fm.Model != "custom-model" {
		t.Errorf("Model = %q, want %q", fm.Model, "custom-model")
	}
	if !strings.Contains(got[0].Content, "body text") {
		t.Error("body text lost after routing")
	}
}

func TestWithRouting_DoesNotMutateInput(t *testing.T) {
	original := "---\ndescription: \"design\"\ntools: ['read']\n---\nbody\n"
	agents := []agent.Agent{{Name: "capiko-sdd-design", Content: original}}

	_ = agent.WithRouting(agents, map[string]string{"design": "gemini-5.4"})

	if agents[0].Content != original {
		t.Errorf("input agent Content mutated:\ngot:  %q\nwant: %q", agents[0].Content, original)
	}
}

// TestWithRouting_RoutedCatalogRejectsBareAlias is the routed-output
// counterpart to TestCatalog_WorkerFrontmatter: it exercises the real
// embedded catalog through WithRouting with a representative models map (a
// mix of "default" and custom values) and asserts no worker's *routed*
// model: field regresses to a bare Anthropic alias.
func TestWithRouting_RoutedCatalogRejectsBareAlias(t *testing.T) {
	agents := loadRealCatalog(t)
	models := map[string]string{
		"orchestrator": "claude-opus-4.8",
		"explore":      "default",
		"propose":      "gemini-5.4",
		"spec":         "default",
		"design":       "gemini-5.4",
		"tasks":        "default",
		"apply":        "gemini-5.4",
		"verify":       "default",
		"archive":      "gemini-5.4",
	}

	routed := agent.WithRouting(agents, models)
	byName := indexByName(routed)

	for _, phase := range allPhases {
		a, ok := byName[phase]
		if !ok {
			t.Errorf("worker %q not found in routed catalog", phase)
			continue
		}
		fm, err := agent.ParseFrontmatter(a.Content)
		if err != nil {
			t.Errorf("worker %q: frontmatter parse error: %v", phase, err)
			continue
		}
		for _, alias := range anthropicAliases {
			if strings.EqualFold(fm.Model, alias) {
				t.Errorf("worker %q: routed model field contains Anthropic alias %q", phase, fm.Model)
			}
		}
	}
}

// --- Helpers ---

// loadRealCatalog loads agents from the real embedded catalog via catalog.LoadAgents.
// It uses a separate helper to avoid a circular import: the catalog package imports
// agent, so we load from the embedded FS directly.
func loadRealCatalog(t *testing.T) []agent.Agent {
	t.Helper()
	// Load from the catalog's embedded agents directory.
	// Since agent_test is in package agent_test, we call catalog.LoadAgents which
	// exercises the embed. We can't import catalog here without a cycle, so instead
	// we load from the real file tree using os.DirFS for the test.
	dir := filepath.Join("..", "catalog", "agents")
	fsys := os.DirFS(dir)
	agents, err := agent.LoadCatalog(fsys)
	if err != nil {
		t.Fatalf("LoadCatalog from real catalog dir: %v", err)
	}
	return agents
}

func indexByName(agents []agent.Agent) map[string]agent.Agent {
	m := make(map[string]agent.Agent, len(agents))
	for _, a := range agents {
		m[a.Name] = a
	}
	return m
}

// makeValidCatalog returns an fstest.MapFS with 9 minimal valid .agent.md files.
func makeValidCatalog(t *testing.T) fs.FS {
	t.Helper()
	names := append(append([]string{}, allPhases...), "capiko-sdd-coordinator")
	fsys := fstest.MapFS{}
	for _, name := range names {
		fsys[name+".agent.md"] = &fstest.MapFile{
			Data: []byte("---\ndescription: \"" + name + "\"\ntools: ['read']\nuser-invocable: false\n---\nbody\n"),
		}
	}
	return fsys
}
