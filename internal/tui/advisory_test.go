package tui

import (
	"strings"
	"testing"
)

func TestAdvisoryMsgSetsText(t *testing.T) {
	a := App{state: appMenu}
	next, cmd := a.Update(advisoryMsg{text: "engram 1.17 recommended"})
	if cmd != nil {
		t.Error("advisoryMsg should not emit a command")
	}
	if got := next.(App).advisory; got != "engram 1.17 recommended" {
		t.Errorf("advisory = %q, want message text", got)
	}
}

func TestEmptyAdvisoryMsgKeepsHidden(t *testing.T) {
	next, _ := App{state: appMenu}.Update(advisoryMsg{})
	if got := next.(App).advisory; got != "" {
		t.Errorf("advisory = %q, want empty", got)
	}
}

func TestAdvisoryShownInMenu(t *testing.T) {
	a := App{state: appMenu, advisory: "known issue with Copilot 2.x"}
	view := a.viewMenu()
	if !strings.Contains(view, "Advisory:") {
		t.Error("menu should show Advisory: prefix")
	}
	if !strings.Contains(view, "known issue with Copilot 2.x") {
		t.Error("menu should show advisory text")
	}
}

func TestAdvisoryHiddenWhenEmpty(t *testing.T) {
	a := App{state: appMenu}
	if strings.Contains(a.viewMenu(), "Advisory:") {
		t.Error("menu should not show Advisory: when empty")
	}
}

func TestAdvisoryShownInPrompt(t *testing.T) {
	a := App{state: appUpdatePrompt, latest: "2.0.0", advisory: "upgrade engram too", cursor: promptDefaultCursor}
	view := a.viewPrompt()
	if !strings.Contains(view, "Advisory:") {
		t.Error("prompt should show Advisory: prefix")
	}
	if !strings.Contains(view, "upgrade engram too") {
		t.Error("prompt should show advisory text")
	}
}

func TestCheckAdvisoryCmdReturnsCmd(t *testing.T) {
	cmd := checkAdvisoryCmd()
	if cmd == nil {
		t.Fatal("checkAdvisoryCmd should return a non-nil Cmd")
	}
}
