package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/martinhg/capiko-ai/internal/backup"
)

// backupsScreen lists snapshots and lets the user restore or delete them.
type backupsScreen struct {
	svc    services
	items  []backup.Manifest
	cursor int
	status string // result of the last action
	err    error
}

func newBackups(svc services) screen {
	s := &backupsScreen{svc: svc}
	s.reload()
	return s
}

func (s *backupsScreen) reload() {
	if s.svc.backup == nil {
		s.items = nil
		return
	}
	items, err := s.svc.backup.List()
	s.items, s.err = items, err
	if s.cursor >= len(s.items) {
		s.cursor = max(0, len(s.items)-1)
	}
}

func (s *backupsScreen) Update(msg tea.Msg) (screen, tea.Cmd) {
	key, ok := msg.(tea.KeyMsg)
	if !ok {
		return s, nil
	}
	switch key.String() {
	case "q", "esc":
		return s, back
	case "up", "k":
		if s.cursor > 0 {
			s.cursor--
		}
	case "down", "j":
		if s.cursor < len(s.items)-1 {
			s.cursor++
		}
	case "r":
		s.restore()
	case "d":
		s.delete()
	}
	return s, nil
}

func (s *backupsScreen) restore() {
	if s.svc.backup == nil || len(s.items) == 0 {
		return
	}
	id := s.items[s.cursor].ID
	if err := s.svc.backup.Restore(s.svc.host.SkillsDir, id); err != nil {
		s.err = err
		return
	}
	s.status = "Restored " + id
}

func (s *backupsScreen) delete() {
	if s.svc.backup == nil || len(s.items) == 0 {
		return
	}
	id := s.items[s.cursor].ID
	if err := s.svc.backup.Delete(id); err != nil {
		s.err = err
		return
	}
	s.status = "Deleted " + id
	s.reload()
}

func (s *backupsScreen) View() string {
	var b strings.Builder
	b.WriteString(titleSty.Render("Manage backups") + "\n\n")

	if s.err != nil {
		b.WriteString(errSty.Render("Error: "+s.err.Error()) + "\n\n")
	}
	if s.status != "" {
		b.WriteString(okSty.Render(s.status) + "\n\n")
	}

	if len(s.items) == 0 {
		b.WriteString(dimSty.Render("No backups yet.") + "\n\n")
		b.WriteString(dimSty.Render("q to go back") + "\n")
		return b.String()
	}

	for i, m := range s.items {
		cursor := "  "
		if i == s.cursor {
			cursor = titleSty.Render(menuCursor)
		}
		detail := fmt.Sprintf("%d skills", len(m.Entries))
		if len(m.Files) > 0 {
			detail = fmt.Sprintf("%d file(s)", len(m.Files))
		}
		line := fmt.Sprintf("%s  %s  (%s)",
			m.CreatedAt.Local().Format("2006-01-02 15:04:05"), m.Reason, detail)
		if i == s.cursor {
			b.WriteString(cursor + titleSty.Render(line) + "\n")
		} else {
			b.WriteString(cursor + textSty.Render(line) + "\n")
		}
	}

	b.WriteString("\n" + dimSty.Render("↑/↓ move · r restore · d delete · q back") + "\n")
	return b.String()
}
