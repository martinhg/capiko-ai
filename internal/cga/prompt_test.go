package cga

import (
	"strings"
	"testing"
)

func TestRulesUsesActionKeywords(t *testing.T) {
	body := Rules("capiko")
	for _, kw := range []string{"REJECT", "REQUIRE", "PREFER"} {
		if !strings.Contains(body, kw) {
			t.Errorf("rules should use the %q action keyword the reviewer understands:\n%s", kw, body)
		}
	}
	for _, section := range []string{"Architecture", "Naming", "Testing"} {
		if !strings.Contains(body, section) {
			t.Errorf("rules should cover the %q section:\n%s", section, body)
		}
	}
}

func TestRulesPersonaPointer(t *testing.T) {
	tests := []struct {
		name        string
		persona     string
		wantContain string
		wantOmit    string
	}{
		{name: "empty persona omits pointer", persona: "", wantOmit: "Persona"},
		{name: "set persona is referenced", persona: "capiko", wantContain: "capiko"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body := Rules(tt.persona)
			if tt.wantContain != "" && !strings.Contains(body, tt.wantContain) {
				t.Errorf("rules should reference persona %q:\n%s", tt.wantContain, body)
			}
			if tt.wantOmit != "" && strings.Contains(body, tt.wantOmit) {
				t.Errorf("rules should omit %q when no persona is set:\n%s", tt.wantOmit, body)
			}
		})
	}
}

func TestBuildPrompt(t *testing.T) {
	tests := []struct {
		name   string
		rules  string
		diff   string
		wantOK bool
	}{
		{name: "non-empty diff", rules: "RULES-TEXT", diff: "diff --git a/x b/x\n+added line", wantOK: true},
		{name: "empty diff", rules: "RULES-TEXT", diff: "", wantOK: false},
		{name: "whitespace-only diff", rules: "RULES-TEXT", diff: "   \n\t", wantOK: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := BuildPrompt(tt.rules, tt.diff)
			if ok != tt.wantOK {
				t.Fatalf("BuildPrompt() ok = %v, want %v", ok, tt.wantOK)
			}
			if !tt.wantOK {
				if got != "" {
					t.Errorf("BuildPrompt() with nothing to review should return empty string, got %q", got)
				}
				return
			}
			if !strings.Contains(got, tt.rules) {
				t.Errorf("prompt missing rules content:\n%s", got)
			}
			if !strings.Contains(got, tt.diff) {
				t.Errorf("prompt missing diff content:\n%s", got)
			}
			if !strings.Contains(got, "PASS") || !strings.Contains(got, "FAIL") {
				t.Errorf("prompt should instruct a machine-parsable PASS/FAIL verdict:\n%s", got)
			}
			if !strings.Contains(got, "verdict") {
				t.Errorf("prompt should reference the verdict field:\n%s", got)
			}
			for _, kw := range []string{"findings", "severity", "CRITICAL", "WARNING", "SUGGESTION"} {
				if !strings.Contains(got, kw) {
					t.Errorf("prompt should instruct the optional findings schema, missing %q:\n%s", kw, got)
				}
			}
		})
	}
}
