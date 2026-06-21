package tui

import (
	"fmt"
	"os"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/martinhg/capiko-ai/internal/sddstatus"
)

// sddStatusGetwd is a test seam over os.Getwd so the screen can be driven against
// a temporary OpenSpec tree, mirroring headroomDetected and installDep.
var sddStatusGetwd = os.Getwd

// sddChange pairs an active OpenSpec change name with its resolved status.
type sddChange struct {
	name   string
	status sddstatus.Status
}

// sddStatusScreen is a read-only dashboard of active SDD changes. The list view
// shows each change with its next phase and task progress; selecting one opens a
// detail view rendering the phase dependency graph (done/in-progress/blocked) and
// any blocked reasons. It re-reads OpenSpec every time it is opened, so returning
// from another screen reflects the latest state.
type sddStatusScreen struct {
	svc     services
	entries []sddChange
	cursor  int
	detail  bool // false = list, true = detail of entries[cursor]
	err     error
}

func newSDDStatus(svc services) screen {
	s := &sddStatusScreen{svc: svc}
	s.reload()
	return s
}

// reload resolves every active OpenSpec change through the status engine. It is
// best-effort: a working-directory or read error is surfaced in the view rather
// than crashing the menu.
func (s *sddStatusScreen) reload() {
	cwd, err := sddStatusGetwd()
	if err != nil {
		s.err = err
		return
	}
	names, err := sddstatus.ListActiveOpenSpecChanges(cwd)
	if err != nil {
		s.err = err
		return
	}
	entries := make([]sddChange, 0, len(names))
	for _, name := range names {
		st, err := sddstatus.Resolve(sddstatus.ResolveOptions{Cwd: cwd, ChangeName: name})
		if err != nil {
			s.err = err
			return
		}
		entries = append(entries, sddChange{name: name, status: st})
	}
	s.entries = entries
	if s.cursor >= len(s.entries) {
		s.cursor = max(0, len(s.entries)-1)
	}
}

func (s *sddStatusScreen) Update(msg tea.Msg) (screen, tea.Cmd) {
	key, ok := msg.(tea.KeyMsg)
	if !ok {
		return s, nil
	}
	switch key.String() {
	case "q", "esc":
		if s.detail {
			s.detail = false
			return s, nil
		}
		return s, back
	case "up", "k":
		if !s.detail && s.cursor > 0 {
			s.cursor--
		}
	case "down", "j":
		if !s.detail && s.cursor < len(s.entries)-1 {
			s.cursor++
		}
	case "enter":
		if !s.detail && len(s.entries) > 0 {
			s.detail = true
		}
	}
	return s, nil
}

func (s *sddStatusScreen) View() string {
	var b strings.Builder
	b.WriteString(titleSty.Render("SDD Status") + "\n\n")

	if s.err != nil {
		b.WriteString(errSty.Render("Error: "+s.err.Error()) + "\n\n")
		b.WriteString(dimSty.Render("q to go back") + "\n")
		return b.String()
	}
	if len(s.entries) == 0 {
		b.WriteString(dimSty.Render("No active SDD changes under openspec/changes.") + "\n\n")
		b.WriteString(dimSty.Render("q to go back") + "\n")
		return b.String()
	}
	if s.detail {
		return s.detailView()
	}

	for i, e := range s.entries {
		summary := dimSty.Render(fmt.Sprintf("next: %s · tasks %d/%d",
			e.status.NextRecommended, e.status.TaskProgress.Completed, e.status.TaskProgress.Total))
		name := pad(e.name, 24)
		if i == s.cursor {
			b.WriteString(titleSty.Render(menuCursor) + titleSty.Render(name) + "  " + summary + "\n")
		} else {
			b.WriteString("  " + textSty.Render(name) + "  " + summary + "\n")
		}
	}

	b.WriteString("\n" + dimSty.Render("↑/↓ move · enter detail · q back") + "\n")
	return b.String()
}

// detailView renders the phase dependency graph for the selected change.
func (s *sddStatusScreen) detailView() string {
	e := s.entries[s.cursor]
	var b strings.Builder
	b.WriteString(titleSty.Render("SDD Status") + dimSty.Render("  ·  "+e.name) + "\n\n")

	d := e.status.Dependencies
	phases := []struct {
		label string
		state sddstatus.DependencyState
	}{
		{"proposal", d.Proposal},
		{"specs", d.Specs},
		{"design", d.Design},
		{"tasks", d.Tasks},
		{"apply", d.Apply},
		{"verify", d.Verify},
		{"archive", d.Archive},
	}
	for _, p := range phases {
		fmt.Fprintf(&b, "  %s  %s\n", phaseGlyph(p.state), phaseLabel(p.label, p.state))
	}

	b.WriteString("\n")
	fmt.Fprintf(&b, "  %s  %s\n", textSty.Render(pad("tasks", 10)),
		textSty.Render(fmt.Sprintf("%d/%d complete", e.status.TaskProgress.Completed, e.status.TaskProgress.Total)))
	fmt.Fprintf(&b, "  %s  %s\n", textSty.Render(pad("next", 10)), okSty.Render(e.status.NextRecommended))

	if len(e.status.BlockedReasons) > 0 {
		b.WriteString("\n" + titleSty.Render("Blocked") + "\n")
		for _, r := range e.status.BlockedReasons {
			b.WriteString(warnSty.Render("  ! "+r) + "\n")
		}
	}

	b.WriteString("\n" + dimSty.Render("esc back to list") + "\n")
	return b.String()
}

// phaseGlyph maps a dependency state to a status marker: done, in-progress (the
// phase the engine would run next), or blocked.
func phaseGlyph(state sddstatus.DependencyState) string {
	switch state {
	case sddstatus.DependencyAllDone:
		return okSty.Render("✓")
	case sddstatus.DependencyReady:
		return warnSty.Render("▶")
	default:
		return dimSty.Render("·")
	}
}

// phaseLabel styles a phase name to match its glyph and appends the human state.
func phaseLabel(label string, state sddstatus.DependencyState) string {
	switch state {
	case sddstatus.DependencyAllDone:
		return textSty.Render(pad(label, 10)) + dimSty.Render("done")
	case sddstatus.DependencyReady:
		return titleSty.Render(pad(label, 10)) + dimSty.Render("in progress")
	default:
		return dimSty.Render(pad(label, 10) + "blocked")
	}
}
