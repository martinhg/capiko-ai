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

	type ruleKey struct {
		event Event
		skill string
	}
	lookup := make(map[ruleKey]Rule)
	for _, r := range rules {
		lookup[ruleKey{r.Event, r.Skill}] = r
	}

	has := func(event Event, skill string) bool {
		_, ok := lookup[ruleKey{event, skill}]
		return ok
	}

	// Risk-proportionate: no blanket pre-commit or pre-push readability.
	if has(PreCommit, "review-readability") {
		t.Error("pre-commit readability should not be a default rule (risk-proportionate)")
	}
	if has(PrePush, "review-readability") {
		t.Error("pre-push readability should not be a default rule (risk-proportionate)")
	}

	// Elevated+: R1 on push and PR.
	if !has(PrePush, "review-risk") {
		t.Error("missing pre-push risk rule")
	}
	if !has(PrePR, "review-risk") {
		t.Error("missing pre-pr risk rule")
	}

	// Critical: full 4R on PR.
	if !has(PrePR, "review-readability") {
		t.Error("missing pre-pr readability rule")
	}
	if !has(PrePR, "review-reliability") {
		t.Error("missing pre-pr reliability rule")
	}
	if !has(PrePR, "review-resilience") {
		t.Error("missing pre-pr resilience rule")
	}

	// Pre-PR risk should be strong; push risk should be advisory.
	if r := lookup[ruleKey{PrePR, "review-risk"}]; r.Mode != Strong {
		t.Errorf("pre-pr review-risk mode = %q, want %q", r.Mode, Strong)
	}
	if r := lookup[ruleKey{PrePush, "review-risk"}]; r.Mode != Advisory {
		t.Errorf("pre-push review-risk mode = %q, want %q", r.Mode, Advisory)
	}

	// All rules should have risk-tier conditions.
	for i, r := range rules {
		if r.When == "" {
			t.Errorf("rule[%d] (%s/%s): missing When condition — risk-proportionate rules must be conditional", i, r.Event, r.Skill)
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
		"| pre-push | review-risk | advisory | risk tier is elevated or higher |",
		"| pre-pr | review-risk | strong | risk tier is elevated or higher |",
		"| pre-pr | review-readability | advisory | risk tier is critical |",
		"| pre-pr | review-reliability | advisory | risk tier is critical |",
		"| pre-pr | review-resilience | advisory | risk tier is critical |",
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
