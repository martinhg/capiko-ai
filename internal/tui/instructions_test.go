package tui

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/martinhg/capiko-ai/internal/backup"
	"github.com/martinhg/capiko-ai/internal/copilot"
)

func TestInstructionsInstallWritesFilesAndBacksUp(t *testing.T) {
	cfgDir := t.TempDir()
	svc := services{
		host:   &copilot.Host{ConfigDir: cfgDir},
		backup: backup.NewStore(t.TempDir()),
	}
	s := newInstructions(svc).(*instructionsScreen)
	if len(s.items) == 0 {
		t.Fatal("no scoped instructions loaded")
	}

	_, cmd := s.Update(key("y")) // confirm
	if s.state != instrApplying {
		t.Fatalf("state = %d, want instrApplying", s.state)
	}
	msg, ok := cmd().(instructionsInstalledMsg)
	if !ok || msg.err != nil {
		t.Fatalf("install failed: %+v", msg)
	}
	if msg.count != len(s.items) {
		t.Errorf("count = %d, want %d", msg.count, len(s.items))
	}

	// files landed in ~/.copilot/instructions/
	if _, err := os.Stat(filepath.Join(cfgDir, "instructions", "go.instructions.md")); err != nil {
		t.Errorf("go.instructions.md not written: %v", err)
	}

	// a backup was taken
	backups, _ := svc.backup.List()
	if len(backups) == 0 || backups[0].Reason != "instructions" {
		t.Errorf("expected an instructions backup, got %+v", backups)
	}

	// reaching done, any key goes back
	s.Update(msg)
	_, c := s.Update(key("enter"))
	if _, ok := c().(backMsg); !ok {
		t.Error("done screen should go back on any key")
	}
}

func TestInstructionsQuitGoesBack(t *testing.T) {
	s := newInstructions(services{host: &copilot.Host{ConfigDir: t.TempDir()}}).(*instructionsScreen)
	_, cmd := s.Update(key("q"))
	if _, ok := cmd().(backMsg); !ok {
		t.Error("q should emit backMsg")
	}
}
