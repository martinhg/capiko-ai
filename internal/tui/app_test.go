package tui

import (
	"os"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/martinhg/capiko-ai/internal/copilot"
)

// readyApp returns an App already on the main menu.
func readyApp(t *testing.T, skillsDir string, installed ...string) App {
	t.Helper()
	inst := map[string]bool{}
	for _, n := range installed {
		inst[n] = true
	}
	next, _ := NewApp(testCatalog(), nil, nil).Update(detectedMsg{
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
			next, _ := NewApp(testCatalog(), nil, nil).Update(tc.msg)
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

func TestEnterOpensReadyScreen(t *testing.T) {
	a := readyApp(t, t.TempDir()) // cursor 0 = Start installation

	next, _ := a.Update(key("enter"))
	app := next.(App)
	if app.state != appScreen {
		t.Fatalf("state = %d, want appScreen", app.state)
	}
	if _, ok := app.active.(*selector); !ok {
		t.Errorf("active = %T, want *selector", app.active)
	}
}

func TestEnterOnComingSoonOpensStub(t *testing.T) {
	a := readyApp(t, t.TempDir())
	a.cursor = 5 // Upgrade + sync (not ready)

	next, _ := a.Update(key("enter"))
	app := next.(App)
	if _, ok := app.active.(soonScreen); !ok {
		t.Errorf("active = %T, want soonScreen", app.active)
	}
}

func TestEnterOpensUpgrade(t *testing.T) {
	a := readyApp(t, t.TempDir())
	a.cursor = 4 // Upgrade tools (now ready)

	next, _ := a.Update(key("enter"))
	app := next.(App)
	if _, ok := app.active.(*upgradeScreen); !ok {
		t.Errorf("active = %T, want *upgradeScreen", app.active)
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
