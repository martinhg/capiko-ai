package tui

import (
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/martinhg/capiko-ai/internal/persona"
	"github.com/martinhg/capiko-ai/internal/skill"
)

// personaScreen lets the user pick the persona capiko writes into Copilot's
// global instructions file. Selecting one applies it and continues to the skill
// selector; Back returns to the menu.
type personaScreen struct {
	svc       services
	catalog   []skill.Skill
	installed map[string]bool
	personas  []persona.Persona
	cursor    int
	state     personaState
	err       error
}

type personaState int

const (
	personaPicking personaState = iota
	personaApplying
	personaFailed
)

type personaAppliedMsg struct{ err error }

func newPersona(svc services, catalog []skill.Skill, installed map[string]bool) screen {
	return &personaScreen{
		svc:       svc,
		catalog:   catalog,
		installed: installed,
		personas:  persona.Available(),
	}
}

func (s *personaScreen) Update(msg tea.Msg) (screen, tea.Cmd) {
	switch msg := msg.(type) {
	case personaAppliedMsg:
		if msg.err != nil {
			s.state, s.err = personaFailed, msg.err
			return s, nil
		}
		// Persona applied — continue to the skill selector.
		return newInstall(s.svc, s.catalog, s.installed), nil
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "esc":
			return s, back
		}
		if s.state == personaFailed {
			return s, back
		}
		switch msg.String() {
		case "up", "k":
			if s.cursor > 0 {
				s.cursor--
			}
		case "down", "j":
			if s.cursor < len(s.personas)-1 {
				s.cursor++
			}
		case "enter":
			s.state = personaApplying
			return s, s.applyCmd(s.personas[s.cursor])
		}
	}
	return s, nil
}

func (s *personaScreen) applyCmd(p persona.Persona) tea.Cmd {
	host, bkp := s.svc.host, s.svc.backup
	return func() tea.Msg {
		if host == nil {
			return personaAppliedMsg{} // nowhere to write; treat as no-op
		}
		path := filepath.Join(host.ConfigDir, "copilot-instructions.md")
		backupRoot := ""
		if bkp != nil {
			backupRoot = bkp.Dir()
		}
		return personaAppliedMsg{err: persona.Apply(path, backupRoot, p)}
	}
}

func (s *personaScreen) View() string {
	var b strings.Builder
	b.WriteString(titleSty.Render("Choose your Persona") + "\n\n")
	b.WriteString(dimSty.Render("Your own Capiko! teaches before it solves.") + "\n\n")

	switch s.state {
	case personaApplying:
		b.WriteString("Applying persona…\n")
		return b.String()
	case personaFailed:
		b.WriteString(errSty.Render("Error: "+s.err.Error()) + "\n\n")
		b.WriteString(dimSty.Render("any key to go back") + "\n")
		return b.String()
	}

	for i, p := range s.personas {
		marker := "  "
		nameSty := textSty
		if i == s.cursor {
			marker = titleSty.Render(menuCursor)
			nameSty = titleSty
		}
		b.WriteString(marker + nameSty.Render(p.Name) + "\n")
		b.WriteString(dimSty.Render("    "+p.Description) + "\n")
	}

	b.WriteString("\n" + dimSty.Render("↑/↓ move · enter select · esc back") + "\n")
	return b.String()
}
