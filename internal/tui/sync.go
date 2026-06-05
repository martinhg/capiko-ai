package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/martinhg/capiko-ai/internal/backup"
	"github.com/martinhg/capiko-ai/internal/copilot"
	"github.com/martinhg/capiko-ai/internal/skill"
	"github.com/martinhg/capiko-ai/internal/state"
)

// RunSync writes every catalog skill to disk (overwriting), snapshotting the
// affected skills first and recording the result in state. It is the headless
// core shared by the Sync screen and the post-upgrade sync in main. A nil store
// or backup degrades gracefully. Returns the number of skills written.
func RunSync(host *copilot.Host, catalog []skill.Skill, store *state.Store, bkp *backup.Store) (int, error) {
	if bkp != nil {
		names := make([]string, len(catalog))
		for i, sk := range catalog {
			names[i] = sk.Name
		}
		if _, err := bkp.Create(host.SkillsDir, "sync", Version, names); err != nil {
			return 0, fmt.Errorf("backup failed, aborting: %w", err)
		}
	}
	recorded := make([]state.Installed, 0, len(catalog))
	for _, sk := range catalog {
		if _, err := sk.Install(host.SkillsDir); err != nil {
			return 0, fmt.Errorf("syncing %s: %w", sk.Name, err)
		}
		recorded = append(recorded, state.Installed{Name: sk.Name, Checksum: state.Checksum(sk.Content)})
	}
	if store != nil {
		if err := store.Apply(Version, recorded, nil); err != nil {
			return 0, fmt.Errorf("recording state: %w", err)
		}
	}
	return len(recorded), nil
}

// syncScreen writes every catalog skill to disk, overwriting so the installed
// skills match the current catalog exactly ("sync configs").
type syncScreen struct {
	svc     services
	catalog []skill.Skill
	state   syncState
	count   int
	err     error
}

type syncState int

const (
	syncConfirm syncState = iota
	syncApplying
	syncDone
	syncFailed
)

type syncedMsg struct {
	count int
	err   error
}

func newSync(svc services, catalog []skill.Skill) screen {
	return &syncScreen{svc: svc, catalog: catalog}
}

func (s *syncScreen) Update(msg tea.Msg) (screen, tea.Cmd) {
	switch msg := msg.(type) {
	case syncedMsg:
		if msg.err != nil {
			s.state, s.err = syncFailed, msg.err
			return s, nil
		}
		s.state, s.count = syncDone, msg.count
		return s, nil
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "esc", "n":
			return s, back
		}
		if s.state == syncConfirm && (msg.String() == "y" || msg.String() == "enter") {
			s.state = syncApplying
			return s, s.syncCmd()
		}
		if s.state == syncDone || s.state == syncFailed {
			return s, back
		}
	}
	return s, nil
}

func (s *syncScreen) syncCmd() tea.Cmd {
	svc, cat := s.svc, s.catalog
	return func() tea.Msg {
		n, err := RunSync(svc.host, cat, svc.state, svc.backup)
		return syncedMsg{count: n, err: err}
	}
}

func (s *syncScreen) View() string {
	var b strings.Builder
	b.WriteString(titleSty.Render("Sync configs") + "\n\n")

	switch s.state {
	case syncApplying:
		b.WriteString("Writing all catalog skills…\n")
	case syncDone:
		b.WriteString(okSty.Render(fmt.Sprintf("Synced %d skill(s) ✓", s.count)) + "\n\n")
		b.WriteString(dimSty.Render("any key to go back") + "\n")
	case syncFailed:
		b.WriteString(errSty.Render("Error: "+s.err.Error()) + "\n\n")
		b.WriteString(dimSty.Render("any key to go back") + "\n")
	default:
		fmt.Fprintf(&b, "Write all %d catalog skill(s) to\n%s,\noverwriting to match the catalog?\n\n", len(s.catalog), s.svc.host.SkillsDir)
		b.WriteString(dimSty.Render("[y to sync · q to go back]") + "\n")
	}
	return b.String()
}
