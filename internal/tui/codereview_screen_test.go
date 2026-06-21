package tui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/martinhg/capiko-ai/internal/state"
)

// withStubGGADetected swaps the gga-availability seam for the screen tests.
func withStubGGADetected(t *testing.T, available bool) {
	t.Helper()
	prev := ggaDetected
	ggaDetected = func() bool { return available }
	t.Cleanup(func() { ggaDetected = prev })
}

func TestCodeReviewToggleEnabled(t *testing.T) {
	withStubGGADetected(t, true)
	s := newCodeReview(services{}).(*codeReviewScreen)
	start := s.enabled
	s.cursor = rowCodeReviewEnabled
	s.Update(key(" "))
	if s.enabled == start {
		t.Error("space on the Enabled row should toggle enabled")
	}
}

func TestCodeReviewCycleProvider(t *testing.T) {
	withStubGGADetected(t, true)
	s := newCodeReview(services{}).(*codeReviewScreen)
	s.cursor = rowCodeReviewProvider
	before := s.providerIdx
	s.Update(key("right"))
	if s.providerIdx == before {
		t.Error("right on the Provider row should cycle the provider")
	}
	s.Update(key("left"))
	if s.providerIdx != before {
		t.Error("left should cycle back to the previous provider")
	}
}

func TestCodeReviewApplyWritesConfig(t *testing.T) {
	withStubGGADetected(t, true)
	stubGGAHooks(t)
	ws := t.TempDir()
	prev := codeReviewGetwd
	codeReviewGetwd = func() (string, error) { return ws, nil }
	t.Cleanup(func() { codeReviewGetwd = prev })

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
	if _, err := os.Stat(filepath.Join(ws, ".gga")); err != nil {
		t.Errorf(".gga should be written on apply: %v", err)
	}
}

func TestCodeReviewBackGoesToMenu(t *testing.T) {
	withStubGGADetected(t, true)
	s := newCodeReview(services{}).(*codeReviewScreen)
	_, cmd := s.Update(key("esc"))
	if cmd == nil {
		t.Fatal("esc should return a command")
	}
	if _, ok := cmd().(backMsg); !ok {
		t.Error("esc should emit backMsg")
	}
}

func TestCodeReviewWarnsWhenGgaMissing(t *testing.T) {
	withStubGGADetected(t, false)
	s := newCodeReview(services{}).(*codeReviewScreen)
	if !strings.Contains(s.View(), "gga") {
		t.Errorf("view should warn when gga is not installed:\n%s", s.View())
	}
}

func TestCodeReviewHydratesFromState(t *testing.T) {
	withStubGGADetected(t, true)
	store := state.NewStore(t.TempDir())
	if err := store.SetCodeReview(&state.CodeReviewRecord{Enabled: true, Provider: "gemini", StrictMode: false}); err != nil {
		t.Fatal(err)
	}
	s := newCodeReview(services{state: store}).(*codeReviewScreen)
	if !s.enabled {
		t.Error("screen should hydrate enabled from state")
	}
	if codeReviewProviders[s.providerIdx] != "gemini" {
		t.Errorf("screen should hydrate provider from state, got %q", codeReviewProviders[s.providerIdx])
	}
}
