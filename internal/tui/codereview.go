package tui

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/martinhg/capiko-ai/internal/backup"
	"github.com/martinhg/capiko-ai/internal/cga"
	"github.com/martinhg/capiko-ai/internal/githooks"
	"github.com/martinhg/capiko-ai/internal/instructions"
	"github.com/martinhg/capiko-ai/internal/state"
)

// cgaGetwd resolves the working directory at apply/construct time for the CGA
// screen. Mirrors teamSyncGetwd.
var cgaGetwd = os.Getwd

const cgaLogRotationCap = 200

// gitRevParseGitDir resolves the git directory for workspace via
// `git -C workspace rev-parse --git-dir`, returning an absolute path.
//
// This is worktree-safe: inside a git worktree, `<workspace>/.git` is a
// *file* (not a directory) containing a `gitdir: ...` pointer to the real
// per-worktree git-dir elsewhere on disk, so a hardcoded
// filepath.Join(workspace, ".git", ...) resolves to a nonexistent path.
// Asking git itself for --git-dir always returns the correct location,
// worktree or not.
//
// Overridable as a package-level var so tests can stub it without requiring
// a real git repository (mirrors cgaGetwd).
var gitRevParseGitDir = func(workspace string) (string, error) {
	out, err := exec.Command("git", "-C", workspace, "rev-parse", "--git-dir").Output()
	if err != nil {
		return "", err
	}
	dir := strings.TrimSpace(string(out))
	if !filepath.IsAbs(dir) {
		dir = filepath.Join(workspace, dir)
	}
	return dir, nil
}

// cgaFindingsLogPath resolves the absolute path to the CGA findings-log
// JSONL file for workspace. Returns "" (findings-log persistence disabled)
// when workspace is not inside a git repository yet, rather than failing
// hook installation outright — findings-log persistence is a best-effort
// add-on to the review hook, never a precondition for it (see the
// Graceful Degradation spec requirement).
func cgaFindingsLogPath(workspace string) string {
	gitDir, err := gitRevParseGitDir(workspace)
	if err != nil {
		return ""
	}
	return filepath.Join(gitDir, "capiko", cga.FindingsLogName)
}

// ggaMarkerStart/ggaMarkerEnd are the marker delimiters gga used for its
// managed AGENTS.md block. gga itself has been fully replaced by CGA; these
// constants exist only so cleanupGGA can remove a prior installation's
// remnants — capiko never writes this block anymore.
const (
	ggaMarkerStart = "<!-- capiko:review:start -->"
	ggaMarkerEnd   = "<!-- capiko:review:end -->"
)

// ggaHookSignature is checked case-insensitively against an existing
// .git/hooks/pre-commit file to detect a gga-owned hook. "gga run" is
// specific to gga's actual hook content, avoiding false positives on
// unrelated words (e.g. "aggregation").
const ggaHookSignature = "gga run"

// cleanupGGA removes any Gentleman Guardian Angel (gga) remnants from
// workspace: the .gga config file, the capiko-managed AGENTS.md rules block,
// and gga's pre-commit hook (detected by signature, since gga owns the whole
// file rather than a marker-delimited block within it). It runs before CGA
// installs its own hook so a fresh CGA install never leaves gga wiring
// behind. Idempotent — a no-op when no remnants are present.
func cleanupGGA(workspace string) error {
	if err := removeGGAFile(workspace); err != nil {
		return err
	}
	if err := removeGGAAgentsBlock(workspace); err != nil {
		return err
	}
	return removeGGAPreCommitHook(workspace)
}

// removeGGAFile removes the .gga config file, if present.
func removeGGAFile(workspace string) error {
	if err := os.Remove(filepath.Join(workspace, ".gga")); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("removing .gga: %w", err)
	}
	return nil
}

// removeGGAAgentsBlock removes the capiko:review marker block from AGENTS.md,
// preserving any user-authored content outside the markers.
func removeGGAAgentsBlock(workspace string) error {
	rulesPath := filepath.Join(workspace, "AGENTS.md")
	content, changed, err := instructions.Render(rulesPath, ggaMarkerStart, ggaMarkerEnd, "")
	if err != nil {
		return err
	}
	if !changed {
		return nil
	}
	return instructions.Write(rulesPath, content)
}

// removeGGAPreCommitHook deletes .git/hooks/pre-commit when it looks
// gga-owned (see ggaHookSignature). A missing hook, or one without the
// signature — a user's own hook, or one CGA already rewrote — is left
// untouched.
func removeGGAPreCommitHook(workspace string) error {
	hookPath := filepath.Join(workspace, ".git", "hooks", "pre-commit")
	raw, err := os.ReadFile(hookPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("reading pre-commit hook: %w", err)
	}
	if !strings.Contains(strings.ToLower(string(raw)), ggaHookSignature) {
		return nil
	}
	if err := os.Remove(hookPath); err != nil {
		return fmt.Errorf("removing gga pre-commit hook: %w", err)
	}
	return nil
}

// applyCGA installs (or re-applies) the Capiko Guardian Angel pre-commit
// review hook into workspace: it first cleans up any gga remnants (gga is
// replaced entirely, with no automatic migration — see cleanupGGA), then
// renders and writes the CGA hook via cga.RenderPreCommitHook + githooks.WriteBlock,
// and records the result in state. Disabling removes the hook block and
// records it off, so sync does not re-apply. Shared by the configure screen
// and the post-sync re-apply.
func applyCGA(workspace string, store *state.Store, bkp *backup.Store, rec *state.CGARecord) error {
	if rec == nil {
		return nil
	}
	if !rec.Enabled {
		return disableCGA(workspace, store, bkp, rec)
	}

	if err := cleanupGGA(workspace); err != nil {
		return err
	}

	persona := activePersona(store)
	logPath := cgaFindingsLogPath(workspace)
	script := cga.RenderPreCommitHook(cga.Rules(persona), rec.StrictMode, rec.Timeout, logPath, cgaLogRotationCap)

	if err := backupCGAHook(bkp, workspace); err != nil {
		return err
	}

	if err := githooks.WriteBlock(workspace, "pre-commit", cga.MarkerStart, cga.MarkerEnd, script); err != nil {
		return fmt.Errorf("writing pre-commit hook: %w", err)
	}

	postScript := cga.RenderPostCommitHook(logPath)
	if err := githooks.WriteBlock(workspace, "post-commit", cga.PostCommitMarkerStart, cga.PostCommitMarkerEnd, postScript); err != nil {
		return fmt.Errorf("writing post-commit hook: %w", err)
	}

	rec.Workspace = workspace
	rec.Checksum = state.Checksum(script)

	if store != nil {
		return store.SetCGA(rec)
	}
	return nil
}

// disableCGA removes CGA's managed pre-commit and post-commit hook blocks
// (backing both up first) and records Enabled:false so sync does not
// re-apply them.
func disableCGA(workspace string, store *state.Store, bkp *backup.Store, rec *state.CGARecord) error {
	if err := backupCGAHook(bkp, workspace); err != nil {
		return err
	}
	if err := githooks.RemoveBlock(workspace, "pre-commit", cga.MarkerStart, cga.MarkerEnd); err != nil {
		return fmt.Errorf("removing pre-commit hook block: %w", err)
	}
	if err := githooks.RemoveBlock(workspace, "post-commit", cga.PostCommitMarkerStart, cga.PostCommitMarkerEnd); err != nil {
		return fmt.Errorf("removing post-commit hook block: %w", err)
	}
	rec.Workspace = workspace
	if store != nil {
		return store.SetCGA(rec)
	}
	return nil
}

// backupCGAHook snapshots the pre-commit and post-commit hook files before a
// CGA mutation, for whichever of the two already exist. Mirrors
// backupTeamSyncHooks.
func backupCGAHook(bkp *backup.Store, workspace string) error {
	if bkp == nil {
		return nil
	}
	var paths []string
	for _, name := range []string{"pre-commit", "post-commit"} {
		hookPath := filepath.Join(workspace, ".git", "hooks", name)
		if _, err := os.Stat(hookPath); err == nil {
			paths = append(paths, hookPath)
		}
	}
	if len(paths) == 0 {
		return nil
	}
	if _, err := bkp.CreateFiles("cga", Version, paths); err != nil {
		return fmt.Errorf("backup failed, aborting: %w", err)
	}
	return nil
}

// activePersona returns the recorded persona id, or "" when unmanaged/unreadable.
func activePersona(store *state.Store) string {
	if store == nil {
		return ""
	}
	if st, err := store.Load(); err == nil {
		return st.Persona
	}
	return ""
}

// ============================================================================
// Configure CGA screen
// ============================================================================

// Row indices on the configure screen.
const (
	rowCGAEnabled = iota
	rowCGAStrict
	rowCGATimeout
	rowCGAApply
	rowCGABack
	cgaRows
)

// Timeout bounds and step for the Timeout row, in seconds. Mirrors the
// default baked into cga.RenderPreCommitHook when a record carries Timeout: 0.
const (
	cgaTimeoutDefault = 120
	cgaTimeoutStep    = 30
	cgaTimeoutMin     = 30
	cgaTimeoutMax     = 600
)

// adjustTimeout adds delta to timeout, clamped to [cgaTimeoutMin, cgaTimeoutMax].
func adjustTimeout(timeout, delta int) int {
	next := timeout + delta
	if next < cgaTimeoutMin {
		return cgaTimeoutMin
	}
	if next > cgaTimeoutMax {
		return cgaTimeoutMax
	}
	return next
}

type cgaScreenState int

const (
	cgaEditing cgaScreenState = iota
	cgaApplying
	cgaDone
	cgaFailed
)

// cgaScreen configures Capiko Guardian Angel (CGA) for the current project:
// it toggles the integration, strict mode, and review timeout, then writes
// the pre-commit review hook.
type cgaScreen struct {
	svc     services
	enabled bool
	strict  bool
	timeout int
	cursor  int
	state   cgaScreenState
	err     error
}

type cgaAppliedMsg struct{ err error }

func newCGA(svc services) screen {
	s := &cgaScreen{svc: svc, strict: true, timeout: cgaTimeoutDefault}
	if svc.state != nil {
		if st, err := svc.state.Load(); err == nil && st.CGA != nil {
			s.enabled = st.CGA.Enabled
			s.strict = st.CGA.StrictMode
			if st.CGA.Timeout > 0 {
				s.timeout = st.CGA.Timeout
			}
		}
	}
	return s
}

func (s *cgaScreen) Update(msg tea.Msg) (screen, tea.Cmd) {
	switch msg := msg.(type) {
	case cgaAppliedMsg:
		if msg.err != nil {
			s.state, s.err = cgaFailed, msg.err
		} else {
			s.state = cgaDone
		}
		return s, nil
	case tea.KeyMsg:
		if s.state == cgaApplying {
			return s, nil
		}
		switch msg.String() {
		case "q", "esc":
			return s, back
		case "up", "k":
			if s.cursor > 0 {
				s.cursor--
			}
		case "down", "j":
			if s.cursor < cgaRows-1 {
				s.cursor++
			}
		case " ":
			s.toggle()
		case "left", "h":
			if s.cursor == rowCGATimeout {
				s.timeout = adjustTimeout(s.timeout, -cgaTimeoutStep)
			}
		case "right", "l":
			if s.cursor == rowCGATimeout {
				s.timeout = adjustTimeout(s.timeout, cgaTimeoutStep)
			}
		case "enter":
			switch s.cursor {
			case rowCGAApply:
				s.state = cgaApplying
				return s, s.applyCmd()
			case rowCGABack:
				return s, back
			default:
				s.toggle()
			}
		}
	}
	return s, nil
}

// toggle flips the boolean on the current row (enabled or strict mode). The
// Timeout row is adjusted via left/right instead — see Update.
func (s *cgaScreen) toggle() {
	switch s.cursor {
	case rowCGAEnabled:
		s.enabled = !s.enabled
	case rowCGAStrict:
		s.strict = !s.strict
	}
}

func (s *cgaScreen) applyCmd() tea.Cmd {
	svc := s.svc
	rec := &state.CGARecord{
		Enabled:    s.enabled,
		StrictMode: s.strict,
		Timeout:    s.timeout,
	}
	return func() tea.Msg {
		ws, err := cgaGetwd()
		if err != nil {
			return cgaAppliedMsg{err: err}
		}
		return cgaAppliedMsg{err: applyCGA(ws, svc.state, svc.backup, rec)}
	}
}

func (s *cgaScreen) View() string {
	var b strings.Builder
	b.WriteString(titleSty.Render("Configure Capiko Guardian Angel (CGA)") + "\n")
	b.WriteString(dimSty.Render("Wire Capiko Guardian Angel into this project: Copilot reviews every commit.") + "\n\n")

	switch s.state {
	case cgaApplying:
		b.WriteString("Applying CGA config…\n")
		return b.String()
	case cgaDone:
		b.WriteString(okSty.Render("CGA configured ✓") + "\n\n")
		b.WriteString(dimSty.Render("any key to go back") + "\n")
		return b.String()
	case cgaFailed:
		b.WriteString(errSty.Render("Error: "+s.err.Error()) + "\n\n")
		b.WriteString(dimSty.Render("any key to go back") + "\n")
		return b.String()
	}

	rows := []struct{ label, value string }{
		{"Enabled", onOff(s.enabled)},
		{"Strict mode", onOff(s.strict)},
		{"Timeout", fmt.Sprintf("%ds", s.timeout)},
		{"Apply", ""},
		{"Back", ""},
	}
	for i, r := range rows {
		label := pad(r.label, 14)
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

	b.WriteString("\n" + dimSty.Render("↑/↓ move · space toggle · ←/→ timeout · enter select · esc back") + "\n")
	return b.String()
}

// onOff renders a boolean as a styled on/off badge.
func onOff(v bool) string {
	if v {
		return okSty.Render("on")
	}
	return dimSty.Render("off")
}
