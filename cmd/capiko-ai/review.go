package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/martinhg/capiko-ai/internal/rdd"
	"github.com/martinhg/capiko-ai/internal/reviewstore"
)

// reviewUsage is the usage block printed for a missing or unknown
// subcommand/verb.
const reviewUsage = "Usage:\n" +
	"  capiko-ai review mode enable\n" +
	"  capiko-ai review mode disable\n" +
	"  capiko-ai review mode status"

// resolveWorkspace resolves the current workspace root for RDD kill-switch
// scoping, defaulting to the process's current working directory.
// Package-level var seam (design "cmd/capiko-ai | resolveWorkspace") so
// tests can stub it without depending on the real cwd.
var resolveWorkspace = func() (string, error) {
	return os.Getwd()
}

// userHomeDirFn resolves the user's home directory, used to locate the
// global-scope kill-switch record at ~/.capiko/review-mode.json.
// Package-level var seam (design "cmd/capiko-ai | userHomeDirFn").
var userHomeDirFn = os.UserHomeDir

// gitCommonDirFn resolves workspace's shared git directory via
// `git -C workspace rev-parse --git-common-dir`, returning an absolute
// path. Mirrors reviewstore's unexported gitCommonDir seam (design
// "gitCommonDirFn (reuses reviewstore.gitCommonDir or wraps it)") — kept as
// its own seam here, rather than importing the unexported symbol, since
// cmd/capiko-ai cannot reach an unexported package-level var in another
// package; this follows cga.go's resolveGitDir precedent of shelling out
// directly instead of depending on reviewstore internals.
var gitCommonDirFn = func(workspace string) (string, error) {
	out, err := exec.Command("git", "-C", workspace, "rev-parse", "--git-common-dir").Output()
	if err != nil {
		return "", fmt.Errorf("git rev-parse --git-common-dir failed: %w", err)
	}
	dir := strings.TrimSpace(string(out))
	if !filepath.IsAbs(dir) {
		dir = filepath.Join(workspace, dir)
	}
	return dir, nil
}

// reviewNow is a seam over time.Now for deterministic ModeRecord.UpdatedAt
// timestamps in tests, mirroring cga.go's cgaNow.
var reviewNow = time.Now

// reviewCommand handles headless RDD kill-switch operations.
//
//	capiko-ai review mode enable|disable|status
func reviewCommand(name string, args []string, out io.Writer) (handled bool, exitCode int, err error) {
	if name != "review" {
		return false, 0, nil
	}

	if len(args) == 0 {
		fmt.Fprintln(out, reviewUsage)
		return true, 1, fmt.Errorf("review: subcommand required (mode)")
	}

	sub := args[0]
	switch sub {
	case "mode":
		return reviewMode(args[1:], out)
	default:
		fmt.Fprintf(out, "review: unknown subcommand %q\n", sub)
		fmt.Fprintln(out, reviewUsage)
		return true, 1, nil
	}
}

// reviewMode dispatches `review mode <verb>` to enable, disable, or status.
func reviewMode(args []string, out io.Writer) (bool, int, error) {
	if len(args) == 0 {
		fmt.Fprintln(out, reviewUsage)
		return true, 1, fmt.Errorf("review mode: verb required (enable, disable, status)")
	}

	verb := args[0]
	switch verb {
	case "enable":
		return reviewModeSet(rdd.ModeManaged, out)
	case "disable":
		return reviewModeSet(rdd.ModeDisabled, out)
	case "status":
		return reviewModeStatus(out)
	default:
		fmt.Fprintf(out, "review: unknown mode verb %q\n", verb)
		fmt.Fprintln(out, reviewUsage)
		return true, 1, nil
	}
}

// cloneModePath resolves the clone-scope kill-switch record path under
// commonDir (design "Storage Layout": <git-common-dir>/capiko/rdd/review-mode.json).
func cloneModePath(commonDir string) string {
	return filepath.Join(commonDir, "capiko", "rdd", "review-mode.json")
}

// globalModePath resolves the global-scope kill-switch record path under
// the user's home directory (design "Storage Layout": ~/.capiko/review-mode.json).
func globalModePath(home string) string {
	return filepath.Join(home, ".capiko", "review-mode.json")
}

// reviewModeSet persists mode as the clone-scope kill-switch record for the
// current workspace (spec rdd-review-cli; design "CLI Command Design").
// Enabling and disabling only ever touch the clone-scope record — never the
// global scope, and never any authority state or receipt (spec
// rdd-kill-switch "No fabrication, no destruction").
func reviewModeSet(mode rdd.ReviewMode, out io.Writer) (bool, int, error) {
	workspace, err := resolveWorkspace()
	if err != nil {
		return true, 1, fmt.Errorf("review: resolving workspace: %w", err)
	}

	commonDir, err := gitCommonDirFn(workspace)
	if err != nil {
		return true, 1, fmt.Errorf("review: resolving git common dir: %w", err)
	}

	clonePath := cloneModePath(commonDir)
	rec := &rdd.ModeRecord{
		SchemaVersion: 1,
		Mode:          mode,
		UpdatedAt:     reviewNow().UTC().Format(time.RFC3339),
	}
	if err := reviewstore.SaveModeRecord(clonePath, rec); err != nil {
		return true, 1, fmt.Errorf("review: saving clone mode record: %w", err)
	}

	fmt.Fprintf(out, "review mode set to %s (clone scope: %s)\n", mode, clonePath)
	return true, 0, nil
}

// reviewModeStatus prints the effective (precedence-resolved) kill-switch
// mode alongside the clone-scope and global-scope records and the git
// common directory (spec rdd-review-cli "Status reports effective mode";
// design "CLI Command Design": "prints: effective mode, clone-scope mode
// (or "not set"), global-scope mode (or "not set"), and the git common
// directory path"). A corrupt record at either scope is reported, not
// fatal — it resolves as absent, so the effective mode still fails closed
// to managed (spec rdd-kill-switch "Fail-closed default").
func reviewModeStatus(out io.Writer) (bool, int, error) {
	workspace, err := resolveWorkspace()
	if err != nil {
		return true, 1, fmt.Errorf("review: resolving workspace: %w", err)
	}

	commonDir, err := gitCommonDirFn(workspace)
	if err != nil {
		return true, 1, fmt.Errorf("review: resolving git common dir: %w", err)
	}

	home, err := userHomeDirFn()
	if err != nil {
		return true, 1, fmt.Errorf("review: resolving home dir: %w", err)
	}

	clonePath := cloneModePath(commonDir)
	cloneRec, cloneErr := reviewstore.LoadModeRecord(clonePath)
	if cloneErr != nil && !errors.Is(cloneErr, reviewstore.ErrCorruptMode) {
		return true, 1, fmt.Errorf("review: loading clone mode record: %w", cloneErr)
	}

	globalPath := globalModePath(home)
	globalRec, globalErr := reviewstore.LoadModeRecord(globalPath)
	if globalErr != nil && !errors.Is(globalErr, reviewstore.ErrCorruptMode) {
		return true, 1, fmt.Errorf("review: loading global mode record: %w", globalErr)
	}

	// Corrupt records resolve as absent at that scope — same fail-closed
	// treatment as a missing file (design "Corruption fails closed").
	effectiveClone := cloneRec
	if cloneErr != nil {
		effectiveClone = nil
	}
	effectiveGlobal := globalRec
	if globalErr != nil {
		effectiveGlobal = nil
	}
	effective := rdd.ResolveMode(effectiveGlobal, effectiveClone)

	fmt.Fprintf(out, "effective mode: %s\n", effective)
	fmt.Fprintf(out, "clone mode:     %s\n", formatModeRecord(cloneRec, cloneErr))
	fmt.Fprintf(out, "global mode:    %s\n", formatModeRecord(globalRec, globalErr))
	fmt.Fprintf(out, "git common dir: %s\n", commonDir)
	return true, 0, nil
}

// formatModeRecord renders a mode record (or its absence/corruption) as a
// short human-readable status word.
func formatModeRecord(rec *rdd.ModeRecord, err error) string {
	if err != nil {
		return fmt.Sprintf("corrupt (%v)", err)
	}
	if rec == nil {
		return "not set"
	}
	return string(rec.Mode)
}
