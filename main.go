// Command capiko-ai is a configurator TUI that mounts the capiko layer
// (skills, and later memory, SDD workflow, and MCP) onto the GitHub Copilot
// CLI — the same pattern gentle-ai uses over Claude Code and other agents.
package main

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/martinhg/capiko-ai/internal/backup"
	"github.com/martinhg/capiko-ai/internal/catalog"
	"github.com/martinhg/capiko-ai/internal/state"
	"github.com/martinhg/capiko-ai/internal/tui"
)

func main() {
	cat, err := catalog.Load()
	if err != nil {
		fmt.Fprintln(os.Stderr, "capiko-ai: loading catalog:", err)
		os.Exit(1)
	}

	// Nil stores degrade gracefully: changes apply but are not recorded or
	// snapshotted.
	store, err := state.DefaultStore()
	if err != nil {
		fmt.Fprintln(os.Stderr, "capiko-ai: warning: state disabled:", err)
	}
	bkp, err := backup.DefaultStore()
	if err != nil {
		fmt.Fprintln(os.Stderr, "capiko-ai: warning: backups disabled:", err)
	}

	if _, err := tea.NewProgram(tui.NewApp(cat, store, bkp)).Run(); err != nil {
		fmt.Fprintln(os.Stderr, "capiko-ai:", err)
		os.Exit(1)
	}
}
