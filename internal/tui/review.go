package tui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

// reviewScreen summarizes the pending change (skills to install/remove, the
// active persona) and gates the apply behind an explicit confirmation. Apply
// hands control back to the selector to run the reconcile; Back returns to the
// selector with the selection intact.
type reviewScreen struct {
	parent  *selector
	install []string
	remove  []string
	persona string
	cursor  int // 0 = Apply, 1 = Back
}

func newReview(parent *selector) screen {
	ins, rem := parent.plan()
	s := &reviewScreen{parent: parent, install: ins, remove: rem}
	if parent.svc.state != nil {
		if st, err := parent.svc.state.Load(); err == nil {
			s.persona = st.Persona
		}
	}
	return s
}

func (s *reviewScreen) Update(msg tea.Msg) (screen, tea.Cmd) {
	key, ok := msg.(tea.KeyMsg)
	if !ok {
		return s, nil
	}
	switch key.String() {
	case "q", "esc":
		return s.parent, nil // back to the selector, selection intact
	case "up", "k":
		if s.cursor > 0 {
			s.cursor--
		}
	case "down", "j":
		if s.cursor < 1 {
			s.cursor++
		}
	case "enter":
		if s.cursor == 1 { // Back
			return s.parent, nil
		}
		// Apply: hand back to the selector and trigger its reconcile so the
		// result view is shown there.
		s.parent.state = selApplying
		return s.parent, s.parent.reconcileCmd()
	}
	return s, nil
}

func (s *reviewScreen) View() string {
	var b strings.Builder
	b.WriteString(titleSty.Render("Review and Confirm") + "\n\n")

	persona := s.persona
	if persona == "" {
		persona = "unmanaged"
	}
	b.WriteString("  " + titleSty.Render(pad("Persona", 10)) + "  " + textSty.Render(persona) + "\n\n")

	b.WriteString(titleSty.Render("To install") + "\n")
	b.WriteString(list(s.install, okSty.Render("  + ")))
	b.WriteString("\n")
	b.WriteString(titleSty.Render("To remove") + "\n")
	b.WriteString(list(s.remove, warnSty.Render("  - ")))
	b.WriteString("\n")

	for i, opt := range []string{"Apply", "Back"} {
		if i == s.cursor {
			b.WriteString(titleSty.Render(menuCursor+opt) + "\n")
		} else {
			b.WriteString(textSty.Render("  "+opt) + "\n")
		}
	}

	b.WriteString("\n" + dimSty.Render("↑/↓ move · enter select · esc back") + "\n")
	return b.String()
}

func list(names []string, prefix string) string {
	if len(names) == 0 {
		return dimSty.Render("  (none)") + "\n"
	}
	var b strings.Builder
	for _, n := range names {
		b.WriteString(prefix + textSty.Render(n) + "\n")
	}
	return b.String()
}
