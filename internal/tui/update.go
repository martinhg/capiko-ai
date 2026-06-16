package tui

import (
	"context"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/martinhg/capiko-ai/internal/release"
	"github.com/martinhg/capiko-ai/internal/state"
)

const (
	releaseCheckTimeout = 5 * time.Second
	updateCheckCooldown = 6 * time.Hour
)

// latestVersionMsg carries a newer release version, or an empty string when the
// build is up to date or the check failed. Either way the banner only shows when
// version is non-empty.
type latestVersionMsg struct{ version string }

// withinCooldown reports whether the last successful update check is recent
// enough to skip this one. A nil timestamp means "never checked".
func withinCooldown(last *time.Time) bool {
	return last != nil && time.Since(*last) < updateCheckCooldown
}

// checkLatestCmd returns a tea.Cmd that queries GitHub Releases off the UI
// thread, respecting the cooldown persisted in state. A nil store disables the
// cooldown (always checks). Any failure is swallowed: the version check is
// best-effort and must never block startup or surface an error to the user.
func checkLatestCmd(store *state.Store) tea.Cmd {
	return func() tea.Msg {
		if store != nil {
			if st, err := store.Load(); err == nil && withinCooldown(st.LastUpdateCheck) {
				return latestVersionMsg{}
			}
		}

		ctx, cancel := context.WithTimeout(context.Background(), releaseCheckTimeout)
		defer cancel()

		latest, err := release.Latest(ctx)
		if err != nil {
			return latestVersionMsg{}
		}

		if store != nil {
			now := time.Now().UTC()
			_ = store.SetLastUpdateCheck(now)
		}

		if release.IsNewer(Version, latest) {
			return latestVersionMsg{version: latest}
		}
		return latestVersionMsg{}
	}
}
