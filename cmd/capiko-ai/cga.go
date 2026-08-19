package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/martinhg/capiko-ai/internal/cga"
	"github.com/martinhg/capiko-ai/internal/clilog"
	"github.com/martinhg/capiko-ai/internal/sddstatus"
	"github.com/martinhg/capiko-ai/internal/state"
)

// cgaLearnThreshold is the minimum evidence count (spec F3.1) for a
// recurring findings-log pattern to become a candidate rule.
const cgaLearnThreshold = 3

// resolveGitDir returns the current workspace's git directory, resolved via
// `git rev-parse --git-dir` so it works correctly from worktrees (where
// `.git` is a file pointing elsewhere, not a directory). It is a
// package-level seam so tests can stub git resolution without a real repo.
var resolveGitDir = func() (string, error) {
	out, err := exec.Command("git", "rev-parse", "--git-dir").Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

// cgaCommand handles headless CGA operations.
//
//	capiko-ai cga findings
func cgaCommand(name string, args []string, out io.Writer) (handled bool, exitCode int, err error) {
	if name != "cga" {
		return false, 0, nil
	}

	if len(args) == 0 {
		fmt.Fprintln(out, cgaUsage)
		return true, 1, fmt.Errorf("cga: subcommand required (findings, learn, rules)")
	}

	sub := args[0]
	switch sub {
	case "findings":
		return cgaFindings(out)
	case "learn":
		rest, verbose := parseVerbose(args[1:])
		_, scope, err := parseScope(rest, cgaScopeValues, cga.ScopeProject)
		if err != nil {
			fmt.Fprintln(out, err)
			return true, 1, err
		}
		cgaLearnLog = newLogger("cga-learn", verbose)
		return cgaLearn(out, cgaStdin, scope)
	case "rules":
		_, scope, err := parseScope(args[1:], cgaRulesScopeValues, cga.ScopeProject)
		if err != nil {
			fmt.Fprintln(out, err)
			return true, 1, err
		}
		return cgaRules(out, scope)
	default:
		fmt.Fprintf(out, "cga: unknown subcommand %q\n", sub)
		fmt.Fprintln(out, cgaUsage)
		return true, 1, nil
	}
}

// cgaUsage is the usage block printed for a missing or unknown subcommand.
const cgaUsage = "Usage:\n" +
	"  capiko-ai cga findings\n" +
	"  capiko-ai cga learn [--scope personal]\n" +
	"  capiko-ai cga rules [--scope project|personal|all]"

// cgaScopeValues is the set of values `--scope` accepts on `cga learn`
// (project|personal) and `cga rules` (project|personal|all — see
// cgaRulesScopeValues).
var cgaScopeValues = []string{cga.ScopeProject, cga.ScopePersonal}

// cgaRulesScopeValues extends cgaScopeValues with "all" — the extra display
// mode only `cga rules` accepts (spec: cga-scope-discipline).
var cgaRulesScopeValues = []string{cga.ScopeProject, cga.ScopePersonal, "all"}

// parseScope extracts "--scope <value>" from args, validates it against
// allowed, and returns the remaining args with the flag and its value
// removed. When "--scope" is absent, it returns defaultScope and args
// unchanged. Mirrors parseVerbose's manual-iteration style (design D3).
func parseScope(args []string, allowed []string, defaultScope string) (rest []string, scope string, err error) {
	scope = defaultScope
	rest = make([]string, 0, len(args))
	for i := 0; i < len(args); i++ {
		if args[i] != "--scope" {
			rest = append(rest, args[i])
			continue
		}
		if i+1 >= len(args) {
			return nil, "", fmt.Errorf("cga: --scope requires a value (%s)", strings.Join(allowed, "|"))
		}
		value := args[i+1]
		i++
		valid := false
		for _, a := range allowed {
			if value == a {
				valid = true
				break
			}
		}
		if !valid {
			return nil, "", fmt.Errorf("cga: invalid --scope value %q, want one of %s", value, strings.Join(allowed, "|"))
		}
		scope = value
	}
	return rest, scope, nil
}

// cgaFindings prints the persisted CGA findings log for the current
// workspace. A git dir resolution failure, missing log file, or empty log
// are all treated as "no findings recorded" rather than an error — matching
// the findings log's graceful-degradation contract (log failures never
// block a commit, and reading them never fails the CLI).
func cgaFindings(out io.Writer) (bool, int, error) {
	gitDir, err := resolveGitDir()
	if err != nil {
		fmt.Fprintln(out, "no findings recorded")
		return true, 0, nil
	}

	logPath := filepath.Join(gitDir, "capiko", cga.FindingsLogName)
	f, err := os.Open(logPath)
	if err != nil {
		fmt.Fprintln(out, "no findings recorded")
		return true, 0, nil
	}
	defer f.Close()

	entries, err := cga.ParseLog(f)
	if err != nil {
		return true, 1, err
	}
	if len(entries) == 0 {
		fmt.Fprintln(out, "no findings recorded")
		return true, 0, nil
	}

	for _, e := range entries {
		fmt.Fprintf(out, "%s  %-10s  %s", e.Timestamp, e.Verdict, e.CommitSHA)
		if e.Reason != "" {
			fmt.Fprintf(out, "  %s", e.Reason)
		}
		fmt.Fprintf(out, "  (%d finding(s))\n", len(e.Findings))
	}
	return true, 0, nil
}

// cgaBaseDir resolves the local persistence root for learned rules. It
// mirrors state.DefaultStore's ~/.capiko convention (design D3) so learned
// rules live alongside other capiko-managed state. It is a seam so tests can
// stub it to a t.TempDir() without touching the real home directory.
var cgaBaseDir = func() (string, error) {
	s, err := state.DefaultStore()
	if err != nil {
		return "", err
	}
	return s.Dir(), nil
}

// cgaProject resolves the engram project identity for the current workspace
// via the shared inference chain (env var → git origin → dir basename). It
// is a seam so tests can pin a project without depending on the real working
// directory or git config.
var cgaProject = func() string {
	cwd, err := os.Getwd()
	if err != nil {
		return ""
	}
	return sddstatus.InferEngramProject(cwd)
}

// cgaStdin is the source of `cga learn` approval-prompt answers. It is a
// seam so tests can inject canned answers via strings.Reader instead of the
// real stdin.
var cgaStdin io.Reader = os.Stdin

// cgaNow is a seam over time.Now for deterministic ApprovedAt timestamps in
// tests.
var cgaNow = time.Now

// cgaLearnLog emits structured learn-loop metrics (spec F3.8): a disabled
// no-op logger by default, wired to a real sink when `--verbose` is passed
// to `cga learn`. It is a package-level var so tests can swap it for a
// buffer-backed logger and assert on the emitted events directly.
var cgaLearnLog = clilog.New(nil, "cga-learn")

// engramSaveRunner shells out to the engram CLI. It is a package-level seam
// (below engramSyncLearnedRule) so tests can assert on the exact argv passed
// to `engram save` without shelling out to a real binary.
var engramSaveRunner = func(args ...string) error {
	return exec.Command("engram", append([]string{"save"}, args...)...).Run()
}

// engramSyncTitleMaxRuleTextLen bounds how much of a rule's text is embedded
// in the engram save title, keeping it a scannable one-liner.
const engramSyncTitleMaxRuleTextLen = 80

// engramSyncLearnedRule is a best-effort seam that syncs an approved rule to
// engram (topic key cga/{project}/learned-rules, spec F3.4) by shelling out
// to the engram CLI. The local JSON store stays canonical — a sync failure
// is non-fatal, surfaced as a warning, and never blocks persistence or the
// exit code (design open question: local file is the sole writable store of
// record; engram sync is best-effort durability, not a dependency).
//
// `engram save` takes title and content as POSITIONAL arguments (verified
// via `engram save --help`, v1.20.0): `engram save <title> <content>
// [--type T] [--project P] [--scope S] [--topic KEY]`. The topic flag is
// `--topic`, not `--topic-key`.
var engramSyncLearnedRule = func(project string, rule cga.LearnedRule) error {
	payload, err := json.Marshal(rule)
	if err != nil {
		return err
	}
	title := fmt.Sprintf("CGA learned rule: %s", truncateRuleText(rule.Text, engramSyncTitleMaxRuleTextLen))
	topic := fmt.Sprintf("cga/%s/learned-rules", project)
	return engramSaveRunner(
		title,
		string(payload),
		"--project", project,
		"--scope", "project",
		"--type", "architecture",
		"--topic", topic,
	)
}

// truncateRuleText shortens s to at most max runes, appending an ellipsis
// when truncated, so engram save titles stay a scannable one-liner.
func truncateRuleText(s string, max int) string {
	r := []rune(s)
	if len(r) <= max {
		return s
	}
	return string(r[:max]) + "…"
}

// cgaLearn runs the learn-loop: detect recurring findings-log patterns
// (spec F3.1), draft candidate rules (F3.2), present each for approval on
// in/out (F3.3), and persist approved rules to the local JSON store (F3.4).
// Every rule approved in this run is stamped with scope (spec CGA Phase 4
// Slice A: "project" by default, "personal" when the caller passed
// --scope personal). A git dir resolution failure or missing/empty log is
// treated as "no patterns detected" rather than an error, matching
// cgaFindings' graceful degradation.
func cgaLearn(out io.Writer, in io.Reader, scope string) (bool, int, error) {
	gitDir, err := resolveGitDir()
	if err != nil {
		fmt.Fprintln(out, "no findings recorded — nothing to learn from")
		return true, 0, nil
	}

	logPath := filepath.Join(gitDir, "capiko", cga.FindingsLogName)
	f, err := os.Open(logPath)
	if err != nil {
		fmt.Fprintln(out, "no findings recorded — nothing to learn from")
		return true, 0, nil
	}
	entries, err := cga.ParseLog(f)
	f.Close()
	if err != nil {
		return true, 1, err
	}

	baseDir, err := cgaBaseDir()
	if err != nil {
		return true, 1, err
	}
	project := cgaProject()

	existing, err := LoadLearnedRules(baseDir, project)
	if err != nil {
		return true, 1, err
	}
	patterns := cga.DetectPatterns(entries, cgaLearnThreshold)
	for _, p := range patterns {
		cgaLearnLog.Event("pattern_detected", cga.DraftRuleText(p))
	}

	if len(patterns) == 0 && len(existing) == 0 {
		fmt.Fprintln(out, "no patterns meet the evidence threshold")
		return true, 0, nil
	}

	// Re-check existing rules against the current detection pass (spec F3.6,
	// design D7): rules whose evidence dropped below threshold are retired
	// automatically, with no user action required.
	active, retired := cga.CheckRetirement(existing, patterns, cgaLearnThreshold)
	for _, r := range retired {
		cgaLearnLog.Event("rule_retired", r.ID)
	}

	activeText := make(map[string]bool, len(active))
	for _, r := range active {
		activeText[r.Text] = true
	}
	var candidates []cga.Pattern
	for _, p := range patterns {
		if !activeText[cga.DraftRuleText(p)] {
			candidates = append(candidates, p)
		}
	}

	if len(candidates) == 0 {
		fmt.Fprintln(out, "no new patterns meet the evidence threshold")
		if err := SaveLearnedRules(baseDir, project, active); err != nil {
			return true, 1, err
		}
		return true, 0, nil
	}

	learned := active
	reader := bufio.NewReader(in)
	for _, p := range candidates {
		text := cga.DraftRuleText(p)
		cgaLearnLog.Event("rule_proposed", text)
		fmt.Fprintf(out, "\nCandidate rule (%s severity, evidence: %d):\n  %s\n", p.Severity, p.EvidenceCount, text)
		if len(p.Files) > 0 {
			fmt.Fprintf(out, "  files: %s\n", strings.Join(p.Files, ", "))
		}
		fmt.Fprint(out, "Approve? [y/N] ")
		line, _ := reader.ReadString('\n')
		answer := strings.ToLower(strings.TrimSpace(line))
		if answer != "y" && answer != "yes" {
			cgaLearnLog.Event("rule_rejected", text)
			fmt.Fprintln(out, "rejected")
			continue
		}

		rule := cga.LearnedRule{
			ID:            state.Checksum(string(p.Severity) + "|" + p.Description),
			Severity:      p.Severity,
			Text:          text,
			EvidenceCount: p.EvidenceCount,
			ApprovedAt:    cgaNow().UTC().Format(time.RFC3339),
			Scope:         scope,
		}
		cgaLearnLog.Event("rule_approved", rule.ID)
		learned = append(learned, rule)
		fmt.Fprintln(out, "approved")

		if rule.Scope != cga.ScopeProject {
			cgaLearnLog.Event("rule_sync_skipped_personal_scope", rule.ID)
			fmt.Fprintln(out, "skipping engram sync: personal-scope rules are local-only")
		} else if syncErr := engramSyncLearnedRule(project, rule); syncErr != nil {
			fmt.Fprintf(out, "warning: engram sync failed: %v\n", syncErr)
		}
	}

	learned = cga.EnforceCap(learned, cga.LearnedRulesCap)
	if err := SaveLearnedRules(baseDir, project, learned); err != nil {
		return true, 1, err
	}

	return true, 0, nil
}

// cgaRules lists the locally persisted learned rules for the current
// workspace's engram project (spec F3.7), filtered by scope
// (project|personal|all, default project; spec CGA Phase 4 Slice A). When no
// rule matches the selected scope, it prints a graceful message and exits 0.
func cgaRules(out io.Writer, scope string) (bool, int, error) {
	baseDir, err := cgaBaseDir()
	if err != nil {
		return true, 1, err
	}
	rules, err := LoadLearnedRules(baseDir, cgaProject())
	if err != nil {
		return true, 1, err
	}
	rules = cga.FilterByScope(rules, scope)
	if len(rules) == 0 {
		fmt.Fprintf(out, "no learned rules (scope: %s)\n", cga.ResolveScope(scope))
		return true, 0, nil
	}
	fmt.Fprintln(out, cga.FormatLearnedRules(rules))
	return true, 0, nil
}

// learnedRulesPath returns the local JSON store path for a project's learned
// rules: "{baseDir}/cga/{project}/learned-rules.json" (spec F3.4, design D3).
func learnedRulesPath(baseDir, project string) string {
	return filepath.Join(baseDir, "cga", project, "learned-rules.json")
}

// LoadLearnedRules reads the local JSON store of learned rules for the given
// baseDir and project. A missing file is not an error — it yields an empty
// slice, matching the store's "no rules yet" steady state.
func LoadLearnedRules(baseDir, project string) ([]cga.LearnedRule, error) {
	data, err := os.ReadFile(learnedRulesPath(baseDir, project))
	if err != nil {
		if os.IsNotExist(err) {
			return []cga.LearnedRule{}, nil
		}
		return nil, err
	}
	var rules []cga.LearnedRule
	if err := json.Unmarshal(data, &rules); err != nil {
		return nil, err
	}
	return rules, nil
}

// SaveLearnedRules writes rules to the local JSON store for baseDir and
// project, creating the containing directory if needed.
func SaveLearnedRules(baseDir, project string, rules []cga.LearnedRule) error {
	dir := filepath.Join(baseDir, "cga", project)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(rules, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(learnedRulesPath(baseDir, project), data, 0o644)
}
