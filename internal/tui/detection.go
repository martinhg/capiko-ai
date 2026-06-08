package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/martinhg/capiko-ai/internal/skill"
	"github.com/martinhg/capiko-ai/internal/state"
	"github.com/martinhg/capiko-ai/internal/sysinfo"
)

// detectionScreen shows a System Detection summary before installation: the host
// environment, the tools capiko relies on, its prerequisites' versions/install
// hints, and which Copilot configs exist. Missing dependencies show an install
// hint; auto-installable ones can be installed via "Install missing". Continue
// leads to the persona/skill flow; Back returns to the menu.
type detectionScreen struct {
	svc        services
	catalog    []skill.Skill
	installed  map[string]bool
	report     sysinfo.Report
	engram     *state.EngramRecord // managed engram config, nil = unmanaged
	cursor     int
	installing bool
	status     string // result of the last install run
}

func newDetection(svc services, catalog []skill.Skill, installed map[string]bool) screen {
	return &detectionScreen{
		svc:       svc,
		catalog:   catalog,
		installed: installed,
		report:    sysinfo.Detect(),
		engram:    loadEngramRecord(svc.state),
	}
}

// loadEngramRecord returns the managed engram config, or nil when unmanaged or
// unreadable — detection is best-effort and never blocks on state.
func loadEngramRecord(store *state.Store) *state.EngramRecord {
	if store == nil {
		return nil
	}
	st, err := store.Load()
	if err != nil {
		return nil
	}
	return st.Engram
}

type depsInstalledMsg struct{ summary string }

// installable returns the missing dependencies capiko can install via one-click.
func (s *detectionScreen) installable() []sysinfo.Dependency {
	var out []sysinfo.Dependency
	for _, d := range s.report.Dependencies {
		if !d.Found && d.Auto {
			out = append(out, d)
		}
	}
	return out
}

func (s *detectionScreen) options() []string {
	if len(s.installable()) > 0 {
		return []string{"Install missing", "Continue", "Back"}
	}
	return []string{"Continue", "Back"}
}

func (s *detectionScreen) Update(msg tea.Msg) (screen, tea.Cmd) {
	switch msg := msg.(type) {
	case depsInstalledMsg:
		s.installing = false
		s.status = msg.summary
		s.report = sysinfo.Detect() // refresh after installing
		s.cursor = 0
		return s, nil
	case tea.KeyMsg:
		if s.installing {
			return s, nil
		}
		switch msg.String() {
		case "q", "esc":
			return s, back
		case "up", "k":
			if s.cursor > 0 {
				s.cursor--
			}
		case "down", "j":
			if s.cursor < len(s.options())-1 {
				s.cursor++
			}
		case "enter":
			switch s.options()[s.cursor] {
			case "Install missing":
				s.installing, s.status = true, ""
				return s, s.installCmd()
			case "Continue":
				return newPersona(s.svc, s.catalog, s.installed), nil
			case "Back":
				return s, back
			}
		}
	}
	return s, nil
}

func (s *detectionScreen) installCmd() tea.Cmd {
	deps := s.installable()
	return func() tea.Msg {
		var ok, failed []string
		for _, d := range deps {
			if err := sysinfo.Install(d); err != nil {
				failed = append(failed, d.Name)
			} else {
				ok = append(ok, d.Name)
			}
		}
		var parts []string
		if len(ok) > 0 {
			parts = append(parts, "installed "+strings.Join(ok, ", "))
		}
		if len(failed) > 0 {
			parts = append(parts, "failed "+strings.Join(failed, ", "))
		}
		return depsInstalledMsg{summary: strings.Join(parts, " · ")}
	}
}

func (s *detectionScreen) View() string {
	var b strings.Builder
	b.WriteString(titleSty.Render("System Detection") + "\n\n")

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

	b.WriteString(titleSty.Render("Tools") + "\n")
	for _, t := range s.report.Tools {
		status := errSty.Render("not found")
		if t.Found {
			status = okSty.Render("found")
		}
		fmt.Fprintf(&b, "  %s  %s\n", textSty.Render(pad(t.Name, 10)), status)
	}
	b.WriteString("\n")

	b.WriteString(titleSty.Render("Dependencies") + "\n")
	for _, d := range s.report.Dependencies {
		line := fmt.Sprintf("  %s  %s", textSty.Render(pad(d.Name, 10)), dependencyStatus(d))
		if !d.Found && d.Install != "" {
			line += dimSty.Render("  → " + d.Install)
		}
		b.WriteString(line + "\n")
	}
	b.WriteString("\n")

	b.WriteString(titleSty.Render("Detected Configs") + "\n")
	for _, c := range s.report.Configs {
		status := errSty.Render("missing")
		if c.Exists {
			status = okSty.Render("present")
		}
		fmt.Fprintf(&b, "  %s  %s\n", textSty.Render(pad(c.Name, 18)), status)
	}
	b.WriteString("\n")

	b.WriteString(titleSty.Render("Engram") + "\n")
	fmt.Fprintf(&b, "  %s  %s\n\n", textSty.Render(pad("status", 18)), engramStatus(s.engram))

	if s.installing {
		b.WriteString("Installing missing dependencies…\n")
		return b.String()
	}
	if s.status != "" {
		b.WriteString(okSty.Render(s.status) + "\n\n")
	}

	for i, opt := range s.options() {
		if i == s.cursor {
			b.WriteString(titleSty.Render(menuCursor+opt) + "\n")
		} else {
			b.WriteString(textSty.Render("  "+opt) + "\n")
		}
	}

	b.WriteString("\n" + dimSty.Render("↑/↓ move · enter select · esc back") + "\n")
	return b.String()
}

// engramStatus renders the managed engram configuration for the detection screen.
func engramStatus(rec *state.EngramRecord) string {
	if rec == nil {
		return dimSty.Render("not configured")
	}
	if !rec.Enabled {
		return dimSty.Render("disabled")
	}
	cloud := "local only"
	if rec.CloudServer != "" {
		cloud = "cloud " + rec.CloudServer
	}
	return okSty.Render("enabled") + dimSty.Render(fmt.Sprintf(" · %s · %s", rec.ArtifactMode, cloud))
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
