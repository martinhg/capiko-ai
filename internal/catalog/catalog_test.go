package catalog

import (
	"strings"
	"testing"
)

func TestLoadEmbedded(t *testing.T) {
	got, err := Load()
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}
	if len(got) == 0 {
		t.Fatal("embedded catalog is empty")
	}

	byName := map[string]bool{}
	for _, s := range got {
		if s.Content == "" {
			t.Errorf("skill %q has empty content", s.Name)
		}
		if s.Description == "" {
			t.Errorf("skill %q has empty description", s.Name)
		}
		byName[s.Name] = true
	}

	if !byName["capiko-hello"] {
		t.Errorf("expected capiko-hello in catalog, got %v", byName)
	}
	if !byName["codebase-docs"] {
		t.Errorf("expected codebase-docs in catalog, got %v", byName)
	}
	if !byName["skill-creator"] {
		t.Errorf("expected skill-creator in catalog, got %v", byName)
	}

	// The SDD skills bundle must be present and parse.
	for _, name := range []string{
		"explore", "propose", "spec", "design", "tasks", "apply", "verify", "archive",
		"init", "onboard",
	} {
		if !byName["sdd-"+name] {
			t.Errorf("expected sdd-%s in catalog", name)
		}
	}
}

// TestTasksSkillEmitsWorkloadGuard pins the review-workload guard contract to the
// sdd-tasks skill body. The shared protocol (sdd-phase-common.md, section F)
// requires sdd-tasks to forecast the 400-line review budget with exact guard
// lines, and the orchestrator reads that forecast before launching apply. If the
// guard lines disappear from the skill, the executor stops emitting them and the
// orchestrator's workload gate silently goes blind — so guard them here.
func TestTasksSkillEmitsWorkloadGuard(t *testing.T) {
	got, err := Load()
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}

	var tasks string
	for _, s := range got {
		if s.Name == "sdd-tasks" {
			tasks = s.Content
			break
		}
	}
	if tasks == "" {
		t.Fatal("sdd-tasks skill not found or has empty content")
	}

	required := []string{
		"Review Workload Forecast",
		"Decision needed before apply:",
		"Chained PRs recommended:",
		"400-line budget risk:",
	}
	for _, want := range required {
		if !strings.Contains(tasks, want) {
			t.Errorf("sdd-tasks skill must contain workload-guard line %q, but it is missing", want)
		}
	}
}
