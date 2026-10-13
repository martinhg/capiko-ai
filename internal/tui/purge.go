package tui

import (
	"fmt"
	"os"
	"strings"

	tea "charm.land/bubbletea/v2"
)

type purgeState int

const (
	purgeConfirm purgeState = iota
	purgeDoing
	purgeDone
	purgeFailed
)

type purgeScreen struct {
	svc    services
	state  purgeState
	report PurgeReport
	err    error
}

func newPurge(svc services) screen {
	return &purgeScreen{svc: svc, state: purgeConfirm}
}

type purgedMsg struct {
	report PurgeReport
	err    error
}

func (s *purgeScreen) Update(msg tea.Msg) (screen, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		switch s.state {
		case purgeConfirm:
			switch msg.String() {
			case "y", "enter":
				s.state = purgeDoing
				svc := s.svc
				return s, func() tea.Msg {
					workspace, _ := os.Getwd()
					rpt, err := PurgeAll(PurgeConfig{
						Host:      svc.host,
						Store:     svc.state,
						Workspace: workspace,
					})
					return purgedMsg{report: rpt, err: err}
				}
			case "n", "esc", "q":
				return s, back
			}
		case purgeDone, purgeFailed:
			return s, back
		}
	case purgedMsg:
		s.report = msg.report
		if msg.err != nil {
			s.err = msg.err
			s.state = purgeFailed
		} else {
			s.state = purgeDone
		}
	}
	return s, nil
}

func (s *purgeScreen) View() string {
	switch s.state {
	case purgeConfirm:
		return s.viewConfirm()
	case purgeDoing:
		return titleSty.Render("Purge capiko") + "\n\n" +
			"Removing all capiko data…\n"
	case purgeDone:
		return s.viewDone()
	case purgeFailed:
		return s.viewFailed()
	}
	return ""
}

func (s *purgeScreen) viewConfirm() string {
	var b strings.Builder
	b.WriteString(warnSty.Render("⚠ Purge capiko") + "\n\n")
	b.WriteString("This will remove ALL capiko data from your system:\n\n")
	b.WriteString(dimSty.Render("  • Managed skills and agents") + "\n")
	b.WriteString(dimSty.Render("  • Instruction blocks in copilot-instructions.md") + "\n")
	b.WriteString(dimSty.Render("  • MCP server entries (engram, headroom)") + "\n")
	b.WriteString(dimSty.Render("  • Copilot CLI hooks") + "\n")
	b.WriteString(dimSty.Render("  • Scoped instruction files") + "\n")
	b.WriteString(dimSty.Render("  • VS Code MCP entries") + "\n")
	b.WriteString(dimSty.Render("  • Workspace git hooks and review state") + "\n")
	b.WriteString(dimSty.Render("  • ~/.capiko/ (state, backups, all data)") + "\n")
	b.WriteString("\n")
	b.WriteString(errSty.Render("This cannot be undone.") + "\n\n")
	b.WriteString(dimSty.Render("y/enter confirm · n/esc cancel") + "\n")
	return b.String()
}

func (s *purgeScreen) viewDone() string {
	var b strings.Builder
	b.WriteString(okSty.Render("✓ Purge complete") + "\n\n")

	sections := []struct {
		label string
		items []string
	}{
		{"Skills", s.report.Skills},
		{"Agents", s.report.Agents},
		{"Instruction blocks", s.report.InstructionBlocks},
		{"MCP entries", s.report.MCPEntries},
		{"Copilot hooks", s.report.CopilotHooks},
		{"Git hooks", s.report.GitHooks},
		{"Scoped instructions", s.report.ScopedInstructions},
		{"Workspace artifacts", s.report.WorkspaceArtifacts},
		{"Home artifacts", s.report.HomeArtifacts},
		{"VS Code entries", s.report.VSCodeEntries},
	}

	for _, sec := range sections {
		if len(sec.items) == 0 {
			continue
		}
		b.WriteString(textSty.Render("  "+sec.label) + "\n")
		for _, item := range sec.items {
			b.WriteString(dimSty.Render("  - "+item) + "\n")
		}
	}

	b.WriteString("\n")
	b.WriteString(fmt.Sprintf("%d item(s) removed.\n", s.report.TotalRemoved()))

	for _, w := range s.report.Warnings {
		b.WriteString(warnSty.Render("! "+w) + "\n")
	}

	b.WriteString("\n")
	b.WriteString(dimSty.Render("To remove the binary: brew uninstall capiko-ai") + "\n\n")
	b.WriteString(dimSty.Render("any key to go back") + "\n")
	return b.String()
}

func (s *purgeScreen) viewFailed() string {
	var b strings.Builder
	b.WriteString(errSty.Render("✗ Purge failed") + "\n\n")
	b.WriteString(s.err.Error() + "\n")
	for _, w := range s.report.Warnings {
		b.WriteString(warnSty.Render("! "+w) + "\n")
	}
	b.WriteString("\n")
	b.WriteString(dimSty.Render("any key to go back") + "\n")
	return b.String()
}
