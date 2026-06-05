package sdd

import (
	"strings"
	"testing"
)

func TestDefaultAssignments(t *testing.T) {
	a := DefaultAssignments()
	if len(a) != len(Phases) {
		t.Fatalf("assignments = %d, want %d", len(a), len(Phases))
	}
	for _, p := range Phases {
		if a[p] != DefaultModel {
			t.Errorf("%s = %q, want %q", p, a[p], DefaultModel)
		}
	}
}

func TestRenderReflectsAssignments(t *testing.T) {
	out := Render(map[string]string{
		"orchestrator": "claude-opus-4.8",
		"spec":         "gemini-5.4",
		// the rest fall back to default
	})

	for _, want := range []string{
		"SDD Orchestrator",
		"| orchestrator | claude-opus-4.8 |",
		"| spec | gemini-5.4 |",
		"| explore | default |", // unspecified phase defaults
		"Task tool",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("rendered block missing %q\n---\n%s", want, out)
		}
	}
}

func TestRenderIgnoresUnknownAndEmpty(t *testing.T) {
	out := Render(map[string]string{
		"orchestrator": "", // empty → default
		"bogus-phase":  "x",
	})
	if !strings.Contains(out, "| orchestrator | default |") {
		t.Error("empty assignment should fall back to default")
	}
	if strings.Contains(out, "bogus-phase") {
		t.Error("unknown phase should be ignored")
	}
}
