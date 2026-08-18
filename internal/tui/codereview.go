package tui

import (
	"fmt"
	"os"
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

// ggaMarkerStart/ggaMarkerEnd are the marker delimiters gga used for its
// managed AGENTS.md block. gga itself has been fully replaced by CGA; these
// constants exist only so cleanupGGA can remove a prior installation's
// remnants — capiko never writes this block anymore.
const (
	ggaMarkerStart = "<!-- capiko:review:start -->"
	ggaMarkerEnd   = "<!-- capiko:review:end -->"
)

// ggaHookSignature is checked case-insensitively against an existing
// .git/hooks/pre-commit file to detect a gga-owned hook. gga installs its
// hook directly via the external `gga install` command rather than through
// capiko's marker-delimited githooks.WriteBlock, so there is no marker to key
// off — the whole file is gga's when this signature is present.
const ggaHookSignature = "gga"

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
// renders and writes the CGA hook via cga.RenderHook + githooks.WriteBlock,
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
	script := cga.RenderHook(cga.Rules(persona), rec.StrictMode, rec.Timeout)

	if err := backupCGAHook(bkp, workspace); err != nil {
		return err
	}

	if err := githooks.WriteBlock(workspace, "pre-commit", cga.MarkerStart, cga.MarkerEnd, script); err != nil {
		return fmt.Errorf("writing pre-commit hook: %w", err)
	}

	rec.Workspace = workspace
	rec.Checksum = state.Checksum(script)

	if store != nil {
		return store.SetCGA(rec)
	}
	return nil
}

// disableCGA removes CGA's managed pre-commit hook block (backing it up
// first) and records Enabled:false so sync does not re-apply it.
func disableCGA(workspace string, store *state.Store, bkp *backup.Store, rec *state.CGARecord) error {
	if err := backupCGAHook(bkp, workspace); err != nil {
		return err
	}
	if err := githooks.RemoveBlock(workspace, "pre-commit", cga.MarkerStart, cga.MarkerEnd); err != nil {
		return fmt.Errorf("removing pre-commit hook block: %w", err)
	}
	rec.Workspace = workspace
	if store != nil {
		return store.SetCGA(rec)
	}
	return nil
}

// backupCGAHook snapshots the pre-commit hook file before a CGA mutation,
// when it already exists. Mirrors backupTeamSyncHooks.
func backupCGAHook(bkp *backup.Store, workspace string) error {
	if bkp == nil {
		return nil
	}
	hookPath := filepath.Join(workspace, ".git", "hooks", "pre-commit")
	if _, err := os.Stat(hookPath); err != nil {
		return nil
	}
	if _, err := bkp.CreateFiles("cga", Version, []string{hookPath}); err != nil {
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
// Configure code review screen
//
// NOTE: this screen still uses the codeReviewScreen/codeReview* names and its
// pre-CGA row layout (no Timeout control yet). It is repurposed here only
// enough to compile against applyCGA/state.CGARecord after the gga backend
// and internal/codereview package were removed. The full screen rewrite
// (rename to cgaScreen, add a Timeout row, update copy/labels) is tracked as
// a separate, later work unit.
// ============================================================================

// Row indices on the configure screen.
const (
	rowCodeReviewEnabled = iota
	rowCodeReviewStrict
	rowCodeReviewApply
	rowCodeReviewBack
	codeReviewRows
)

type codeReviewState int

const (
	codeReviewEditing codeReviewState = iota
	codeReviewApplying
	codeReviewDone
	codeReviewFailed
)

// codeReviewScreen configures Capiko Guardian Angel (CGA) for the current
// project: it toggles the integration and strict mode, then writes the
// pre-commit review hook.
type codeReviewScreen struct {
	svc     services
	enabled bool
	strict  bool
	cursor  int
	state   codeReviewState
	err     error
}

type codeReviewAppliedMsg struct{ err error }

func newCodeReview(svc services) screen {
	s := &codeReviewScreen{svc: svc, strict: true}
	if svc.state != nil {
		if st, err := svc.state.Load(); err == nil && st.CGA != nil {
			s.enabled = st.CGA.Enabled
			s.strict = st.CGA.StrictMode
		}
	}
	return s
}

func (s *codeReviewScreen) Update(msg tea.Msg) (screen, tea.Cmd) {
	switch msg := msg.(type) {
	case codeReviewAppliedMsg:
		if msg.err != nil {
			s.state, s.err = codeReviewFailed, msg.err
		} else {
			s.state = codeReviewDone
		}
		return s, nil
	case tea.KeyMsg:
		if s.state == codeReviewApplying {
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
			if s.cursor < codeReviewRows-1 {
				s.cursor++
			}
		case " ":
			s.toggle()
		case "enter":
			switch s.cursor {
			case rowCodeReviewApply:
				s.state = codeReviewApplying
				return s, s.applyCmd()
			case rowCodeReviewBack:
				return s, back
			default:
				s.toggle()
			}
		}
	}
	return s, nil
}

// toggle flips the boolean on the current row (enabled or strict mode).
func (s *codeReviewScreen) toggle() {
	switch s.cursor {
	case rowCodeReviewEnabled:
		s.enabled = !s.enabled
	case rowCodeReviewStrict:
		s.strict = !s.strict
	}
}

func (s *codeReviewScreen) applyCmd() tea.Cmd {
	svc := s.svc
	rec := &state.CGARecord{
		Enabled:    s.enabled,
		StrictMode: s.strict,
	}
	return func() tea.Msg {
		ws, err := cgaGetwd()
		if err != nil {
			return codeReviewAppliedMsg{err: err}
		}
		return codeReviewAppliedMsg{err: applyCGA(ws, svc.state, svc.backup, rec)}
	}
}

func (s *codeReviewScreen) View() string {
	var b strings.Builder
	b.WriteString(titleSty.Render("Configure code review") + "\n")
	b.WriteString(dimSty.Render("Wire Capiko Guardian Angel into this project: Copilot reviews every commit.") + "\n\n")

	switch s.state {
	case codeReviewApplying:
		b.WriteString("Applying code-review config…\n")
		return b.String()
	case codeReviewDone:
		b.WriteString(okSty.Render("Code review configured ✓") + "\n\n")
		b.WriteString(dimSty.Render("any key to go back") + "\n")
		return b.String()
	case codeReviewFailed:
		b.WriteString(errSty.Render("Error: "+s.err.Error()) + "\n\n")
		b.WriteString(dimSty.Render("any key to go back") + "\n")
		return b.String()
	}

	rows := []struct{ label, value string }{
		{"Enabled", onOff(s.enabled)},
		{"Strict mode", onOff(s.strict)},
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

	b.WriteString("\n" + dimSty.Render("↑/↓ move · space toggle · enter select · esc back") + "\n")
	return b.String()
}

// onOff renders a boolean as a styled on/off badge.
func onOff(v bool) string {
	if v {
		return okSty.Render("on")
	}
	return dimSty.Render("off")
}
