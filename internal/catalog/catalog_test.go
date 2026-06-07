package catalog

import (
	"strings"
	"testing"

	"github.com/martinhg/capiko-ai/internal/skill"
)

// TestLoadAgents_ReturnsNine exercises the embedded agents catalog at test
// time so any authoring error is caught before the binary ships.
func TestLoadAgents_ReturnsNine(t *testing.T) {
	agents, err := LoadAgents()
	if err != nil {
		t.Fatalf("LoadAgents error: %v", err)
	}
	if len(agents) != 9 {
		t.Fatalf("expected 9 agents from embedded catalog, got %d", len(agents))
	}
	for _, a := range agents {
		if a.Name == "" {
			t.Error("agent has empty name")
		}
		if a.Description == "" {
			t.Errorf("agent %q has empty description", a.Name)
		}
		if a.Content == "" {
			t.Errorf("agent %q has empty content", a.Name)
		}
	}
}

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
	if !byName["skill-registry"] {
		t.Errorf("expected skill-registry in catalog, got %v", byName)
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

// TestSDDStrategyDecisionFlow pins the delivery-strategy + chain-strategy
// decision flow to the delegation layer. The orchestrator block describes it,
// but the coordinator agent is the real delegation target that must apply the
// Review Workload Guard before launching apply, and the shared contract must
// document both chain strategies. Without this, the orchestrator's flow has no
// receiver in the agent that actually routes the phases.
func TestSDDStrategyDecisionFlow(t *testing.T) {
	agents, err := LoadAgents()
	if err != nil {
		t.Fatalf("LoadAgents error: %v", err)
	}
	var coord string
	for _, a := range agents {
		if a.Name == "capiko-sdd-coordinator" {
			coord = a.Content
			break
		}
	}
	if coord == "" {
		t.Fatal("capiko-sdd-coordinator agent not found in catalog")
	}
	for _, want := range []string{"Review Workload", "ask-on-risk", "stacked-to-main", "feature-branch-chain"} {
		if !strings.Contains(coord, want) {
			t.Errorf("coordinator agent must carry the strategy flow %q, but it is missing", want)
		}
	}

	// The shared contract documents both chain strategies for every phase.
	got, err := Load()
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}
	var common string
	for i := range got {
		if got[i].Name == "sdd-shared" {
			for _, f := range got[i].Extra {
				if f.Path == "sdd-phase-common.md" {
					common = f.Content
				}
			}
		}
	}
	if common == "" {
		t.Fatal("sdd-shared must ship sdd-phase-common.md as an Extra file")
	}
	for _, want := range []string{"stacked-to-main", "feature-branch-chain"} {
		if !strings.Contains(common, want) {
			t.Errorf("sdd-phase-common.md must document chain strategy %q", want)
		}
	}
}

// TestApplySkillShipsStrictTDDReference pins the strict-TDD reference file to the
// sdd-apply bundle. The orchestrator's strict-TDD forwarding tells the apply
// executor to "follow strict-tdd.md"; if that reference file stops shipping with
// the skill, the forwarding dangles and the executor falls back to ad-hoc TDD.
// Guard both that the file ships as an Extra and that the SKILL.md points to it.
func TestApplySkillShipsStrictTDDReference(t *testing.T) {
	got, err := Load()
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}

	var apply *skill.Skill
	for i := range got {
		if got[i].Name == "sdd-apply" {
			apply = &got[i]
			break
		}
	}
	if apply == nil {
		t.Fatal("sdd-apply skill not found in catalog")
	}

	var ref string
	for _, f := range apply.Extra {
		if f.Path == "strict-tdd.md" {
			ref = f.Content
			break
		}
	}
	if ref == "" {
		t.Fatal("sdd-apply must ship strict-tdd.md as an Extra file with content")
	}

	// The reference must carry the core RED→GREEN protocol vocabulary.
	for _, want := range []string{"RED", "GREEN", "failing test"} {
		if !strings.Contains(ref, want) {
			t.Errorf("strict-tdd.md must cover %q", want)
		}
	}

	// The SKILL.md must point the executor at the reference file.
	if !strings.Contains(apply.Content, "strict-tdd.md") {
		t.Error("sdd-apply SKILL.md must reference strict-tdd.md so the executor loads it")
	}
}

// TestVerifySkillShipsStrictTDDReference pins the strict-TDD verification
// reference to the sdd-verify bundle. The orchestrator's strict-TDD forwarding
// tells the verify executor to follow strict-tdd-verify.md; if it stops shipping,
// the forwarding dangles and TDD compliance goes unchecked at verify time.
func TestVerifySkillShipsStrictTDDReference(t *testing.T) {
	got, err := Load()
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}

	var verify *skill.Skill
	for i := range got {
		if got[i].Name == "sdd-verify" {
			verify = &got[i]
			break
		}
	}
	if verify == nil {
		t.Fatal("sdd-verify skill not found in catalog")
	}

	var ref string
	for _, f := range verify.Extra {
		if f.Path == "strict-tdd-verify.md" {
			ref = f.Content
			break
		}
	}
	if ref == "" {
		t.Fatal("sdd-verify must ship strict-tdd-verify.md as an Extra file with content")
	}

	// The reference must carry the core verification vocabulary: it audits whether
	// the change was built test-first and grades findings by severity.
	for _, want := range []string{"test-first", "CRITICAL", "WARNING"} {
		if !strings.Contains(ref, want) {
			t.Errorf("strict-tdd-verify.md must cover %q", want)
		}
	}

	// The SKILL.md must point the executor at the reference file.
	if !strings.Contains(verify.Content, "strict-tdd-verify.md") {
		t.Error("sdd-verify SKILL.md must reference strict-tdd-verify.md so the executor loads it")
	}
}

// TestSDDAgentsForwardStrictTDD pins the structural strict-TDD forwarding to the
// delegation targets. The orchestrator forwards `strict_tdd: true` into the
// apply/verify handoff; the worker .agent.md bodies are the direct delegation
// targets, so they MUST detect that signal and load the right reference file. The
// coordinator MUST carry the rule to forward it. Without this, the forwarding the
// orchestrator block promises has no receiver and the worker silently skips TDD.
func TestSDDAgentsForwardStrictTDD(t *testing.T) {
	agents, err := LoadAgents()
	if err != nil {
		t.Fatalf("LoadAgents error: %v", err)
	}
	byName := map[string]string{}
	for _, a := range agents {
		byName[a.Name] = a.Content
	}

	cases := []struct {
		name     string
		contains []string
	}{
		// Apply/verify workers detect the forwarded signal and load their protocol.
		{"capiko-sdd-apply", []string{"strict_tdd: true", "strict-tdd.md"}},
		{"capiko-sdd-verify", []string{"strict_tdd: true", "strict-tdd-verify.md"}},
		// The coordinator must carry the rule to forward the signal downstream.
		{"capiko-sdd-coordinator", []string{"strict_tdd: true", "forward"}},
	}
	for _, c := range cases {
		body, ok := byName[c.name]
		if !ok {
			t.Errorf("agent %q not found in catalog", c.name)
			continue
		}
		for _, want := range c.contains {
			if !strings.Contains(body, want) {
				t.Errorf("agent %q must carry strict-TDD forwarding %q, but it is missing", c.name, want)
			}
		}
	}
}
