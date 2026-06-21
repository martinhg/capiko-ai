package tui

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/martinhg/capiko-ai/internal/copilot"
	"github.com/martinhg/capiko-ai/internal/state"
	"github.com/martinhg/capiko-ai/internal/sysinfo"
)

func TestDetectionShowsEngramStatus(t *testing.T) {
	configured := &detectionScreen{
		report: sysinfo.Report{},
		engram: &state.EngramRecord{Enabled: true, ArtifactMode: "hybrid", CloudServer: "https://e.example.com"},
	}
	v := configured.View()
	if !strings.Contains(v, "Engram") || !strings.Contains(v, "hybrid") || !strings.Contains(v, "https://e.example.com") {
		t.Errorf("detection should show the configured engram status:\n%s", v)
	}

	unmanaged := &detectionScreen{report: sysinfo.Report{}}
	if !strings.Contains(unmanaged.View(), "not configured") {
		t.Error("detection should show engram as not configured when unmanaged")
	}
}

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

// TestInstallCmdReportsResults drives installCmd's command directly (no async
// teatest needed): with the install seam stubbed, one dep succeeds and one fails,
// so the resulting summary must report both outcomes joined together.
func TestInstallCmdReportsResults(t *testing.T) {
	prev := installDep
	installDep = func(d sysinfo.Dependency) error {
		if d.Name == "pnpm" {
			return nil
		}
		return errors.New("boom")
	}
	t.Cleanup(func() { installDep = prev })

	s := &detectionScreen{report: sysinfo.Report{Dependencies: []sysinfo.Dependency{
		{Name: "pnpm", Found: false, Auto: true, Install: "brew install pnpm"},
		{Name: "fd", Found: false, Auto: true, Install: "brew install fd"},
	}}}

	msg, ok := s.installCmd()().(depsInstalledMsg)
	if !ok {
		t.Fatalf("installCmd did not return depsInstalledMsg")
	}
	if !strings.Contains(msg.summary, "installed pnpm") {
		t.Errorf("summary missing success: %q", msg.summary)
	}
	if !strings.Contains(msg.summary, "failed fd") {
		t.Errorf("summary missing failure: %q", msg.summary)
	}
}

// TestInstallCmdAllSucceed covers the success-only branch: no "failed" segment
// appears when every dependency installs cleanly.
func TestInstallCmdAllSucceed(t *testing.T) {
	prev := installDep
	installDep = func(sysinfo.Dependency) error { return nil }
	t.Cleanup(func() { installDep = prev })

	s := &detectionScreen{report: sysinfo.Report{Dependencies: []sysinfo.Dependency{
		{Name: "pnpm", Found: false, Auto: true, Install: "brew install pnpm"},
	}}}

	msg := s.installCmd()().(depsInstalledMsg)
	if msg.summary != "installed pnpm" {
		t.Errorf("summary = %q, want %q", msg.summary, "installed pnpm")
	}
}

// TestLoadEngramRecord covers all three branches: nil store, an unreadable store
// (corrupt state.json), and a store with managed engram config.
func TestLoadEngramRecord(t *testing.T) {
	if loadEngramRecord(nil) != nil {
		t.Error("nil store should yield nil record")
	}

	badDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(badDir, "state.json"), []byte("{not json"), 0o644); err != nil {
		t.Fatal(err)
	}
	if loadEngramRecord(state.NewStore(badDir)) != nil {
		t.Error("unreadable state should yield nil record")
	}

	store := state.NewStore(t.TempDir())
	if err := store.SetEngram(&state.EngramRecord{Enabled: true, ArtifactMode: "hybrid"}); err != nil {
		t.Fatal(err)
	}
	rec := loadEngramRecord(store)
	if rec == nil || !rec.Enabled || rec.ArtifactMode != "hybrid" {
		t.Errorf("loadEngramRecord = %+v, want managed enabled record", rec)
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
