package tui

import (
	"strings"
	"testing"
)

func TestSoonAnyKeyGoesBack(t *testing.T) {
	s := newSoon("Future feature")
	_, cmd := s.Update(key("enter"))
	if cmd == nil {
		t.Fatal("a key press should emit a command")
	}
	if _, ok := cmd().(backMsg); !ok {
		t.Error("any key on the soon screen should go back to the menu")
	}
}

func TestSoonNonKeyMsgIsInert(t *testing.T) {
	s := newSoon("Future feature")
	next, cmd := s.Update(backMsg{}) // a non-key message
	if cmd != nil {
		t.Error("non-key messages should not emit a command")
	}
	if next.View() != s.View() {
		t.Error("non-key messages should not change the screen")
	}
}

func TestSoonViewShowsTitleAndPlaceholder(t *testing.T) {
	out := newSoon("Future feature").View()
	if !strings.Contains(out, "Future feature") {
		t.Errorf("view should show the title, got:\n%s", out)
	}
	if !strings.Contains(out, "Coming soon.") {
		t.Errorf("view should show the placeholder, got:\n%s", out)
	}
}
