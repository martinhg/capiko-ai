package tui

import (
	"testing"

	"github.com/martinhg/capiko-ai/internal/copilot"
)

func newDetectionT(t *testing.T) *detectionScreen {
	t.Helper()
	svc := services{host: &copilot.Host{SkillsDir: t.TempDir()}}
	s, ok := newDetection(svc, testCatalog(), map[string]bool{}).(*detectionScreen)
	if !ok {
		t.Fatal("newDetection did not return *detectionScreen")
	}
	return s
}

func TestDetectionContinueOpensInstall(t *testing.T) {
	s := newDetectionT(t)
	// cursor starts at 0 = Continue
	next, _ := s.Update(key("enter"))
	if _, ok := next.(*selector); !ok {
		t.Errorf("Continue should open the install selector, got %T", next)
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
