// Command capiko-ai is a configurator TUI that mounts the capiko layer
// (skills, memory, the SDD workflow, and MCP) onto the GitHub Copilot CLI.
package main

import (
	"fmt"
	"io"
	"os"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/martinhg/capiko-ai/internal/agent"
	"github.com/martinhg/capiko-ai/internal/backup"
	"github.com/martinhg/capiko-ai/internal/catalog"
	"github.com/martinhg/capiko-ai/internal/copilot"
	"github.com/martinhg/capiko-ai/internal/release"
	"github.com/martinhg/capiko-ai/internal/skill"
	"github.com/martinhg/capiko-ai/internal/state"
	"github.com/martinhg/capiko-ai/internal/tui"
	"github.com/martinhg/capiko-ai/internal/versions"
)

// envPostUpgradeSync is set across the self-update re-exec so the freshly
// installed binary syncs skills with its new catalog on startup (the
// "Upgrade + sync" flow).
const envPostUpgradeSync = "CAPIKO_POST_UPGRADE_SYNC"

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
		case "sdd-status", "sdd-continue":
			// The native SDD engine: print authoritative change status without
			// launching the TUI, so the agent can shell out for it.
			if _, err := sddCommand(os.Args[1], os.Args[2:], os.Stdout); err != nil {
				fmt.Fprintln(os.Stderr, "capiko-ai:", err)
				os.Exit(1)
			}
			return
		case "skill-registry":
			// The native skill-registry engine: print the skill index so an
			// orchestrator can resolve exact SKILL.md paths to inject into
			// sub-agents, without launching the TUI.
			if _, err := skillRegistryCommand(os.Args[1], os.Args[2:], os.Stdout); err != nil {
				fmt.Fprintln(os.Stderr, "capiko-ai:", err)
				os.Exit(1)
			}
			return
		}
	}

	cat, err := catalog.Load()
	if err != nil {
		fmt.Fprintln(os.Stderr, "capiko-ai: loading catalog:", err)
		os.Exit(1)
	}
	agentCat, err := catalog.LoadAgents()
	if err != nil {
		fmt.Fprintln(os.Stderr, "capiko-ai: loading agent catalog:", err)
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

	// Post-upgrade sync: the previous process upgraded the binary and re-exec'd
	// with this flag set, so this (new) binary syncs skills with its new catalog
	// before the menu opens. Done here, not inside the TUI, so it runs with the
	// new embedded catalog.
	if os.Getenv(envPostUpgradeSync) == "1" {
		os.Unsetenv(envPostUpgradeSync)
		postUpgradeSync(copilot.Detect, cat, agentCat, store, bkp, os.Stdout)
	}

	final, err := tea.NewProgram(tui.NewApp(cat, agentCat, store, bkp)).Run()
	if err != nil {
		fmt.Fprintln(os.Stderr, "capiko-ai:", err)
		os.Exit(1)
	}

	// A successful self-update quits the TUI with the restart flag set; re-exec
	// so the freshly installed binary takes over in the same terminal.
	if app, ok := final.(tui.App); ok && app.ShouldRestart() {
		if app.ShouldSyncAfterRestart() {
			os.Setenv(envPostUpgradeSync, "1")
		}
		if err := release.Restart(); err != nil {
			fmt.Fprintln(os.Stderr, "capiko-ai: restart after update failed; please relaunch:", err)
		}
	}
}

// postUpgradeSync detects the Copilot host and syncs the catalog into it,
// reporting the outcome. Detection is injected so the flow is testable; a nil
// host (Copilot not installed/initialized) is a silent no-op.
func postUpgradeSync(detect func() (*copilot.Host, error), cat []skill.Skill, agentCat []agent.Agent, store *state.Store, bkp *backup.Store, out io.Writer) {
	host, err := detect()
	if err != nil || host == nil {
		return
	}
	if n, err := tui.RunSync(host, cat, agentCat, store, bkp); err != nil {
		fmt.Fprintln(out, "capiko-ai: post-upgrade sync failed:", err)
	} else {
		fmt.Fprintf(out, "capiko-ai: synced %d item(s) after upgrade\n", n)
	}
}
