package tui

import (
	"os"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/martinhg/capiko-ai/internal/copilot"
	"github.com/martinhg/capiko-ai/internal/state"
)

// readyApp returns an App already on the main menu.
func readyApp(t *testing.T, skillsDir string, installed ...string) App {
	t.Helper()
	inst := map[string]bool{}
	for _, n := range installed {
		inst[n] = true
	}
	next, _ := NewApp(testCatalog(), nil, nil, nil).Update(detectedMsg{
		host:      &copilot.Host{SkillsDir: skillsDir},
		installed: inst,
	})
	a, ok := next.(App)
	if !ok {
		t.Fatalf("Update returned %T, want App", next)
	}
	if a.state != appMenu {
		t.Fatalf("setup: state = %d, want appMenu", a.state)
	}
	return a
}

func TestAppDetectTransitions(t *testing.T) {
	tests := []struct {
		name string
		msg  detectedMsg
		want appState
	}{
		{"not installed", detectedMsg{}, appNotFound},
		{"error", detectedMsg{err: os.ErrPermission}, appFailed},
		{"ready", detectedMsg{host: &copilot.Host{}}, appMenu},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			next, _ := NewApp(testCatalog(), nil, nil, nil).Update(tc.msg)
			if got := next.(App).state; got != tc.want {
				t.Errorf("state = %d, want %d", got, tc.want)
			}
		})
	}
}

func TestMenuCursorClamps(t *testing.T) {
	a := readyApp(t, t.TempDir())

	next, _ := a.Update(key("up"))
	if next.(App).cursor != 0 {
		t.Errorf("cursor moved above 0")
	}

	a = next.(App)
	for range len(menuItems) + 3 {
		next, _ = a.Update(key("down"))
		a = next.(App)
	}
	if want := len(menuItems) - 1; a.cursor != want {
		t.Errorf("cursor = %d, want %d", a.cursor, want)
	}
}

func TestEnterOpensDetection(t *testing.T) {
	a := readyApp(t, t.TempDir()) // cursor 0 = Start installation

	next, _ := a.Update(key("enter"))
	app := next.(App)
	if app.state != appScreen {
		t.Fatalf("state = %d, want appScreen", app.state)
	}
	if _, ok := app.active.(*detectionScreen); !ok {
		t.Errorf("active = %T, want *detectionScreen", app.active)
	}
}

func TestEnterOpensSDD(t *testing.T) {
	a := readyApp(t, t.TempDir())
	a.cursor = 4 // Configure SDD

	next, _ := a.Update(key("enter"))
	app := next.(App)
	if _, ok := app.active.(*sddScreen); !ok {
		t.Errorf("active = %T, want *sddScreen", app.active)
	}
}

func TestEnterOpensHeadroom(t *testing.T) {
	a := readyApp(t, t.TempDir())
	a.cursor = 6 // Configure headroom

	next, _ := a.Update(key("enter"))
	if _, ok := next.(App).active.(*headroomScreen); !ok {
		t.Errorf("active = %T, want *headroomScreen", next.(App).active)
	}
}

func TestEnterOpensUpgrade(t *testing.T) {
	a := readyApp(t, t.TempDir())
	a.cursor = 7 // Upgrade tools

	next, _ := a.Update(key("enter"))
	app := next.(App)
	up, ok := app.active.(*upgradeScreen)
	if !ok {
		t.Fatalf("active = %T, want *upgradeScreen", app.active)
	}
	if up.withSync {
		t.Error("Upgrade tools should open the plain upgrade, not sync mode")
	}
}

func TestEnterOpensUpgradeSync(t *testing.T) {
	a := readyApp(t, t.TempDir())
	a.cursor = 8 // Upgrade + sync

	next, _ := a.Update(key("enter"))
	app := next.(App)
	up, ok := app.active.(*upgradeScreen)
	if !ok {
		t.Fatalf("active = %T, want *upgradeScreen", app.active)
	}
	if !up.withSync {
		t.Error("Upgrade + sync should open the screen in sync mode")
	}
}

func TestEnterOpensBackups(t *testing.T) {
	a := readyApp(t, t.TempDir())
	a.cursor = 3 // Manage backups (now ready)

	next, _ := a.Update(key("enter"))
	app := next.(App)
	if _, ok := app.active.(*backupsScreen); !ok {
		t.Errorf("active = %T, want *backupsScreen", app.active)
	}
}

func TestEnterOpensInstructions(t *testing.T) {
	a := readyApp(t, t.TempDir())
	a.cursor = 9 // Install instructions

	next, _ := a.Update(key("enter"))
	if _, ok := next.(App).active.(*instructionsScreen); !ok {
		t.Errorf("active = %T, want *instructionsScreen", next.(App).active)
	}
}

func TestBackReturnsToMenu(t *testing.T) {
	a := readyApp(t, t.TempDir())
	a.state, a.active = appScreen, newSoon("x")

	next, cmd := a.Update(backMsg{})
	app := next.(App)
	if app.state != appMenu {
		t.Errorf("state = %d, want appMenu", app.state)
	}
	if app.active != nil {
		t.Error("active should be cleared on back")
	}
	if cmd == nil {
		t.Error("back should refresh via detectCmd")
	}
}

func TestQuitMenuItemExits(t *testing.T) {
	a := readyApp(t, t.TempDir())
	a.cursor = len(menuItems) - 1 // Quit

	_, cmd := a.Update(key("enter"))
	if cmd == nil {
		t.Fatal("Quit item returned no command")
	}
	if _, ok := cmd().(tea.QuitMsg); !ok {
		t.Errorf("Quit item did not quit, got %T", cmd())
	}
}

func TestQuitKeys(t *testing.T) {
	for _, k := range []string{"q", "ctrl+c"} {
		t.Run(k, func(t *testing.T) {
			_, cmd := readyApp(t, t.TempDir()).Update(key(k))
			if cmd == nil {
				t.Fatalf("%q returned no command", k)
			}
			if _, ok := cmd().(tea.QuitMsg); !ok {
				t.Errorf("%q did not quit", k)
			}
		})
	}
}

// TestApp_StaleBanner_IncludesAgents asserts that when both skills and agents are stale,
// the staleBanner mentions exact counts for each group.
// Spec: TUISurfacesAgentsAlongsideSkills / Scenario: Drift screen shows drifted agents.
func TestApp_StaleBanner_IncludesAgents(t *testing.T) {
	store := state.NewStore(t.TempDir())
	// Record one skill with a stale checksum.
	if err := store.Apply("1.0.0", []state.Installed{{Name: "capiko-hello", Checksum: "stale-skill"}}, nil); err != nil {
		t.Fatal(err)
	}
	// Record one agent with a stale checksum.
	if err := store.ApplyAgents("1.0.0", []state.Installed{{Name: "capiko-sdd-explore", Checksum: "stale-agent"}}, nil); err != nil {
		t.Fatal(err)
	}

	agents := testAgentCatalog() // 2 agents; capiko-sdd-explore has stale checksum, capiko-sdd-apply is missing from state
	next, _ := NewApp(testCatalog(), agents, store, nil).Update(detectedMsg{
		host:      &copilot.Host{SkillsDir: t.TempDir()},
		installed: map[string]bool{"capiko-hello": true},
	})
	a, ok := next.(App)
	if !ok {
		t.Fatalf("Update returned %T, want App", next)
	}

	banner := a.staleBanner()
	// 1 stale skill + 2 stale agents (explore stale, apply missing-from-state).
	// The banner must mention both counts explicitly.
	if strings.TrimSpace(banner) == "" {
		t.Fatal("staleBanner should not be empty when agents are stale")
	}
	if !strings.Contains(banner, "1 skill") {
		t.Errorf("staleBanner should mention '1 skill', got: %q", banner)
	}
	if !strings.Contains(banner, "2 agents") {
		t.Errorf("staleBanner should mention '2 agents', got: %q", banner)
	}
}

// TestApp_StaleBanner_AgentsOnly_Singular asserts that when only one agent is stale,
// the banner reads "1 agent out of date" (no skill segment, singular noun).
// Spec: TUISurfacesAgentsAlongsideSkills / Scenario: Drift screen shows drifted agents.
func TestApp_StaleBanner_AgentsOnly_Singular(t *testing.T) {
	banner := App{staleAgents: []string{"capiko-sdd-explore"}}.staleBanner()
	if !strings.Contains(banner, "1 agent out of date") {
		t.Errorf("want '1 agent out of date' in banner, got: %q", banner)
	}
	if strings.Contains(banner, "skill") {
		t.Errorf("banner should not mention skills when only agents are stale, got: %q", banner)
	}
}

// TestApp_StaleBanner_AgentsOnly_Plural asserts that when multiple agents are stale,
// the banner reads "2 agents out of date" (plural noun, no skill segment).
// Spec: TUISurfacesAgentsAlongsideSkills / Scenario: Drift screen shows drifted agents.
func TestApp_StaleBanner_AgentsOnly_Plural(t *testing.T) {
	banner := App{staleAgents: []string{"capiko-sdd-explore", "capiko-sdd-apply"}}.staleBanner()
	if !strings.Contains(banner, "2 agents out of date") {
		t.Errorf("want '2 agents out of date' in banner, got: %q", banner)
	}
	if strings.Contains(banner, "skill") {
		t.Errorf("banner should not mention skills when only agents are stale, got: %q", banner)
	}
}

// TestApp_DetectCmd_PopulatesInstalledAgents asserts that after a detectedMsg,
// a.installedAgents is populated from the host.
// Spec: TUISurfacesAgentsAlongsideSkills / Scenario: Install screen shows agents.
func TestApp_DetectCmd_PopulatesInstalledAgents(t *testing.T) {
	agentsDir := t.TempDir()
	// Write one .agent.md file in agentsDir to simulate an installed agent.
	mustWriteFile(t, agentsDir, "capiko-sdd-explore.agent.md", "---\ndescription: test\ntools: [read]\nuser-invocable: false\n---\n")

	host := &copilot.Host{
		SkillsDir: t.TempDir(),
		AgentsDir: agentsDir,
	}

	next, _ := NewApp(testCatalog(), testAgentCatalog(), nil, nil).Update(detectedMsg{
		host:            host,
		installed:       map[string]bool{},
		installedAgents: map[string]bool{"capiko-sdd-explore": true},
	})
	a, ok := next.(App)
	if !ok {
		t.Fatalf("Update returned %T, want App", next)
	}
	if !a.installedAgents["capiko-sdd-explore"] {
		t.Errorf("installedAgents should contain capiko-sdd-explore, got %v", a.installedAgents)
	}
}

// mustWriteFile is a test helper that writes content to path/filename.
func mustWriteFile(t *testing.T, dir, filename, content string) {
	t.Helper()
	p := dir + "/" + filename
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
