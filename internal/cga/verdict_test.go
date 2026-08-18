package cga

import "testing"

func TestParseVerdict(t *testing.T) {
	tests := []struct {
		name       string
		input      string
		wantResult VerdictResult
		wantReason string
	}{
		{
			name:       "well-formed pass",
			input:      `{"verdict":"PASS"}`,
			wantResult: VerdictPass,
		},
		{
			name:       "well-formed fail with reason",
			input:      `{"verdict":"FAIL","reason":"missing tests for the new branch"}`,
			wantResult: VerdictFail,
			wantReason: "missing tests for the new branch",
		},
		{
			name:       "fail without reason",
			input:      `{"verdict":"FAIL"}`,
			wantResult: VerdictFail,
		},
		{
			name:       "malformed json",
			input:      `{"verdict": "PASS"`,
			wantResult: VerdictAmbiguous,
		},
		{
			name:       "missing verdict field",
			input:      `{"reason":"no verdict field at all"}`,
			wantResult: VerdictAmbiguous,
		},
		{
			name:       "empty input",
			input:      "",
			wantResult: VerdictAmbiguous,
		},
		{
			name:       "unrecognized verdict value",
			input:      `{"verdict":"MAYBE"}`,
			wantResult: VerdictAmbiguous,
		},
		{
			name:       "verdict json embedded in surrounding prose",
			input:      "Here is my review.\n\n{\"verdict\":\"PASS\"}\n",
			wantResult: VerdictPass,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ParseVerdict(tt.input)
			if got.Result != tt.wantResult {
				t.Errorf("ParseVerdict(%q).Result = %v, want %v", tt.input, got.Result, tt.wantResult)
			}
			if got.Reason != tt.wantReason {
				t.Errorf("ParseVerdict(%q).Reason = %q, want %q", tt.input, got.Reason, tt.wantReason)
			}
		})
	}
}
