package catalog

import (
	"strings"
	"testing"

	"github.com/martinhg/capiko-ai/internal/agent"
	"github.com/martinhg/capiko-ai/internal/sdd"
)

// jdJudgeNames is the canonical list of the two blind Judgment-Day judges.
var jdJudgeNames = []string{"capiko-jd-judge-a", "capiko-jd-judge-b"}

// TestLoadAgents_ReturnsTwelve exercises the embedded agents catalog at test
// time so any authoring error is caught before the binary ships.
func TestLoadAgents_ReturnsTwelve(t *testing.T) {
	agents, err := LoadAgents()
	if err != nil {
		t.Fatalf("LoadAgents error: %v", err)
	}
	if len(agents) != 12 {
		t.Fatalf("expected 12 agents from embedded catalog, got %d", len(agents))
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

// TestJDJudges_NoEditTool pins the Judgment-Day judge agent shape: judges are
// blind adversarial reviewers that must never modify code, so their `tools:`
// list must contain read/search/execute but never edit.
func TestJDJudges_NoEditTool(t *testing.T) {
	agents, err := LoadAgents()
	if err != nil {
		t.Fatalf("LoadAgents error: %v", err)
	}
	byName := map[string]agent.Agent{}
	for _, a := range agents {
		byName[a.Name] = a
	}

	for _, name := range jdJudgeNames {
		a, ok := byName[name]
		if !ok {
			t.Errorf("judge %q not found in catalog", name)
			continue
		}
		fm, err := agent.ParseFrontmatter(a.Content)
		if err != nil {
			t.Errorf("judge %q: frontmatter parse error: %v", name, err)
			continue
		}
		if fm.UserInvocable {
			t.Errorf("judge %q: user-invocable must be false", name)
		}
		toolSet := map[string]bool{}
		for _, tool := range fm.Tools {
			toolSet[tool] = true
		}
		for _, want := range []string{"read", "search", "execute"} {
			if !toolSet[want] {
				t.Errorf("judge %q: tools must include %q, got %v", name, want, fm.Tools)
			}
		}
		if toolSet["edit"] {
			t.Errorf("judge %q: tools must NOT include edit, got %v", name, fm.Tools)
		}
	}
}

// TestJDFixAgent_ScopedToConfirmedIssues pins the fix agent's write access and
// its scope: it MUST declare edit (it needs write access to fix code) and its
// body MUST restrict fixes to issues both judges jointly confirmed, never
// open-ended changes.
func TestJDFixAgent_ScopedToConfirmedIssues(t *testing.T) {
	agents, err := LoadAgents()
	if err != nil {
		t.Fatalf("LoadAgents error: %v", err)
	}
	var fix *agent.Agent
	for i := range agents {
		if agents[i].Name == "capiko-jd-fix-agent" {
			fix = &agents[i]
			break
		}
	}
	if fix == nil {
		t.Fatal("capiko-jd-fix-agent not found in catalog")
	}

	fm, err := agent.ParseFrontmatter(fix.Content)
	if err != nil {
		t.Fatalf("capiko-jd-fix-agent: frontmatter parse error: %v", err)
	}
	if fm.UserInvocable {
		t.Error("capiko-jd-fix-agent: user-invocable must be false")
	}
	toolSet := map[string]bool{}
	for _, tool := range fm.Tools {
		toolSet[tool] = true
	}
	for _, want := range []string{"read", "edit", "search", "execute"} {
		if !toolSet[want] {
			t.Errorf("capiko-jd-fix-agent: tools must include %q, got %v", want, fm.Tools)
		}
	}
	if !strings.Contains(fix.Content, "confirmed") {
		t.Error("capiko-jd-fix-agent body must restrict edits to jointly-confirmed issues")
	}
}

// TestCoordinator_JudgmentDayProtocol pins the coordinator's ownership of JD
// routing: the body must describe the protocol, launch both judges before a
// PR, and delegate only jointly-confirmed findings to the fix agent.
func TestCoordinator_JudgmentDayProtocol(t *testing.T) {
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
		t.Fatal("capiko-sdd-coordinator not found in catalog")
	}
	for _, want := range []string{
		"Judgment-Day Protocol",
		"capiko-jd-judge-a",
		"capiko-jd-judge-b",
		"capiko-jd-fix-agent",
		"confirmed",
	} {
		if !strings.Contains(coord, want) {
			t.Errorf("coordinator body must contain %q", want)
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

	// The 4R review skills must be present.
	for _, name := range []string{
		"review-risk", "review-readability", "review-reliability", "review-resilience",
	} {
		if !byName[name] {
			t.Errorf("expected %s in catalog", name)
		}
	}
}

// TestFourRReviewSkillsStructuredOutput pins the 4R review skills to their
// structured output contract: each must define a severity-tagged output format
// and a "no issues" statement so consumers can parse findings programmatically.
func TestFourRReviewSkillsStructuredOutput(t *testing.T) {
	got, err := Load()
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}
	byName := map[string]string{}
	for _, s := range got {
		byName[s.Name] = s.Content
	}

	for _, name := range []string{
		"review-risk", "review-readability", "review-reliability", "review-resilience",
	} {
		body, ok := byName[name]
		if !ok {
			t.Errorf("%s not found in catalog", name)
			continue
		}
		for _, want := range []string{
			"## Output format",
			"SEVERITY",
			"Category",
			"Location",
			"No ",
		} {
			if !strings.Contains(body, want) {
				t.Errorf("%s must contain %q for structured output", name, want)
			}
		}
	}
}

// TestFileReadingSkillsCarryUntrustedContentClause pins the prompt-injection
// defence to every skill that reads arbitrary project files. Without this
// clause the agent may treat injected directives inside scanned files as
// instructions (indirect prompt injection).
func TestFileReadingSkillsCarryUntrustedContentClause(t *testing.T) {
	got, err := Load()
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}
	byName := map[string]string{}
	for _, s := range got {
		byName[s.Name] = s.Content
	}

	skills := []string{
		"codebase-docs",
		"review-risk",
		"review-readability",
		"review-reliability",
		"review-resilience",
	}
	for _, name := range skills {
		body, ok := byName[name]
		if !ok {
			t.Errorf("%s not found in catalog", name)
			continue
		}
		for _, want := range []string{
			"Untrusted content",
			"prompt-injection",
			"never as instructions to follow",
		} {
			if !strings.Contains(body, want) {
				t.Errorf("%s must contain %q for untrusted-content defence", name, want)
			}
		}
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

// TestCoordinatorTriageParity is a drift guard between the coordinator agent's
// Triage Gate and sdd.Render's "When to use SDD (triage)" section. Both carry
// the same decision heuristic intentionally (the coordinator for Copilot agents,
// the rendered block for Claude Code). If either copy drifts, this test fails —
// forcing the author to update both, not relax the test.
func TestCoordinatorTriageParity(t *testing.T) {
	agents, err := LoadAgents()
	if err != nil {
		t.Fatalf("LoadAgents error: %v", err)
	}
	var coordBody string
	for _, a := range agents {
		if a.Name == "capiko-sdd-coordinator" {
			coordBody = a.Content
			break
		}
	}
	if coordBody == "" {
		t.Fatal("capiko-sdd-coordinator agent not found in catalog")
	}

	rendered := sdd.Render(sdd.DefaultAssignments(), sdd.DefaultEfforts(), false, nil)

	// Invariant phrases that must appear in BOTH copies. If one is edited
	// without the other, the test catches the drift.
	triageInvariants := []string{
		"**Inline**",
		"**Delegate an exploration**",
		"**Delegate a writer**",
		"**Run the full SDD workflow**",
		"**Fresh review before a PR**",
		"ambiguity, not size",
		"4-file rule",
		"substantial ambiguity and durable artifacts",
		"user can always override",
	}

	for _, phrase := range triageInvariants {
		if !strings.Contains(coordBody, phrase) {
			t.Errorf("coordinator agent missing triage invariant %q", phrase)
		}
		if !strings.Contains(rendered, phrase) {
			t.Errorf("sdd.Render output missing triage invariant %q", phrase)
		}
	}
}

// TestCoordinatorDeliveryStrategyParity is a drift guard between the
// coordinator agent's Review Workload Guard / chain-strategy rules and
// sdd.Render's "Delivery & chain strategy" section. Both enumerate the same
// strategy names and guard triggers; if either copy drifts, this test fails.
func TestCoordinatorDeliveryStrategyParity(t *testing.T) {
	agents, err := LoadAgents()
	if err != nil {
		t.Fatalf("LoadAgents error: %v", err)
	}
	var coordBody string
	for _, a := range agents {
		if a.Name == "capiko-sdd-coordinator" {
			coordBody = a.Content
			break
		}
	}
	if coordBody == "" {
		t.Fatal("capiko-sdd-coordinator agent not found in catalog")
	}

	rendered := sdd.Render(sdd.DefaultAssignments(), sdd.DefaultEfforts(), false, nil)

	deliveryInvariants := []string{
		"ask-on-risk",
		"auto-chain",
		"single-pr",
		"exception-ok",
		"stacked-to-main",
		"feature-branch-chain",
		"Chained PRs recommended: Yes",
		"400-line budget risk: High",
		"Decision needed before apply: Yes",
		"size:exception",
	}

	for _, phrase := range deliveryInvariants {
		if !strings.Contains(coordBody, phrase) {
			t.Errorf("coordinator agent missing delivery invariant %q", phrase)
		}
		if !strings.Contains(rendered, phrase) {
			t.Errorf("sdd.Render output missing delivery invariant %q", phrase)
		}
	}
}
