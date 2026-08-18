package main

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/martinhg/capiko-ai/internal/cga"
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
