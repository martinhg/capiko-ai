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
	"github.com/martinhg/capiko-ai/internal/release"
	"github.com/martinhg/capiko-ai/internal/state"
	"github.com/martinhg/capiko-ai/internal/tui"
	"github.com/martinhg/capiko-ai/internal/versions"
)

func main() {
	// version is handled before anything else so installers and CI can read the
	// build-injected version (ldflags set internal/tui.Version) without launching
	// the full-screen TUI. Both `version` and `-v`/`--version` are accepted so the
	// POSIX and PowerShell install scripts can share one binary contract.
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "version", "-v", "--version":
			fmt.Println("capiko-ai", tui.Version)
			fmt.Println("targets GitHub Copilot CLI", versions.CopilotCLI)
			return
		}
	}

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

	final, err := tea.NewProgram(tui.NewApp(cat, store, bkp)).Run()
	if err != nil {
		fmt.Fprintln(os.Stderr, "capiko-ai:", err)
		os.Exit(1)
	}

	// A successful self-update quits the TUI with the restart flag set; re-exec
	// so the freshly installed binary takes over in the same terminal.
	if app, ok := final.(tui.App); ok && app.ShouldRestart() {
		if err := release.Restart(); err != nil {
			fmt.Fprintln(os.Stderr, "capiko-ai: restart after update failed; please relaunch:", err)
		}
	}
}
