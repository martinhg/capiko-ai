package tui

import (
	"context"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/martinhg/capiko-ai/internal/release"
)

const releaseCheckTimeout = 5 * time.Second

// latestVersionMsg carries a newer release version, or an empty string when the
// build is up to date or the check failed. Either way the banner only shows when
// version is non-empty.
type latestVersionMsg struct{ version string }

// checkLatestCmd queries GitHub Releases off the UI thread. Any failure is
// swallowed on purpose: the version check is best-effort and must never block
// startup or surface an error to the user.
func checkLatestCmd() tea.Msg {
	ctx, cancel := context.WithTimeout(context.Background(), releaseCheckTimeout)
	defer cancel()

	latest, err := release.Latest(ctx)
	if err != nil {
		return latestVersionMsg{}
	}
	if release.IsNewer(Version, latest) {
		return latestVersionMsg{version: latest}
	}
	return latestVersionMsg{}
}
