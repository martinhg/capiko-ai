package main

import (
	"fmt"
	"io"

	"github.com/martinhg/capiko-ai/internal/skillregistry"
)

// skillRegistryCommand runs the native skill-registry engine. handled is false
// when name is not the skill-registry command, so main falls through to the
// configurator TUI.
//
//	capiko-ai skill-registry [--cwd <path>]
//
// It prints the registry index to out, so an orchestrator can shell out for a
// fresh index instead of reading a stale file.
func skillRegistryCommand(name string, args []string, out io.Writer) (handled bool, err error) {
	if name != "skill-registry" {
		return false, nil
	}
	opts, err := parseSkillRegistryArgs(args)
	if err != nil {
		return true, err
	}
	reg, err := skillregistry.Resolve(opts)
	if err != nil {
		return true, err
	}
	fmt.Fprint(out, skillregistry.RenderMarkdown(reg))
	return true, nil
}

// parseSkillRegistryArgs parses the optional --cwd <path> flag. Home resolution
// is left to the engine (the user's real home), so only the workspace root is
// overridable here.
func parseSkillRegistryArgs(args []string) (skillregistry.ResolveOptions, error) {
	var opts skillregistry.ResolveOptions
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--cwd":
			if i+1 >= len(args) {
				return opts, fmt.Errorf("--cwd requires a path")
			}
			opts.Cwd = args[i+1]
			i++
		default:
			return opts, fmt.Errorf("unknown argument: %s", args[i])
		}
	}
	return opts, nil
}
