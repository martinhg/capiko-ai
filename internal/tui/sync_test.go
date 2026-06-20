package tui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/martinhg/capiko-ai/internal/agent"
	"github.com/martinhg/capiko-ai/internal/copilot"
	"github.com/martinhg/capiko-ai/internal/state"
)

func TestSyncWritesWholeCatalog(t *testing.T) {
	dir := t.TempDir()
	s, ok := newSync(services{host: &copilot.Host{SkillsDir: dir}}, testCatalog(), nil).(*syncScreen)
	if !ok {
		t.Fatal("newSync did not return *syncScreen")
	}

	_, cmd := s.Update(key("y"))
	if s.state != syncApplying {
		t.Fatalf("state = %d, want syncApplying", s.state)
	}

	sm, ok := cmd().(syncedMsg)
	if !ok || sm.err != nil {
		t.Fatalf("sync failed: %+v", sm)
	}
	if sm.count != len(testCatalog()) {
		t.Errorf("count = %d, want %d", sm.count, len(testCatalog()))
	}
	for _, sk := range testCatalog() {
		if _, err := os.Stat(filepath.Join(dir, sk.Name, "SKILL.md")); err != nil {
			t.Errorf("%s not written: %v", sk.Name, err)
		}
	}

	s.Update(sm)
	if s.state != syncDone {
		t.Errorf("state = %d, want syncDone", s.state)
	}
}

func TestRunSyncWritesCatalogAndRecordsState(t *testing.T) {
	dir := t.TempDir()
	store := state.NewStore(t.TempDir())

	n, err := RunSync(&copilot.Host{SkillsDir: dir}, testCatalog(), nil, store, nil)
	if err != nil {
		t.Fatalf("RunSync: %v", err)
	}
	if n != len(testCatalog()) {
		t.Errorf("count = %d, want %d", n, len(testCatalog()))
	}

	st, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	for _, sk := range testCatalog() {
		if _, err := os.Stat(filepath.Join(dir, sk.Name, "SKILL.md")); err != nil {
			t.Errorf("%s not written: %v", sk.Name, err)
		}
		rec, ok := st.Skills[sk.Name]
		if !ok {
			t.Errorf("%s not recorded in state", sk.Name)
			continue
		}
		if rec.Checksum != state.Checksum(sk.Content) {
			t.Errorf("%s checksum = %q, want content checksum", sk.Name, rec.Checksum)
		}
	}
}

func TestRunSyncReappliesPersona(t *testing.T) {
	cfgDir := t.TempDir()
	host := &copilot.Host{ConfigDir: cfgDir, SkillsDir: filepath.Join(cfgDir, "skills")}
	store := state.NewStore(t.TempDir())
	if err := store.SetPersona("capiko"); err != nil {
		t.Fatal(err)
	}

	if _, err := RunSync(host, testCatalog(), nil, store, nil); err != nil {
		t.Fatalf("RunSync: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(cfgDir, "copilot-instructions.md"))
	if err != nil {
		t.Fatalf("persona instructions not written by sync: %v", err)
	}
	if !strings.Contains(string(data), "capiko:persona:start") {
		t.Errorf("sync did not re-apply the persona block: %q", data)
	}
}

// TestRunSyncReappliesScopedInstructions asserts that once scoped instructions
// are managed (recorded in state), sync re-applies them into the home dir AND any
// COPILOT_CUSTOM_INSTRUCTIONS_DIRS, so they track the catalog like persona/SDD.
func TestRunSyncReappliesScopedInstructions(t *testing.T) {
	cfgDir := t.TempDir()
	customDir := t.TempDir()
	t.Setenv("COPILOT_CUSTOM_INSTRUCTIONS_DIRS", customDir)
	host := &copilot.Host{ConfigDir: cfgDir, SkillsDir: filepath.Join(cfgDir, "skills")}
	store := state.NewStore(t.TempDir())
	if err := store.SetInstructionsInstalled(true); err != nil {
		t.Fatal(err)
	}

	if _, err := RunSync(host, testCatalog(), nil, store, nil); err != nil {
		t.Fatalf("RunSync: %v", err)
	}

	for _, dir := range []string{filepath.Join(cfgDir, "instructions"), customDir} {
		if _, err := os.Stat(filepath.Join(dir, "go.instructions.md")); err != nil {
			t.Errorf("sync did not re-apply scoped instructions into %s: %v", dir, err)
		}
	}
}

// TestRunSyncSkipsInstructionsWhenUnmanaged asserts sync does NOT create scoped
// instruction files for a user who never installed them (flag unset), mirroring
// how persona/SDD re-apply only when configured.
func TestRunSyncSkipsInstructionsWhenUnmanaged(t *testing.T) {
	t.Setenv("COPILOT_CUSTOM_INSTRUCTIONS_DIRS", "")
	cfgDir := t.TempDir()
	host := &copilot.Host{ConfigDir: cfgDir, SkillsDir: filepath.Join(cfgDir, "skills")}
	store := state.NewStore(t.TempDir())

	if _, err := RunSync(host, testCatalog(), nil, store, nil); err != nil {
		t.Fatalf("RunSync: %v", err)
	}

	if _, err := os.Stat(filepath.Join(cfgDir, "instructions", "go.instructions.md")); err == nil {
		t.Error("sync wrote scoped instructions for an unmanaged user")
	}
}

func TestSyncQuitGoesBack(t *testing.T) {
	s, _ := newSync(services{host: &copilot.Host{SkillsDir: t.TempDir()}}, testCatalog(), nil).(*syncScreen)
	_, cmd := s.Update(key("q"))
	if _, ok := cmd().(backMsg); !ok {
		t.Error("q should emit backMsg")
	}
}

// testAgentCatalog returns a small agent catalog for TUI tests.
func testAgentCatalog() []agent.Agent {
	return []agent.Agent{
		{
			Name:        "capiko-sdd-explore",
			Description: "SDD explore phase",
			Content:     "---\ndescription: SDD explore phase\ntools: [read]\nuser-invocable: false\n---\nExplore.",
		},
		{
			Name:        "capiko-sdd-apply",
			Description: "SDD apply phase",
			Content:     "---\ndescription: SDD apply phase\ntools: [read,edit,execute]\nuser-invocable: false\n---\nApply.",
		},
	}
}

// TestRunSync_InstallsAgents asserts that RunSync writes agent files into AgentsDir
// and calls ApplyAgents so agent state is recorded.
// Spec: TUISurfacesAgentsAlongsideSkills / Scenario: Install screen shows agents.
func TestRunSync_InstallsAgents(t *testing.T) {
	skillsDir := t.TempDir()
	agentsDir := t.TempDir()
	store := state.NewStore(t.TempDir())
	host := &copilot.Host{SkillsDir: skillsDir, AgentsDir: agentsDir}
	agents := testAgentCatalog()

	n, err := RunSync(host, testCatalog(), agents, store, nil)
	if err != nil {
		t.Fatalf("RunSync: %v", err)
	}

	// Each agent file must be written to AgentsDir.
	for _, a := range agents {
		p := filepath.Join(agentsDir, a.Name+".agent.md")
		if _, err := os.Stat(p); err != nil {
			t.Errorf("agent file %s not written: %v", a.Name, err)
		}
	}

	// State must record agents.
	st, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	for _, a := range agents {
		if _, ok := st.Agents[a.Name]; !ok {
			t.Errorf("agent %s not recorded in state", a.Name)
		}
	}

	// Total count must include skills + agents.
	want := len(testCatalog()) + len(agents)
	if n != want {
		t.Errorf("count = %d, want %d (skills + agents)", n, want)
	}
}

// TestRunSync_AgentCountReturned asserts the returned count includes agents.
// Spec: TUISurfacesAgentsAlongsideSkills / Scenario: Install screen shows agents.
func TestRunSync_AgentCountReturned(t *testing.T) {
	host := &copilot.Host{SkillsDir: t.TempDir(), AgentsDir: t.TempDir()}
	agents := testAgentCatalog()

	n, err := RunSync(host, testCatalog(), agents, nil, nil)
	if err != nil {
		t.Fatalf("RunSync: %v", err)
	}
	want := len(testCatalog()) + len(agents)
	if n != want {
		t.Errorf("count = %d, want %d", n, want)
	}
}

// TestSyncDoneView_ShowsAgentsSection asserts that when sync completes with an agent
// catalog, the done view includes a distinct "Agents" section listing each agent name.
// Spec: TUISurfacesAgentsAlongsideSkills / Scenario: Install screen shows agents.
func TestSyncDoneView_ShowsEngramAdvisory(t *testing.T) {
	s := &syncScreen{
		catalog:       testCatalog(),
		state:         syncDone,
		skillNames:    []string{"capiko-hello"},
		engramWarning: "engram 1.16.3 is behind the recommended 1.17.0",
	}
	view := s.View()
	if !strings.Contains(view, "engram 1.16.3 is behind") {
		t.Errorf("syncDone view should surface the engram advisory, got:\n%s", view)
	}
}

func TestSyncDoneView_NoEngramAdvisoryWhenEmpty(t *testing.T) {
	s := &syncScreen{
		catalog:    testCatalog(),
		state:      syncDone,
		skillNames: []string{"capiko-hello"},
	}
	if strings.Contains(s.View(), "behind the recommended") {
		t.Errorf("syncDone view should not show an advisory when none is set:\n%s", s.View())
	}
}

func TestSyncDoneView_ShowsAgentsSection(t *testing.T) {
	agents := testAgentCatalog()
	s := &syncScreen{
		catalog:      testCatalog(),
		agentCatalog: agents,
		state:        syncDone,
		skillNames:   []string{"capiko-hello", "capiko-conventions", "capiko-pr"},
		agentNames:   []string{"capiko-sdd-explore", "capiko-sdd-apply"},
	}
	view := s.View()

	if !strings.Contains(view, "Agents") {
		t.Errorf("syncDone view missing 'Agents' section, got:\n%s", view)
	}
	for _, a := range agents {
		if !strings.Contains(view, a.Name) {
			t.Errorf("syncDone view missing agent name %q, got:\n%s", a.Name, view)
		}
	}
}

// TestSyncDoneView_ShowsSkillsSection asserts that the done view still shows a "Skills"
// section listing each skill name, parallel to the "Agents" section.
// Spec: TUISurfacesAgentsAlongsideSkills / Scenario: Install screen shows agents.
func TestSyncDoneView_ShowsSkillsSection(t *testing.T) {
	s := &syncScreen{
		catalog:    testCatalog(),
		state:      syncDone,
		skillNames: []string{"capiko-hello", "capiko-conventions"},
	}
	view := s.View()

	if !strings.Contains(view, "Skills") {
		t.Errorf("syncDone view missing 'Skills' section, got:\n%s", view)
	}
	for _, sk := range testCatalog()[:2] {
		if !strings.Contains(view, sk.Name) {
			t.Errorf("syncDone view missing skill name %q, got:\n%s", sk.Name, view)
		}
	}
}
