// Package headless renders the outcome of a headless capiko-ai command
// (install/sync/uninstall) as text or JSON. It is a pure render package: no
// domain imports beyond the tui.ReconcileResult conversion helper, so it
// stays free of any dependency on the Copilot host or the filesystem.
package headless

import (
	"encoding/json"
	"fmt"
	"io"

	"github.com/martinhg/capiko-ai/internal/tui"
)

// ItemChanges lists the names of skills and agents installed or removed by a
// headless command.
type ItemChanges struct {
	InstalledSkills []string `json:"installed_skills"`
	InstalledAgents []string `json:"installed_agents"`
	RemovedSkills   []string `json:"removed_skills"`
	RemovedAgents   []string `json:"removed_agents"`
}

// CommandResult is the JSON-serializable outcome of a headless command.
type CommandResult struct {
	OK           bool        `json:"ok"`
	Command      string      `json:"command"`
	Items        ItemChanges `json:"items"`
	TotalChanged int         `json:"total_changed"`
	Error        string      `json:"error"`
}

// FromReconcileResult converts a tui.ReconcileResult (plus the error from the
// operation that produced it, if any) into a CommandResult ready to render.
// OK is true exactly when err is nil.
func FromReconcileResult(cmd string, r tui.ReconcileResult, err error) CommandResult {
	if err != nil {
		return CommandResult{
			OK:      false,
			Command: cmd,
			Error:   err.Error(),
		}
	}
	return CommandResult{
		OK:      true,
		Command: cmd,
		Items: ItemChanges{
			InstalledSkills: r.InstalledSkills,
			InstalledAgents: r.InstalledAgents,
			RemovedSkills:   r.RemovedSkills,
			RemovedAgents:   r.RemovedAgents,
		},
		TotalChanged: r.TotalChanged(),
	}
}

// RenderJSON writes the result to w as indented JSON.
func RenderJSON(w io.Writer, r CommandResult) error {
	b, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return err
	}
	_, err = fmt.Fprintln(w, string(b))
	return err
}

// RenderText writes the result to w as a human-readable summary: a header
// line, indented +/- item lists grouped by Skills/Agents, and a trailing
// count line. An empty sync-style result (nothing installed or removed, no
// error) renders a "no drift" message instead of empty sections. RenderText
// does not know the process exit code, so an error result only renders the
// message line — exit-code formatting is the caller's responsibility.
func RenderText(w io.Writer, r CommandResult) {
	fmt.Fprintf(w, "capiko-ai %s\n\n", r.Command)

	if r.Error != "" {
		fmt.Fprintln(w, r.Error)
		return
	}

	wroteSection := false
	if len(r.Items.InstalledSkills) > 0 || len(r.Items.RemovedSkills) > 0 {
		fmt.Fprintln(w, "  Skills")
		for _, name := range r.Items.InstalledSkills {
			fmt.Fprintf(w, "  + %s\n", name)
		}
		for _, name := range r.Items.RemovedSkills {
			fmt.Fprintf(w, "  - %s\n", name)
		}
		fmt.Fprintln(w)
		wroteSection = true
	}
	if len(r.Items.InstalledAgents) > 0 || len(r.Items.RemovedAgents) > 0 {
		fmt.Fprintln(w, "  Agents")
		for _, name := range r.Items.InstalledAgents {
			fmt.Fprintf(w, "  + %s\n", name)
		}
		for _, name := range r.Items.RemovedAgents {
			fmt.Fprintf(w, "  - %s\n", name)
		}
		fmt.Fprintln(w)
		wroteSection = true
	}

	if !wroteSection {
		if r.Command == "sync" {
			fmt.Fprintln(w, "No drift detected, nothing to sync.")
		} else {
			fmt.Fprintln(w, "Nothing to do.")
		}
		return
	}

	installedCount := len(r.Items.InstalledSkills) + len(r.Items.InstalledAgents)
	removedCount := len(r.Items.RemovedSkills) + len(r.Items.RemovedAgents)
	switch {
	case installedCount > 0 && removedCount == 0:
		fmt.Fprintf(w, "%d item(s) installed.\n", r.TotalChanged)
	case removedCount > 0 && installedCount == 0:
		fmt.Fprintf(w, "%d item(s) removed.\n", r.TotalChanged)
	default:
		fmt.Fprintf(w, "%d item(s) changed.\n", r.TotalChanged)
	}
}
