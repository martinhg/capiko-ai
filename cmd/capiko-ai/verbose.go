package main

import (
	"io"
	"os"

	"github.com/martinhg/capiko-ai/internal/clilog"
)

// verboseLogSink is where --verbose JSON-lines diagnostics go. It is a package var
// so tests can capture the output without touching the real stderr. stdout is left
// alone so scripted output stays clean.
var verboseLogSink io.Writer = os.Stderr

// parseVerbose strips a --verbose flag from args and reports whether it was
// present. Subcommands call it before their own flag parsing so --verbose works
// uniformly across every command and never reaches the command-specific parser.
func parseVerbose(args []string) (rest []string, verbose bool) {
	rest = make([]string, 0, len(args))
	for _, a := range args {
		if a == "--verbose" {
			verbose = true
			continue
		}
		rest = append(rest, a)
	}
	return rest, verbose
}

// newLogger builds a command logger writing to verboseLogSink when verbose, or a
// disabled no-op logger otherwise.
func newLogger(command string, verbose bool) *clilog.Logger {
	if !verbose {
		return clilog.New(nil, command)
	}
	return clilog.New(verboseLogSink, command)
}
