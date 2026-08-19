package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/martinhg/capiko-ai/internal/cga"
	"github.com/martinhg/capiko-ai/internal/clilog"
)

func withStubGitDir(t *testing.T, dir string, err error) {
	t.Helper()
	prev := resolveGitDir
	resolveGitDir = func() (string, error) { return dir, err }
	t.Cleanup(func() { resolveGitDir = prev })
}

func withStubCgaBaseDir(t *testing.T, dir string) {
	t.Helper()
	prev := cgaBaseDir
	cgaBaseDir = func() (string, error) { return dir, nil }
	t.Cleanup(func() { cgaBaseDir = prev })
}

func withStubCgaProject(t *testing.T, project string) {
	t.Helper()
	prev := cgaProject
	cgaProject = func() string { return project }
	t.Cleanup(func() { cgaProject = prev })
}

func writeFindingsLog(t *testing.T, gitDir string, content string) {
	t.Helper()
	logDir := filepath.Join(gitDir, "capiko")
	if err := os.MkdirAll(logDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(logDir, cga.FindingsLogName), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// --- Phase 2: `parseScope` flag parsing ---

func TestParseScope(t *testing.T) {
	allowed := []string{"project", "personal", "all"}

	t.Run("valid value extracted and stripped from rest", func(t *testing.T) {
		rest, scope, err := parseScope([]string{"--verbose", "--scope", "personal", "extra"}, allowed, "project")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if scope != "personal" {
			t.Errorf("scope = %q, want personal", scope)
		}
		if strings.Join(rest, ",") != "--verbose,extra" {
			t.Errorf("rest = %v, want [--verbose extra] without --scope personal", rest)
		}
	})

	t.Run("absent flag returns defaultScope and args unchanged", func(t *testing.T) {
		rest, scope, err := parseScope([]string{"--verbose"}, allowed, "project")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if scope != "project" {
			t.Errorf("scope = %q, want project (default)", scope)
		}
		if strings.Join(rest, ",") != "--verbose" {
			t.Errorf("rest = %v, want unchanged", rest)
		}
	})

	t.Run("invalid value returns error", func(t *testing.T) {
		_, _, err := parseScope([]string{"--scope", "bogus"}, allowed, "project")
		if err == nil {
			t.Fatal("expected error for invalid --scope value")
		}
	})
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

// --- Task 3.2: `cga learn` core flow ---

const threePeatWarningLog = `{"timestamp":"2026-08-16T10:00:00Z","parent_sha":"a","commit_sha":"a1","verdict":"FAIL","findings":[{"file":"a.go","severity":"WARNING","description":"missing test coverage"}]}
{"timestamp":"2026-08-17T10:00:00Z","parent_sha":"a1","commit_sha":"b1","verdict":"FAIL","findings":[{"file":"b.go","severity":"WARNING","description":"missing test coverage"}]}
{"timestamp":"2026-08-18T10:00:00Z","parent_sha":"b1","commit_sha":"c1","verdict":"FAIL","findings":[{"file":"c.go","severity":"WARNING","description":"missing test coverage"}]}
`

func TestCgaLearnApprovalPersists(t *testing.T) {
	gitDir := t.TempDir()
	writeFindingsLog(t, gitDir, threePeatWarningLog)
	withStubGitDir(t, gitDir, nil)

	baseDir := t.TempDir()
	withStubCgaBaseDir(t, baseDir)
	withStubCgaProject(t, "acme/repo")
	withStubEngramSync(t, func(string, cga.LearnedRule) error { return nil })

	var buf bytes.Buffer
	handled, exitCode, err := cgaLearn(&buf, strings.NewReader("y\n"), cga.ScopeProject)
	if !handled || exitCode != 0 || err != nil {
		t.Fatalf("handled=%v exitCode=%d err=%v", handled, exitCode, err)
	}

	out := buf.String()
	if !strings.Contains(out, "REQUIRE: missing test coverage") {
		t.Errorf("expected drafted rule text in output:\n%s", out)
	}
	if !strings.Contains(out, "evidence: 3") && !strings.Contains(out, "evidence count: 3") {
		t.Errorf("expected evidence count in output:\n%s", out)
	}

	rules, err := LoadLearnedRules(baseDir, "acme/repo")
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if len(rules) != 1 {
		t.Fatalf("want 1 learned rule, got %d: %+v", len(rules), rules)
	}
	if rules[0].Text != "REQUIRE: missing test coverage" {
		t.Errorf("rule text = %q", rules[0].Text)
	}
	if rules[0].EvidenceCount != 3 {
		t.Errorf("evidence count = %d, want 3", rules[0].EvidenceCount)
	}
	if rules[0].Severity != cga.SeverityWarning {
		t.Errorf("severity = %q, want WARNING", rules[0].Severity)
	}
	if rules[0].ID == "" {
		t.Error("expected non-empty rule ID")
	}
	if rules[0].ApprovedAt == "" {
		t.Error("expected non-empty ApprovedAt")
	}
}

// --- Task 3.3: rejection path ---

func TestCgaLearnRejectionDiscardsRule(t *testing.T) {
	gitDir := t.TempDir()
	writeFindingsLog(t, gitDir, threePeatWarningLog)
	withStubGitDir(t, gitDir, nil)

	baseDir := t.TempDir()
	withStubCgaBaseDir(t, baseDir)
	withStubCgaProject(t, "acme/repo")

	var buf bytes.Buffer
	handled, exitCode, err := cgaLearn(&buf, strings.NewReader("n\n"), cga.ScopeProject)
	if !handled || exitCode != 0 || err != nil {
		t.Fatalf("handled=%v exitCode=%d err=%v", handled, exitCode, err)
	}
	if !strings.Contains(buf.String(), "rejected") {
		t.Errorf("expected 'rejected' in output:\n%s", buf.String())
	}

	rules, err := LoadLearnedRules(baseDir, "acme/repo")
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if len(rules) != 0 {
		t.Fatalf("want 0 learned rules after rejection, got %d: %+v", len(rules), rules)
	}
}

// --- Task 3.4: empty-state ---

const belowThresholdLog = `{"timestamp":"2026-08-16T10:00:00Z","parent_sha":"a","commit_sha":"a1","verdict":"FAIL","findings":[{"file":"a.go","severity":"WARNING","description":"missing test coverage"}]}
{"timestamp":"2026-08-17T10:00:00Z","parent_sha":"a1","commit_sha":"b1","verdict":"FAIL","findings":[{"file":"b.go","severity":"WARNING","description":"missing test coverage"}]}
`

func TestCgaLearnEmptyStateGracefulExit(t *testing.T) {
	gitDir := t.TempDir()
	writeFindingsLog(t, gitDir, belowThresholdLog)
	withStubGitDir(t, gitDir, nil)

	baseDir := t.TempDir()
	withStubCgaBaseDir(t, baseDir)
	withStubCgaProject(t, "acme/repo")

	var buf bytes.Buffer
	handled, exitCode, err := cgaLearn(&buf, strings.NewReader(""), cga.ScopeProject)
	if !handled || exitCode != 0 || err != nil {
		t.Fatalf("handled=%v exitCode=%d err=%v", handled, exitCode, err)
	}
	if !strings.Contains(buf.String(), "no patterns meet the evidence threshold") {
		t.Errorf("expected graceful empty-state message, got:\n%s", buf.String())
	}

	rules, err := LoadLearnedRules(baseDir, "acme/repo")
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if len(rules) != 0 {
		t.Fatalf("want 0 learned rules, got %d: %+v", len(rules), rules)
	}
}

func TestCgaLearnRetiresRulesBelowThreshold(t *testing.T) {
	gitDir := t.TempDir()
	writeFindingsLog(t, gitDir, belowThresholdLog)
	withStubGitDir(t, gitDir, nil)

	baseDir := t.TempDir()
	withStubCgaBaseDir(t, baseDir)
	withStubCgaProject(t, "acme/repo")

	existing := []cga.LearnedRule{
		{ID: "old", Severity: cga.SeverityWarning, Text: "REQUIRE: missing test coverage", EvidenceCount: 3, ApprovedAt: "2026-08-01T00:00:00Z"},
	}
	if err := SaveLearnedRules(baseDir, "acme/repo", existing); err != nil {
		t.Fatalf("seed: %v", err)
	}

	var buf bytes.Buffer
	handled, exitCode, err := cgaLearn(&buf, strings.NewReader(""), cga.ScopeProject)
	if !handled || exitCode != 0 || err != nil {
		t.Fatalf("handled=%v exitCode=%d err=%v", handled, exitCode, err)
	}

	rules, err := LoadLearnedRules(baseDir, "acme/repo")
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if len(rules) != 0 {
		t.Fatalf("want rule retired (0 active rules), got %d: %+v", len(rules), rules)
	}
}

// --- Task 3.5: `cga rules` subcommand ---

func TestCgaRulesPrintsFormattedList(t *testing.T) {
	baseDir := t.TempDir()
	withStubCgaBaseDir(t, baseDir)
	withStubCgaProject(t, "acme/repo")

	rules := []cga.LearnedRule{
		{ID: "abc", Severity: cga.SeverityWarning, Text: "REQUIRE: missing test coverage", EvidenceCount: 4, ApprovedAt: "2026-08-18T10:00:00Z"},
	}
	if err := SaveLearnedRules(baseDir, "acme/repo", rules); err != nil {
		t.Fatalf("seed: %v", err)
	}

	var buf bytes.Buffer
	handled, exitCode, err := cgaRules(&buf, cga.ScopeProject)
	if !handled || exitCode != 0 || err != nil {
		t.Fatalf("handled=%v exitCode=%d err=%v", handled, exitCode, err)
	}
	if buf.String() != cga.FormatLearnedRules(rules)+"\n" {
		t.Errorf("got:\n%s\nwant:\n%s\n", buf.String(), cga.FormatLearnedRules(rules)+"\n")
	}
}

func TestCgaRulesEmptyStoreMessage(t *testing.T) {
	baseDir := t.TempDir()
	withStubCgaBaseDir(t, baseDir)
	withStubCgaProject(t, "acme/repo")

	var buf bytes.Buffer
	handled, exitCode, err := cgaRules(&buf, cga.ScopeProject)
	if !handled || exitCode != 0 || err != nil {
		t.Fatalf("handled=%v exitCode=%d err=%v", handled, exitCode, err)
	}
	if !strings.Contains(buf.String(), "no learned rules") {
		t.Errorf("expected 'no learned rules' in output:\n%s", buf.String())
	}
}

// --- Phase 4: `cga rules --scope` ---

func TestCgaRulesScopeFilters(t *testing.T) {
	baseDir := t.TempDir()
	withStubCgaBaseDir(t, baseDir)
	withStubCgaProject(t, "acme/repo")

	rules := []cga.LearnedRule{
		{ID: "p1", Severity: cga.SeverityWarning, Text: "project rule 1", EvidenceCount: 4, ApprovedAt: "2026-08-18T10:00:00Z", Scope: cga.ScopeProject},
		{ID: "p2", Severity: cga.SeverityCritical, Text: "project rule 2", EvidenceCount: 5, ApprovedAt: "2026-08-18T10:00:00Z", Scope: cga.ScopeProject},
		{ID: "s1", Severity: cga.SeveritySuggestion, Text: "personal rule 1", EvidenceCount: 3, ApprovedAt: "2026-08-18T10:00:00Z", Scope: cga.ScopePersonal},
	}
	if err := SaveLearnedRules(baseDir, "acme/repo", rules); err != nil {
		t.Fatalf("seed: %v", err)
	}

	t.Run("default (project) shows only project-scope rules", func(t *testing.T) {
		var buf bytes.Buffer
		handled, exitCode, err := cgaCommand("cga", []string{"rules"}, &buf)
		if !handled || exitCode != 0 || err != nil {
			t.Fatalf("handled=%v exitCode=%d err=%v", handled, exitCode, err)
		}
		out := buf.String()
		if !strings.Contains(out, "project rule 1") || !strings.Contains(out, "project rule 2") {
			t.Errorf("expected both project rules in output:\n%s", out)
		}
		if strings.Contains(out, "personal rule 1") {
			t.Errorf("did not expect personal rule in default output:\n%s", out)
		}
	})

	t.Run("--scope personal shows only personal rules", func(t *testing.T) {
		var buf bytes.Buffer
		handled, exitCode, err := cgaCommand("cga", []string{"rules", "--scope", "personal"}, &buf)
		if !handled || exitCode != 0 || err != nil {
			t.Fatalf("handled=%v exitCode=%d err=%v", handled, exitCode, err)
		}
		out := buf.String()
		if !strings.Contains(out, "personal rule 1") {
			t.Errorf("expected personal rule in output:\n%s", out)
		}
		if strings.Contains(out, "project rule 1") || strings.Contains(out, "project rule 2") {
			t.Errorf("did not expect project rules in personal-scope output:\n%s", out)
		}
	})

	t.Run("--scope all shows every rule labeled", func(t *testing.T) {
		var buf bytes.Buffer
		handled, exitCode, err := cgaCommand("cga", []string{"rules", "--scope", "all"}, &buf)
		if !handled || exitCode != 0 || err != nil {
			t.Fatalf("handled=%v exitCode=%d err=%v", handled, exitCode, err)
		}
		out := buf.String()
		for _, want := range []string{"project rule 1", "project rule 2", "personal rule 1"} {
			if !strings.Contains(out, want) {
				t.Errorf("expected %q in --scope all output:\n%s", want, out)
			}
		}
	})
}

func TestCgaRulesScopeFilterEmptyResultIsGraceful(t *testing.T) {
	baseDir := t.TempDir()
	withStubCgaBaseDir(t, baseDir)
	withStubCgaProject(t, "acme/repo")

	personalOnly := []cga.LearnedRule{
		{ID: "s1", Severity: cga.SeverityWarning, Text: "personal only rule", EvidenceCount: 3, ApprovedAt: "2026-08-18T10:00:00Z", Scope: cga.ScopePersonal},
	}
	if err := SaveLearnedRules(baseDir, "acme/repo", personalOnly); err != nil {
		t.Fatalf("seed: %v", err)
	}

	var buf bytes.Buffer
	handled, exitCode, err := cgaCommand("cga", []string{"rules", "--scope", "project"}, &buf)
	if !handled || exitCode != 0 || err != nil {
		t.Fatalf("handled=%v exitCode=%d err=%v", handled, exitCode, err)
	}
	if !strings.Contains(buf.String(), "no learned rules") {
		t.Errorf("expected graceful empty message, got:\n%s", buf.String())
	}
}

// --- Task 3.6: engram sync (best-effort) ---

func withStubEngramSync(t *testing.T, fn func(project string, rule cga.LearnedRule) error) {
	t.Helper()
	prev := engramSyncLearnedRule
	engramSyncLearnedRule = fn
	t.Cleanup(func() { engramSyncLearnedRule = prev })
}

func TestCgaLearnSyncsApprovedRuleToEngram(t *testing.T) {
	gitDir := t.TempDir()
	writeFindingsLog(t, gitDir, threePeatWarningLog)
	withStubGitDir(t, gitDir, nil)

	baseDir := t.TempDir()
	withStubCgaBaseDir(t, baseDir)
	withStubCgaProject(t, "acme/repo")

	var syncedProject string
	var syncedRule cga.LearnedRule
	calls := 0
	withStubEngramSync(t, func(project string, rule cga.LearnedRule) error {
		calls++
		syncedProject = project
		syncedRule = rule
		return nil
	})

	var buf bytes.Buffer
	handled, exitCode, err := cgaLearn(&buf, strings.NewReader("y\n"), cga.ScopeProject)
	if !handled || exitCode != 0 || err != nil {
		t.Fatalf("handled=%v exitCode=%d err=%v", handled, exitCode, err)
	}
	if calls != 1 {
		t.Fatalf("want 1 engram sync call, got %d", calls)
	}
	if syncedProject != "acme/repo" {
		t.Errorf("synced project = %q, want acme/repo", syncedProject)
	}
	if syncedRule.Text != "REQUIRE: missing test coverage" {
		t.Errorf("synced rule text = %q", syncedRule.Text)
	}
}

func TestCgaLearnEngramSyncFailureIsNonFatal(t *testing.T) {
	gitDir := t.TempDir()
	writeFindingsLog(t, gitDir, threePeatWarningLog)
	withStubGitDir(t, gitDir, nil)

	baseDir := t.TempDir()
	withStubCgaBaseDir(t, baseDir)
	withStubCgaProject(t, "acme/repo")

	withStubEngramSync(t, func(project string, rule cga.LearnedRule) error {
		return errors.New("engram unavailable")
	})

	var buf bytes.Buffer
	handled, exitCode, err := cgaLearn(&buf, strings.NewReader("y\n"), cga.ScopeProject)
	if !handled || exitCode != 0 || err != nil {
		t.Fatalf("handled=%v exitCode=%d err=%v, want non-fatal sync failure", handled, exitCode, err)
	}
	if !strings.Contains(buf.String(), "engram") {
		t.Errorf("expected a warning mentioning engram in output:\n%s", buf.String())
	}

	rules, loadErr := LoadLearnedRules(baseDir, "acme/repo")
	if loadErr != nil {
		t.Fatalf("load: %v", loadErr)
	}
	if len(rules) != 1 {
		t.Fatalf("want rule persisted locally despite sync failure, got %d", len(rules))
	}
}

// --- Task 3.7: metrics events ---

func withStubCgaLearnLog(t *testing.T, buf *bytes.Buffer) {
	t.Helper()
	prev := cgaLearnLog
	cgaLearnLog = clilog.New(buf, "cga-learn")
	t.Cleanup(func() { cgaLearnLog = prev })
}

func eventNames(t *testing.T, jsonLines string) []string {
	t.Helper()
	var names []string
	for _, line := range strings.Split(strings.TrimSpace(jsonLines), "\n") {
		if line == "" {
			continue
		}
		var e clilog.Entry
		if err := json.Unmarshal([]byte(line), &e); err != nil {
			t.Fatalf("invalid JSON line %q: %v", line, err)
		}
		names = append(names, e.Event)
	}
	return names
}

const twoPatternLog = `{"timestamp":"2026-08-16T10:00:00Z","parent_sha":"a","commit_sha":"a1","verdict":"FAIL","findings":[{"file":"a.go","severity":"WARNING","description":"missing test coverage"},{"file":"x.go","severity":"CRITICAL","description":"errors swallowed silently"}]}
{"timestamp":"2026-08-17T10:00:00Z","parent_sha":"a1","commit_sha":"b1","verdict":"FAIL","findings":[{"file":"b.go","severity":"WARNING","description":"missing test coverage"},{"file":"y.go","severity":"CRITICAL","description":"errors swallowed silently"}]}
{"timestamp":"2026-08-18T10:00:00Z","parent_sha":"b1","commit_sha":"c1","verdict":"FAIL","findings":[{"file":"c.go","severity":"WARNING","description":"missing test coverage"},{"file":"z.go","severity":"CRITICAL","description":"errors swallowed silently"}]}
`

func TestCgaLearnEmitsMetricsEvents(t *testing.T) {
	gitDir := t.TempDir()
	writeFindingsLog(t, gitDir, twoPatternLog)
	withStubGitDir(t, gitDir, nil)

	baseDir := t.TempDir()
	withStubCgaBaseDir(t, baseDir)
	withStubCgaProject(t, "acme/repo")
	withStubEngramSync(t, func(string, cga.LearnedRule) error { return nil })

	stale := []cga.LearnedRule{
		{ID: "stale", Severity: cga.SeverityWarning, Text: "REQUIRE: some rule with no current evidence", EvidenceCount: 3, ApprovedAt: "2026-08-01T00:00:00Z"},
	}
	if err := SaveLearnedRules(baseDir, "acme/repo", stale); err != nil {
		t.Fatalf("seed: %v", err)
	}

	var logBuf bytes.Buffer
	withStubCgaLearnLog(t, &logBuf)

	var out bytes.Buffer
	// Approve the first candidate (WARNING), reject the second (CRITICAL).
	handled, exitCode, err := cgaLearn(&out, strings.NewReader("y\nn\n"), cga.ScopeProject)
	if !handled || exitCode != 0 || err != nil {
		t.Fatalf("handled=%v exitCode=%d err=%v", handled, exitCode, err)
	}

	names := eventNames(t, logBuf.String())
	for _, want := range []string{"pattern_detected", "rule_proposed", "rule_approved", "rule_rejected", "rule_retired"} {
		found := false
		for _, n := range names {
			if n == want {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected %q event, got events: %v", want, names)
		}
	}
}

func TestCgaCommandRulesWiredIn(t *testing.T) {
	baseDir := t.TempDir()
	withStubCgaBaseDir(t, baseDir)
	withStubCgaProject(t, "acme/repo")

	var buf bytes.Buffer
	handled, exitCode, err := cgaCommand("cga", []string{"rules"}, &buf)
	if !handled || exitCode != 0 || err != nil {
		t.Fatalf("handled=%v exitCode=%d err=%v", handled, exitCode, err)
	}
}

// --- Phase 3: wire `cga learn --scope` ---

func withStubCgaStdin(t *testing.T, r io.Reader) {
	t.Helper()
	prev := cgaStdin
	cgaStdin = r
	t.Cleanup(func() { cgaStdin = prev })
}

func TestCgaLearnScopePersonalPersists(t *testing.T) {
	gitDir := t.TempDir()
	writeFindingsLog(t, gitDir, threePeatWarningLog)
	withStubGitDir(t, gitDir, nil)

	baseDir := t.TempDir()
	withStubCgaBaseDir(t, baseDir)
	withStubCgaProject(t, "acme/repo")
	withStubEngramSync(t, func(string, cga.LearnedRule) error { return nil })
	withStubCgaStdin(t, strings.NewReader("y\n"))

	var buf bytes.Buffer
	handled, exitCode, err := cgaCommand("cga", []string{"learn", "--scope", "personal"}, &buf)
	if !handled || exitCode != 0 || err != nil {
		t.Fatalf("handled=%v exitCode=%d err=%v", handled, exitCode, err)
	}

	rules, err := LoadLearnedRules(baseDir, "acme/repo")
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if len(rules) != 1 {
		t.Fatalf("want 1 learned rule, got %d: %+v", len(rules), rules)
	}
	if rules[0].Scope != cga.ScopePersonal {
		t.Errorf("scope = %q, want %q", rules[0].Scope, cga.ScopePersonal)
	}
}

func TestCgaLearnDefaultScopeIsProject(t *testing.T) {
	gitDir := t.TempDir()
	writeFindingsLog(t, gitDir, threePeatWarningLog)
	withStubGitDir(t, gitDir, nil)

	baseDir := t.TempDir()
	withStubCgaBaseDir(t, baseDir)
	withStubCgaProject(t, "acme/repo")
	withStubEngramSync(t, func(string, cga.LearnedRule) error { return nil })
	withStubCgaStdin(t, strings.NewReader("y\n"))

	var buf bytes.Buffer
	handled, exitCode, err := cgaCommand("cga", []string{"learn"}, &buf)
	if !handled || exitCode != 0 || err != nil {
		t.Fatalf("handled=%v exitCode=%d err=%v", handled, exitCode, err)
	}

	rules, err := LoadLearnedRules(baseDir, "acme/repo")
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if len(rules) != 1 {
		t.Fatalf("want 1 learned rule, got %d: %+v", len(rules), rules)
	}
	if rules[0].Scope != cga.ScopeProject {
		t.Errorf("scope = %q, want %q", rules[0].Scope, cga.ScopeProject)
	}
}

func TestCgaLearnInvalidScopeRejected(t *testing.T) {
	gitDir := t.TempDir()
	writeFindingsLog(t, gitDir, threePeatWarningLog)
	withStubGitDir(t, gitDir, nil)

	baseDir := t.TempDir()
	withStubCgaBaseDir(t, baseDir)
	withStubCgaProject(t, "acme/repo")
	withStubCgaStdin(t, strings.NewReader("y\n"))

	var buf bytes.Buffer
	handled, exitCode, err := cgaCommand("cga", []string{"learn", "--scope", "bogus"}, &buf)
	if !handled || exitCode != 1 || err == nil {
		t.Fatalf("handled=%v exitCode=%d err=%v, want handled=true exitCode=1 err!=nil", handled, exitCode, err)
	}

	rules, loadErr := LoadLearnedRules(baseDir, "acme/repo")
	if loadErr != nil {
		t.Fatalf("load: %v", loadErr)
	}
	if len(rules) != 0 {
		t.Fatalf("want 0 rules persisted after invalid --scope, got %d: %+v", len(rules), rules)
	}
}
