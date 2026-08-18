package cga

import (
	"reflect"
	"testing"
)

func TestNormalizeDescription(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "mixed case is lowercased",
			input: "Missing Tests For The New Branch",
			want:  "missing tests for the new branch",
		},
		{
			name:  "extra internal whitespace collapses",
			input: "var   named    x   incorrectly",
			want:  "var named x incorrectly",
		},
		{
			name:  "leading and trailing whitespace trimmed",
			input: "   consider a comment   ",
			want:  "consider a comment",
		},
		{
			name:  "trailing punctuation stripped",
			input: "missing test for the new branch.",
			want:  "missing test for the new branch",
		},
		{
			name:  "multiple trailing punctuation stripped",
			input: "is this really ok?!",
			want:  "is this really ok",
		},
		{
			name:  "combined case whitespace and punctuation",
			input: "  Var Named X  Incorrectly.  ",
			want:  "var named x incorrectly",
		},
		{
			name:  "empty string stays empty",
			input: "",
			want:  "",
		},
		{
			name:  "whitespace only becomes empty",
			input: "   \t  ",
			want:  "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := NormalizeDescription(tt.input)
			if got != tt.want {
				t.Errorf("NormalizeDescription(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

// entryWithFinding is a small helper to keep the DetectPatterns table
// declarations focused on evidence shape rather than struct literal noise.
func entryWithFinding(timestamp string, findings ...Finding) LogEntry {
	return LogEntry{Timestamp: timestamp, Verdict: "FAIL", Findings: findings}
}

func TestDetectPatterns(t *testing.T) {
	tests := []struct {
		name      string
		entries   []LogEntry
		threshold int
		want      []Pattern
	}{
		{
			name:      "empty log returns nil",
			entries:   nil,
			threshold: 3,
			want:      nil,
		},
		{
			name: "below threshold returns nil",
			entries: []LogEntry{
				entryWithFinding("2026-08-16T10:00:00Z", Finding{File: "a.go", Severity: SeverityWarning, Description: "missing test coverage"}),
				entryWithFinding("2026-08-17T10:00:00Z", Finding{File: "b.go", Severity: SeverityWarning, Description: "Missing Test Coverage"}),
			},
			threshold: 3,
			want:      nil,
		},
		{
			name: "exactly threshold returns one candidate",
			entries: []LogEntry{
				entryWithFinding("2026-08-16T10:00:00Z", Finding{File: "a.go", Severity: SeverityWarning, Description: "missing test coverage"}),
				entryWithFinding("2026-08-17T10:00:00Z", Finding{File: "b.go", Severity: SeverityWarning, Description: "Missing Test Coverage."}),
				entryWithFinding("2026-08-18T10:00:00Z", Finding{File: "c.go", Severity: SeverityWarning, Description: "MISSING   test coverage"}),
			},
			threshold: 3,
			want: []Pattern{
				{
					Severity:      SeverityWarning,
					Description:   "missing test coverage",
					EvidenceCount: 3,
					Files:         []string{"a.go", "b.go", "c.go"},
					FirstSeen:     "2026-08-16T10:00:00Z",
					LastSeen:      "2026-08-18T10:00:00Z",
				},
			},
		},
		{
			name: "multiple patterns sorted by evidence count descending",
			entries: []LogEntry{
				entryWithFinding("2026-08-10T10:00:00Z", Finding{File: "a.go", Severity: SeverityCritical, Description: "errors swallowed silently"}),
				entryWithFinding("2026-08-11T10:00:00Z", Finding{File: "b.go", Severity: SeverityCritical, Description: "errors swallowed silently"}),
				entryWithFinding("2026-08-12T10:00:00Z", Finding{File: "c.go", Severity: SeverityCritical, Description: "errors swallowed silently"}),
				entryWithFinding("2026-08-13T10:00:00Z", Finding{File: "d.go", Severity: SeverityWarning, Description: "var named x"}),
				entryWithFinding("2026-08-14T10:00:00Z", Finding{File: "e.go", Severity: SeverityWarning, Description: "var named x"}),
				entryWithFinding("2026-08-15T10:00:00Z", Finding{File: "f.go", Severity: SeverityWarning, Description: "var named x"}),
				entryWithFinding("2026-08-16T10:00:00Z", Finding{File: "g.go", Severity: SeverityWarning, Description: "var named x"}),
				entryWithFinding("2026-08-17T10:00:00Z", Finding{File: "h.go", Severity: SeverityWarning, Description: "var named x"}),
			},
			threshold: 3,
			want: []Pattern{
				{
					Severity:      SeverityWarning,
					Description:   "var named x",
					EvidenceCount: 5,
					Files:         []string{"d.go", "e.go", "f.go", "g.go", "h.go"},
					FirstSeen:     "2026-08-13T10:00:00Z",
					LastSeen:      "2026-08-17T10:00:00Z",
				},
				{
					Severity:      SeverityCritical,
					Description:   "errors swallowed silently",
					EvidenceCount: 3,
					Files:         []string{"a.go", "b.go", "c.go"},
					FirstSeen:     "2026-08-10T10:00:00Z",
					LastSeen:      "2026-08-12T10:00:00Z",
				},
			},
		},
		{
			name: "duplicate files across matching evidence are deduplicated",
			entries: []LogEntry{
				entryWithFinding("2026-08-16T10:00:00Z", Finding{File: "a.go", Severity: SeverityWarning, Description: "missing test coverage"}),
				entryWithFinding("2026-08-17T10:00:00Z", Finding{File: "a.go", Severity: SeverityWarning, Description: "missing test coverage"}),
				entryWithFinding("2026-08-18T10:00:00Z", Finding{File: "a.go", Severity: SeverityWarning, Description: "missing test coverage"}),
			},
			threshold: 3,
			want: []Pattern{
				{
					Severity:      SeverityWarning,
					Description:   "missing test coverage",
					EvidenceCount: 3,
					Files:         []string{"a.go"},
					FirstSeen:     "2026-08-16T10:00:00Z",
					LastSeen:      "2026-08-18T10:00:00Z",
				},
			},
		},
		{
			name: "same description different severity produces separate patterns",
			entries: []LogEntry{
				entryWithFinding("2026-08-16T10:00:00Z", Finding{File: "a.go", Severity: SeverityCritical, Description: "var named x"}),
				entryWithFinding("2026-08-17T10:00:00Z", Finding{File: "b.go", Severity: SeverityCritical, Description: "var named x"}),
				entryWithFinding("2026-08-18T10:00:00Z", Finding{File: "c.go", Severity: SeverityCritical, Description: "var named x"}),
				entryWithFinding("2026-08-19T10:00:00Z", Finding{File: "d.go", Severity: SeverityWarning, Description: "var named x"}),
				entryWithFinding("2026-08-20T10:00:00Z", Finding{File: "e.go", Severity: SeverityWarning, Description: "var named x"}),
			},
			threshold: 3,
			want: []Pattern{
				{
					Severity:      SeverityCritical,
					Description:   "var named x",
					EvidenceCount: 3,
					Files:         []string{"a.go", "b.go", "c.go"},
					FirstSeen:     "2026-08-16T10:00:00Z",
					LastSeen:      "2026-08-18T10:00:00Z",
				},
			},
		},
		{
			name: "entries without findings are ignored",
			entries: []LogEntry{
				{Timestamp: "2026-08-16T10:00:00Z", Verdict: "PASS"},
				entryWithFinding("2026-08-17T10:00:00Z", Finding{File: "a.go", Severity: SeverityWarning, Description: "missing test coverage"}),
				{Timestamp: "2026-08-18T10:00:00Z", Verdict: "PASS"},
			},
			threshold: 3,
			want:      nil,
		},
		{
			name: "multiple findings in the same entry count once toward evidence",
			entries: []LogEntry{
				entryWithFinding("2026-08-16T10:00:00Z",
					Finding{File: "a.go", Severity: SeverityWarning, Description: "missing test coverage"},
					Finding{File: "b.go", Severity: SeverityWarning, Description: "missing test coverage"},
				),
				entryWithFinding("2026-08-17T10:00:00Z", Finding{File: "c.go", Severity: SeverityWarning, Description: "missing test coverage"}),
			},
			threshold: 3,
			want:      nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := DetectPatterns(tt.entries, tt.threshold)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("DetectPatterns() = %+v, want %+v", got, tt.want)
			}
		})
	}
}
