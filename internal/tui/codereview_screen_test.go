package tui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/martinhg/capiko-ai/internal/cga"
	"github.com/martinhg/capiko-ai/internal/state"
)

func TestCodeReviewToggleEnabled(t *testing.T) {
	s := newCodeReview(services{}).(*codeReviewScreen)
	start := s.enabled
	s.cursor = rowCodeReviewEnabled
	s.Update(key(" "))
	if s.enabled == start {
		t.Error("space on the Enabled row should toggle enabled")
	}
}

func TestCodeReviewToggleStrict(t *testing.T) {
	s := newCodeReview(services{}).(*codeReviewScreen)
	start := s.strict
	s.cursor = rowCodeReviewStrict
	s.Update(key(" "))
	if s.strict == start {
		t.Error("space on the Strict mode row should toggle strict")
	}
}

func TestCodeReviewApplyWritesConfig(t *testing.T) {
	ws := t.TempDir()
	prev := cgaGetwd
	cgaGetwd = func() (string, error) { return ws, nil }
	t.Cleanup(func() { cgaGetwd = prev })

	s := newCodeReview(services{state: state.NewStore(t.TempDir())}).(*codeReviewScreen)
	s.enabled = true
	s.cursor = rowCodeReviewApply

	_, cmd := s.Update(key("enter"))
	if cmd == nil {
		t.Fatal("enter on Apply should return a command")
	}
	msg := cmd()
	next, _ := s.Update(msg)
	if next.(*codeReviewScreen).state != codeReviewDone {
		t.Fatalf("apply should reach done state, got %d", next.(*codeReviewScreen).state)
	}
	hook, err := os.ReadFile(filepath.Join(ws, ".git", "hooks", "pre-commit"))
	if err != nil {
		t.Fatalf("pre-commit hook should be written on apply: %v", err)
	}
	if !strings.Contains(string(hook), cga.MarkerStart) {
		t.Errorf("pre-commit hook missing CGA marker:\n%s", hook)
	}
}

func TestCodeReviewBackGoesToMenu(t *testing.T) {
	s := newCodeReview(services{}).(*codeReviewScreen)
	_, cmd := s.Update(key("esc"))
	if cmd == nil {
		t.Fatal("esc should return a command")
	}
	if _, ok := cmd().(backMsg); !ok {
		t.Error("esc should emit backMsg")
	}
}

func TestCodeReviewHydratesFromState(t *testing.T) {
	store := state.NewStore(t.TempDir())
	if err := store.SetCGA(&state.CGARecord{Enabled: true, StrictMode: false}); err != nil {
		t.Fatal(err)
	}
	s := newCodeReview(services{state: store}).(*codeReviewScreen)
	if !s.enabled {
		t.Error("screen should hydrate enabled from state")
	}
	if s.strict {
		t.Error("screen should hydrate strict mode from state")
	}
}
