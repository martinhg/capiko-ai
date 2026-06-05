package tui

import "github.com/charmbracelet/lipgloss"

// Brand palette — one cohesive scheme used across the whole UI.
var (
	brandColor = lipgloss.Color("#E8B05A") // warm amber (capybara/brand accent)

	titleSty = lipgloss.NewStyle().Bold(true).Foreground(brandColor)     // headings, selection
	textSty  = lipgloss.NewStyle().Foreground(lipgloss.Color("#ECE3D4")) // body text (cream)
	dimSty   = lipgloss.NewStyle().Foreground(lipgloss.Color("#998A77")) // secondary (warm gray)
	okSty    = lipgloss.NewStyle().Foreground(lipgloss.Color("#84B26A")) // success
	warnSty  = lipgloss.NewStyle().Foreground(lipgloss.Color("#E0894C")) // caution
	errSty   = lipgloss.NewStyle().Foreground(lipgloss.Color("#D9554A")) // error
)

// head renders the compact capiko banner shown above sub-screens.
func head() string {
	return titleSty.Render("capiko-ai") + dimSty.Render("  ·  Copilot configurator") + "\n\n"
}
