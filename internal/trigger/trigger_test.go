package trigger

import (
	"strings"
	"testing"
)

func TestValidEvent(t *testing.T) {
	for _, e := range Events {
		if !ValidEvent(e) {
			t.Errorf("ValidEvent(%q) = false, want true", e)
		}
	}
	if ValidEvent("bogus") {
		t.Error("ValidEvent(bogus) = true, want false")
	}
}

func TestValidMode(t *testing.T) {
	if !ValidMode(Advisory) {
		t.Error("ValidMode(advisory) = false")
	}
	if !ValidMode(Strong) {
		t.Error("ValidMode(strong) = false")
	}
	if ValidMode("block") {
		t.Error("ValidMode(block) = true, want false")
	}
}

func TestRuleValidate(t *testing.T) {
	good := Rule{Event: PreCommit, Skill: "review-readability", Mode: Advisory}
	if err := good.Validate(); err != nil {
		t.Errorf("valid rule: %v", err)
	}

	for name, r := range map[string]Rule{
		"bad event": {Event: "bogus", Skill: "x", Mode: Advisory},
		"bad mode":  {Event: PrePR, Skill: "x", Mode: "block"},
		"no skill":  {Event: PrePR, Skill: "", Mode: Advisory},
	} {
		if err := r.Validate(); err == nil {
			t.Errorf("%s: expected error", name)
		}
	}
}

func TestDefaultRulesValid(t *testing.T) {
	rules := DefaultRules()
	if len(rules) == 0 {
		t.Fatal("DefaultRules returned empty")
	}
	for i, r := range rules {
		if err := r.Validate(); err != nil {
			t.Errorf("rule[%d]: %v", i, err)
		}
	}
}

func TestDefaultRulesContent(t *testing.T) {
	rules := DefaultRules()

	has := func(event Event, skill string) bool {
		for _, r := range rules {
			if r.Event == event && r.Skill == skill {
				return true
			}
		}
		return false
	}

	if !has(PreCommit, "review-readability") {
		t.Error("missing pre-commit readability rule")
	}
	if !has(PrePush, "review-readability") {
		t.Error("missing pre-push readability rule")
	}
	if !has(PrePR, "review-risk") {
		t.Error("missing pre-pr risk rule")
	}
	if !has(PrePR, "review-readability") {
		t.Error("missing pre-pr readability rule")
	}
	if !has(PrePR, "review-reliability") {
		t.Error("missing pre-pr reliability rule")
	}
	if !has(PrePR, "review-resilience") {
		t.Error("missing pre-pr resilience rule")
	}

	// risk should be strong mode
	for _, r := range rules {
		if r.Event == PrePR && r.Skill == "review-risk" && r.Mode != Strong {
			t.Errorf("pre-pr review-risk mode = %q, want %q", r.Mode, Strong)
		}
	}
}

func TestRenderEmpty(t *testing.T) {
	if out := Render(nil); out != "" {
		t.Errorf("Render(nil) = %q, want empty", out)
	}
}

func TestRenderStructure(t *testing.T) {
	out := Render(DefaultRules())

	for _, want := range []string{
		"## Trigger Rules (capiko)",
		"### Modes",
		"advisory",
		"strong",
		"### Active rules",
		"| Event | Skill | Mode | Condition |",
		"### Event definitions",
		"pre-commit",
		"pre-push",
		"pre-pr",
		"post-sdd-phase",
		"### Behavior",
		"Never block the action",
		"strongest mode first",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("rendered block missing %q", want)
		}
	}
}

func TestRenderRuleRows(t *testing.T) {
	out := Render(DefaultRules())

	for _, want := range []string{
		"| pre-commit | review-readability | advisory |",
		"| pre-push | review-readability | advisory |",
		"| pre-pr | review-risk | strong |",
		"| pre-pr | review-readability | advisory |",
		"| pre-pr | review-reliability | advisory |",
		"| pre-pr | review-resilience | advisory |",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("rule row missing %q", want)
		}
	}
}

func TestRenderCondition(t *testing.T) {
	rules := []Rule{
		{Event: PrePR, Skill: "review-risk", Mode: Strong, When: "diff touches auth/"},
	}
	out := Render(rules)
	if !strings.Contains(out, "diff touches auth/") {
		t.Error("condition not rendered")
	}
}

func TestRenderNoConditionShowsDash(t *testing.T) {
	rules := []Rule{
		{Event: PreCommit, Skill: "review-readability", Mode: Advisory},
	}
	out := Render(rules)
	if !strings.Contains(out, "| — |") {
		t.Error("missing dash for empty condition")
	}
}
