package tui

import (
	"strings"
	"testing"
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
