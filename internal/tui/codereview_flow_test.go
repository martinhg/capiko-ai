package tui

import (
	"strings"
	"testing"

	"github.com/martinhg/capiko-ai/internal/copilot"
)

func installSelector(t *testing.T) *selector {
	t.Helper()
	s, ok := newInstall(services{host: &copilot.Host{SkillsDir: t.TempDir()}}, testCatalog(), map[string]bool{}).(*selector)
	if !ok {
		t.Fatal("newInstall did not return *selector")
	}
	return s
}

func TestInstallDoneOffersCodeReview(t *testing.T) {
	s := installSelector(t)
	s.state = selDone
	next, _ := s.Update(key("c"))
	if _, ok := next.(*codeReviewScreen); !ok {
		t.Errorf("c on the install-done screen should open code review, got %T", next)
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

func TestUninstallDoneDoesNotOfferCodeReview(t *testing.T) {
	s, _ := newUninstall(services{host: &copilot.Host{SkillsDir: t.TempDir()}}, testCatalog(), map[string]bool{"capiko-hello": true}).(*selector)
	s.state = selDone
	next, cmd := s.Update(key("c"))
	if _, ok := next.(*codeReviewScreen); ok {
		t.Error("uninstall flow must not offer code review")
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
