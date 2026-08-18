package tui

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/martinhg/capiko-ai/internal/cga"
	"github.com/martinhg/capiko-ai/internal/state"
)

// ---------------------------------------------------------------------------
// loadLearnedRules (CGA Phase 3 PR4 — learn-loop hook re-render wiring)
// ---------------------------------------------------------------------------

func TestLoadLearnedRulesFileMissingReturnsNilNoError(t *testing.T) {
	baseDir := t.TempDir()

	rules, err := loadLearnedRulesFile(baseDir, "acme/repo")
	if err != nil {
		t.Fatalf("loadLearnedRulesFile: %v", err)
	}
	if rules != nil {
		t.Errorf("expected nil rules for missing store, got %+v", rules)
	}
}

func TestLoadLearnedRulesFileReturnsParsedRules(t *testing.T) {
	baseDir := t.TempDir()
	project := "acme/repo"
	want := []cga.LearnedRule{
		{ID: "abc123", Severity: cga.SeverityWarning, Text: "REQUIRE: add regression test", EvidenceCount: 4, ApprovedAt: "2026-01-01T00:00:00Z"},
	}
	writeLearnedRulesFixture(t, baseDir, project, want)

	got, err := loadLearnedRulesFile(baseDir, project)
	if err != nil {
		t.Fatalf("loadLearnedRulesFile: %v", err)
	}
	if len(got) != 1 || got[0].Text != want[0].Text || got[0].EvidenceCount != want[0].EvidenceCount {
		t.Errorf("loadLearnedRulesFile = %+v, want %+v", got, want)
	}
}

// writeLearnedRulesFixture writes rules to the same local JSON store layout
// LoadLearnedRules/SaveLearnedRules use in cmd/capiko-ai ({baseDir}/cga/{project}/learned-rules.json).
func writeLearnedRulesFixture(t *testing.T, baseDir, project string, rules []cga.LearnedRule) {
	t.Helper()
	dir := filepath.Join(baseDir, "cga", project)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	data, err := json.MarshalIndent(rules, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "learned-rules.json"), data, 0o644); err != nil {
		t.Fatal(err)
	}
}

// ---------------------------------------------------------------------------
// applyCGA composition (4.2) + LearnedRulesChecksum (4.3)
// ---------------------------------------------------------------------------

// withStubLearnedRules swaps loadLearnedRules for the duration of the test.
func withStubLearnedRules(t *testing.T, rules []cga.LearnedRule, err error) {
	t.Helper()
	prev := loadLearnedRules
	loadLearnedRules = func(string) ([]cga.LearnedRule, error) { return rules, err }
	t.Cleanup(func() { loadLearnedRules = prev })
}

func TestApplyCGAComposesLearnedRulesWhenPresent(t *testing.T) {
	ws := t.TempDir()
	store := state.NewStore(t.TempDir())
	learned := []cga.LearnedRule{
		{Severity: cga.SeverityWarning, Text: "REQUIRE: add regression test before merging", EvidenceCount: 4},
	}
	withStubLearnedRules(t, learned, nil)

	if err := applyCGA(ws, store, nil, &state.CGARecord{Enabled: true}); err != nil {
		t.Fatalf("applyCGA: %v", err)
	}

	hook, err := os.ReadFile(filepath.Join(ws, ".git", "hooks", "pre-commit"))
	if err != nil {
		t.Fatalf("pre-commit hook not written: %v", err)
	}
	if !strings.Contains(string(hook), "Learned Rules") || !strings.Contains(string(hook), "REQUIRE: add regression test before merging") {
		t.Errorf("pre-commit hook should include the learned rules block:\n%s", hook)
	}
}

func TestApplyCGAUnchangedWhenNoLearnedRules(t *testing.T) {
	ws := t.TempDir()
	store := state.NewStore(t.TempDir())
	withStubLearnedRules(t, nil, nil)

	if err := applyCGA(ws, store, nil, &state.CGARecord{Enabled: true}); err != nil {
		t.Fatalf("applyCGA: %v", err)
	}

	hook, err := os.ReadFile(filepath.Join(ws, ".git", "hooks", "pre-commit"))
	if err != nil {
		t.Fatalf("pre-commit hook not written: %v", err)
	}
	if strings.Contains(string(hook), "Learned Rules") {
		t.Errorf("pre-commit hook should not include a learned rules block when none exist:\n%s", hook)
	}
}

func TestApplyCGAUpdatesLearnedRulesChecksum(t *testing.T) {
	ws := t.TempDir()
	store := state.NewStore(t.TempDir())
	learned := []cga.LearnedRule{
		{Severity: cga.SeverityWarning, Text: "REQUIRE: add regression test before merging", EvidenceCount: 4},
	}
	withStubLearnedRules(t, learned, nil)

	rec := &state.CGARecord{Enabled: true}
	if err := applyCGA(ws, store, nil, rec); err != nil {
		t.Fatalf("applyCGA: %v", err)
	}
	if rec.LearnedRulesChecksum == "" {
		t.Fatal("applyCGA should record a non-empty LearnedRulesChecksum when learned rules are present")
	}

	first := rec.LearnedRulesChecksum

	// Re-render with the same learned rules: checksum must stay stable.
	rec2 := &state.CGARecord{Enabled: true}
	if err := applyCGA(ws, store, nil, rec2); err != nil {
		t.Fatalf("applyCGA (re-render): %v", err)
	}
	if rec2.LearnedRulesChecksum != first {
		t.Errorf("LearnedRulesChecksum should be stable when learned rules are unchanged: got %q, want %q", rec2.LearnedRulesChecksum, first)
	}

	// Re-render with different learned rules: checksum must change.
	withStubLearnedRules(t, []cga.LearnedRule{
		{Severity: cga.SeverityCritical, Text: "REJECT if: secrets are committed in plaintext", EvidenceCount: 5},
	}, nil)
	rec3 := &state.CGARecord{Enabled: true}
	if err := applyCGA(ws, store, nil, rec3); err != nil {
		t.Fatalf("applyCGA (changed rules): %v", err)
	}
	if rec3.LearnedRulesChecksum == first {
		t.Error("LearnedRulesChecksum should change when learned rules change")
	}
}

func TestApplyCGAEmptyLearnedRulesChecksumWhenNoneExist(t *testing.T) {
	ws := t.TempDir()
	store := state.NewStore(t.TempDir())
	withStubLearnedRules(t, nil, nil)

	rec := &state.CGARecord{Enabled: true}
	if err := applyCGA(ws, store, nil, rec); err != nil {
		t.Fatalf("applyCGA: %v", err)
	}
	if rec.LearnedRulesChecksum != "" {
		t.Errorf("LearnedRulesChecksum should stay empty when no learned rules exist, got %q", rec.LearnedRulesChecksum)
	}
}

// ---------------------------------------------------------------------------
// Golden: hook rendered with learned rules baked in (4.4)
// ---------------------------------------------------------------------------

func TestApplyCGAGoldenHookWithLearnedRules(t *testing.T) {
	ws := t.TempDir()
	store := state.NewStore(t.TempDir())
	withStubLearnedRules(t, []cga.LearnedRule{
		{Severity: cga.SeverityWarning, Text: "REQUIRE: add regression test before merging", EvidenceCount: 4, ApprovedAt: "2026-01-01T00:00:00Z"},
		{Severity: cga.SeverityCritical, Text: "REJECT if: secrets are committed in plaintext", EvidenceCount: 5, ApprovedAt: "2026-01-02T00:00:00Z"},
	}, nil)

	rec := &state.CGARecord{Enabled: true, StrictMode: true, Timeout: 60}
	if err := applyCGA(ws, store, nil, rec); err != nil {
		t.Fatalf("applyCGA: %v", err)
	}

	hook, err := os.ReadFile(filepath.Join(ws, ".git", "hooks", "pre-commit"))
	if err != nil {
		t.Fatalf("pre-commit hook not written: %v", err)
	}
	golden(t, "cga_hook_learned_rules", string(hook))
}
