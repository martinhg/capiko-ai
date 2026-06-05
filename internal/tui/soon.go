package tui

import tea "github.com/charmbracelet/bubbletea"

// soonScreen is a placeholder for menu options whose engine is not built yet.
type soonScreen struct{ title string }

func newSoon(title string) screen { return soonScreen{title: title} }

func (s soonScreen) Update(msg tea.Msg) (screen, tea.Cmd) {
	if _, ok := msg.(tea.KeyMsg); ok {
		return s, back
	}
	return s, nil
}

func (s soonScreen) View() string {
	return titleSty.Render(s.title) + "\n\n" +
		warnSty.Render("Coming soon.") + "\n\n" +
		dimSty.Render("any key to go back") + "\n"
}
