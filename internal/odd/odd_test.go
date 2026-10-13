package odd

import (
	"strings"
	"testing"
)

func TestRenderContainsProtocol(t *testing.T) {
	out := Render(false)
	for _, want := range []string{
		"## Implementation Routing",
		"Organic Driven Development (ODD)",
		"### ODD protocol (MANDATORY",
		"1. **Authorize.**",
		"2. **Explore.**",
		"3. **Resolve uncertainty.**",
		"4. **Classify.**",
		"5. **Track before the first write.**",
		"6. **Implement task by task.**",
		"7. **Close.**",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("protocol missing %q", want)
		}
	}
}

func TestRenderContainsTaskSize(t *testing.T) {
	out := Render(false)
	for _, want := range []string{
		"### Task Size",
		"**small**",
		"**Understood**",
		"**Contained risk**",
		"**Resumable from the diff**",
		"resume test",
		"**High risk**",
		"**Risk line**",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("task size section missing %q", want)
		}
	}
}

func TestRenderContainsRoutes(t *testing.T) {
	out := Render(false)
	for _, want := range []string{
		"**Direct inline:**",
		"**Delegated direct:**",
		"include-custom-instructions: true",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("routes missing %q", want)
		}
	}
}

func TestRenderContainsDelegationTriggers(t *testing.T) {
	out := Render(false)
	for _, want := range []string{
		"### Mandatory Delegation Triggers",
		"**Ask trigger:**",
		"**Mapping trigger:**",
		"**Verification trigger:**",
		"**Tracking trigger:**",
		"**Writer trigger:**",
		"**Parallel writers:**",
		"**Preparation trigger:**",
		"**Output budget:**",
		"**Long-session backstop:**",
		"**Route declaration:**",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("delegation triggers missing %q", want)
		}
	}
}

func TestRenderContainsODDDetail(t *testing.T) {
	out := Render(false)
	for _, want := range []string{
		"### Organic Driven Development",
		"odd/tasks/<feature-name>.md",
		"mem_context",
		"mem_search",
		"mem_get_observation",
		"test-first policy",
		"observe RED before implementation",
		"work-unit commit",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("ODD detail missing %q", want)
		}
	}
}

func TestRenderContainsDeliveryStrategy(t *testing.T) {
	out := Render(false)
	for _, want := range []string{
		"ask-on-risk",
		"auto-chain",
		"single-pr",
		"exception-ok",
		"stacked-to-main",
		"feature-branch-chain",
		"400 authored changed lines",
		"work-unit-commits",
		"chained-pr",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("delivery strategy missing %q", want)
		}
	}
}

func TestRenderEvidenceBudgetValues(t *testing.T) {
	out := Render(false)
	for _, want := range []string{
		"3 calls",
		"10k tokens",
		"5 sequential lookups",
		"2k tokens",
		"150k parent-context tokens",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("evidence budget missing %q", want)
		}
	}
}

func TestRenderNoRDD(t *testing.T) {
	out := Render(false)
	if strings.Contains(out, "review trigger rules") {
		t.Error("RDD clauses should be absent when disabled")
	}
	if !strings.Contains(out, "Delegated Verification Gate") {
		t.Error("non-RDD verification should reference Delegated Verification Gate")
	}
	if strings.Contains(out, "### Risk Tiers") {
		t.Error("Risk Tiers section should be absent when RDD is disabled")
	}
}

func TestRenderWithRDD(t *testing.T) {
	out := Render(true)
	for _, want := range []string{
		"review trigger rules",
		"risk tier",
		"lenses proportionate",
		"R1 (Risk)",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("RDD clause missing %q when enabled", want)
		}
	}
}

func TestRenderRiskTiers(t *testing.T) {
	out := Render(true)
	for _, want := range []string{
		"### Risk Tiers and Lens Selection",
		"**Standard** (0)",
		"**Elevated** (1)",
		"**Critical** (4)",
		"R1 R2 R3 R4",
		"agent may raise a tier but never lower",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("risk tiers section missing %q", want)
		}
	}
}

func TestRenderRiskTiersLensSelection(t *testing.T) {
	out := Render(true)
	for _, want := range []string{
		"Self-verify",
		"delegate R1 (Risk) as independent verifier",
		"delegate R1 (Risk, strong), R2 (Readability), R3 (Reliability), R4 (Resilience)",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("lens selection guidance missing %q", want)
		}
	}
}

func TestRenderNoSDD(t *testing.T) {
	out := Render(false)
	if strings.Contains(out, "SDD") {
		t.Error("ODD block should not mention SDD")
	}
}

func TestRenderIncludeCustomInstructions(t *testing.T) {
	out := Render(false)
	count := strings.Count(out, "include-custom-instructions: true")
	if count < 1 {
		t.Error("should instruct workers to inherit custom instructions")
	}
}

func TestRenderTaskTool(t *testing.T) {
	out := Render(false)
	if !strings.Contains(out, "Task tool") {
		t.Error("should reference Copilot CLI's Task tool for delegation")
	}
}

func TestRenderHighRiskItems(t *testing.T) {
	out := Render(false)
	for _, want := range []string{
		"(1) data or irreversible effects",
		"(2) security",
		"(3) contracts others consume",
		"(4) concurrency",
		"(5) delivery or environment",
		"(6) no test would catch",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("high-risk item missing %q", want)
		}
	}
}

func TestMarkers(t *testing.T) {
	if MarkerStart == MarkerEnd {
		t.Fatal("markers must differ")
	}
	if !strings.HasPrefix(MarkerStart, "<!--") || !strings.HasSuffix(MarkerStart, "-->") {
		t.Error("start marker should be an HTML comment")
	}
	if !strings.HasPrefix(MarkerEnd, "<!--") || !strings.HasSuffix(MarkerEnd, "-->") {
		t.Error("end marker should be an HTML comment")
	}
	if !strings.Contains(MarkerStart, "odd") {
		t.Error("start marker should contain 'odd'")
	}
}

func assertContains(t *testing.T, out string, wants ...string) {
	t.Helper()
	for _, want := range wants {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q", want)
		}
	}
}

func assertNotContains(t *testing.T, out string, nots ...string) {
	t.Helper()
	for _, not := range nots {
		if strings.Contains(out, not) {
			t.Errorf("unexpected %q", not)
		}
	}
}

func TestRenderCoreContainsProtocol(t *testing.T) {
	assertContains(t, RenderCore(false),
		"## Implementation Routing", "### ODD protocol (MANDATORY",
		"1. **Authorize.**", "7. **Close.**")
}

func TestRenderCoreContainsTaskSize(t *testing.T) {
	assertContains(t, RenderCore(false), "### Task Size", "**High risk**", "**Risk line**")
}

func TestRenderCoreNoRoutes(t *testing.T) {
	assertNotContains(t, RenderCore(true), "**Direct inline:**", "### Mandatory Delegation Triggers")
}

func TestRenderCoreNoODDDetail(t *testing.T) {
	assertNotContains(t, RenderCore(true), "### Organic Driven Development")
}

func TestRenderCoreWithRDD(t *testing.T) {
	assertContains(t, RenderCore(true), "### Risk Tiers and Lens Selection")
}

func TestRenderCoreNoRDD(t *testing.T) {
	assertNotContains(t, RenderCore(false), "### Risk Tiers and Lens Selection")
}

func TestRenderModulesCount(t *testing.T) {
	for _, rdd := range []bool{false, true} {
		if got := len(RenderModules(rdd)); got != 2 {
			t.Errorf("rdd=%v: got %d modules, want 2", rdd, got)
		}
	}
}

func TestRenderModulesFrontmatter(t *testing.T) {
	for _, m := range RenderModules(false) {
		if !strings.HasPrefix(m.Content, "---\napplyTo: \"**\"\n---\n\n") {
			t.Errorf("%s: missing applyTo frontmatter", m.Name)
		}
	}
}

func TestRenderModulesDelegation(t *testing.T) {
	mods := RenderModules(false)
	if mods[0].Name != "capiko-delegation" {
		t.Fatalf("first module = %q", mods[0].Name)
	}
	assertContains(t, mods[0].Content, "**Direct inline:**", "### Mandatory Delegation Triggers")
}

func TestRenderModulesODDDetail(t *testing.T) {
	mods := RenderModules(true)
	if mods[1].Name != "capiko-odd-detail" {
		t.Fatalf("second module = %q", mods[1].Name)
	}
	assertContains(t, mods[1].Content, "### Organic Driven Development", "Apply one default test-first policy")
}
