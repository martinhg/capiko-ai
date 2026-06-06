package sddstatus

import (
	"encoding/json"
	"fmt"
	"strings"
)

// RenderJSON serializes a status as indented JSON — the capiko.sdd-status payload.
func RenderJSON(status Status) (string, error) {
	b, err := json.MarshalIndent(status, "", "  ")
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// changeLabel is the change name for headings, or a placeholder when unresolved.
func changeLabel(status Status) string {
	if status.ChangeName != nil {
		return *status.ChangeName
	}
	return "unresolved"
}

// jsonBlock renders the status as a fenced ```json block, best-effort.
func jsonBlock(status Status) []string {
	b, err := json.MarshalIndent(status, "", "  ")
	if err != nil {
		b = []byte("{}")
	}
	return []string{"", "### JSON", "```json", string(b), "```"}
}

// RenderMarkdown renders a human-readable status summary for orchestrators.
func RenderMarkdown(status Status) string {
	lines := []string{
		fmt.Sprintf("## SDD Status: %s", changeLabel(status)),
		"",
		fmt.Sprintf("schema: %s@%d", status.SchemaName, status.SchemaVersion),
		fmt.Sprintf("store: %s", status.ArtifactStore),
		fmt.Sprintf("planning_home: %s", status.PlanningHome.Path),
		fmt.Sprintf("next: %s", status.NextRecommended),
		"",
		"### Summary",
		fmt.Sprintf("- apply: %s", status.Dependencies.Apply),
		fmt.Sprintf("- verify: %s", status.Dependencies.Verify),
		fmt.Sprintf("- archive: %s", status.Dependencies.Archive),
		fmt.Sprintf("- tasks: %d/%d complete", status.TaskProgress.Completed, status.TaskProgress.Total),
	}
	lines = append(lines, blockedReasonsBlock(status)...)
	lines = append(lines, jsonBlock(status)...)
	return strings.Join(lines, "\n")
}

// RenderDispatcherMarkdown renders the routing view for the sdd-continue command.
func RenderDispatcherMarkdown(status Status) string {
	lines := []string{
		fmt.Sprintf("## Native SDD Dispatcher: %s", changeLabel(status)),
		"",
		"Native status is authoritative. Route by next_recommended and dependency state, not prompt inference.",
		"",
		fmt.Sprintf("next_recommended: %s", status.NextRecommended),
		"",
		"### Dependency States",
		fmt.Sprintf("- proposal: %s", status.Dependencies.Proposal),
		fmt.Sprintf("- specs: %s", status.Dependencies.Specs),
		fmt.Sprintf("- design: %s", status.Dependencies.Design),
		fmt.Sprintf("- tasks: %s", status.Dependencies.Tasks),
		fmt.Sprintf("- apply: %s", status.Dependencies.Apply),
		fmt.Sprintf("- verify: %s", status.Dependencies.Verify),
		fmt.Sprintf("- archive: %s", status.Dependencies.Archive),
		fmt.Sprintf("- task_progress: %d/%d complete", status.TaskProgress.Completed, status.TaskProgress.Total),
	}
	lines = append(lines, blockedReasonsBlock(status)...)
	lines = append(lines, jsonBlock(status)...)
	return strings.Join(lines, "\n")
}

func blockedReasonsBlock(status Status) []string {
	if len(status.BlockedReasons) == 0 {
		return nil
	}
	lines := []string{"", "### Blocked Reasons"}
	for _, reason := range status.BlockedReasons {
		lines = append(lines, "- "+reason)
	}
	return lines
}
