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

func ruleIDs(rules []LearnedRule) map[string]bool {
	ids := make(map[string]bool, len(rules))
	for _, r := range rules {
		ids[r.ID] = true
	}
	return ids
}

func TestEnforceCap(t *testing.T) {
	atCap := []LearnedRule{
		{ID: "a", EvidenceCount: 5, ApprovedAt: "2026-08-01T00:00:00Z"},
		{ID: "b", EvidenceCount: 4, ApprovedAt: "2026-08-02T00:00:00Z"},
		{ID: "c", EvidenceCount: 3, ApprovedAt: "2026-08-03T00:00:00Z"},
	}

	tests := []struct {
		name    string
		rules   []LearnedRule
		cap     int
		want    int      // expected output length
		keepIDs []string // must be present in output
		dropIDs []string // must be absent from output
	}{
		{
			name:  "under cap is a no-op",
			rules: atCap,
			cap:   25,
			want:  3,
		},
		{
			name:  "exactly at cap is a no-op",
			rules: atCap,
			cap:   3,
			want:  3,
		},
		{
			name: "over cap trims lowest evidence count first",
			rules: []LearnedRule{
				{ID: "high", EvidenceCount: 10, ApprovedAt: "2026-08-01T00:00:00Z"},
				{ID: "mid", EvidenceCount: 5, ApprovedAt: "2026-08-01T00:00:00Z"},
				{ID: "low", EvidenceCount: 3, ApprovedAt: "2026-08-01T00:00:00Z"},
			},
			cap:     2,
			want:    2,
			keepIDs: []string{"high", "mid"},
			dropIDs: []string{"low"},
		},
		{
			name: "tie on evidence count trims oldest ApprovedAt first",
			rules: []LearnedRule{
				{ID: "newer", EvidenceCount: 4, ApprovedAt: "2026-08-10T00:00:00Z"},
				{ID: "older", EvidenceCount: 4, ApprovedAt: "2026-08-01T00:00:00Z"},
				{ID: "highest", EvidenceCount: 9, ApprovedAt: "2026-08-05T00:00:00Z"},
			},
			cap:     2,
			want:    2,
			keepIDs: []string{"highest", "newer"},
			dropIDs: []string{"older"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := EnforceCap(tt.rules, tt.cap)
			if len(got) != tt.want {
				t.Fatalf("EnforceCap() len = %d, want %d", len(got), tt.want)
			}
			ids := ruleIDs(got)
			for _, id := range tt.keepIDs {
				if !ids[id] {
					t.Errorf("EnforceCap() dropped %q, expected it kept; result=%+v", id, got)
				}
			}
			for _, id := range tt.dropIDs {
				if ids[id] {
					t.Errorf("EnforceCap() kept %q, expected it dropped; result=%+v", id, got)
				}
			}
		})
	}
}

func TestCheckRetirement(t *testing.T) {
	stillStrong := LearnedRule{ID: "strong", Severity: SeverityWarning, Text: "REQUIRE: missing test coverage"}
	droppedBelow := LearnedRule{ID: "dropped", Severity: SeverityCritical, Text: "REJECT if: errors swallowed silently"}
	noEvidence := LearnedRule{ID: "gone", Severity: SeveritySuggestion, Text: "PREFER: consider a comment"}

	tests := []struct {
		name        string
		rules       []LearnedRule
		patterns    []Pattern
		threshold   int
		wantActive  []string
		wantRetired []string
	}{
		{
			name:  "evidence still at or above threshold stays active",
			rules: []LearnedRule{stillStrong},
			patterns: []Pattern{
				{Severity: SeverityWarning, Description: "missing test coverage", EvidenceCount: 5},
			},
			threshold:  3,
			wantActive: []string{"strong"},
		},
		{
			name:  "evidence dropped below threshold is retired",
			rules: []LearnedRule{droppedBelow},
			patterns: []Pattern{
				{Severity: SeverityCritical, Description: "errors swallowed silently", EvidenceCount: 2},
			},
			threshold:   3,
			wantRetired: []string{"dropped"},
		},
		{
			name:        "no matching pattern at all is retired",
			rules:       []LearnedRule{noEvidence},
			patterns:    nil,
			threshold:   3,
			wantRetired: []string{"gone"},
		},
		{
			name:  "mixed set partitions correctly",
			rules: []LearnedRule{stillStrong, droppedBelow, noEvidence},
			patterns: []Pattern{
				{Severity: SeverityWarning, Description: "missing test coverage", EvidenceCount: 5},
				{Severity: SeverityCritical, Description: "errors swallowed silently", EvidenceCount: 2},
			},
			threshold:   3,
			wantActive:  []string{"strong"},
			wantRetired: []string{"dropped", "gone"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			active, retired := CheckRetirement(tt.rules, tt.patterns, tt.threshold)
			if got := ruleIDs(active); len(got) != len(tt.wantActive) {
				t.Errorf("active = %+v, want ids %v", active, tt.wantActive)
			} else {
				for _, id := range tt.wantActive {
					if !got[id] {
						t.Errorf("active missing %q, got %+v", id, active)
					}
				}
			}
			if got := ruleIDs(retired); len(got) != len(tt.wantRetired) {
				t.Errorf("retired = %+v, want ids %v", retired, tt.wantRetired)
			} else {
				for _, id := range tt.wantRetired {
					if !got[id] {
						t.Errorf("retired missing %q, got %+v", id, retired)
					}
				}
			}
		})
	}
}
