package skill

import (
	"fmt"
	"sort"
	"strings"
)

// HasTrigger reports whether a skill description declares a load trigger — the
// "Trigger:" clause Copilot uses to decide when to surface the skill. A skill
// without a specific trigger never loads on its own, so this is capiko's minimum
// quality bar for a directly-triggered skill (matching the Gentleman-Skills
// SKILL_TEMPLATE standard).
func HasTrigger(description string) bool {
	i := strings.Index(strings.ToLower(description), "trigger:")
	if i < 0 {
		return false
	}
	rest := description[i+len("trigger:"):]
	return strings.TrimSpace(rest) != ""
}

// ValidateTriggers reports skills whose description lacks a Trigger clause. Shared
// library skills — those depended upon by another skill via depends_on — are
// exempt: they load by dependency, not by trigger, so they need no Trigger (e.g.
// sdd-shared). A nil return means every directly-triggered skill declares one.
func ValidateTriggers(catalog []Skill) error {
	depended := make(map[string]bool)
	for _, s := range catalog {
		for _, dep := range s.DependsOn {
			depended[dep] = true
		}
	}

	var missing []string
	for _, s := range catalog {
		if depended[s.Name] {
			continue // shared skill, loaded via dependency
		}
		if !HasTrigger(s.Description) {
			missing = append(missing, s.Name)
		}
	}
	if len(missing) > 0 {
		sort.Strings(missing)
		return fmt.Errorf("skills missing a Trigger clause in their description: %s", strings.Join(missing, ", "))
	}
	return nil
}
