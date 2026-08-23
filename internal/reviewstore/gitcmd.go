package reviewstore

import (
	"bytes"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/martinhg/capiko-ai/internal/rdd"
)

// runGit runs `git -C workspace <args...>` and returns trimmed stdout. On
// failure it wraps the error with the command and captured stderr so
// callers get an actionable message (design Error Handling "Git command
// failure: wrapped exec.ExitError").
func runGit(workspace string, args ...string) (string, error) {
	cmd := exec.Command("git", append([]string{"-C", workspace}, args...)...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = err.Error()
		}
		return "", fmt.Errorf("git %s failed: %s", strings.Join(args, " "), msg)
	}
	return strings.TrimSpace(string(out)), nil
}

// GitCommonDir resolves workspace's shared git directory via
// `git -C workspace rev-parse --git-common-dir`, returning an absolute
// path. Unlike --git-dir, --git-common-dir resolves to the SAME directory
// across every worktree of a repository, which is exactly what worktree-
// shared authority storage needs (design "Worktree-shared storage",
// spec rdd-authority-store "Shared across worktrees"). Package-level var
// seam so callers can stub it in tests.
var GitCommonDir = func(workspace string) (string, error) {
	dir, err := runGit(workspace, "rev-parse", "--git-common-dir")
	if err != nil {
		return "", err
	}
	if !filepath.IsAbs(dir) {
		dir = filepath.Join(workspace, dir)
	}
	return dir, nil
}

// gitWriteTree writes workspace's current index as a tree object via
// `git -C workspace write-tree` and returns its hash. Package-level var
// seam.
var gitWriteTree = func(workspace string) (string, error) {
	return runGit(workspace, "write-tree")
}

// gitRevParseTree resolves ref to its tree hash via
// `git -C workspace rev-parse ref^{tree}`. Package-level var seam.
var gitRevParseTree = func(workspace, ref string) (string, error) {
	return runGit(workspace, "rev-parse", ref+"^{tree}")
}

// gitLsTree lists every entry of treeHash via
// `git -C workspace ls-tree -r --full-tree treeHash` and returns each
// entry's path and file mode. Package-level var seam.
var gitLsTree = func(workspace, treeHash string) ([]rdd.PathMode, error) {
	out, err := runGit(workspace, "ls-tree", "-r", "--full-tree", treeHash)
	if err != nil {
		return nil, err
	}
	return parseLsTree(out)
}

// gitDiffTree lists the paths and new file modes that changed between
// baseTree and candidateTree via
// `git -C workspace diff-tree -r --no-commit-id baseTree candidateTree`.
// Package-level var seam.
var gitDiffTree = func(workspace, baseTree, candidateTree string) ([]rdd.PathMode, error) {
	out, err := runGit(workspace, "diff-tree", "-r", "--no-commit-id", baseTree, candidateTree)
	if err != nil {
		return nil, err
	}
	return parseDiffTree(out)
}

// gitRemoteOriginURL resolves the "origin" remote URL via
// `git -C workspace remote get-url origin`. Returns an error if no origin
// remote is configured; callers (e.g. BuildIdentity, in a later PR) decide
// whether to fall back to another repository identifier. Package-level var
// seam.
var gitRemoteOriginURL = func(workspace string) (string, error) {
	return runGit(workspace, "remote", "get-url", "origin")
}

// gitIsBareRepo reports whether workspace is a bare repository via
// `git -C workspace rev-parse --is-bare-repository`. Package-level var
// seam.
var gitIsBareRepo = func(workspace string) (bool, error) {
	out, err := runGit(workspace, "rev-parse", "--is-bare-repository")
	if err != nil {
		return false, err
	}
	return out == "true", nil
}

// gitIsShallowRepo reports whether workspace is a shallow clone via
// `git -C workspace rev-parse --is-shallow-repository`. Package-level var
// seam.
var gitIsShallowRepo = func(workspace string) (bool, error) {
	out, err := runGit(workspace, "rev-parse", "--is-shallow-repository")
	if err != nil {
		return false, err
	}
	return out == "true", nil
}

// gitMergeBase resolves the merge-base commit of commitA and commitB via
// `git -C workspace merge-base commitA commitB` (spec relation-algebra;
// design "Seam Catalog"). BuildAncestryEvidence (below) uses the result as
// the common ancestor both candidates' patch-ids are diffed against.
// Package-level var seam.
var gitMergeBase = func(workspace, commitA, commitB string) (string, error) {
	return runGit(workspace, "merge-base", commitA, commitB)
}

// gitPatchID computes the stable patch-id of the diff from base to candidate
// via `git -C workspace diff base candidate | git -C workspace patch-id
// --stable` (spec relation-algebra; design "Seam Catalog"). Unlike the other
// seams, patch-id reads its diff from stdin rather than accepting a ref pair
// directly, so this wires two exec.Commands together with a pipe instead of
// using runGit's single-command helper. Package-level var seam.
var gitPatchID = func(workspace, base, candidate string) (string, error) {
	diffCmd := exec.Command("git", "-C", workspace, "diff", base, candidate)
	patchIDCmd := exec.Command("git", "-C", workspace, "patch-id", "--stable")

	pipe, err := diffCmd.StdoutPipe()
	if err != nil {
		return "", fmt.Errorf("git patch-id: failed to wire diff pipe: %v", err)
	}
	patchIDCmd.Stdin = pipe

	var patchIDOut, diffErr, patchIDErr bytes.Buffer
	diffCmd.Stderr = &diffErr
	patchIDCmd.Stdout = &patchIDOut
	patchIDCmd.Stderr = &patchIDErr

	if err := patchIDCmd.Start(); err != nil {
		return "", fmt.Errorf("git patch-id: failed to start: %v", err)
	}
	if err := diffCmd.Run(); err != nil {
		msg := strings.TrimSpace(diffErr.String())
		if msg == "" {
			msg = err.Error()
		}
		_ = patchIDCmd.Wait()
		return "", fmt.Errorf("git diff %s %s failed: %s", base, candidate, msg)
	}
	if err := patchIDCmd.Wait(); err != nil {
		msg := strings.TrimSpace(patchIDErr.String())
		if msg == "" {
			msg = err.Error()
		}
		return "", fmt.Errorf("git patch-id failed: %s", msg)
	}

	out := strings.TrimSpace(patchIDOut.String())
	if out == "" {
		// An empty diff (candidate identical to base) produces no patch-id
		// output. This is a meaningful signal, not an error: rdd.Compare
		// treats an empty patch-id as "no changes relative to base", which
		// is exactly what a provable contraction looks like.
		return "", nil
	}
	// patch-id output is "<patch-id> <commit-or-blank>"; only the first
	// field is the stable patch-id.
	fields := strings.Fields(out)
	return fields[0], nil
}

// parseLsTree parses `git ls-tree -r --full-tree` output — lines of the
// form "<mode> <type> <object>\t<path>" — into path/mode pairs.
func parseLsTree(out string) ([]rdd.PathMode, error) {
	if out == "" {
		return nil, nil
	}
	lines := strings.Split(out, "\n")
	entries := make([]rdd.PathMode, 0, len(lines))
	for _, line := range lines {
		if line == "" {
			continue
		}
		meta, path, ok := strings.Cut(line, "\t")
		if !ok {
			return nil, fmt.Errorf("gitLsTree: unexpected line format: %q", line)
		}
		fields := strings.Fields(meta)
		if len(fields) < 1 {
			return nil, fmt.Errorf("gitLsTree: unexpected metadata: %q", meta)
		}
		entries = append(entries, rdd.PathMode{Path: path, Mode: fields[0]})
	}
	return entries, nil
}

// parseDiffTree parses `git diff-tree -r --no-commit-id` output — lines of
// the form ":<old_mode> <new_mode> <old_sha> <new_sha> <status>\t<path>" —
// into path/new-mode pairs.
func parseDiffTree(out string) ([]rdd.PathMode, error) {
	if out == "" {
		return nil, nil
	}
	lines := strings.Split(out, "\n")
	entries := make([]rdd.PathMode, 0, len(lines))
	for _, line := range lines {
		if line == "" {
			continue
		}
		meta, path, ok := strings.Cut(line, "\t")
		if !ok {
			return nil, fmt.Errorf("gitDiffTree: unexpected line format: %q", line)
		}
		fields := strings.Fields(meta)
		if len(fields) < 2 {
			return nil, fmt.Errorf("gitDiffTree: unexpected metadata: %q", meta)
		}
		entries = append(entries, rdd.PathMode{Path: path, Mode: fields[1]})
	}
	return entries, nil
}
