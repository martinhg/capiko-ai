// Package tui — teamsync.go: pure helpers, apply/disable logic, and the
// Configure team sync TUI screen for the Engram team-sync feature.
package tui

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

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

// Package-level seams for the team-sync screen. Replaced in tests to avoid
// real filesystem access, exec calls, or home-directory reads.
var (
	// teamSyncGetwd resolves the working directory at apply/construct time.
	// Mirrors codeReviewGetwd in codereview.go.
	teamSyncGetwd = os.Getwd

	// engramDetected reports whether the engram binary is on PATH. Cached on
	// the screen at construction so View() is deterministic in tests and CI.
	engramDetected = func() bool { _, err := exec.LookPath("engram"); return err == nil }

	// teamSyncDetectConflict is the conflict-detection seam used by
	// applyTeamSync and newTeamSync. Tests replace it to control whether a
	// conflict is reported without touching the real filesystem.
	teamSyncDetectConflict = detectHookConflict
)

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

// ============================================================================
// Configure team sync screen (WU-6)
// ============================================================================

// teamSyncState is the phase of the team-sync configuration screen.
type teamSyncState int

const (
	teamSyncEditing  teamSyncState = iota // user is configuring the toggle/ack
	teamSyncApplying                      // apply command is running (goroutine)
	teamSyncDone                          // apply succeeded
	teamSyncFailed                        // apply returned an error
)

// Row indices for the team-sync configure screen.
const (
	rowTeamSyncEnabled = iota // Enabled toggle
	rowTeamSyncAck            // scope-leak acknowledgment
	rowTeamSyncApply          // Apply action
	rowTeamSyncBack           // Back to main menu
	teamSyncRows              // sentinel — count of rows
)

// teamSyncScreen configures capiko's managed git-hook team-sync wiring.
// A single Enabled toggle writes (or removes) both the post-merge and pre-push
// hook blocks. The scope-leak acknowledgment must be set before Apply is
// allowed when Enabled is true — disabling never requires ack (REQ-5.4).
type teamSyncScreen struct {
	svc             services
	state           teamSyncState
	cursor          int
	enabled         bool
	ack             bool
	conflictReason  string // non-empty: a competing hook framework was detected
	conflictProject string // project name resolved for the conflict-banner commands
	engramAvailable bool   // cached from engramDetected() at construction (A-D1)
	ackHint         bool   // true: show "acknowledge required" inline hint
	err             error
}

// teamSyncAppliedMsg is the message emitted by applyCmd when the goroutine
// completes. A nil err means success; a non-nil err means failure.
type teamSyncAppliedMsg struct{ err error }

// newTeamSync constructs the Configure team sync screen. It pre-detects
// the engram binary and hook framework conflicts at construction time so
// View() is fully deterministic — no filesystem calls at render time.
// Mirrors newCodeReview and newHeadroom.
func newTeamSync(svc services) screen {
	ws, _ := teamSyncGetwd()
	conflictReason := teamSyncDetectConflict(ws)
	var conflictProject string
	if conflictReason != "" {
		conflictProject = resolveProject(ws)
	}
	s := &teamSyncScreen{
		svc:             svc,
		conflictReason:  conflictReason,
		conflictProject: conflictProject,
		engramAvailable: engramDetected(),
	}
	if svc.state != nil {
		if st, err := svc.state.Load(); err == nil && st.TeamSync != nil {
			s.enabled = st.TeamSync.Enabled
			// ack always resets to false — each enable requires fresh confirmation
			// (REQ-5.5).
		}
	}
	return s
}

// Update handles key messages and internal messages for the team-sync screen.
func (s *teamSyncScreen) Update(msg tea.Msg) (screen, tea.Cmd) {
	switch msg := msg.(type) {
	case teamSyncAppliedMsg:
		if msg.err != nil {
			s.state, s.err = teamSyncFailed, msg.err
		} else {
			s.state = teamSyncDone
		}
		return s, nil

	case tea.KeyMsg:
		// While applying, ignore all keys.
		if s.state == teamSyncApplying {
			return s, nil
		}
		// Any key in a terminal state returns to the menu.
		if s.state == teamSyncDone || s.state == teamSyncFailed {
			return s, back
		}
		switch msg.String() {
		case "q", "esc":
			return s, back
		case "up", "k":
			if s.cursor > 0 {
				s.cursor--
			}
		case "down", "j":
			if s.cursor < teamSyncRows-1 {
				s.cursor++
			}
		case " ":
			s.toggle()
		case "enter":
			switch s.cursor {
			case rowTeamSyncApply:
				// Ack gate: block apply if user wants to enable but hasn't ack'd.
				if s.enabled && !s.ack {
					s.ackHint = true
					return s, nil
				}
				s.state = teamSyncApplying
				return s, s.applyCmd()
			case rowTeamSyncBack:
				return s, back
			default:
				s.toggle()
			}
		}
	}
	return s, nil
}

// toggle flips the boolean on the current row.
func (s *teamSyncScreen) toggle() {
	switch s.cursor {
	case rowTeamSyncEnabled:
		s.enabled = !s.enabled
		s.ackHint = false // clear the hint when the user changes the toggle
	case rowTeamSyncAck:
		s.ack = !s.ack
		if s.ack {
			s.ackHint = false // clear the hint once ack is given
		}
	}
}

// applyCmd returns a tea.Cmd that runs apply/disable in a goroutine and emits
// teamSyncAppliedMsg when done. Mirrors codeReviewScreen.applyCmd.
func (s *teamSyncScreen) applyCmd() tea.Cmd {
	svc := s.svc
	enabled := s.enabled
	return func() tea.Msg {
		ws, err := teamSyncGetwd()
		if err != nil {
			return teamSyncAppliedMsg{err: err}
		}
		if enabled {
			rec := &state.TeamSyncRecord{Enabled: true, Workspace: ws}
			err = applyTeamSync(ws, svc.state, svc.backup, rec)
		} else {
			err = disableTeamSync(ws, svc.state, svc.backup)
		}
		return teamSyncAppliedMsg{err: err}
	}
}

// View renders the team-sync configuration screen.
func (s *teamSyncScreen) View() string {
	var b strings.Builder
	b.WriteString(titleSty.Render("Configure team sync") + "\n\n")

	switch s.state {
	case teamSyncApplying:
		b.WriteString("Applying team sync configuration…\n")
		return b.String()

	case teamSyncDone:
		if s.enabled {
			b.WriteString(okSty.Render("Team sync enabled ✓") + "\n\n")
			if s.conflictReason != "" {
				b.WriteString(warnSty.Render("Hook framework conflict — hooks were NOT written.") + "\n")
				b.WriteString(dimSty.Render(s.conflictReason) + "\n\n")
				b.WriteString(dimSty.Render("Run these manually in your hook framework:") + "\n")
				b.WriteString(dimSty.Render("  post-merge: "+renderPostMerge()) + "\n")
				b.WriteString(dimSty.Render("  pre-push:   "+renderPrePush(s.conflictProject)) + "\n\n")
			}
		} else {
			b.WriteString(okSty.Render("Team sync disabled ✓") + "\n\n")
		}
		b.WriteString(dimSty.Render("any key to go back") + "\n")
		return b.String()

	case teamSyncFailed:
		b.WriteString(errSty.Render("Error: "+s.err.Error()) + "\n\n")
		b.WriteString(dimSty.Render("any key to go back") + "\n")
		return b.String()
	}

	// Editing state — description and scope-leak warning.
	b.WriteString(dimSty.Render("Wire engram memory sharing via git hooks.") + "\n")
	b.WriteString(dimSty.Render("post-merge: import team memories after pull/merge.") + "\n")
	b.WriteString(dimSty.Render("pre-push:   export your memories and remind to commit .engram/.") + "\n\n")

	b.WriteString(warnSty.Render("! Scope-leak warning: engram sync has no scope filter.") + "\n")
	b.WriteString(dimSty.Render("  scope:personal memories for this project WILL be committed to git.") + "\n")
	b.WriteString(dimSty.Render("  Mitigations:") + "\n")
	b.WriteString(dimSty.Render("    (a) Wrap sensitive content in <private>…</private> tags.") + "\n")
	b.WriteString(dimSty.Render("    (b) Use a separate project name for personal memories.") + "\n\n")

	// Conflict banner — visible whenever a competing framework is detected.
	if s.conflictReason != "" {
		b.WriteString(warnSty.Render("! Hook framework conflict: "+s.conflictReason) + "\n")
		b.WriteString(dimSty.Render("  Hooks cannot be written automatically. Run these in your framework:") + "\n")
		b.WriteString(dimSty.Render("    post-merge: "+renderPostMerge()) + "\n")
		b.WriteString(dimSty.Render("    pre-push:   "+renderPrePush(s.conflictProject)) + "\n\n")
	}

	// Row definitions.
	rows := []struct{ label, value string }{
		{"Enabled", onOff(s.enabled)},
		{"I understand the scope-leak risk", onOff(s.ack)},
		{"Apply", ""},
		{"Back", ""},
	}
	for i, r := range rows {
		label := pad(r.label, 34)
		if i == s.cursor {
			b.WriteString(titleSty.Render(menuCursor) + titleSty.Render(label))
		} else {
			b.WriteString("  " + textSty.Render(label))
		}
		if r.value != "" {
			b.WriteString("  " + dimSty.Render(r.value))
		}
		b.WriteString("\n")
	}

	// Ack-required inline hint — shown when the user pressed Apply without ack.
	if s.ackHint {
		b.WriteString("\n" + warnSty.Render("  Acknowledge the scope-leak warning before enabling.") + "\n")
	}

	// Engram install hint — shown when engram is absent from PATH (A-D1).
	// capiko configures; it never installs. The hint is informational only.
	if !s.engramAvailable {
		b.WriteString("\n" + warnSty.Render("! engram is not on PATH.") + "\n")
		b.WriteString(dimSty.Render("  The git hooks need engram to run. capiko never installs it for you.") + "\n")
		b.WriteString(dimSty.Render("  Install: https://github.com/Gentleman-Programming/engram") + "\n")
	}

	b.WriteString("\n" + dimSty.Render("↑/↓ move · space toggle · enter select · esc back") + "\n")
	return b.String()
}
