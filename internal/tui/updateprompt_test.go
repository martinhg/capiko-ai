package tui

import (
	"strings"
	"testing"
)

func TestPromptShownWhenUpdateAvailable(t *testing.T) {
	a := App{state: appMenu, latest: ""}
	next, _ := a.Update(latestVersionMsg{version: "2.0.0"})
	app := next.(App)
	if app.state != appUpdatePrompt {
		t.Errorf("state = %d, want appUpdatePrompt", app.state)
	}
	if app.cursor != promptDefaultCursor {
		t.Errorf("cursor = %d, want %d (Keep current)", app.cursor, promptDefaultCursor)
	}
}

func TestPromptNotShownAfterMenuInteraction(t *testing.T) {
	a := App{state: appMenu, menuTouched: true}
	next, _ := a.Update(latestVersionMsg{version: "2.0.0"})
	app := next.(App)
	if app.state != appMenu {
		t.Errorf("state = %d, want appMenu (prompt skipped after interaction)", app.state)
	}
}

func TestPromptNotShownWhenNoUpdate(t *testing.T) {
	a := App{state: appMenu}
	next, _ := a.Update(latestVersionMsg{})
	if next.(App).state != appMenu {
		t.Error("empty latestVersionMsg should stay on menu")
	}
}

func TestPromptNotShownBeforeDetection(t *testing.T) {
	a := App{state: appDetecting}
	next, _ := a.Update(latestVersionMsg{version: "2.0.0"})
	app := next.(App)
	if app.state != appDetecting {
		t.Errorf("state = %d, want appDetecting (prompt should not fire before detection)", app.state)
	}
	if app.latest != "2.0.0" {
		t.Error("latest should still be stored even when prompt is not shown")
	}
}

func TestPromptDefaultCursorOnKeepCurrent(t *testing.T) {
	a := App{state: appMenu}
	next, _ := a.Update(latestVersionMsg{version: "2.0.0"})
	if next.(App).cursor != promptDefaultCursor {
		t.Errorf("cursor = %d, want %d", next.(App).cursor, promptDefaultCursor)
	}
}

func TestPromptKeepCurrentGoesToMenu(t *testing.T) {
	a := App{state: appUpdatePrompt, latest: "2.0.0", cursor: promptDefaultCursor}
	next, _ := a.Update(key("enter"))
	app := next.(App)
	if app.state != appMenu {
		t.Errorf("state = %d, want appMenu", app.state)
	}
	if app.cursor != 0 {
		t.Errorf("menu cursor = %d, want 0 after prompt dismissal", app.cursor)
	}
	if !app.menuTouched {
		t.Error("menuTouched should be true after dismissing prompt")
	}
}

func TestPromptEscGoesToMenu(t *testing.T) {
	a := App{state: appUpdatePrompt, latest: "2.0.0", cursor: 0}
	next, _ := a.Update(key("esc"))
	if next.(App).state != appMenu {
		t.Errorf("state = %d, want appMenu", next.(App).state)
	}
}

func TestPromptShortcutCGoesToMenu(t *testing.T) {
	a := App{state: appUpdatePrompt, latest: "2.0.0", cursor: 0}
	next, _ := a.Update(key("c"))
	if next.(App).state != appMenu {
		t.Errorf("state = %d, want appMenu", next.(App).state)
	}
}

func TestPromptUpdateNowGoesToUpgrade(t *testing.T) {
	a := App{state: appUpdatePrompt, latest: "2.0.0", cursor: 0}
	next, _ := a.Update(key("enter"))
	app := next.(App)
	if app.state != appScreen {
		t.Errorf("state = %d, want appScreen", app.state)
	}
	if app.active == nil {
		t.Fatal("active screen should be set")
	}
	if _, ok := app.active.(*upgradeScreen); !ok {
		t.Errorf("active = %T, want *upgradeScreen", app.active)
	}
}

func TestPromptShortcutUGoesToUpgrade(t *testing.T) {
	a := App{state: appUpdatePrompt, latest: "2.0.0", cursor: 2}
	next, _ := a.Update(key("u"))
	app := next.(App)
	if app.state != appScreen {
		t.Errorf("state = %d, want appScreen", app.state)
	}
	if _, ok := app.active.(*upgradeScreen); !ok {
		t.Errorf("active = %T, want *upgradeScreen", app.active)
	}
}

func TestPromptViewChangesOpensBrowser(t *testing.T) {
	var opened string
	orig := browserOpen
	browserOpen = func(url string) error { opened = url; return nil }
	t.Cleanup(func() { browserOpen = orig })

	a := App{state: appUpdatePrompt, latest: "2.0.0", cursor: 1}
	next, _ := a.Update(key("enter"))
	app := next.(App)
	if app.state != appUpdatePrompt {
		t.Errorf("state = %d, want appUpdatePrompt (should stay on prompt)", app.state)
	}
	want := "https://github.com/martinhg/capiko-ai/releases/tag/v2.0.0"
	if opened != want {
		t.Errorf("opened = %q, want %q", opened, want)
	}
}

func TestPromptShortcutVOpensBrowser(t *testing.T) {
	var opened string
	orig := browserOpen
	browserOpen = func(url string) error { opened = url; return nil }
	t.Cleanup(func() { browserOpen = orig })

	a := App{state: appUpdatePrompt, latest: "2.0.0", cursor: 0}
	next, _ := a.Update(key("v"))
	if next.(App).state != appUpdatePrompt {
		t.Error("v shortcut should stay on prompt")
	}
	if opened == "" {
		t.Error("v shortcut should open browser")
	}
}

func TestPromptNavigation(t *testing.T) {
	a := App{state: appUpdatePrompt, latest: "2.0.0", cursor: promptDefaultCursor}

	next, _ := a.Update(key("up"))
	if next.(App).cursor != 1 {
		t.Errorf("cursor after up = %d, want 1", next.(App).cursor)
	}

	next, _ = next.(App).Update(key("up"))
	if next.(App).cursor != 0 {
		t.Errorf("cursor after 2x up = %d, want 0", next.(App).cursor)
	}

	next, _ = next.(App).Update(key("up"))
	if next.(App).cursor != 0 {
		t.Errorf("cursor should not go below 0, got %d", next.(App).cursor)
	}

	next, _ = next.(App).Update(key("down"))
	if next.(App).cursor != 1 {
		t.Errorf("cursor after down = %d, want 1", next.(App).cursor)
	}
}

func TestPromptQuitExits(t *testing.T) {
	a := App{state: appUpdatePrompt, latest: "2.0.0"}
	_, cmd := a.Update(key("q"))
	if cmd == nil {
		t.Fatal("q should emit a quit command")
	}
}

func TestPromptViewRendersOptions(t *testing.T) {
	a := App{state: appUpdatePrompt, latest: "2.0.0", cursor: promptDefaultCursor}
	view := a.viewPrompt()
	for _, item := range promptItems {
		if !strings.Contains(view, item) {
			t.Errorf("prompt view should contain %q", item)
		}
	}
	if !strings.Contains(view, "2.0.0") {
		t.Error("prompt view should show the available version")
	}
}
