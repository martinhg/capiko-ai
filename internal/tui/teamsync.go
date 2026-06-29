// Package tui — teamsync.go: apply/disable logic and pure helpers for the
// Engram team-sync feature. The TUI screen (teamSyncScreen) will be added in a
// later work unit; only the state-writing apply/disable layer lives here now.
package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/martinhg/capiko-ai/internal/backup"
	"github.com/martinhg/capiko-ai/internal/engram"
	"github.com/martinhg/capiko-ai/internal/githooks"
	"github.com/martinhg/capiko-ai/internal/state"
)

// ---------------------------------------------------------------------------
// Marker constants for the managed hook blocks.
// ---------------------------------------------------------------------------

const (
	teamSyncPostMergeMarkerStart = "# >>> capiko:team-sync:post-merge >>>"
	teamSyncPostMergeMarkerEnd   = "# <<< capiko:team-sync:post-merge <<<"
	teamSyncPrePushMarkerStart   = "# >>> capiko:team-sync:pre-push >>>"
	teamSyncPrePushMarkerEnd     = "# <<< capiko:team-sync:pre-push <<<"
)

// ---------------------------------------------------------------------------
// detectHookConflict — WU-3
// ---------------------------------------------------------------------------

// hooksPathRe matches a hooksPath key in .git/config and captures the value.
// The key is unique to [core] so a key-anchored regex is sufficient (no section
// parsing needed). Mirrors projectFromGitConfig in internal/sddstatus/engram.go.
var hooksPathRe = regexp.MustCompile(`(?mi)^\s*hooksPath\s*=\s*(\S.*?)\s*$`)

// detectHookConflict inspects workspace for signs of a competing hook framework.
// It reads .git/config for a non-default core.hooksPath and checks for framework
// signal files at the workspace root. Returns the first conflict reason found, or
// "" when the workspace is safe to manage. No git binary is invoked.
func detectHookConflict(workspace string) string {
	// 1. Check .git/config for a non-default hooksPath.
	gitConfigPath := filepath.Join(workspace, ".git", "config")
	if raw, err := os.ReadFile(gitConfigPath); err == nil {
		if m := hooksPathRe.FindSubmatch(raw); m != nil {
			val := strings.TrimSpace(string(m[1]))
			if val != ".git/hooks" {
				return fmt.Sprintf("core.hooksPath is set to %q in .git/config", val)
			}
		}
	}

	// 2. Framework signal files at the workspace root.
	signals := []struct {
		path   string
		reason string
	}{
		{".husky", "husky is configured (.husky/ directory found)"},
		{"lefthook.yml", "lefthook is configured (lefthook.yml found)"},
		{".lefthook.yml", "lefthook is configured (.lefthook.yml found)"},
		{".pre-commit-config.yaml", "pre-commit is configured (.pre-commit-config.yaml found)"},
	}
	for _, s := range signals {
		if _, err := os.Stat(filepath.Join(workspace, s.path)); err == nil {
			return s.reason
		}
	}

	return ""
}

// ---------------------------------------------------------------------------
// resolveProject — WU-3 helper (feeds hook-body builders)
// ---------------------------------------------------------------------------

// resolveProject returns the project name for the workspace. It delegates to
// engram.ReadProjectName (reads .engram/config.json) and falls back to the
// directory basename, mirroring applyEngramConfig in internal/tui/engram.go.
func resolveProject(workspace string) string {
	return engram.ReadProjectName(workspace)
}

// ---------------------------------------------------------------------------
// shSingleQuote — WU-4
// ---------------------------------------------------------------------------

// shSingleQuote wraps s in POSIX single quotes, escaping any literal single
// quotes inside s as '\”. This neutralises all shell metacharacters, including
// $, `, \\, and ; — closing the injection vector for project names embedded in
// hook scripts.
//
// Example: shSingleQuote("it's") → 'it'\”s'
func shSingleQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

// ---------------------------------------------------------------------------
// Hook-body builders — WU-4
// ---------------------------------------------------------------------------

// renderPostMerge returns the shell snippet for the post-merge hook.
// It imports engram memories after a merge. The || true guard ensures the hook
// never blocks a merge even when engram is absent or fails.
func renderPostMerge() string {
	return "engram sync --import || true"
}

// renderPrePush returns the shell snippet for the pre-push hook for the given
// project name. It exports engram memories before a push and echoes a reminder
// to commit .engram/ so the next team member receives the memories. The
// || true guard ensures the hook never blocks a push.
func renderPrePush(name string) string {
	return fmt.Sprintf(
		"engram sync --project %s || true\necho 'Remember to commit .engram/ so teammates receive your memories.'",
		shSingleQuote(name),
	)
}

// ---------------------------------------------------------------------------
// Seam for conflict detection (swapped in tests).
// ---------------------------------------------------------------------------

// teamSyncDetectConflict is the conflict-detection seam used by applyTeamSync.
// Tests replace it to control whether a conflict is reported without touching
// the real filesystem.
var teamSyncDetectConflict = detectHookConflict

// ---------------------------------------------------------------------------
// applyTeamSync / disableTeamSync — WU-5
// ---------------------------------------------------------------------------

// applyTeamSync writes capiko's managed hook blocks (post-merge and pre-push)
// into the workspace. It backs up any existing hook files first (snapshot-
// before-mutate), resolves the project name, and records the result in state.
//
// Conflict behaviour (warn-and-continue): when a competing hook framework is
// detected, capiko records the conflict reason in state, skips all writes, and
// returns nil — it never refuses to run, never blocks the user's workflow.
//
// Engram-absent-from-PATH: not detected here; the TUI screen renders the hint
// via engramDetected seam. The hooks themselves use `|| true` so they never
// block git operations even when engram is missing.
func applyTeamSync(workspace string, store *state.Store, bkp *backup.Store, rec *state.TeamSyncRecord) error {
	if rec == nil {
		return nil
	}

	// Resolve the project name at apply time and embed it in both the hook
	// body and the persisted record so future drift/doctor checks can verify.
	// Resolved BEFORE the conflict check so the manual-command guidance can
	// render `engram sync --project <name>` even when hooks are skipped.
	rec.Project = resolveProject(workspace)

	// Conflict detection: warn-and-continue, never refuse.
	if conflict := teamSyncDetectConflict(workspace); conflict != "" {
		rec.Conflict = conflict
		if store != nil {
			return store.SetTeamSync(rec)
		}
		return nil
	}

	// Snapshot existing hook files before any mutation (snapshot-before-mutate).
	if err := backupTeamSyncHooks(bkp, workspace); err != nil {
		return err
	}

	// Write the post-merge hook block (import).
	if err := githooks.WriteBlock(
		workspace, "post-merge",
		teamSyncPostMergeMarkerStart, teamSyncPostMergeMarkerEnd,
		renderPostMerge(),
	); err != nil {
		return fmt.Errorf("writing post-merge hook: %w", err)
	}

	// Write the pre-push hook block (export + reminder).
	if err := githooks.WriteBlock(
		workspace, "pre-push",
		teamSyncPrePushMarkerStart, teamSyncPrePushMarkerEnd,
		renderPrePush(rec.Project),
	); err != nil {
		return fmt.Errorf("writing pre-push hook: %w", err)
	}

	if store != nil {
		return store.SetTeamSync(rec)
	}
	return nil
}

// disableTeamSync removes capiko's managed hook blocks from the workspace,
// backs up the hook files first, and records Enabled:false in state so sync
// does not re-apply. Mirrors disableCodeReview.
func disableTeamSync(workspace string, store *state.Store, bkp *backup.Store) error {
	// Snapshot existing hook files before removal.
	if err := backupTeamSyncHooks(bkp, workspace); err != nil {
		return err
	}

	if err := githooks.RemoveBlock(
		workspace, "post-merge",
		teamSyncPostMergeMarkerStart, teamSyncPostMergeMarkerEnd,
	); err != nil {
		return fmt.Errorf("removing post-merge hook block: %w", err)
	}
	if err := githooks.RemoveBlock(
		workspace, "pre-push",
		teamSyncPrePushMarkerStart, teamSyncPrePushMarkerEnd,
	); err != nil {
		return fmt.Errorf("removing pre-push hook block: %w", err)
	}

	if store != nil {
		return store.SetTeamSync(&state.TeamSyncRecord{Enabled: false, Workspace: workspace})
	}
	return nil
}

// backupTeamSyncHooks snapshots whichever of the two managed hook files
// currently exist, before a team-sync mutation. A first write has nothing to
// back up and is silently skipped. Mirrors backupCodeReviewFiles.
func backupTeamSyncHooks(bkp *backup.Store, workspace string) error {
	if bkp == nil {
		return nil
	}
	candidates := []string{
		filepath.Join(workspace, ".git", "hooks", "post-merge"),
		filepath.Join(workspace, ".git", "hooks", "pre-push"),
	}
	var existing []string
	for _, p := range candidates {
		if _, err := os.Stat(p); err == nil {
			existing = append(existing, p)
		}
	}
	if len(existing) == 0 {
		return nil
	}
	if _, err := bkp.CreateFiles("team-sync", Version, existing); err != nil {
		return fmt.Errorf("backup failed, aborting: %w", err)
	}
	return nil
}
