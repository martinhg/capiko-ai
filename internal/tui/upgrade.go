package tui

import (
	"context"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/martinhg/capiko-ai/internal/release"
)

// upgradeScreen drives the self-update flow for the "Upgrade tools" menu item:
// confirm, run the upgrade off the UI thread, then signal a restart so the new
// binary takes over.
type upgradeScreen struct {
	current string
	latest  string // empty when already up to date or unknown
	state   upgradeState
	err     error
}

type upgradeState int

const (
	upgradeConfirm upgradeState = iota
	upgradeUpToDate
	upgradeApplying
	upgradeDone
	upgradeFailed
)

// upgradedMsg reports the result of the async upgrade.
type upgradedMsg struct{ err error }

// restartMsg bubbles up to the App, which quits the program so main can re-exec
// into the freshly installed binary.
type restartMsg struct{}

func newUpgrade(latest string) screen {
	s := &upgradeScreen{current: Version, latest: latest}
	if latest == "" {
		s.state = upgradeUpToDate
	}
	return s
}

func (s *upgradeScreen) Update(msg tea.Msg) (screen, tea.Cmd) {
	switch msg := msg.(type) {
	case upgradedMsg:
		if msg.err != nil {
			s.state, s.err = upgradeFailed, msg.err
			return s, nil
		}
		s.state = upgradeDone
		return s, func() tea.Msg { return restartMsg{} }
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "esc", "n":
			return s, back
		}
		if s.state == upgradeConfirm && (msg.String() == "y" || msg.String() == "enter") {
			s.state = upgradeApplying
			return s, s.upgradeCmd()
		}
		if s.state == upgradeUpToDate || s.state == upgradeFailed {
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

func (s *upgradeScreen) View() string {
	var b strings.Builder
	b.WriteString(titleSty.Render("Upgrade capiko-ai") + "\n\n")

	switch s.state {
	case upgradeUpToDate:
		b.WriteString(okSty.Render("You're on the latest version ("+s.current+") ✓") + "\n\n")
		b.WriteString(dimSty.Render("any key to go back") + "\n")
	case upgradeApplying:
		b.WriteString("Updating " + s.current + " → " + s.latest + "…\n")
	case upgradeDone:
		b.WriteString(okSty.Render("Updated to "+s.latest+" ✓ — restarting…") + "\n")
	case upgradeFailed:
		b.WriteString(errSty.Render("Error: "+s.err.Error()) + "\n\n")
		b.WriteString(dimSty.Render("any key to go back") + "\n")
	default:
		b.WriteString(textSty.Render("A new version is available: "+s.current+" → "+s.latest) + "\n\n")
		b.WriteString(dimSty.Render("[y to update · q to go back]") + "\n")
	}
	return b.String()
}
