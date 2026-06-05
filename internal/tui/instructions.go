package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/martinhg/capiko-ai/internal/backup"
	"github.com/martinhg/capiko-ai/internal/copilot"
	"github.com/martinhg/capiko-ai/internal/scoped"
)

// installInstructions writes the curated scoped instruction files into
// ~/.copilot/instructions/, snapshotting the targets first. Returns the count.
func installInstructions(host *copilot.Host, bkp *backup.Store) (int, error) {
	ins, err := scoped.Load()
	if err != nil {
		return 0, err
	}
	dir := scoped.Dir(host.ConfigDir)
	if bkp != nil {
		paths := make([]string, len(ins))
		for i, in := range ins {
			paths[i] = scoped.Path(dir, in)
		}
		if _, err := bkp.CreateFiles("instructions", Version, paths); err != nil {
			return 0, fmt.Errorf("backup failed, aborting: %w", err)
		}
	}
	for _, in := range ins {
		if _, err := scoped.Install(dir, in); err != nil {
			return 0, fmt.Errorf("installing %s: %w", in.Name, err)
		}
	}
	return len(ins), nil
}

// instructionsScreen installs capiko's curated scoped instruction files, which
// Copilot applies per-file via each file's applyTo glob.
type instructionsScreen struct {
	svc   services
	items []scoped.Instruction
	state instrState
	count int
	err   error
}

type instrState int

const (
	instrConfirm instrState = iota
	instrApplying
	instrDone
	instrFailed
)

type instructionsInstalledMsg struct {
	count int
	err   error
}

func newInstructions(svc services) screen {
	items, _ := scoped.Load()
	return &instructionsScreen{svc: svc, items: items}
}

func (s *instructionsScreen) Update(msg tea.Msg) (screen, tea.Cmd) {
	switch msg := msg.(type) {
	case instructionsInstalledMsg:
		if msg.err != nil {
			s.state, s.err = instrFailed, msg.err
			return s, nil
		}
		s.state, s.count = instrDone, msg.count
		return s, nil
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "esc", "n":
			return s, back
		}
		if s.state == instrConfirm && (msg.String() == "y" || msg.String() == "enter") {
			s.state = instrApplying
			return s, s.applyCmd()
		}
		if s.state == instrDone || s.state == instrFailed {
			return s, back
		}
	}
	return s, nil
}

func (s *instructionsScreen) applyCmd() tea.Cmd {
	host, bkp := s.svc.host, s.svc.backup
	return func() tea.Msg {
		n, err := installInstructions(host, bkp)
		return instructionsInstalledMsg{count: n, err: err}
	}
}

func (s *instructionsScreen) View() string {
	var b strings.Builder
	b.WriteString(titleSty.Render("Install scoped instructions") + "\n\n")

	switch s.state {
	case instrApplying:
		b.WriteString("Writing instruction files…\n")
		return b.String()
	case instrDone:
		b.WriteString(okSty.Render(fmt.Sprintf("Installed %d instruction file(s) ✓", s.count)) + "\n\n")
		b.WriteString(dimSty.Render("any key to go back") + "\n")
		return b.String()
	case instrFailed:
		b.WriteString(errSty.Render("Error: "+s.err.Error()) + "\n\n")
		b.WriteString(dimSty.Render("any key to go back") + "\n")
		return b.String()
	}

	b.WriteString(dimSty.Render(fmt.Sprintf("Write %d file(s) to ~/.copilot/instructions/ (applied per-file via their applyTo globs).", len(s.items))) + "\n\n")
	for _, in := range s.items {
		b.WriteString(textSty.Render("  "+in.Name+".instructions.md") + "\n")
	}
	b.WriteString("\n" + dimSty.Render("[y to install · q to go back]") + "\n")
	return b.String()
}
