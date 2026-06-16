package tui

import (
	"strings"
	"testing"
	"time"
)

func TestLatestVersionMsgSetsLatest(t *testing.T) {
	next, cmd := App{state: appMenu}.Update(latestVersionMsg{version: "0.2.0"})
	if cmd != nil {
		t.Error("latestVersionMsg should not emit a command")
	}
	if got := next.(App).latest; got != "0.2.0" {
		t.Errorf("latest = %q, want %q", got, "0.2.0")
	}
}

func TestEmptyLatestVersionMsgKeepsBannerHidden(t *testing.T) {
	next, _ := App{state: appMenu}.Update(latestVersionMsg{})
	if got := next.(App).latest; got != "" {
		t.Errorf("latest = %q, want empty", got)
	}
}

func TestUpdateBannerVisibility(t *testing.T) {
	tests := []struct {
		name   string
		latest string
		shown  bool
	}{
		{"hidden when unknown", "", false},
		{"hidden when equal to current", Version, false},
		{"shown when newer", "9.9.9", true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			banner := App{latest: tc.latest}.updateBanner()
			if shown := strings.TrimSpace(banner) != ""; shown != tc.shown {
				t.Errorf("banner shown = %v, want %v (banner=%q)", shown, tc.shown, banner)
			}
		})
	}
}

func TestWithinCooldown(t *testing.T) {
	tests := []struct {
		name string
		last *time.Time
		want bool
	}{
		{"nil means never checked", nil, false},
		{"recent check within cooldown", timePtr(time.Now().Add(-1 * time.Hour)), true},
		{"check at cooldown boundary minus 1s", timePtr(time.Now().Add(-updateCheckCooldown + time.Second)), true},
		{"expired cooldown", timePtr(time.Now().Add(-7 * time.Hour)), false},
		{"far past", timePtr(time.Now().Add(-30 * 24 * time.Hour)), false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := withinCooldown(tc.last); got != tc.want {
				t.Errorf("withinCooldown = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestCheckLatestCmdNilStoreAlwaysChecks(t *testing.T) {
	cmd := checkLatestCmd(nil)
	if cmd == nil {
		t.Fatal("checkLatestCmd(nil) should return a non-nil Cmd")
	}
}

func timePtr(t time.Time) *time.Time { return &t }
