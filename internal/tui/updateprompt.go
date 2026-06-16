package tui

import (
	"fmt"
	"os/exec"
	"runtime"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/martinhg/capiko-ai/internal/release"
)

var promptItems = []string{"Update now", "View changes", "Keep current"}

const promptDefaultCursor = 2 // "Keep current" — a stray Enter is safe

// browserOpen is the function used to open a URL in the user's browser. It is a
// package-level var so tests can replace it without spawning a real browser.
var browserOpen = defaultBrowserOpen

func defaultBrowserOpen(url string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	case "linux":
		cmd = exec.Command("xdg-open", url)
	case "windows":
		cmd = exec.Command("cmd", "/c", "start", url)
	default:
		return fmt.Errorf("unsupported platform: %s", runtime.GOOS)
	}
	return cmd.Start()
}

func (a App) updatePrompt(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "up", "k":
		if a.cursor > 0 {
			a.cursor--
		}
	case "down", "j":
		if a.cursor < len(promptItems)-1 {
			a.cursor++
		}
	case "u":
		return a.selectPrompt(0)
	case "v":
		return a.selectPrompt(1)
	case "c", "esc":
		return a.selectPrompt(2)
	case "enter":
		return a.selectPrompt(a.cursor)
	case "q":
		return a, tea.Quit
	}
	return a, nil
}

func (a App) selectPrompt(idx int) (tea.Model, tea.Cmd) {
	switch idx {
	case 0: // Update now
		a.state = appScreen
		a.active = newUpgrade(a.svc, a.latest)
		a.menuTouched = true
		return a, nil
	case 1: // View changes
		_ = browserOpen(release.ReleaseURL(a.latest))
		return a, nil
	default: // Keep current
		a.state = appMenu
		a.cursor = 0
		a.menuTouched = true
		return a, nil
	}
}

func (a App) viewPrompt() string {
	var b strings.Builder

	b.WriteString(logo())
	b.WriteString("\n\n")
	b.WriteString(titleSty.Render("Capiko AI - v"+Version) + dimSty.Render("  ·  "+tagline))
	b.WriteString("\n")
	b.WriteString(warnSty.Render(fmt.Sprintf("Update available: capiko-ai %s → %s", Version, a.latest)))
	b.WriteString("\n\n")

	for i, label := range promptItems {
		if i == a.cursor {
			b.WriteString(titleSty.Render(menuCursor+label) + "\n")
		} else {
			b.WriteString(textSty.Render("  "+label) + "\n")
		}
	}

	b.WriteString("\n")
	b.WriteString(dimSty.Render("↑/↓ move · enter select · u update · v view · c keep"))

	return menuBoxSty.Render(b.String()) + "\n"
}
