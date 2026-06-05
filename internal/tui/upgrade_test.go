package tui

import "testing"

func TestUpgradeUpToDateWhenNoLatest(t *testing.T) {
	s := newUpgrade("").(*upgradeScreen)
	if s.state != upgradeUpToDate {
		t.Fatalf("state = %d, want upgradeUpToDate", s.state)
	}
	_, cmd := s.Update(key("enter"))
	if cmd == nil {
		t.Fatal("any key should go back from up-to-date")
	}
	if _, ok := cmd().(backMsg); !ok {
		t.Error("up-to-date screen should emit backMsg on any key")
	}
}

func TestUpgradeConfirmStartsApply(t *testing.T) {
	s := newUpgrade("9.9.9").(*upgradeScreen)
	if s.state != upgradeConfirm {
		t.Fatalf("state = %d, want upgradeConfirm", s.state)
	}
	_, cmd := s.Update(key("y"))
	if s.state != upgradeApplying {
		t.Errorf("state = %d, want upgradeApplying", s.state)
	}
	if cmd == nil {
		t.Error("confirming should kick off the upgrade command")
	}
}

func TestUpgradeSuccessSignalsRestart(t *testing.T) {
	s := newUpgrade("9.9.9").(*upgradeScreen)
	_, cmd := s.Update(upgradedMsg{})
	if s.state != upgradeDone {
		t.Fatalf("state = %d, want upgradeDone", s.state)
	}
	if cmd == nil {
		t.Fatal("success should emit a restart command")
	}
	if _, ok := cmd().(restartMsg); !ok {
		t.Error("success should bubble a restartMsg")
	}
}

func TestUpgradeFailureGoesBack(t *testing.T) {
	s := newUpgrade("9.9.9").(*upgradeScreen)
	s.Update(upgradedMsg{err: errTest})
	if s.state != upgradeFailed {
		t.Fatalf("state = %d, want upgradeFailed", s.state)
	}
	_, cmd := s.Update(key("enter"))
	if _, ok := cmd().(backMsg); !ok {
		t.Error("failed screen should go back on any key")
	}
}

func TestUpgradeQuitGoesBack(t *testing.T) {
	s := newUpgrade("9.9.9").(*upgradeScreen)
	_, cmd := s.Update(key("q"))
	if _, ok := cmd().(backMsg); !ok {
		t.Error("q should emit backMsg")
	}
}

func TestRestartMsgQuitsWithFlag(t *testing.T) {
	next, cmd := App{state: appScreen}.Update(restartMsg{})
	app := next.(App)
	if !app.ShouldRestart() {
		t.Error("restartMsg should set the restart flag")
	}
	if cmd == nil {
		t.Fatal("restartMsg should quit the program")
	}
}

// errTest is a sentinel error for table tests.
var errTest = testError("boom")

type testError string

func (e testError) Error() string { return string(e) }
