package main

import (
	"fmt"
	"io"
	"strings"

	"github.com/martinhg/capiko-ai/internal/sddstatus"
)

// sddCommand runs the native SDD subcommands. handled is false when name is not
// an SDD command, so main falls through to the configurator TUI.
//
//	capiko-ai sdd-status   [change] [--cwd <path>] [--json]
//	capiko-ai sdd-continue [change] [--cwd <path>]
func sddCommand(name string, args []string, out io.Writer) (handled bool, err error) {
	switch name {
	case "sdd-status", "sdd-continue":
	default:
		return false, nil
	}

	opts, jsonOut, err := parseSDDArgs(args)
	if err != nil {
		return true, err
	}
	status, err := sddstatus.Resolve(opts)
	if err != nil {
		return true, err
	}

	switch name {
	case "sdd-status":
		if jsonOut {
			payload, err := sddstatus.RenderJSON(status)
			if err != nil {
				return true, err
			}
			fmt.Fprintln(out, payload)
		} else {
			fmt.Fprintln(out, sddstatus.RenderMarkdown(status))
		}
	case "sdd-continue":
		fmt.Fprintln(out, sddstatus.RenderDispatcherMarkdown(status))
	}
	return true, nil
}

// parseSDDArgs parses an optional positional change name plus --cwd <path> and
// --json flags.
func parseSDDArgs(args []string) (sddstatus.ResolveOptions, bool, error) {
	var opts sddstatus.ResolveOptions
	var jsonOut bool
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "--json":
			jsonOut = true
		case arg == "--cwd":
			if i+1 >= len(args) {
				return opts, false, fmt.Errorf("--cwd requires a path")
			}
			i++
			opts.Cwd = args[i]
		case strings.HasPrefix(arg, "--cwd="):
			opts.Cwd = strings.TrimPrefix(arg, "--cwd=")
		case strings.HasPrefix(arg, "-"):
			return opts, false, fmt.Errorf("unknown flag %q", arg)
		default:
			if opts.ChangeName != "" {
				return opts, false, fmt.Errorf("unexpected argument %q", arg)
			}
			opts.ChangeName = arg
		}
	}
	return opts, jsonOut, nil
}
