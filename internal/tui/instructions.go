package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/martinhg/capiko-ai/internal/backup"
	"github.com/martinhg/capiko-ai/internal/copilot"
	"github.com/martinhg/capiko-ai/internal/scoped"
)

// instructionTargets is every directory the curated scoped instructions are
// written into: the home ~/.copilot/instructions/ plus any dirs Copilot loads
// via COPILOT_CUSTOM_INSTRUCTIONS_DIRS, so every instruction source Copilot reads
// stays in sync with capiko's catalog.
func instructionTargets(host *copilot.Host) []string {
	return append([]string{scoped.Dir(host.ConfigDir)}, copilot.CustomInstructionDirs()...)
}

// installInstructions writes the curated scoped instruction files into every
// target directory, snapshotting the targets first. Returns the total number of
// files written (files × target dirs).
func installInstructions(host *copilot.Host, bkp *backup.Store) (int, error) {
	ins, err := scoped.Load()
	if err != nil {
		return 0, err
	}
	dirs := instructionTargets(host)
	if bkp != nil {
		var paths []string
		for _, dir := range dirs {
			for _, in := range ins {
				paths = append(paths, scoped.Path(dir, in))
			}
		}
		if _, err := bkp.CreateFiles("instructions", Version, paths); err != nil {
			return 0, fmt.Errorf("backup failed, aborting: %w", err)
		}
	}
	count := 0
	for _, dir := range dirs {
		for _, in := range ins {
			if _, err := scoped.Install(dir, in); err != nil {
				return count, fmt.Errorf("installing %s into %s: %w", in.Name, dir, err)
			}
			count++
		}
	}
	return count, nil
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
		b.WriteString(okSty.Render(fmt.Sprintf("Installed %d instruction file(s) across %d location(s) ✓", s.count, len(s.targetLabels()))) + "\n\n")
		b.WriteString(dimSty.Render("any key to go back") + "\n")
		return b.String()
	case instrFailed:
		b.WriteString(errSty.Render("Error: "+s.err.Error()) + "\n\n")
		b.WriteString(dimSty.Render("any key to go back") + "\n")
		return b.String()
	}

	b.WriteString(dimSty.Render(fmt.Sprintf("Write %d file(s) to each target, applied per-file via their applyTo globs.", len(s.items))) + "\n\n")
	b.WriteString(dimSty.Render("Target(s):") + "\n")
	for _, l := range s.targetLabels() {
		b.WriteString(textSty.Render("  "+l) + "\n")
	}
	b.WriteString("\n" + dimSty.Render("Files:") + "\n")
	for _, in := range s.items {
		b.WriteString(textSty.Render("  "+in.Name+".instructions.md") + "\n")
	}
	b.WriteString("\n" + dimSty.Render("[y to install · q to go back]") + "\n")
	return b.String()
}

// targetLabels are the display labels for every directory instructions are
// written to: a friendly, host-independent label for the home dir plus any
// COPILOT_CUSTOM_INSTRUCTIONS_DIRS entries shown verbatim.
func (s *instructionsScreen) targetLabels() []string {
	return append([]string{"~/.copilot/instructions/"}, copilot.CustomInstructionDirs()...)
}
