package tui

import (
	"testing"

	"github.com/martinhg/capiko-ai/internal/copilot"
	"github.com/martinhg/capiko-ai/internal/sysinfo"
)

// newDetectionT builds the screen directly (no real sysinfo.Detect, which would
// shell out) since these tests only exercise navigation and transitions.
func newDetectionT(t *testing.T) *detectionScreen {
	t.Helper()
	return &detectionScreen{
		svc:       services{host: &copilot.Host{SkillsDir: t.TempDir()}},
		catalog:   testCatalog(),
		installed: map[string]bool{},
	}
}

func TestDetectionContinueOpensPersona(t *testing.T) {
	s := newDetectionT(t)
	// cursor starts at 0 = Continue
	next, _ := s.Update(key("enter"))
	if _, ok := next.(*personaScreen); !ok {
		t.Errorf("Continue should open the persona screen, got %T", next)
	}
}

func TestDetectionBackGoesToMenu(t *testing.T) {
	s := newDetectionT(t)
	s.cursor = 1 // Back
	_, cmd := s.Update(key("enter"))
	if _, ok := cmd().(backMsg); !ok {
		t.Error("Back should emit backMsg")
	}
}

func TestDetectionQuitGoesToMenu(t *testing.T) {
	s := newDetectionT(t)
	_, cmd := s.Update(key("esc"))
	if _, ok := cmd().(backMsg); !ok {
		t.Error("esc should emit backMsg")
	}
}

func TestDetectionInstallMissingOption(t *testing.T) {
	s := &detectionScreen{
		report: sysinfo.Report{Dependencies: []sysinfo.Dependency{
			{Name: "pnpm", Required: true, Found: false, Install: "brew install pnpm", Auto: true},
		}},
	}
	if opts := s.options(); opts[0] != "Install missing" {
		t.Fatalf("options = %v, want Install missing first", opts)
	}
	// cursor 0 = Install missing; enter kicks off the install (don't run it here)
	_, cmd := s.Update(key("enter"))
	if !s.installing {
		t.Error("enter on Install missing should set installing")
	}
	if cmd == nil {
		t.Error("Install missing should return a command")
	}
}

func TestDetectionDepsInstalledClearsInstalling(t *testing.T) {
	s := newDetectionT(t)
	s.installing = true
	next, _ := s.Update(depsInstalledMsg{summary: "installed pnpm"})
	ds := next.(*detectionScreen)
	if ds.installing {
		t.Error("depsInstalledMsg should clear installing")
	}
	if ds.status != "installed pnpm" {
		t.Errorf("status = %q, want installed pnpm", ds.status)
	}
}

func TestDetectionCursorClamps(t *testing.T) {
	s := newDetectionT(t)
	s.Update(key("up")) // already at 0
	if s.cursor != 0 {
		t.Errorf("cursor = %d, want 0", s.cursor)
	}
	s.Update(key("down"))
	s.Update(key("down")) // clamp at 1
	if s.cursor != 1 {
		t.Errorf("cursor = %d, want 1", s.cursor)
	}
}
