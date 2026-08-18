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
		return cgaLearn(out, cgaStdin)
	case "rules":
		return cgaRules(out)
	default:
		fmt.Fprintf(out, "cga: unknown subcommand %q\n", sub)
		fmt.Fprintln(out, cgaUsage)
		return true, 1, nil
	}
}

// cgaUsage is the usage block printed for a missing or unknown subcommand.
const cgaUsage = "Usage:\n" +
	"  capiko-ai cga findings\n" +
	"  capiko-ai cga learn\n" +
	"  capiko-ai cga rules"

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

// cgaLearn runs the learn-loop: detect recurring findings-log patterns
// (spec F3.1), draft candidate rules (F3.2), present each for approval on
// in/out (F3.3), and persist approved rules to the local JSON store (F3.4).
// A git dir resolution failure or missing/empty log is treated as "no
// patterns detected" rather than an error, matching cgaFindings' graceful
// degradation.
func cgaLearn(out io.Writer, in io.Reader) (bool, int, error) {
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

	if len(patterns) == 0 && len(existing) == 0 {
		fmt.Fprintln(out, "no patterns meet the evidence threshold")
		return true, 0, nil
	}

	// Re-check existing rules against the current detection pass (spec F3.6,
	// design D7): rules whose evidence dropped below threshold are retired
	// automatically, with no user action required.
	active, _ := cga.CheckRetirement(existing, patterns, cgaLearnThreshold)

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
		fmt.Fprintf(out, "\nCandidate rule (%s severity, evidence: %d):\n  %s\n", p.Severity, p.EvidenceCount, text)
		if len(p.Files) > 0 {
			fmt.Fprintf(out, "  files: %s\n", strings.Join(p.Files, ", "))
		}
		fmt.Fprint(out, "Approve? [y/N] ")
		line, _ := reader.ReadString('\n')
		answer := strings.ToLower(strings.TrimSpace(line))
		if answer != "y" && answer != "yes" {
			fmt.Fprintln(out, "rejected")
			continue
		}

		rule := cga.LearnedRule{
			ID:            state.Checksum(string(p.Severity) + "|" + p.Description),
			Severity:      p.Severity,
			Text:          text,
			EvidenceCount: p.EvidenceCount,
			ApprovedAt:    cgaNow().UTC().Format(time.RFC3339),
		}
		learned = append(learned, rule)
		fmt.Fprintln(out, "approved")
	}

	learned = cga.EnforceCap(learned, cga.LearnedRulesCap)
	if err := SaveLearnedRules(baseDir, project, learned); err != nil {
		return true, 1, err
	}

	return true, 0, nil
}

// cgaRules lists the locally persisted learned rules for the current
// workspace's engram project (spec F3.7).
func cgaRules(out io.Writer) (bool, int, error) {
	baseDir, err := cgaBaseDir()
	if err != nil {
		return true, 1, err
	}
	rules, err := LoadLearnedRules(baseDir, cgaProject())
	if err != nil {
		return true, 1, err
	}
	if len(rules) == 0 {
		fmt.Fprintln(out, "no learned rules")
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
