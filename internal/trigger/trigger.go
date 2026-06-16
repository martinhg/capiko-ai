// Package trigger defines declarative rules that teach the LLM when to suggest
// running review skills. Rules are rendered as instructional text injected into
// Copilot's instructions file — zero hooks, zero runtime dispatch.
package trigger

import (
	"fmt"
	"strings"
)

// Marker delimiters for the trigger-rules block inside the instructions file.
const (
	MarkerStart = "<!-- capiko:trigger:start -->"
	MarkerEnd   = "<!-- capiko:trigger:end -->"
)

// Event is a development lifecycle moment that can fire trigger rules.
type Event string

const (
	PreCommit    Event = "pre-commit"
	PrePush      Event = "pre-push"
	PrePR        Event = "pre-pr"
	PostSDDPhase Event = "post-sdd-phase"
)

// Events lists every valid event, in lifecycle order.
var Events = []Event{PreCommit, PrePush, PrePR, PostSDDPhase}

// ValidEvent reports whether e is a known event.
func ValidEvent(e Event) bool {
	for _, v := range Events {
		if v == e {
			return true
		}
	}
	return false
}

// Mode controls how strongly the LLM should recommend running the skill.
type Mode string

const (
	Advisory Mode = "advisory" // suggest; user decides
	Strong   Mode = "strong"   // insist once; still advisory, no hard gate
)

// ValidMode reports whether m is a known mode.
func ValidMode(m Mode) bool {
	return m == Advisory || m == Strong
}

// Rule binds a skill to a development event.
type Rule struct {
	Event Event  `json:"event"`
	Skill string `json:"skill"`
	Mode  Mode   `json:"mode"`
	When  string `json:"when,omitempty"` // optional condition, e.g. "diff touches auth/"
}

// Validate reports problems with the rule. A nil return means valid.
func (r Rule) Validate() error {
	if !ValidEvent(r.Event) {
		return fmt.Errorf("unknown event %q", r.Event)
	}
	if !ValidMode(r.Mode) {
		return fmt.Errorf("unknown mode %q", r.Mode)
	}
	if strings.TrimSpace(r.Skill) == "" {
		return fmt.Errorf("empty skill name")
	}
	return nil
}

// DefaultRules returns the recommended trigger rule set:
// readability on pre-commit/push, full 4R on pre-PR (risk as strong).
func DefaultRules() []Rule {
	return []Rule{
		{Event: PreCommit, Skill: "review-readability", Mode: Advisory},
		{Event: PrePush, Skill: "review-readability", Mode: Advisory},
		{Event: PrePR, Skill: "review-risk", Mode: Strong},
		{Event: PrePR, Skill: "review-readability", Mode: Advisory},
		{Event: PrePR, Skill: "review-reliability", Mode: Advisory},
		{Event: PrePR, Skill: "review-resilience", Mode: Advisory},
	}
}

// Render produces the instructional text block for the given rules. An empty
// slice yields an empty string (which removes the section via Inject).
func Render(rules []Rule) string {
	if len(rules) == 0 {
		return ""
	}

	var b strings.Builder

	b.WriteString("## Trigger Rules (capiko)\n\n")
	b.WriteString("Before certain development actions, check whether a review skill should run. These rules are declarative and advisory — you read them and self-trigger; there are no automated hooks.\n\n")

	b.WriteString("### Modes\n\n")
	b.WriteString("- **advisory** — suggest the review and explain why. If the user declines, proceed without blocking.\n")
	b.WriteString("- **strong** — recommend firmly and explain the risk of skipping. Still advisory — the user decides — but insist once before accepting a skip.\n\n")

	b.WriteString("### Active rules\n\n")
	b.WriteString("| Event | Skill | Mode | Condition |\n")
	b.WriteString("| --- | --- | --- | --- |\n")
	for _, r := range rules {
		cond := "—"
		if r.When != "" {
			cond = r.When
		}
		fmt.Fprintf(&b, "| %s | %s | %s | %s |\n", r.Event, r.Skill, r.Mode, cond)
	}

	b.WriteString("\n### Event definitions\n\n")
	b.WriteString("- **pre-commit** — fires when the user is about to commit (`git commit`, \"commit this\"). Suggest matching reviews before the commit.\n")
	b.WriteString("- **pre-push** — fires when the user is about to push. Suggest matching reviews before pushing.\n")
	b.WriteString("- **pre-pr** — fires when the user is about to create or finalize a PR. Suggest all matching reviews before creating the PR.\n")
	b.WriteString("- **post-sdd-phase** — fires after an SDD phase completes successfully. Suggest matching reviews on the phase output.\n\n")

	b.WriteString("### Behavior\n\n")
	b.WriteString("When a trigger fires:\n\n")
	b.WriteString("1. Look up the skill by name (run `capiko-ai skill-registry` or scan `~/.copilot/skills/`).\n")
	b.WriteString("2. If the skill is installed, suggest running it before the action.\n")
	b.WriteString("3. For **strong** rules, explain the risk of skipping and recommend firmly.\n")
	b.WriteString("4. For **advisory** rules, offer lightly: \"Would you like me to run [skill] first?\"\n")
	b.WriteString("5. If the user declines, proceed. Never block the action.\n")
	b.WriteString("6. When multiple rules match the same event, suggest all matching skills together, strongest mode first.\n")
	b.WriteString("7. If a rule has a condition, evaluate it against the current context (e.g. check whether the diff touches the named path) and skip the rule when the condition is not met.\n")

	return b.String()
}
