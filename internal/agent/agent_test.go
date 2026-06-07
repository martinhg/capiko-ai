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
