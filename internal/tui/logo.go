package tui

import "github.com/charmbracelet/lipgloss"

var (
	capyBrown = lipgloss.NewStyle().Foreground(lipgloss.Color("#A0703B"))
	capyRed   = lipgloss.NewStyle().Foreground(lipgloss.Color("#E23B3B"))
	capyWhite = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#FFFFFF"))
)

// logo renders the capiko mascot: a round brown capybara in a red shirt with a
// white "KO" beside it (capy + KO). Colors collapse to plain shapes under a
// non-color profile, so golden snapshots stay deterministic.
func logo() string {
	capy := lipgloss.JoinVertical(lipgloss.Center,
		capyBrown.Render("╭▔▔▔▔▔▔▔╮"),
		capyBrown.Render("╱ ●   ● ╲"),
		capyBrown.Render("│   ▾   │"),
		capyBrown.Render("│  ╲▁╱  │"),
		capyRed.Render("│ ▆▆▆▆▆ │"),
		capyRed.Render("╲▆▆▆▆▆▆▆╱"),
		capyBrown.Render("╰┳▔▔▔┳╯"),
		capyBrown.Render(" ╹   ╹ "),
	)
	ko := lipgloss.JoinVertical(lipgloss.Left,
		capyWhite.Render("█ █ ███"),
		capyWhite.Render("██  █ █"),
		capyWhite.Render("█ █ ███"),
	)
	return lipgloss.JoinHorizontal(lipgloss.Center, capy, "   ", ko)
}
