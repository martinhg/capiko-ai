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
	}, false)
	if strings.Contains(out, "Strict TDD") {
		t.Error("strict TDD section should be absent when off")
	}

	for _, want := range []string{
		"SDD Orchestrator",
		"| orchestrator | claude-opus-4.8 |",
		"| spec | gemini-5.4 |",
		"| explore | default |", // unspecified phase defaults
		"Task tool",
		"openspec/changes/", // OpenSpec store
		"openspec/specs/",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("rendered block missing %q\n---\n%s", want, out)
		}
	}
}

func TestRenderTriageGate(t *testing.T) {
	out := Render(nil, false)

	for _, want := range []string{
		"When to use SDD (triage)",
		"1–3 files to decide or verify",
		"git/state check",
		"4-file rule",
		"Delegate a writer",
		"2+ non-trivial files with new logic",
		"proposal → spec/design → tasks → apply → verify → archive",
		"Fresh review before a PR",
		"When in doubt, stay inline",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("triage gate missing %q\n---\n%s", want, out)
		}
	}
}

func TestRenderSkillResolution(t *testing.T) {
	out := Render(nil, false)
	for _, want := range []string{
		"### Skill resolution",
		"capiko-ai skill-registry",
		"Pass paths, not summaries",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("skill-resolution rule missing %q\n---\n%s", want, out)
		}
	}
}

func TestRenderDeliveryChainStrategy(t *testing.T) {
	out := Render(nil, false)
	for _, want := range []string{
		"### Delivery & chain strategy",
		// The four delivery strategies, asked once and cached.
		"ask-on-risk",
		"auto-chain",
		"single-pr",
		"exception-ok",
		// The guard that resolves them after tasks, before apply.
		"Review Workload Forecast",
		"size:exception",
		// The two chain strategies, asked when chaining.
		"stacked-to-main",
		"feature-branch-chain",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("delivery/chain strategy section missing %q\n---\n%s", want, out)
		}
	}
}

func TestRenderArtifactStore(t *testing.T) {
	out := Render(nil, false)
	for _, want := range []string{
		"### Artifact store",
		// The four modes, with hybrid as the default.
		"hybrid",
		"engram",
		"openspec",
		"none",
		// In engram/hybrid mode the agent reads engram directly.
		"mem_search",
		"mem_get_observation",
		// The native engine stays openspec-only.
		"sdd-status",
		// Multi-repo project attribution.
		".engram/config.json",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("artifact store section missing %q\n---\n%s", want, out)
		}
	}
}

func TestRenderStrictTDD(t *testing.T) {
	out := Render(nil, true)
	if !strings.Contains(out, "Strict TDD") || !strings.Contains(out, "failing test FIRST") {
		t.Errorf("strict TDD section missing when on:\n%s", out)
	}
}

// TestRenderStrictTDDForwarding pins the structural forwarding contract: when
// strict TDD is on, the orchestrator block must instruct the coordinator to
// FORWARD the strict-TDD signal into the apply/verify sub-agent handoff (the
// `strict_tdd: true` token the reference files key off), not merely state the
// rule. When off, that token must be absent so the worker takes the standard flow.
func TestRenderStrictTDDForwarding(t *testing.T) {
	on := Render(nil, true)
	for _, want := range []string{"forward", "strict_tdd: true", "test command"} {
		if !strings.Contains(on, want) {
			t.Errorf("strict-TDD forwarding instruction missing %q when on:\n%s", want, on)
		}
	}

	off := Render(nil, false)
	if strings.Contains(off, "strict_tdd: true") {
		t.Error("forwarding token strict_tdd: true must not appear when strict TDD is off")
	}
}

func TestRenderIgnoresUnknownAndEmpty(t *testing.T) {
	out := Render(map[string]string{
		"orchestrator": "", // empty → default
		"bogus-phase":  "x",
	}, false)
	if !strings.Contains(out, "| orchestrator | default |") {
		t.Error("empty assignment should fall back to default")
	}
	if strings.Contains(out, "bogus-phase") {
		t.Error("unknown phase should be ignored")
	}
}
