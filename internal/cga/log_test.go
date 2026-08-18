package cga

import (
	"reflect"
	"strings"
	"testing"
)

func TestParseLog(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    []LogEntry
		wantErr bool
	}{
		{
			name: "valid jsonl round-trips in order",
			input: `{"timestamp":"2026-08-18T14:30:00Z","parent_sha":"abc1234","commit_sha":"pending","verdict":"FAIL","reason":"missing tests","findings":[{"file":"a.go","line":10,"severity":"CRITICAL","description":"var named x"}]}
{"timestamp":"2026-08-18T14:31:00Z","parent_sha":"def5678","commit_sha":"def5678","verdict":"PASS"}
`,
			want: []LogEntry{
				{
					Timestamp: "2026-08-18T14:30:00Z",
					ParentSHA: "abc1234",
					CommitSHA: "pending",
					Verdict:   "FAIL",
					Reason:    "missing tests",
					Findings: []Finding{
						{File: "a.go", Line: 10, Severity: SeverityCritical, Description: "var named x"},
					},
				},
				{
					Timestamp: "2026-08-18T14:31:00Z",
					ParentSHA: "def5678",
					CommitSHA: "def5678",
					Verdict:   "PASS",
				},
			},
		},
		{
			name: "malformed line skipped, other entries still returned",
			input: `{"timestamp":"2026-08-18T14:30:00Z","parent_sha":"abc1234","commit_sha":"pending","verdict":"PASS"}
not valid json at all
{"timestamp":"2026-08-18T14:31:00Z","parent_sha":"def5678","commit_sha":"def5678","verdict":"FAIL"}
`,
			want: []LogEntry{
				{Timestamp: "2026-08-18T14:30:00Z", ParentSHA: "abc1234", CommitSHA: "pending", Verdict: "PASS"},
				{Timestamp: "2026-08-18T14:31:00Z", ParentSHA: "def5678", CommitSHA: "def5678", Verdict: "FAIL"},
			},
		},
		{
			name:  "empty input returns empty slice, no error",
			input: "",
			want:  nil,
		},
		{
			name:  "blank lines are skipped without error",
			input: "\n\n   \n",
			want:  nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseLog(strings.NewReader(tt.input))
			if (err != nil) != tt.wantErr {
				t.Fatalf("ParseLog() error = %v, wantErr %v", err, tt.wantErr)
			}
			if len(got) != len(tt.want) {
				t.Fatalf("ParseLog() returned %d entries, want %d: %+v", len(got), len(tt.want), got)
			}
			for i := range got {
				if !reflect.DeepEqual(got[i], tt.want[i]) {
					t.Errorf("entry[%d] = %+v, want %+v", i, got[i], tt.want[i])
				}
			}
		})
	}
}
