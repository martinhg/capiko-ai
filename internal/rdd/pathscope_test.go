package rdd

import (
	"encoding/json"
	"reflect"
	"testing"
)

func TestPathScopeCheck(t *testing.T) {
	tests := []struct {
		name            string
		correctionPaths []string
		findingPaths    []string
		want            bool
	}{
		{
			name:            "empty correction paths is always in scope",
			correctionPaths: nil,
			findingPaths:    []string{"a.go"},
			want:            true,
		},
		{
			name:            "empty correction paths in scope even with no findings",
			correctionPaths: []string{},
			findingPaths:    nil,
			want:            true,
		},
		{
			name:            "single path subset of findings",
			correctionPaths: []string{"a.go"},
			findingPaths:    []string{"a.go", "b.go"},
			want:            true,
		},
		{
			name:            "all correction paths present in findings",
			correctionPaths: []string{"a.go", "b.go"},
			findingPaths:    []string{"a.go", "b.go", "c.go"},
			want:            true,
		},
		{
			name:            "exact match single path",
			correctionPaths: []string{"a.go"},
			findingPaths:    []string{"a.go"},
			want:            true,
		},
		{
			name:            "out-of-scope path rejected",
			correctionPaths: []string{"c.go"},
			findingPaths:    []string{"a.go"},
			want:            false,
		},
		{
			name:            "partial overlap rejected — one out-of-scope path is enough to fail",
			correctionPaths: []string{"a.go", "c.go"},
			findingPaths:    []string{"a.go", "b.go"},
			want:            false,
		},
		{
			name:            "correction paths present but findings empty rejected",
			correctionPaths: []string{"a.go"},
			findingPaths:    nil,
			want:            false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := PathScopeCheck(tt.correctionPaths, tt.findingPaths); got != tt.want {
				t.Errorf("PathScopeCheck(%v, %v) = %v, want %v", tt.correctionPaths, tt.findingPaths, got, tt.want)
			}
		})
	}
}

func TestExtractFindingPaths(t *testing.T) {
	tests := []struct {
		name     string
		findings json.RawMessage
		want     []string
	}{
		{
			name:     "single issue with path",
			findings: json.RawMessage(`{"issues":[{"path":"a.go"}]}`),
			want:     []string{"a.go"},
		},
		{
			name:     "multiple issues, multiple paths",
			findings: json.RawMessage(`{"issues":[{"path":"a.go"},{"path":"b.go"}]}`),
			want:     []string{"a.go", "b.go"},
		},
		{
			name:     "duplicate paths deduplicated",
			findings: json.RawMessage(`{"issues":[{"path":"a.go"},{"path":"a.go"},{"path":"b.go"}]}`),
			want:     []string{"a.go", "b.go"},
		},
		{
			name:     "issue without path field is skipped",
			findings: json.RawMessage(`{"issues":[{"path":"a.go"},{"severity":"high"}]}`),
			want:     []string{"a.go"},
		},
		{
			name:     "empty path field is skipped",
			findings: json.RawMessage(`{"issues":[{"path":""},{"path":"a.go"}]}`),
			want:     []string{"a.go"},
		},
		{
			name:     "empty issues array returns nil",
			findings: json.RawMessage(`{"issues":[]}`),
			want:     nil,
		},
		{
			name:     "missing issues key returns nil",
			findings: json.RawMessage(`{}`),
			want:     nil,
		},
		{
			name:     "malformed JSON returns nil",
			findings: json.RawMessage(`{not valid json`),
			want:     nil,
		},
		{
			name:     "nil findings returns nil",
			findings: nil,
			want:     nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ExtractFindingPaths(tt.findings)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("ExtractFindingPaths(%s) = %v, want %v", tt.findings, got, tt.want)
			}
		})
	}
}
