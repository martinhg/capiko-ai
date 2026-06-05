package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/martinhg/capiko-ai/internal/skill"
	"github.com/martinhg/capiko-ai/internal/sysinfo"
	"github.com/martinhg/capiko-ai/internal/versions"
)

// detectionScreen shows a System Detection summary before installation: the host
// environment and whether the tools capiko relies on are present. Continue leads
// to the skill selector; Back returns to the menu.
type detectionScreen struct {
	svc       services
	catalog   []skill.Skill
	installed map[string]bool
	report    sysinfo.Report
	hasConfig bool
	cursor    int // 0 = Continue, 1 = Back
}

func newDetection(svc services, catalog []skill.Skill, installed map[string]bool) screen {
	return &detectionScreen{
		svc:       svc,
		catalog:   catalog,
		installed: installed,
		report:    sysinfo.Detect(),
		// Detection only yields a host when ~/.copilot exists, so a non-nil host
		// means the config is present.
		hasConfig: svc.host != nil,
	}
}

func (s *detectionScreen) Update(msg tea.Msg) (screen, tea.Cmd) {
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
		if s.cursor < 1 {
			s.cursor++
		}
	case "enter":
		if s.cursor == 0 { // Continue → pick skills
			return newInstall(s.svc, s.catalog, s.installed), nil
		}
		return s, back // Back → menu
	}
	return s, nil
}

func (s *detectionScreen) View() string {
	var b strings.Builder
	b.WriteString(titleSty.Render("System Detection") + "\n\n")

	fmt.Fprintf(&b, "  %s  %s\n", titleSty.Render("OS"), textSty.Render(s.report.OS+" ("+s.report.Arch+")"))
	fmt.Fprintf(&b, "  %s  %s\n\n", titleSty.Render("Shell"), textSty.Render(s.report.Shell))

	b.WriteString(titleSty.Render("Tools") + "\n")
	for _, tool := range s.report.Tools {
		status := errSty.Render("not found")
		if tool.Found {
			status = okSty.Render("found")
		}
		fmt.Fprintf(&b, "  %s  %s\n", textSty.Render(pad(tool.Name, 8)), status)
	}
	b.WriteString("\n")

	config := errSty.Render("missing")
	if s.hasConfig {
		config = okSty.Render("present")
	}
	fmt.Fprintf(&b, "  %s  %s\n", textSty.Render(pad("~/.copilot", 12)), config)
	fmt.Fprintf(&b, "  %s  %s\n\n", textSty.Render(pad("Copilot CLI target", 12)), dimSty.Render(versions.CopilotCLI))

	options := []string{"Continue", "Back"}
	for i, opt := range options {
		if i == s.cursor {
			b.WriteString(titleSty.Render(menuCursor+opt) + "\n")
		} else {
			b.WriteString(textSty.Render("  "+opt) + "\n")
		}
	}

	b.WriteString("\n" + dimSty.Render("↑/↓ move · enter select · esc back") + "\n")
	return b.String()
}

// pad right-pads s with spaces to at least width columns, for column alignment.
func pad(s string, width int) string {
	if len(s) >= width {
		return s
	}
	return s + strings.Repeat(" ", width-len(s))
}
