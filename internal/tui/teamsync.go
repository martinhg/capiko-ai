// Package tui — teamsync.go: pure helpers for the Engram team-sync feature.
// No Bubbletea screen, no state writes, no shell invocations live here.
// All policy and UX (screen, apply/disable) will be added in a later work unit.
package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/martinhg/capiko-ai/internal/engram"
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
