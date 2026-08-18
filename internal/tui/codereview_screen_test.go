package tui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/martinhg/capiko-ai/internal/cga"
	"github.com/martinhg/capiko-ai/internal/state"
)

func TestCGAToggleEnabled(t *testing.T) {
	s := newCGA(services{}).(*cgaScreen)
	start := s.enabled
	s.cursor = rowCGAEnabled
	s.Update(key(" "))
	if s.enabled == start {
		t.Error("space on the Enabled row should toggle enabled")
	}
}

func TestCGAToggleStrict(t *testing.T) {
	s := newCGA(services{}).(*cgaScreen)
	start := s.strict
	s.cursor = rowCGAStrict
	s.Update(key(" "))
	if s.strict == start {
		t.Error("space on the Strict mode row should toggle strict")
	}
}

func TestCGADefaultTimeout(t *testing.T) {
	s := newCGA(services{}).(*cgaScreen)
	if s.timeout != cgaTimeoutDefault {
		t.Errorf("timeout = %d, want default %d", s.timeout, cgaTimeoutDefault)
	}
}

func TestCGATimeoutAdjust(t *testing.T) {
	s := newCGA(services{}).(*cgaScreen)
	s.cursor = rowCGATimeout
	start := s.timeout

	s.Update(key("right"))
	if s.timeout != start+cgaTimeoutStep {
		t.Errorf("timeout after right = %d, want %d", s.timeout, start+cgaTimeoutStep)
	}

	s.Update(key("left"))
	s.Update(key("left"))
	if s.timeout != start-cgaTimeoutStep {
		t.Errorf("timeout after left,left = %d, want %d", s.timeout, start-cgaTimeoutStep)
	}
}

func TestCGATimeoutIgnoredOffTimeoutRow(t *testing.T) {
	s := newCGA(services{}).(*cgaScreen)
	s.cursor = rowCGAEnabled
	start := s.timeout
	s.Update(key("right"))
	if s.timeout != start {
		t.Errorf("right on a non-timeout row should not change timeout, got %d want %d", s.timeout, start)
	}
}

func TestCGATimeoutClampsToMin(t *testing.T) {
	s := newCGA(services{}).(*cgaScreen)
	s.cursor = rowCGATimeout
	for i := 0; i < 20; i++ {
		s.Update(key("left"))
	}
	if s.timeout != cgaTimeoutMin {
		t.Errorf("timeout = %d, want clamped to min %d", s.timeout, cgaTimeoutMin)
	}
}

func TestCGATimeoutClampsToMax(t *testing.T) {
	s := newCGA(services{}).(*cgaScreen)
	s.cursor = rowCGATimeout
	for i := 0; i < 20; i++ {
		s.Update(key("right"))
	}
	if s.timeout != cgaTimeoutMax {
		t.Errorf("timeout = %d, want clamped to max %d", s.timeout, cgaTimeoutMax)
	}
}

func TestCGAApplyWritesConfig(t *testing.T) {
	ws := t.TempDir()
	prev := cgaGetwd
	cgaGetwd = func() (string, error) { return ws, nil }
	t.Cleanup(func() { cgaGetwd = prev })

	s := newCGA(services{state: state.NewStore(t.TempDir())}).(*cgaScreen)
	s.enabled = true
	s.cursor = rowCGAApply

	_, cmd := s.Update(key("enter"))
	if cmd == nil {
		t.Fatal("enter on Apply should return a command")
	}
	msg := cmd()
	next, _ := s.Update(msg)
	if next.(*cgaScreen).state != cgaDone {
		t.Fatalf("apply should reach done state, got %d", next.(*cgaScreen).state)
	}
	hook, err := os.ReadFile(filepath.Join(ws, ".git", "hooks", "pre-commit"))
	if err != nil {
		t.Fatalf("pre-commit hook should be written on apply: %v", err)
	}
	if !strings.Contains(string(hook), cga.MarkerStart) {
		t.Errorf("pre-commit hook missing CGA marker:\n%s", hook)
	}
}

func TestCGABackGoesToMenu(t *testing.T) {
	s := newCGA(services{}).(*cgaScreen)
	_, cmd := s.Update(key("esc"))
	if cmd == nil {
		t.Fatal("esc should return a command")
	}
	if _, ok := cmd().(backMsg); !ok {
		t.Error("esc should emit backMsg")
	}
}

func TestCGAHydratesFromState(t *testing.T) {
	store := state.NewStore(t.TempDir())
	if err := store.SetCGA(&state.CGARecord{Enabled: true, StrictMode: false, Timeout: 90}); err != nil {
		t.Fatal(err)
	}
	s := newCGA(services{state: store}).(*cgaScreen)
	if !s.enabled {
		t.Error("screen should hydrate enabled from state")
	}
	if s.strict {
		t.Error("screen should hydrate strict mode from state")
	}
	if s.timeout != 90 {
		t.Errorf("timeout = %d, want hydrated 90", s.timeout)
	}
}

func TestCGAHydratesDefaultTimeoutWhenZero(t *testing.T) {
	store := state.NewStore(t.TempDir())
	if err := store.SetCGA(&state.CGARecord{Enabled: true, StrictMode: true, Timeout: 0}); err != nil {
		t.Fatal(err)
	}
	s := newCGA(services{state: store}).(*cgaScreen)
	if s.timeout != cgaTimeoutDefault {
		t.Errorf("timeout = %d, want default %d when state carries a zero timeout", s.timeout, cgaTimeoutDefault)
	}
}
