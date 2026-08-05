package main

import (
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
)

// commands is the canonical list of subcommands for usage and did-you-mean.
// Keep sorted. Each entry is name → one-line description.
var commands = map[string]string{
	"doctor":         "Run ecosystem health checks",
	"help":           "Show this help message",
	"install":        "Install all catalog skills and agents",
	"sdd-continue":   "Continue the next SDD phase",
	"sdd-status":     "Show SDD change status",
	"skill-registry": "Print the skill index for orchestrators",
	"sync":           "Sync all managed skills and agents",
	"uninstall":      "Remove all managed skills and agents",
	"version":        "Print version information",
}

func usage(out io.Writer) {
	fmt.Fprintln(out, "capiko-ai — configurator for the capiko layer on GitHub Copilot CLI")
	fmt.Fprintln(out)
	fmt.Fprintln(out, "Usage:")
	fmt.Fprintln(out, "  capiko-ai                Launch the configurator TUI")
	fmt.Fprintln(out, "  capiko-ai <command>      Run a command")
	fmt.Fprintln(out, "  capiko-ai help           Show this help message")
	fmt.Fprintln(out)
	fmt.Fprintln(out, "Commands:")

	// Sort command names for stable output.
	names := make([]string, 0, len(commands))
	for n := range commands {
		names = append(names, n)
	}
	sort.Strings(names)

	for _, n := range names {
		fmt.Fprintf(out, "  %-18s %s\n", n, commands[n])
	}

	fmt.Fprintln(out)
	fmt.Fprintln(out, "Flags:")
	fmt.Fprintln(out, "  --help, -h       Show help for a command")
	fmt.Fprintln(out, "  --version, -v    Print version information")
}

// suggest returns the closest known command to unknown, or "" if nothing is
// close enough. Uses simple prefix/substring matching — no Levenshtein needed
// for a small command set.
func suggest(unknown string) string {
	unknown = strings.ToLower(unknown)
	// Exact prefix match first.
	for name := range commands {
		if strings.HasPrefix(name, unknown) {
			return name
		}
	}
	// Substring match.
	for name := range commands {
		if strings.Contains(name, unknown) {
			return name
		}
	}
	return ""
}

// isTerminal reports whether f is connected to a terminal.
func isTerminal(f *os.File) bool {
	fi, err := f.Stat()
	if err != nil {
		return false
	}
	return fi.Mode()&os.ModeCharDevice != 0
}
