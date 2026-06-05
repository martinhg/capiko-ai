package tui

import (
	"context"
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/martinhg/capiko-ai/internal/release"
	"github.com/martinhg/capiko-ai/internal/skill"
)

// upgradeScreen drives the self-update flow for the "Upgrade tools" and
// "Upgrade + sync" menu items: confirm, run the upgrade off the UI thread, then
// signal a restart so the new binary takes over. In sync mode it also syncs
// skills — after the restart when the binary changed (so the new catalog is
// used), or in place when already on the latest version.
type upgradeScreen struct {
	svc      services
	catalog  []skill.Skill
	current  string
	latest   string // empty when already up to date or unknown
	withSync bool
	state    upgradeState
	count    int // skills written, in the sync paths
	err      error
}

type upgradeState int

const (
	upgradeConfirm upgradeState = iota
	upgradeUpToDate
	upgradeApplying // binary upgrade running
	upgradeSyncing  // in-place sync running (already on latest)
	upgradeDone     // binary upgraded, restarting
	upgradeSynced   // in-place sync finished, no restart
	upgradeFailed
)

// upgradedMsg reports the result of the async binary upgrade.
type upgradedMsg struct{ err error }

// restartMsg bubbles up to the App, which quits the program so main can re-exec
// into the freshly installed binary. sync requests a post-restart skill sync.
type restartMsg struct{ sync bool }

func newUpgrade(svc services, latest string) screen {
	s := &upgradeScreen{svc: svc, current: Version, latest: latest}
	if latest == "" {
		s.state = upgradeUpToDate
	}
	return s
}

// newUpgradeSync upgrades the binary (if needed) and syncs skills. Unlike the
// plain upgrade it never short-circuits on "up to date" — there is always a sync
// to offer.
func newUpgradeSync(svc services, catalog []skill.Skill, latest string) screen {
	return &upgradeScreen{svc: svc, catalog: catalog, current: Version, latest: latest, withSync: true}
}

func (s *upgradeScreen) Update(msg tea.Msg) (screen, tea.Cmd) {
	switch msg := msg.(type) {
	case upgradedMsg:
		if msg.err != nil {
			s.state, s.err = upgradeFailed, msg.err
			return s, nil
		}
		s.state = upgradeDone
		sync := s.withSync
		return s, func() tea.Msg { return restartMsg{sync: sync} }
	case syncedMsg:
		if msg.err != nil {
			s.state, s.err = upgradeFailed, msg.err
			return s, nil
		}
		s.state, s.count = upgradeSynced, msg.count
		return s, nil
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "esc", "n":
			return s, back
		}
		if s.state == upgradeConfirm && (msg.String() == "y" || msg.String() == "enter") {
			// Already on the latest binary: nothing to upgrade, just sync in place.
			if s.withSync && s.latest == "" {
				s.state = upgradeSyncing
				return s, s.syncCmd()
			}
			s.state = upgradeApplying
			return s, s.upgradeCmd()
		}
		if s.state == upgradeUpToDate || s.state == upgradeSynced || s.state == upgradeFailed {
			return s, back
		}
	}
	return s, nil
}

func (s *upgradeScreen) upgradeCmd() tea.Cmd {
	latest := s.latest
	return func() tea.Msg {
		return upgradedMsg{err: release.Upgrade(context.Background(), latest)}
	}
}

func (s *upgradeScreen) syncCmd() tea.Cmd {
	svc, cat := s.svc, s.catalog
	return func() tea.Msg {
		n, err := RunSync(svc.host, cat, svc.state, svc.backup)
		return syncedMsg{count: n, err: err}
	}
}

func (s *upgradeScreen) View() string {
	var b strings.Builder
	title := "Upgrade capiko-ai"
	if s.withSync {
		title = "Upgrade + sync"
	}
	b.WriteString(titleSty.Render(title) + "\n\n")

	switch s.state {
	case upgradeUpToDate:
		b.WriteString(okSty.Render("You're on the latest version ("+s.current+") ✓") + "\n\n")
		b.WriteString(dimSty.Render("any key to go back") + "\n")
	case upgradeApplying:
		b.WriteString("Updating " + s.current + " → " + s.latest + "…\n")
	case upgradeSyncing:
		b.WriteString("Syncing skills…\n")
	case upgradeDone:
		msg := "Updated to " + s.latest + " ✓ — restarting…"
		if s.withSync {
			msg = "Updated to " + s.latest + " ✓ — restarting & syncing…"
		}
		b.WriteString(okSty.Render(msg) + "\n")
	case upgradeSynced:
		b.WriteString(okSty.Render(fmt.Sprintf("Synced %d skill(s) ✓", s.count)) + "\n\n")
		b.WriteString(dimSty.Render("any key to go back") + "\n")
	case upgradeFailed:
		b.WriteString(errSty.Render("Error: "+s.err.Error()) + "\n\n")
		b.WriteString(dimSty.Render("any key to go back") + "\n")
	default:
		switch {
		case s.withSync && s.latest == "":
			b.WriteString(textSty.Render("You're on the latest ("+s.current+"). Sync skills to this version now?") + "\n\n")
		case s.withSync:
			b.WriteString(textSty.Render("Update "+s.current+" → "+s.latest+" and sync skills?") + "\n\n")
		default:
			b.WriteString(textSty.Render("A new version is available: "+s.current+" → "+s.latest) + "\n\n")
		}
		b.WriteString(dimSty.Render("[y to proceed · q to go back]") + "\n")
	}
	return b.String()
}
