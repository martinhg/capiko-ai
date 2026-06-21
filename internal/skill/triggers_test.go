package skill

import (
	"strings"
	"testing"
)

func TestHasTrigger(t *testing.T) {
	cases := []struct {
		desc string
		want bool
	}{
		{"Review CSS. Trigger: editing or reviewing CSS/SCSS.", true},
		{"description with TRIGGER: case-insensitive match", true},
		{"no trigger clause at all", false},
		{"dangling Trigger:", false},
		{"Trigger:   ", false},
	}
	for _, c := range cases {
		if got := HasTrigger(c.desc); got != c.want {
			t.Errorf("HasTrigger(%q) = %v, want %v", c.desc, got, c.want)
		}
	}
}

func TestValidateTriggersExemptsSharedSkills(t *testing.T) {
	catalog := []Skill{
		{Name: "sdd-apply", Description: "Implement tasks. Trigger: when applying.", DependsOn: []string{"sdd-shared"}},
		{Name: "sdd-shared", Description: "Shared SDD phase content."}, // no Trigger, but depended upon
	}
	if err := ValidateTriggers(catalog); err != nil {
		t.Errorf("shared (depended-upon) skill should be exempt from the Trigger rule: %v", err)
	}
}

func TestValidateTriggersFlagsMissing(t *testing.T) {
	catalog := []Skill{
		{Name: "good", Description: "Does a thing. Trigger: when you want a thing."},
		{Name: "weak", Description: "Does a thing but never says when."},
	}
	err := ValidateTriggers(catalog)
	if err == nil {
		t.Fatal("a non-shared skill without a Trigger clause should fail validation")
	}
	if !strings.Contains(err.Error(), "weak") {
		t.Errorf("error should name the offending skill, got: %v", err)
	}
	if strings.Contains(err.Error(), "good") {
		t.Errorf("error should not flag a skill that has a Trigger, got: %v", err)
	}
}
