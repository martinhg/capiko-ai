package tui

import (
	"strings"
	"testing"

	"github.com/martinhg/capiko-ai/internal/copilot"
)

func TestUpgradeUpToDateWhenNoLatest(t *testing.T) {
	s := newUpgrade(services{}, "").(*upgradeScreen)
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
	s := newUpgrade(services{}, "9.9.9").(*upgradeScreen)
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

func TestUpgradeSuccessSignalsRestartWithoutSync(t *testing.T) {
	s := newUpgrade(services{}, "9.9.9").(*upgradeScreen)
	_, cmd := s.Update(upgradedMsg{})
	if s.state != upgradeDone {
		t.Fatalf("state = %d, want upgradeDone", s.state)
	}
	msg, ok := cmd().(restartMsg)
	if !ok {
		t.Fatal("success should bubble a restartMsg")
	}
	if msg.sync {
		t.Error("plain upgrade should not request a post-restart sync")
	}
}

func TestUpgradeFailureGoesBack(t *testing.T) {
	s := newUpgrade(services{}, "9.9.9").(*upgradeScreen)
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
	s := newUpgrade(services{}, "9.9.9").(*upgradeScreen)
	_, cmd := s.Update(key("q"))
	if _, ok := cmd().(backMsg); !ok {
		t.Error("q should emit backMsg")
	}
}

func TestUpgradeSyncSuccessRequestsSync(t *testing.T) {
	s := newUpgradeSync(services{}, testCatalog(), "9.9.9").(*upgradeScreen)
	if s.state != upgradeConfirm {
		t.Fatalf("state = %d, want upgradeConfirm (sync mode never short-circuits)", s.state)
	}
	_, cmd := s.Update(upgradedMsg{})
	msg, ok := cmd().(restartMsg)
	if !ok {
		t.Fatal("success should bubble a restartMsg")
	}
	if !msg.sync {
		t.Error("upgrade+sync should request a post-restart sync")
	}
}

func TestUpgradeSyncWhenUpToDateSyncsInPlace(t *testing.T) {
	svc := services{host: &copilot.Host{SkillsDir: t.TempDir()}}
	s := newUpgradeSync(svc, testCatalog(), "").(*upgradeScreen)
	if s.state != upgradeConfirm {
		t.Fatalf("state = %d, want upgradeConfirm even when up to date", s.state)
	}

	_, cmd := s.Update(key("y"))
	if s.state != upgradeSyncing {
		t.Fatalf("state = %d, want upgradeSyncing", s.state)
	}

	synced, ok := cmd().(syncedMsg)
	if !ok || synced.err != nil {
		t.Fatalf("in-place sync failed: %+v", synced)
	}
	next, _ := s.Update(synced)
	us := next.(*upgradeScreen)
	if us.state != upgradeSynced {
		t.Errorf("state = %d, want upgradeSynced", us.state)
	}
	if us.count != len(testCatalog()) {
		t.Errorf("synced count = %d, want %d", us.count, len(testCatalog()))
	}
}

func TestRestartMsgQuitsWithFlag(t *testing.T) {
	next, cmd := App{state: appScreen}.Update(restartMsg{})
	app := next.(App)
	if !app.ShouldRestart() {
		t.Error("restartMsg should set the restart flag")
	}
	if app.ShouldSyncAfterRestart() {
		t.Error("restartMsg without sync should not request a post-restart sync")
	}
	if cmd == nil {
		t.Fatal("restartMsg should quit the program")
	}
}

func TestRestartMsgWithSyncSetsPostSync(t *testing.T) {
	next, _ := App{state: appScreen}.Update(restartMsg{sync: true})
	if !next.(App).ShouldSyncAfterRestart() {
		t.Error("restartMsg{sync:true} should request a post-restart sync")
	}
}

// errTest is a sentinel error for table tests.
var errTest = testError("boom")

type testError string

func (e testError) Error() string { return string(e) }

func TestUpgradeViewPerState(t *testing.T) {
	tests := []struct {
		name   string
		screen *upgradeScreen
		want   []string
	}{
		{
			name:   "confirm shows version jump and prompt",
			screen: &upgradeScreen{current: "1.0.0", latest: "1.1.0", state: upgradeConfirm},
			want:   []string{"Upgrade capiko-ai", "1.0.0 → 1.1.0", "y to proceed"},
		},
		{
			name:   "up to date",
			screen: &upgradeScreen{current: "1.0.0", state: upgradeUpToDate},
			want:   []string{"latest version (1.0.0)", "any key to go back"},
		},
		{
			name:   "applying shows progress",
			screen: &upgradeScreen{current: "1.0.0", latest: "1.1.0", state: upgradeApplying},
			want:   []string{"Updating 1.0.0 → 1.1.0"},
		},
		{
			name:   "done signals restart",
			screen: &upgradeScreen{latest: "1.1.0", state: upgradeDone},
			want:   []string{"Updated to 1.1.0", "restarting"},
		},
		{
			name:   "synced reports count",
			screen: &upgradeScreen{state: upgradeSynced, count: 3},
			want:   []string{"Synced 3 skill(s)", "any key to go back"},
		},
		{
			name:   "failed shows the error",
			screen: &upgradeScreen{state: upgradeFailed, err: errTest},
			want:   []string{"Error: boom", "any key to go back"},
		},
		{
			name:   "sync confirm when already on latest offers in-place sync",
			screen: &upgradeScreen{current: "1.0.0", withSync: true, state: upgradeConfirm},
			want:   []string{"Upgrade + sync", "Sync skills to this version", "y to proceed"},
		},
		{
			name:   "sync confirm with a newer version offers update and sync",
			screen: &upgradeScreen{current: "1.0.0", latest: "1.1.0", withSync: true, state: upgradeConfirm},
			want:   []string{"Upgrade + sync", "1.0.0 → 1.1.0 and sync skills"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out := tt.screen.View()
			for _, want := range tt.want {
				if !strings.Contains(out, want) {
					t.Errorf("view missing %q, got:\n%s", want, out)
				}
			}
		})
	}
}
