package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/martinhg/capiko-ai/internal/copilot"
	"github.com/martinhg/capiko-ai/internal/state"
	"github.com/martinhg/capiko-ai/internal/tui"
)

type purgeInputs struct {
	host         *copilot.Host
	hostExitCode int
	hostErr      error
	store        *state.Store
	workspace    string
}

var gatherPurgeInputs = func() (purgeInputs, error) {
	host, exitCode, hostErr := requireHost(copilot.Detect)
	in := purgeInputs{host: host, hostExitCode: exitCode, hostErr: hostErr}
	if exitCode != 0 {
		return in, nil
	}

	store, _ := state.DefaultStore()
	in.store = store

	cwd, err := os.Getwd()
	if err != nil {
		return in, fmt.Errorf("resolving workspace: %w", err)
	}
	in.workspace = cwd
	return in, nil
}

// PurgeResult is the JSON-serializable outcome of the purge command.
type PurgeResult struct {
	OK                 bool     `json:"ok"`
	Command            string   `json:"command"`
	Skills             []string `json:"skills,omitempty"`
	Agents             []string `json:"agents,omitempty"`
	InstructionBlocks  []string `json:"instruction_blocks,omitempty"`
	MCPEntries         []string `json:"mcp_entries,omitempty"`
	CopilotHooks       []string `json:"copilot_hooks,omitempty"`
	GitHooks           []string `json:"git_hooks,omitempty"`
	ScopedInstructions []string `json:"scoped_instructions,omitempty"`
	WorkspaceArtifacts []string `json:"workspace_artifacts,omitempty"`
	HomeArtifacts      []string `json:"home_artifacts,omitempty"`
	VSCodeEntries      []string `json:"vscode_entries,omitempty"`
	TotalRemoved       int      `json:"total_removed"`
	Warnings           []string `json:"warnings,omitempty"`
	Error              string   `json:"error,omitempty"`
}

func purgeResultFromReport(rpt tui.PurgeReport) PurgeResult {
	return PurgeResult{
		OK:                 true,
		Command:            "purge",
		Skills:             rpt.Skills,
		Agents:             rpt.Agents,
		InstructionBlocks:  rpt.InstructionBlocks,
		MCPEntries:         rpt.MCPEntries,
		CopilotHooks:       rpt.CopilotHooks,
		GitHooks:           rpt.GitHooks,
		ScopedInstructions: rpt.ScopedInstructions,
		WorkspaceArtifacts: rpt.WorkspaceArtifacts,
		HomeArtifacts:      rpt.HomeArtifacts,
		VSCodeEntries:      rpt.VSCodeEntries,
		TotalRemoved:       rpt.TotalRemoved(),
		Warnings:           rpt.Warnings,
	}
}

func purgeResultError(errMsg string) PurgeResult {
	return PurgeResult{
		OK:      false,
		Command: "purge",
		Error:   errMsg,
	}
}

// purgeCommand removes every trace of capiko from the system.
//
//	capiko-ai purge [--json] [--confirm]
func purgeCommand(name string, args []string, out io.Writer) (handled bool, exitCode int, err error) {
	if name != "purge" {
		return false, 0, nil
	}
	args, verbose := parseVerbose(args)
	log := newLogger("purge", verbose)
	asJSON, confirm, err := parsePurgeArgs(args)
	if err != nil {
		return true, 1, err
	}

	in, err := gatherPurgeInputs()
	if err != nil {
		return true, 1, err
	}

	if in.hostExitCode != 0 {
		msg := "GitHub Copilot CLI not found"
		if in.hostExitCode == 1 && in.hostErr != nil {
			msg = in.hostErr.Error()
		}
		renderPurge(out, purgeResultError(msg), asJSON)
		return true, in.hostExitCode, in.hostErr
	}

	if !confirm {
		renderPurgePreview(out, asJSON)
		return true, 0, nil
	}

	donePurge := log.Step("purge-all")
	rpt, err := tui.PurgeAll(tui.PurgeConfig{
		Host:      in.host,
		Store:     in.store,
		Workspace: in.workspace,
	})
	if err != nil {
		donePurge("error")
		r := purgeResultError(err.Error())
		r.Warnings = rpt.Warnings
		renderPurge(out, r, asJSON)
		return true, 1, nil
	}
	donePurge("ok")

	renderPurge(out, purgeResultFromReport(rpt), asJSON)
	return true, 0, nil
}

func renderPurge(out io.Writer, r PurgeResult, asJSON bool) {
	if asJSON {
		b, err := json.MarshalIndent(r, "", "  ")
		if err == nil {
			fmt.Fprintln(out, string(b))
		}
		return
	}

	fmt.Fprintln(out, "capiko-ai purge")
	fmt.Fprintln(out)

	if r.Error != "" {
		fmt.Fprintln(out, r.Error)
		return
	}

	sections := []struct {
		label string
		items []string
	}{
		{"Skills", r.Skills},
		{"Agents", r.Agents},
		{"Instruction blocks", r.InstructionBlocks},
		{"MCP entries", r.MCPEntries},
		{"Copilot hooks", r.CopilotHooks},
		{"Git hooks", r.GitHooks},
		{"Scoped instructions", r.ScopedInstructions},
		{"Workspace artifacts", r.WorkspaceArtifacts},
		{"Home artifacts", r.HomeArtifacts},
		{"VS Code entries", r.VSCodeEntries},
	}

	for _, s := range sections {
		if len(s.items) == 0 {
			continue
		}
		fmt.Fprintf(out, "  %s\n", s.label)
		for _, item := range s.items {
			fmt.Fprintf(out, "  - %s\n", item)
		}
		fmt.Fprintln(out)
	}

	if r.TotalRemoved == 0 {
		fmt.Fprintln(out, "Nothing to remove.")
	} else {
		fmt.Fprintf(out, "%d item(s) removed.\n", r.TotalRemoved)
	}

	for _, w := range r.Warnings {
		fmt.Fprintf(out, "! %s\n", w)
	}

	fmt.Fprintln(out)
	fmt.Fprintln(out, "To remove the binary itself:")
	fmt.Fprintln(out, "  brew uninstall capiko-ai    # if installed via Homebrew")
	fmt.Fprintln(out, "  go clean -i github.com/martinhg/capiko-ai/cmd/capiko-ai  # if installed via go install")
}

func renderPurgePreview(out io.Writer, asJSON bool) {
	if asJSON {
		r := PurgeResult{OK: true, Command: "purge", Error: "run with --confirm to proceed"}
		b, _ := json.MarshalIndent(r, "", "  ")
		fmt.Fprintln(out, string(b))
		return
	}

	fmt.Fprintln(out, "capiko-ai purge")
	fmt.Fprintln(out)
	fmt.Fprintln(out, "This will remove ALL capiko data from your system:")
	fmt.Fprintln(out)
	fmt.Fprintln(out, "  - Managed skills and agents")
	fmt.Fprintln(out, "  - Instruction blocks in copilot-instructions.md")
	fmt.Fprintln(out, "  - MCP server entries (engram, headroom)")
	fmt.Fprintln(out, "  - Copilot CLI hooks (capiko-*)")
	fmt.Fprintln(out, "  - Scoped instruction files")
	fmt.Fprintln(out, "  - VS Code MCP entries")
	fmt.Fprintln(out, "  - Workspace git hooks and review state")
	fmt.Fprintln(out, "  - ~/.capiko/ (state, backups, all data)")
	fmt.Fprintln(out)
	fmt.Fprintln(out, "Run with --confirm to proceed.")
}

func parsePurgeArgs(args []string) (asJSON, confirm bool, err error) {
	for _, a := range args {
		switch a {
		case "--json":
			asJSON = true
		case "--confirm":
			confirm = true
		default:
			return false, false, fmt.Errorf("purge: unknown argument %q", a)
		}
	}
	return asJSON, confirm, nil
}
