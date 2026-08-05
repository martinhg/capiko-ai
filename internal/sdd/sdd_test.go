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
	}, map[string]string{
		"explore": "high", // override default low
	}, false, nil)
	if strings.Contains(out, "Strict TDD") {
		t.Error("strict TDD section should be absent when off")
	}

	for _, want := range []string{
		"SDD Orchestrator",
		"| orchestrator | claude-opus-4.8 | high |",
		"| spec | gemini-5.4 | medium |",
		"| explore | default | high |", // effort overridden, model defaults
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
	out := Render(nil, nil, false, nil)

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

func TestRenderResultContract(t *testing.T) {
	out := Render(nil, nil, false, nil)
	for _, want := range []string{
		"### Result contract",
		"status",
		"executive_summary",
		"artifacts",
		"next_recommended",
		"risks",
		"skill_resolution",
		"malformed result",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("result contract section missing %q\n---\n%s", want, out)
		}
	}
}

func TestRenderExecutionMode(t *testing.T) {
	out := Render(nil, nil, false, nil)
	for _, want := range []string{
		"### Execution mode",
		"Automatic",
		"Interactive",
		"ask once and cache",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("execution mode section missing %q\n---\n%s", want, out)
		}
	}
}

func TestRenderAutomaticModeGatekeeper(t *testing.T) {
	out := Render(nil, nil, false, nil)
	for _, want := range []string{
		"### Automatic mode gatekeeper",
		"Inline checks (all phases)",
		"Result contract completeness",
		"Artifact retrievability",
		"Scope consistency",
		"Fresh-context review",
		"Anti-hallucination",
		"Routing coherence",
		"On gate failure",
		"Re-run the failed phase once",
		"corrective feedback",
		"retry also fails",
		"Never skip a gate failure",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("automatic mode gatekeeper section missing %q\n---\n%s", want, out)
		}
	}
}

func TestRenderSkillResolution(t *testing.T) {
	out := Render(nil, nil, false, nil)
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
	out := Render(nil, nil, false, nil)
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
	out := Render(nil, nil, false, nil)
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

func TestDefaultEfforts(t *testing.T) {
	e := DefaultEfforts()
	if len(e) != len(Phases) {
		t.Fatalf("efforts = %d, want %d", len(e), len(Phases))
	}
	want := map[string]string{
		"orchestrator": "high", "explore": "low", "propose": "medium",
		"spec": "medium", "design": "high", "tasks": "low",
		"apply": "medium", "verify": "high", "archive": "low",
	}
	for p, w := range want {
		if e[p] != w {
			t.Errorf("%s effort = %q, want %q", p, e[p], w)
		}
	}
}

func TestRenderEffortColumn(t *testing.T) {
	out := Render(nil, nil, false, nil)
	for _, want := range []string{
		"| Phase | Model | Effort |",
		"Reasoning effort:",
		"| explore | default | low |",
		"| design | default | high |",
		"| tasks | default | low |",
		"| apply | default | medium |",
		"| verify | default | high |",
		"| archive | default | low |",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("effort column missing %q\n---\n%s", want, out)
		}
	}
}

func TestRenderNoLifecycleGuardrails(t *testing.T) {
	out := Render(nil, nil, false, nil)
	if strings.Contains(out, "### Engram lifecycle guardrails") {
		t.Error("lifecycle guardrails were deduplicated into memory.go — must not appear in SDD block")
	}
}

func TestRenderStrictTDD(t *testing.T) {
	out := Render(nil, nil, true, nil)
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
	on := Render(nil, nil, true, nil)
	for _, want := range []string{"forward", "strict_tdd: true", "test command"} {
		if !strings.Contains(on, want) {
			t.Errorf("strict-TDD forwarding instruction missing %q when on:\n%s", want, on)
		}
	}

	off := Render(nil, nil, false, nil)
	if strings.Contains(off, "strict_tdd: true") {
		t.Error("forwarding token strict_tdd: true must not appear when strict TDD is off")
	}
}

func TestRenderIgnoresUnknownAndEmpty(t *testing.T) {
	out := Render(map[string]string{
		"orchestrator": "", // empty → default
		"bogus-phase":  "x",
	}, nil, false, nil)
	if !strings.Contains(out, "| orchestrator | default | high |") {
		t.Error("empty assignment should fall back to default")
	}
	if strings.Contains(out, "bogus-phase") {
		t.Error("unknown phase should be ignored")
	}
}

// TestRenderModelFallbackOmittedWhenEmpty pins NFR-1: the fallback section must
// be entirely absent — not merely empty — for every shape of "no fallback
// configured" (nil, empty map, or a map whose only value is empty/whitespace).
func TestRenderModelFallbackOmittedWhenEmpty(t *testing.T) {
	for _, fb := range []map[string]string{nil, {}, {"apply": ""}} {
		out := Render(nil, nil, false, fb)
		if strings.Contains(out, "Model fallback on exhaustion") {
			t.Errorf("fallback section should be absent for %v", fb)
		}
	}
}

// TestRenderIgnoresUnknownFallbackPhase mirrors TestRenderIgnoresUnknownAndEmpty:
// unknown phase keys are silently dropped, and an empty value does not count as
// "configured" (A2/A3 in the spec — unlike normalize, missing phases stay absent
// rather than being filled with a default).
func TestRenderIgnoresUnknownFallbackPhase(t *testing.T) {
	out := Render(map[string]string{"apply": "claude-opus-4.8"}, nil, false, map[string]string{
		"apply":       "claude-sonnet-4.6",
		"bogus-phase": "x",
		"spec":        "",
	})
	if !strings.Contains(out, "| apply | claude-opus-4.8 | claude-sonnet-4.6 |") {
		t.Error("configured fallback for a known phase should render")
	}
	if strings.Contains(out, "bogus-phase") {
		t.Error("unknown fallback phase should be ignored")
	}
}

// TestRenderModelFallback pins REQ-6 through REQ-9: the fallback table, all
// seven detection heuristics, and the retry/terminal rule labels.
func TestRenderModelFallback(t *testing.T) {
	out := Render(map[string]string{"apply": "claude-opus-4.8"}, nil, false, map[string]string{
		"apply": "claude-sonnet-4.6", "verify": "gpt-5.2",
	})
	for _, want := range []string{
		"### Model fallback on exhaustion",
		"| apply | claude-opus-4.8 | claude-sonnet-4.6 |",
		"| verify | default | gpt-5.2 |",
		"rate limit", "quota exceeded", "429", "resource_exhausted",
		"insufficient_quota", "overloaded", "capacity",
		"Retry rule", "Terminal rule", "Forwarding",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("fallback section missing %q\n---\n%s", want, out)
		}
	}
}

// TestRenderModelFallbackForwarding pins REQ-10: a machine-parseable
// `fallback_model:` token must accompany the delegation when a fallback is
// configured, and must be entirely absent when it isn't — mirroring
// TestRenderStrictTDDForwarding's on/off contract.
func TestRenderModelFallbackForwarding(t *testing.T) {
	on := Render(nil, nil, false, map[string]string{"apply": "claude-sonnet-4.6"})
	for _, want := range []string{"Forwarding", "fallback_model:"} {
		if !strings.Contains(on, want) {
			t.Errorf("fallback forwarding missing %q when on:\n%s", want, on)
		}
	}
	off := Render(nil, nil, false, nil)
	if strings.Contains(off, "fallback_model:") {
		t.Error("fallback_model token must not appear when no fallback is configured")
	}
}

// TestRenderModelFallbackExcludesContextLength pins REQ-7: the exclusion of
// "context length exceeded" from the exhaustion heuristics must appear as an
// explicit negative instruction, not merely be absent from the pattern list.
func TestRenderModelFallbackExcludesContextLength(t *testing.T) {
	out := Render(nil, nil, false, map[string]string{"apply": "claude-sonnet-4.6"})
	if !strings.Contains(out, "context length exceeded") {
		t.Error("fallback section should mention context-length exclusion")
	}
	if !strings.Contains(out, "Do NOT treat") {
		t.Error("context-length exclusion should be an explicit negative instruction, not implied")
	}
}
