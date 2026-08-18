package tui

import (
	"strings"
	"testing"

	"github.com/martinhg/capiko-ai/internal/copilot"
	"github.com/martinhg/capiko-ai/internal/state"
)

func installSelector(t *testing.T) *selector {
	t.Helper()
	s, ok := newInstall(services{host: &copilot.Host{SkillsDir: t.TempDir()}}, testCatalog(), map[string]bool{}).(*selector)
	if !ok {
		t.Fatal("newInstall did not return *selector")
	}
	return s
}

func TestInstallDoneOffersCGA(t *testing.T) {
	s := installSelector(t)
	s.state = selDone
	next, _ := s.Update(key("c"))
	if _, ok := next.(*cgaScreen); !ok {
		t.Errorf("c on the install-done screen should open CGA, got %T", next)
	}
}

func TestInstallDoneAnyOtherKeyGoesBack(t *testing.T) {
	s := installSelector(t)
	s.state = selDone
	_, cmd := s.Update(key("x"))
	if cmd == nil {
		t.Fatal("a non-c key on install-done should return a command")
	}
	if _, ok := cmd().(backMsg); !ok {
		t.Error("a non-c key on install-done should go back to the menu")
	}
}

func TestUninstallDoneDoesNotOfferCGA(t *testing.T) {
	s, _ := newUninstall(services{host: &copilot.Host{SkillsDir: t.TempDir()}}, testCatalog(), map[string]bool{"capiko-hello": true}).(*selector)
	s.state = selDone
	next, cmd := s.Update(key("c"))
	if _, ok := next.(*cgaScreen); ok {
		t.Error("uninstall flow must not offer CGA")
	}
	if cmd == nil || func() bool { _, ok := cmd().(backMsg); return !ok }() {
		t.Error("c on uninstall-done should just go back to the menu")
	}
}

func TestInstallDoneViewOffersCodeReview(t *testing.T) {
	s := installSelector(t)
	s.state = selDone
	if !strings.Contains(s.View(), "code review") {
		t.Errorf("install-done view should offer code review:\n%s", s.View())
	}
}

// TestCGAEndToEndEnableApplyPersists drives the full flow from the app menu:
// open the CGA screen, enable it, toggle strict mode, adjust the timeout,
// apply, and verify CGARecord is persisted to state with the edited values.
func TestCGAEndToEndEnableApplyPersists(t *testing.T) {
	ws := t.TempDir()
	prev := cgaGetwd
	cgaGetwd = func() (string, error) { return ws, nil }
	t.Cleanup(func() { cgaGetwd = prev })

	store := state.NewStore(t.TempDir())
	a := readyApp(t, t.TempDir())
	a.svc.state = store

	// Navigate to "Configure code review" (menu item id "code-review").
	idx := -1
	for i, it := range menuItems {
		if it.id == "code-review" {
			idx = i
			break
		}
	}
	if idx == -1 {
		t.Fatal("menuItems has no code-review entry")
	}
	a.cursor = idx

	next, _ := a.Update(key("enter"))
	app, ok := next.(App)
	if !ok {
		t.Fatalf("enter should return App, got %T", next)
	}
	s, ok := app.active.(*cgaScreen)
	if !ok {
		t.Fatalf("active screen = %T, want *cgaScreen", app.active)
	}

	// Enable.
	s.cursor = rowCGAEnabled
	s.Update(key(" "))
	if !s.enabled {
		t.Fatal("space on Enabled row should enable CGA")
	}

	// Toggle strict mode off.
	s.cursor = rowCGAStrict
	s.Update(key(" "))
	if s.strict {
		t.Fatal("space on Strict mode row should disable strict mode")
	}

	// Bump the timeout up one step.
	s.cursor = rowCGATimeout
	startTimeout := s.timeout
	s.Update(key("right"))
	if s.timeout != startTimeout+cgaTimeoutStep {
		t.Fatalf("timeout = %d, want %d after right", s.timeout, startTimeout+cgaTimeoutStep)
	}

	// Apply.
	s.cursor = rowCGAApply
	_, cmd := s.Update(key("enter"))
	if cmd == nil {
		t.Fatal("enter on Apply should return a command")
	}
	msg := cmd()
	next2, _ := s.Update(msg)
	applied, ok := next2.(*cgaScreen)
	if !ok || applied.state != cgaDone {
		t.Fatalf("apply should reach done state, got %+v", next2)
	}

	st, err := store.Load()
	if err != nil {
		t.Fatalf("load state: %v", err)
	}
	if st.CGA == nil {
		t.Fatal("state should record a CGARecord after apply")
	}
	if !st.CGA.Enabled {
		t.Error("CGARecord.Enabled should be true")
	}
	if st.CGA.StrictMode {
		t.Error("CGARecord.StrictMode should be false")
	}
	if st.CGA.Timeout != startTimeout+cgaTimeoutStep {
		t.Errorf("CGARecord.Timeout = %d, want %d", st.CGA.Timeout, startTimeout+cgaTimeoutStep)
	}
}
