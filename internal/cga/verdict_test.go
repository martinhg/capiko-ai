package cga

import (
	"reflect"
	"testing"
)

func TestParseVerdict(t *testing.T) {
	tests := []struct {
		name         string
		input        string
		wantResult   VerdictResult
		wantReason   string
		wantFindings []Finding
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
		{
			name:       "well-formed findings array",
			input:      `{"verdict":"FAIL","reason":"bad naming","findings":[{"file":"a.go","line":10,"severity":"CRITICAL","description":"var named x"},{"file":"b.go","severity":"SUGGESTION","description":"consider a comment"}]}`,
			wantResult: VerdictFail,
			wantReason: "bad naming",
			wantFindings: []Finding{
				{File: "a.go", Line: 10, Severity: SeverityCritical, Description: "var named x"},
				{File: "b.go", Line: 0, Severity: SeveritySuggestion, Description: "consider a comment"},
			},
		},
		{
			name:         "findings field absent",
			input:        `{"verdict":"PASS"}`,
			wantResult:   VerdictPass,
			wantFindings: nil,
		},
		{
			name:         "findings field malformed (not an array)",
			input:        `{"verdict":"PASS","findings":"not-an-array"}`,
			wantResult:   VerdictPass,
			wantFindings: nil,
		},
		{
			name:       "findings entry with line omitted",
			input:      `{"verdict":"PASS","findings":[{"file":"c.go","severity":"WARNING","description":"missing test"}]}`,
			wantResult: VerdictPass,
			wantFindings: []Finding{
				{File: "c.go", Line: 0, Severity: SeverityWarning, Description: "missing test"},
			},
		},
		{
			name:       "self-contradictory PASS with CRITICAL finding",
			input:      `{"verdict":"PASS","findings":[{"file":"d.go","severity":"CRITICAL","description":"should have failed"}]}`,
			wantResult: VerdictPass,
			wantFindings: []Finding{
				{File: "d.go", Line: 0, Severity: SeverityCritical, Description: "should have failed"},
			},
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
			if !reflect.DeepEqual(got.Findings, tt.wantFindings) {
				t.Errorf("ParseVerdict(%q).Findings = %+v, want %+v", tt.input, got.Findings, tt.wantFindings)
			}
		})
	}
}

func TestFormatFindings(t *testing.T) {
	tests := []struct {
		name     string
		findings []Finding
		want     string
	}{
		{
			name:     "empty",
			findings: nil,
			want:     "",
		},
		{
			name: "single with line",
			findings: []Finding{
				{File: "a.go", Line: 10, Severity: SeverityCritical, Description: "var named x"},
			},
			want: "[CRITICAL] a.go:10 - var named x",
		},
		{
			name: "single without line",
			findings: []Finding{
				{File: "b.go", Line: 0, Severity: SeveritySuggestion, Description: "consider a comment"},
			},
			want: "[SUGGESTION] b.go - consider a comment",
		},
		{
			name: "multiple mixed",
			findings: []Finding{
				{File: "a.go", Line: 10, Severity: SeverityCritical, Description: "var named x"},
				{File: "b.go", Line: 0, Severity: SeveritySuggestion, Description: "consider a comment"},
				{File: "c.go", Line: 5, Severity: SeverityWarning, Description: "missing test"},
			},
			want: "[CRITICAL] a.go:10 - var named x\n[SUGGESTION] b.go - consider a comment\n[WARNING] c.go:5 - missing test",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FormatFindings(tt.findings)
			if got != tt.want {
				t.Errorf("FormatFindings(%+v) = %q, want %q", tt.findings, got, tt.want)
			}
		})
	}
}
