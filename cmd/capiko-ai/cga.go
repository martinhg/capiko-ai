package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/martinhg/capiko-ai/internal/cga"
	"github.com/martinhg/capiko-ai/internal/state"
)

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
		fmt.Fprintln(out, "Usage:")
		fmt.Fprintln(out, "  capiko-ai cga findings")
		return true, 1, fmt.Errorf("cga: subcommand required (findings)")
	}

	sub := args[0]
	switch sub {
	case "findings":
		return cgaFindings(out)
	default:
		fmt.Fprintf(out, "cga: unknown subcommand %q\n", sub)
		fmt.Fprintln(out, "Usage:")
		fmt.Fprintln(out, "  capiko-ai cga findings")
		return true, 1, nil
	}
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
