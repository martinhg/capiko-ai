package cga

import "testing"

func TestComposeRules(t *testing.T) {
	tests := []struct {
		name    string
		static  string
		learned []LearnedRule
		want    string
	}{
		{
			name:    "empty learned rules returns static unchanged",
			static:  "# Code Review Rules\n\n- REJECT if: bad thing\n",
			learned: nil,
			want:    "# Code Review Rules\n\n- REJECT if: bad thing\n",
		},
		{
			name:    "nil static with empty learned stays empty",
			static:  "",
			learned: nil,
			want:    "",
		},
		{
			name:   "single learned rule appends Learned Rules section",
			static: "# Code Review Rules\n\n- REJECT if: bad thing\n",
			learned: []LearnedRule{
				{Severity: SeverityWarning, Text: "REQUIRE: missing test coverage"},
			},
			want: "# Code Review Rules\n\n- REJECT if: bad thing\n\n## Learned Rules\n\n- REQUIRE: missing test coverage\n",
		},
		{
			name:   "multiple learned rules render one line each in order",
			static: "static rules\n",
			learned: []LearnedRule{
				{Severity: SeverityCritical, Text: "REJECT if: errors swallowed silently"},
				{Severity: SeverityWarning, Text: "REQUIRE: missing test coverage"},
				{Severity: SeveritySuggestion, Text: "PREFER: consider a comment"},
			},
			want: "static rules\n\n## Learned Rules\n\n" +
				"- REJECT if: errors swallowed silently\n" +
				"- REQUIRE: missing test coverage\n" +
				"- PREFER: consider a comment\n",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ComposeRules(tt.static, tt.learned)
			if got != tt.want {
				t.Errorf("ComposeRules() = %q, want %q", got, tt.want)
			}
		})
	}
}
