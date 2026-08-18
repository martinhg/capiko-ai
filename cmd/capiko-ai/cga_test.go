package main

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/martinhg/capiko-ai/internal/cga"
)

func withStubGitDir(t *testing.T, dir string, err error) {
	t.Helper()
	prev := resolveGitDir
	resolveGitDir = func() (string, error) { return dir, err }
	t.Cleanup(func() { resolveGitDir = prev })
}

func TestCgaCommandNotHandledForOtherName(t *testing.T) {
	var buf bytes.Buffer
	handled, exitCode, err := cgaCommand("backup", nil, &buf)
	if handled || exitCode != 0 || err != nil {
		t.Fatalf("cga should not handle %q", "backup")
	}
}

func TestCgaCommandRequiresSubcommand(t *testing.T) {
	var buf bytes.Buffer
	handled, exitCode, err := cgaCommand("cga", nil, &buf)
	if !handled || exitCode != 1 || err == nil {
		t.Fatalf("handled=%v exitCode=%d err=%v, want handled=true exitCode=1 err!=nil", handled, exitCode, err)
	}
}

func TestCgaCommandUnknownSubcommand(t *testing.T) {
	var buf bytes.Buffer
	handled, exitCode, _ := cgaCommand("cga", []string{"bogus"}, &buf)
	if !handled || exitCode != 1 {
		t.Fatalf("handled=%v exitCode=%d, want handled=true exitCode=1", handled, exitCode)
	}
	if !strings.Contains(buf.String(), "unknown") {
		t.Errorf("expected 'unknown' in output:\n%s", buf.String())
	}
	if !strings.Contains(buf.String(), "Usage:") {
		t.Errorf("expected usage in output:\n%s", buf.String())
	}
}

func TestCgaFindingsPrintsEntries(t *testing.T) {
	gitDir := t.TempDir()
	logDir := filepath.Join(gitDir, "capiko")
	if err := os.MkdirAll(logDir, 0o755); err != nil {
		t.Fatal(err)
	}
	logContent := `{"timestamp":"2026-08-18T14:30:00Z","parent_sha":"abc1234","commit_sha":"def5678","verdict":"FAIL","reason":"missing tests","findings":[{"file":"a.go","line":10,"severity":"CRITICAL","description":"var named x"}]}
{"timestamp":"2026-08-18T14:31:00Z","parent_sha":"def5678","commit_sha":"ghi9012","verdict":"PASS"}
`
	if err := os.WriteFile(filepath.Join(logDir, "cga-findings.jsonl"), []byte(logContent), 0o644); err != nil {
		t.Fatal(err)
	}

	withStubGitDir(t, gitDir, nil)
	var buf bytes.Buffer
	handled, exitCode, err := cgaCommand("cga", []string{"findings"}, &buf)
	if !handled || exitCode != 0 || err != nil {
		t.Fatalf("handled=%v exitCode=%d err=%v", handled, exitCode, err)
	}
	out := buf.String()
	if !strings.Contains(out, "2026-08-18T14:30:00Z") {
		t.Errorf("expected timestamp in output:\n%s", out)
	}
	if !strings.Contains(out, "FAIL") || !strings.Contains(out, "PASS") {
		t.Errorf("expected both verdicts in output:\n%s", out)
	}
}

func TestCgaFindingsNoLogDirectory(t *testing.T) {
	gitDir := t.TempDir()
	withStubGitDir(t, gitDir, nil)
	var buf bytes.Buffer
	handled, exitCode, err := cgaCommand("cga", []string{"findings"}, &buf)
	if !handled || exitCode != 0 || err != nil {
		t.Fatalf("handled=%v exitCode=%d err=%v", handled, exitCode, err)
	}
	if !strings.Contains(buf.String(), "no findings recorded") {
		t.Errorf("expected 'no findings recorded' in output:\n%s", buf.String())
	}
}

func TestCgaFindingsEmptyLog(t *testing.T) {
	gitDir := t.TempDir()
	logDir := filepath.Join(gitDir, "capiko")
	if err := os.MkdirAll(logDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(logDir, "cga-findings.jsonl"), nil, 0o644); err != nil {
		t.Fatal(err)
	}

	withStubGitDir(t, gitDir, nil)
	var buf bytes.Buffer
	handled, exitCode, err := cgaCommand("cga", []string{"findings"}, &buf)
	if !handled || exitCode != 0 || err != nil {
		t.Fatalf("handled=%v exitCode=%d err=%v", handled, exitCode, err)
	}
	if !strings.Contains(buf.String(), "no findings recorded") {
		t.Errorf("expected 'no findings recorded' in output:\n%s", buf.String())
	}
}

func TestCgaFindingsGitDirResolutionFails(t *testing.T) {
	withStubGitDir(t, "", errors.New("not a git repository"))
	var buf bytes.Buffer
	handled, exitCode, err := cgaCommand("cga", []string{"findings"}, &buf)
	if !handled || exitCode != 0 || err != nil {
		t.Fatalf("handled=%v exitCode=%d err=%v", handled, exitCode, err)
	}
	if !strings.Contains(buf.String(), "no findings recorded") {
		t.Errorf("expected 'no findings recorded' in output:\n%s", buf.String())
	}
}

// --- Task 3.1: local JSON store ---

func TestLoadLearnedRulesMissingFileReturnsEmpty(t *testing.T) {
	baseDir := t.TempDir()
	rules, err := LoadLearnedRules(baseDir, "acme/repo")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(rules) != 0 {
		t.Fatalf("want empty slice, got %v", rules)
	}
}

func TestSaveLoadLearnedRulesRoundTrip(t *testing.T) {
	baseDir := t.TempDir()
	want := []cga.LearnedRule{
		{ID: "abc123", Severity: cga.SeverityWarning, Text: "REQUIRE: missing test coverage", EvidenceCount: 4, ApprovedAt: "2026-08-18T10:00:00Z"},
		{ID: "def456", Severity: cga.SeverityCritical, Text: "REJECT if: errors swallowed silently", EvidenceCount: 3, ApprovedAt: "2026-08-17T10:00:00Z"},
	}

	if err := SaveLearnedRules(baseDir, "acme/repo", want); err != nil {
		t.Fatalf("save: %v", err)
	}

	got, err := LoadLearnedRules(baseDir, "acme/repo")
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v, want %+v", got, want)
	}

	path := filepath.Join(baseDir, "cga", "acme/repo", "learned-rules.json")
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("expected file at %s: %v", path, err)
	}
}
