package catalog

import (
	"strings"
	"testing"

	"github.com/martinhg/capiko-ai/internal/agent"
)

// jdJudgeNames is the canonical list of the two blind Judgment-Day judges.
var jdJudgeNames = []string{"capiko-jd-judge-a", "capiko-jd-judge-b"}

// TestLoadAgents_ReturnsThree exercises the embedded agents catalog at test
// time so any authoring error is caught before the binary ships.
func TestLoadAgents_ReturnsThree(t *testing.T) {
	agents, err := LoadAgents()
	if err != nil {
		t.Fatalf("LoadAgents error: %v", err)
	}
	if len(agents) != 3 {
		t.Fatalf("expected 3 agents from embedded catalog, got %d", len(agents))
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
