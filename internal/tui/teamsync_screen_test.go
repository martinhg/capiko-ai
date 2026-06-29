package tui

import (
	"strings"
	"testing"

	"github.com/martinhg/capiko-ai/internal/state"
)

// ============================================================================
// teamSyncScreen — Update()-driven tests (WU-6 T-12)
// ============================================================================

// withStubEngramDetected swaps the engramDetected seam for screen tests and
// restores it automatically when the test ends.
func withStubEngramDetected(t *testing.T, detected bool) {
	t.Helper()
	prev := engramDetected
	engramDetected = func() bool { return detected }
	t.Cleanup(func() { engramDetected = prev })
}

// withStubTeamSyncGetwd swaps the teamSyncGetwd seam and restores it
// automatically when the test ends.
func withStubTeamSyncGetwd(t *testing.T, ws string) {
	t.Helper()
	prev := teamSyncGetwd
	teamSyncGetwd = func() (string, error) { return ws, nil }
	t.Cleanup(func() { teamSyncGetwd = prev })
}

// buildTeamSync builds a hermetic teamSyncScreen directly (no real detection,
// no real state, no backup — for pure navigation / toggle / state-machine tests).
func buildTeamSync() *teamSyncScreen {
	return &teamSyncScreen{}
}

// ---------------------------------------------------------------------------
// Cursor navigation
// ---------------------------------------------------------------------------

func TestTeamSyncCursorBoundsDown(t *testing.T) {
	s := buildTeamSync()
	// Drive cursor past the last row.
	for i := 0; i < teamSyncRows+5; i++ {
		s.Update(key("down"))
	}
	if s.cursor != teamSyncRows-1 {
		t.Errorf("cursor = %d past last row, want clamped at %d", s.cursor, teamSyncRows-1)
	}
}

func TestTeamSyncCursorBoundsUp(t *testing.T) {
	s := buildTeamSync()
	s.Update(key("up")) // already at 0
	if s.cursor != 0 {
		t.Errorf("cursor = %d at top, want clamped at 0", s.cursor)
	}
}

// ---------------------------------------------------------------------------
// Toggle rows
// ---------------------------------------------------------------------------

func TestTeamSyncToggleEnabled(t *testing.T) {
	s := buildTeamSync()
	s.cursor = rowTeamSyncEnabled
	before := s.enabled
	s.Update(key("space"))
	if s.enabled == before {
		t.Error("space on Enabled row should toggle enabled")
	}
}

func TestTeamSyncToggleAck(t *testing.T) {
	s := buildTeamSync()
	s.cursor = rowTeamSyncAck
	before := s.ack
	s.Update(key("space"))
	if s.ack == before {
		t.Error("space on Ack row should toggle ack")
	}
}

// ---------------------------------------------------------------------------
// Ack gate
// ---------------------------------------------------------------------------

func TestTeamSyncAckGateBlocksApply(t *testing.T) {
	s := buildTeamSync()
	s.engramAvailable = true
	s.enabled = true
	s.ack = false
	s.cursor = rowTeamSyncApply

	_, cmd := s.Update(key("enter"))
	if cmd != nil {
		t.Error("Apply with enabled=true and ack=false should emit no command (ack gate)")
	}
	if s.state != teamSyncEditing {
		t.Errorf("state = %v, want teamSyncEditing when ack gate blocks apply", s.state)
	}
	// View() must communicate that acknowledgment is required.
	if !strings.Contains(s.View(), "cknowledg") {
		t.Errorf("View() should contain an ack-required hint when gate blocks:\n%s", s.View())
	}
}

func TestTeamSyncAckGateAllowsApply(t *testing.T) {
	withStubTeamSyncGetwd(t, t.TempDir())

	s := buildTeamSync()
	s.svc = services{state: state.NewStore(t.TempDir())}
	s.enabled = true
	s.ack = true
	s.cursor = rowTeamSyncApply

	_, cmd := s.Update(key("enter"))
	if cmd == nil {
		t.Fatal("Apply with enabled=true and ack=true should emit a command")
	}
	if s.state != teamSyncApplying {
		t.Errorf("state = %v, want teamSyncApplying after apply is triggered", s.state)
	}
}

func TestTeamSyncApplyWithoutAckWhenDisabling(t *testing.T) {
	// Disabling must NOT require ack (REQ-5.4).
	withStubTeamSyncGetwd(t, t.TempDir())

	s := buildTeamSync()
	s.svc = services{state: state.NewStore(t.TempDir())}
	s.enabled = false
	s.ack = false
	s.cursor = rowTeamSyncApply

	_, cmd := s.Update(key("enter"))
	if cmd == nil {
		t.Fatal("Apply with enabled=false should emit a command even without ack")
	}
}

// ---------------------------------------------------------------------------
// Back row
// ---------------------------------------------------------------------------

func TestTeamSyncBackEmitsBackMsg(t *testing.T) {
	s := buildTeamSync()
	s.cursor = rowTeamSyncBack

	_, cmd := s.Update(key("enter"))
	if cmd == nil {
		t.Fatal("enter on Back should emit a command")
	}
	if _, ok := cmd().(backMsg); !ok {
		t.Error("enter on Back should emit backMsg")
	}
}

// ---------------------------------------------------------------------------
// Applied message state transitions
// ---------------------------------------------------------------------------

func TestTeamSyncAppliedMsgSuccess(t *testing.T) {
	s := buildTeamSync()
	s.Update(teamSyncAppliedMsg{err: nil})
	if s.state != teamSyncDone {
		t.Errorf("state = %v, want teamSyncDone after successful apply", s.state)
	}
}

func TestTeamSyncAppliedMsgFailure(t *testing.T) {
	s := buildTeamSync()
	s.Update(teamSyncAppliedMsg{err: errTest})
	if s.state != teamSyncFailed {
		t.Errorf("state = %v, want teamSyncFailed after failed apply", s.state)
	}
}

// ---------------------------------------------------------------------------
// Seam-driven view assertions
// ---------------------------------------------------------------------------

func TestTeamSyncViewShowsInstallHintWhenEngramAbsent(t *testing.T) {
	s := buildTeamSync()
	s.engramAvailable = false
	view := s.View()
	if !strings.Contains(view, "engram") || !strings.Contains(view, "PATH") {
		t.Errorf("absent engram should show install hint in View():\n%s", view)
	}
}

func TestTeamSyncViewNoInstallHintWhenEngramPresent(t *testing.T) {
	s := buildTeamSync()
	s.engramAvailable = true
	view := s.View()
	if strings.Contains(view, "not on PATH") {
		t.Errorf("present engram should not show install hint:\n%s", view)
	}
}

func TestTeamSyncConflictBannerShowsManualCommands(t *testing.T) {
	s := buildTeamSync()
	s.engramAvailable = true
	s.conflictReason = "husky is configured (.husky/ directory found)"
	s.conflictProject = "my-team"
	view := s.View()
	if !strings.Contains(view, "engram sync --import") {
		t.Errorf("conflict banner must show post-merge command:\n%s", view)
	}
	if !strings.Contains(view, "engram sync --project") {
		t.Errorf("conflict banner must show pre-push command:\n%s", view)
	}
}

// ---------------------------------------------------------------------------
// newTeamSync — hydrates from state
// ---------------------------------------------------------------------------

func TestTeamSyncHydratesFromState(t *testing.T) {
	withStubEngramDetected(t, true)
	withStubTeamSyncGetwd(t, t.TempDir())
	// Stub conflict detection to avoid real filesystem checks.
	origConflict := teamSyncDetectConflict
	teamSyncDetectConflict = func(_ string) string { return "" }
	t.Cleanup(func() { teamSyncDetectConflict = origConflict })

	store := state.NewStore(t.TempDir())
	if err := store.SetTeamSync(&state.TeamSyncRecord{Enabled: true}); err != nil {
		t.Fatal(err)
	}
	s := newTeamSync(services{state: store}).(*teamSyncScreen)
	if !s.enabled {
		t.Error("screen should hydrate enabled from state")
	}
	// Ack always resets to false on construction (REQ-5.5).
	if s.ack {
		t.Error("ack should always reset to false on screen construction")
	}
}
