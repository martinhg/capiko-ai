package cgastore

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/martinhg/capiko-ai/internal/cga"
)

func TestDefaultStorePath(t *testing.T) {
	got := DefaultStorePath("/base", "acme/repo")
	want := filepath.Join("/base", "cga", "acme/repo", "learned-rules.json")
	if got != want {
		t.Errorf("DefaultStorePath = %q, want %q", got, want)
	}
}

func TestLoadLearnedRulesMissingFileReturnsEmpty(t *testing.T) {
	path := DefaultStorePath(t.TempDir(), "acme/repo")

	rules, err := LoadLearnedRules(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(rules) != 0 {
		t.Fatalf("want empty slice, got %v", rules)
	}
}

func TestSaveLoadLearnedRulesRoundTrip(t *testing.T) {
	path := DefaultStorePath(t.TempDir(), "acme/repo")
	want := []cga.LearnedRule{
		{ID: "abc123", Severity: cga.SeverityWarning, Text: "REQUIRE: missing test coverage", EvidenceCount: 4, ApprovedAt: "2026-08-18T10:00:00Z"},
		{ID: "def456", Severity: cga.SeverityCritical, Text: "REJECT if: errors swallowed silently", EvidenceCount: 3, ApprovedAt: "2026-08-17T10:00:00Z"},
	}

	if err := SaveLearnedRules(path, want); err != nil {
		t.Fatalf("save: %v", err)
	}

	got, err := LoadLearnedRules(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v, want %+v", got, want)
	}

	if _, err := os.Stat(path); err != nil {
		t.Fatalf("expected file at %s: %v", path, err)
	}
}

func TestLoadLearnedRulesInvalidJSONReturnsError(t *testing.T) {
	dir := t.TempDir()
	path := DefaultStorePath(dir, "acme/repo")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("not json"), 0o644); err != nil {
		t.Fatal(err)
	}

	if _, err := LoadLearnedRules(path); err == nil {
		t.Fatal("expected error for invalid JSON, got nil")
	}
}
