package tui

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/martinhg/capiko-ai/internal/backup"
	"github.com/martinhg/capiko-ai/internal/copilot"
	"github.com/martinhg/capiko-ai/internal/state"
)

func newPersonaT(t *testing.T) (*personaScreen, services, string) {
	t.Helper()
	cfgDir := t.TempDir()
	svc := services{
		host:   &copilot.Host{ConfigDir: cfgDir, SkillsDir: filepath.Join(cfgDir, "skills")},
		state:  state.NewStore(t.TempDir()),
		backup: backup.NewStore(t.TempDir()),
	}
	s, ok := newPersona(svc, testCatalog(), map[string]bool{}).(*personaScreen)
	if !ok {
		t.Fatal("newPersona did not return *personaScreen")
	}
	return s, svc, filepath.Join(cfgDir, "copilot-instructions.md")
}

func TestPersonaApplyWritesRecordsBacksUpThenOpensInstall(t *testing.T) {
	s, svc, path := newPersonaT(t)

	// cursor 0 = Capiko; enter kicks off apply
	_, cmd := s.Update(key("enter"))
	if s.state != personaApplying {
		t.Fatalf("state = %d, want personaApplying", s.state)
	}
	msg := cmd()
	applied, ok := msg.(personaAppliedMsg)
	if !ok || applied.err != nil {
		t.Fatalf("apply failed: %+v", msg)
	}

	// the instructions file now carries the persona block
	data, err := os.ReadFile(path)
	if err != nil || len(data) == 0 {
		t.Fatalf("instructions not written: %v", err)
	}

	// the choice is recorded in state
	st, _ := svc.state.Load()
	if st.Persona != "capiko" {
		t.Errorf("state persona = %q, want capiko", st.Persona)
	}

	// a backup of the (absent) prior file was taken
	backups, _ := svc.backup.List()
	if len(backups) == 0 || backups[0].Reason != "persona" {
		t.Errorf("expected a persona backup, got %+v", backups)
	}

	// feeding the applied msg back transitions to the install selector
	next, _ := s.Update(applied)
	if _, ok := next.(*selector); !ok {
		t.Errorf("after apply should open the selector, got %T", next)
	}
}

func TestPersonaQuitGoesBack(t *testing.T) {
	s, _, _ := newPersonaT(t)
	_, cmd := s.Update(key("esc"))
	if _, ok := cmd().(backMsg); !ok {
		t.Error("esc should emit backMsg")
	}
}

func TestPersonaCursorClamps(t *testing.T) {
	s, _, _ := newPersonaT(t)
	s.Update(key("up")) // at top
	if s.cursor != 0 {
		t.Errorf("cursor = %d, want 0", s.cursor)
	}
	for range len(s.personas) + 2 {
		s.Update(key("down"))
	}
	if want := len(s.personas) - 1; s.cursor != want {
		t.Errorf("cursor = %d, want %d", s.cursor, want)
	}
}

func TestPersonaFailureGoesBack(t *testing.T) {
	s, _, _ := newPersonaT(t)
	s.Update(personaAppliedMsg{err: errTest})
	if s.state != personaFailed {
		t.Fatalf("state = %d, want personaFailed", s.state)
	}
	_, cmd := s.Update(key("enter"))
	if _, ok := cmd().(backMsg); !ok {
		t.Error("failed screen should go back on any key")
	}
}
