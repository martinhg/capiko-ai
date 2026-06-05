package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/martinhg/capiko-ai/internal/skill"
	"github.com/martinhg/capiko-ai/internal/sysinfo"
)

// detectionScreen shows a System Detection summary before installation: the host
// environment, the tools capiko relies on, its prerequisites' versions, and which
// Copilot configs already exist. Continue leads to the skill selector; Back
// returns to the menu.
type detectionScreen struct {
	svc       services
	catalog   []skill.Skill
	installed map[string]bool
	report    sysinfo.Report
	cursor    int // 0 = Continue, 1 = Back
}

func newDetection(svc services, catalog []skill.Skill, installed map[string]bool) screen {
	return &detectionScreen{
		svc:       svc,
		catalog:   catalog,
		installed: installed,
		report:    sysinfo.Detect(),
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

	// System
	supported := errSty.Render("No")
	if s.report.Supported {
		supported = okSty.Render("Yes")
	}
	row := func(label, value string) {
		fmt.Fprintf(&b, "  %s  %s\n", titleSty.Render(pad(label, 10)), value)
	}
	row("OS", textSty.Render(s.report.OS+" ("+s.report.Arch+")"))
	row("Shell", textSty.Render(s.report.Shell))
	row("Supported", supported)
	b.WriteString("\n")

	// Tools
	b.WriteString(titleSty.Render("Tools") + "\n")
	for _, t := range s.report.Tools {
		status := errSty.Render("not found")
		if t.Found {
			status = okSty.Render("found")
		}
		fmt.Fprintf(&b, "  %s  %s\n", textSty.Render(pad(t.Name, 10)), status)
	}
	b.WriteString("\n")

	// Dependencies
	b.WriteString(titleSty.Render("Dependencies") + "\n")
	for _, d := range s.report.Dependencies {
		fmt.Fprintf(&b, "  %s  %s\n", textSty.Render(pad(d.Name, 10)), dependencyStatus(d))
	}
	b.WriteString("\n")

	// Detected Configs
	b.WriteString(titleSty.Render("Detected Configs") + "\n")
	for _, c := range s.report.Configs {
		status := errSty.Render("missing")
		if c.Exists {
			status = okSty.Render("present")
		}
		fmt.Fprintf(&b, "  %s  %s\n", textSty.Render(pad(c.Name, 18)), status)
	}
	b.WriteString("\n")

	for i, opt := range []string{"Continue", "Back"} {
		if i == s.cursor {
			b.WriteString(titleSty.Render(menuCursor+opt) + "\n")
		} else {
			b.WriteString(textSty.Render("  "+opt) + "\n")
		}
	}

	b.WriteString("\n" + dimSty.Render("↑/↓ move · enter select · esc back") + "\n")
	return b.String()
}

func dependencyStatus(d sysinfo.Dependency) string {
	if d.Found {
		v := d.Version
		if v == "" {
			v = "found"
		}
		out := okSty.Render(v)
		if !d.Required {
			out += dimSty.Render(" (optional)")
		}
		return out
	}
	if d.Required {
		return errSty.Render("not found (required)")
	}
	return dimSty.Render("not found (optional)")
}

// pad right-pads s with spaces to at least width columns, for column alignment.
func pad(s string, width int) string {
	if len(s) >= width {
		return s
	}
	return s + strings.Repeat(" ", width-len(s))
}
