package tui

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/martinhg/capiko-ai/internal/backup"
	"github.com/martinhg/capiko-ai/internal/codereview"
	"github.com/martinhg/capiko-ai/internal/instructions"
	"github.com/martinhg/capiko-ai/internal/state"
)

// gga seams, swapped in tests so the flow never shells out to a real gga binary or
// touches a real git repo. capiko configures gga; it never installs the binary.
var (
	ggaInstallHook   = func(workspace string) error { return runGGA(workspace, "install") }
	ggaUninstallHook = func(workspace string) error { return runGGA(workspace, "uninstall") }
	ggaDetected      = func() bool { _, err := exec.LookPath("gga"); return err == nil }
	codeReviewGetwd  = os.Getwd
)

// runGGA invokes the gga CLI in the given workspace.
func runGGA(workspace string, args ...string) error {
	cmd := exec.Command("gga", args...)
	cmd.Dir = workspace
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("gga %s: %w: %s", strings.Join(args, " "), err, strings.TrimSpace(string(out)))
	}
	return nil
}

// applyCodeReview writes capiko's managed gga configuration into the workspace:
// the .gga config, the curated AGENTS.md rules block (preserving any user-authored
// rules in the same file), and the git hook — then records the choice in state.
// Disabling removes capiko's managed block and the hook and records it off, so sync
// does not re-apply. Shared by the configure screen and the post-sync re-apply.
func applyCodeReview(workspace string, store *state.Store, bkp *backup.Store, rec *state.CodeReviewRecord) error {
	if rec == nil {
		return nil
	}
	rulesPath := filepath.Join(workspace, ggaRulesFile(rec))
	ggaPath := filepath.Join(workspace, ".gga")

	if !rec.Enabled {
		return disableCodeReview(workspace, store, bkp, rec, rulesPath)
	}

	// Render the managed AGENTS.md block for the active persona, injecting it into
	// any existing rules file so user-authored content survives.
	persona := activePersona(store)
	content, changed, err := instructions.Render(rulesPath, codereview.MarkerStart, codereview.MarkerEnd, codereview.Rules(persona))
	if err != nil {
		return err
	}

	if err := backupCodeReviewFiles(bkp, rulesPath, ggaPath); err != nil {
		return err
	}

	if changed {
		if err := instructions.Write(rulesPath, content); err != nil {
			return err
		}
	}
	cfg := codereview.RenderConfig(codereviewConfig(rec))
	if err := os.WriteFile(ggaPath, []byte(cfg), 0o644); err != nil {
		return fmt.Errorf("writing .gga: %w", err)
	}
	if err := ggaInstallHook(workspace); err != nil {
		return err
	}
	if store != nil {
		return store.SetCodeReview(rec)
	}
	return nil
}

// disableCodeReview removes capiko's managed AGENTS.md block and the git hook
// (backing the rules file up first), then records the disabled state so sync does
// not re-apply. The .gga file is left in place — it is the user's config now.
func disableCodeReview(workspace string, store *state.Store, bkp *backup.Store, rec *state.CodeReviewRecord, rulesPath string) error {
	content, changed, err := instructions.Render(rulesPath, codereview.MarkerStart, codereview.MarkerEnd, "")
	if err != nil {
		return err
	}
	if changed {
		if err := backupCodeReviewFiles(bkp, rulesPath); err != nil {
			return err
		}
		if err := instructions.Write(rulesPath, content); err != nil {
			return err
		}
	}
	if err := ggaUninstallHook(workspace); err != nil {
		return err
	}
	if store != nil {
		return store.SetCodeReview(rec)
	}
	return nil
}

// backupCodeReviewFiles snapshots the given paths that already exist, before a
// code-review mutation. A first write has nothing to back up.
func backupCodeReviewFiles(bkp *backup.Store, paths ...string) error {
	if bkp == nil {
		return nil
	}
	var existing []string
	for _, p := range paths {
		if _, err := os.Stat(p); err == nil {
			existing = append(existing, p)
		}
	}
	if len(existing) == 0 {
		return nil
	}
	if _, err := bkp.CreateFiles("code-review", Version, existing); err != nil {
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

// ggaRulesFile is the rules file gga reads, defaulting to AGENTS.md.
func ggaRulesFile(rec *state.CodeReviewRecord) string {
	if rec.RulesFile != "" {
		return rec.RulesFile
	}
	return "AGENTS.md"
}

// ============================================================================
// Configure code review screen
// ============================================================================

// codeReviewProviders are the gga providers offered on the configure screen.
var codeReviewProviders = []string{"claude", "gemini", "codex", "opencode", "ollama:llama3.2", "lmstudio", "github:gpt-4o"}

// Row indices on the configure screen.
const (
	rowCodeReviewEnabled = iota
	rowCodeReviewProvider
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

// codeReviewScreen configures Gentleman Guardian Angel (gga) for the current
// project: it toggles the integration, picks the review provider and strict mode,
// then writes the .gga config, the curated AGENTS.md block, and the git hook.
type codeReviewScreen struct {
	svc          services
	enabled      bool
	providerIdx  int
	strict       bool
	cursor       int
	state        codeReviewState
	ggaAvailable bool
	err          error
}

type codeReviewAppliedMsg struct{ err error }

func newCodeReview(svc services) screen {
	s := &codeReviewScreen{svc: svc, strict: true, ggaAvailable: ggaDetected()}
	if svc.state != nil {
		if st, err := svc.state.Load(); err == nil && st.CodeReview != nil {
			s.enabled = st.CodeReview.Enabled
			s.strict = st.CodeReview.StrictMode
			s.providerIdx = providerIndex(st.CodeReview.Provider)
		}
	}
	return s
}

// providerIndex returns the offered-providers index for name, or 0 when unknown.
func providerIndex(name string) int {
	for i, p := range codeReviewProviders {
		if p == name {
			return i
		}
	}
	return 0
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
		case "left", "h":
			if s.cursor == rowCodeReviewProvider {
				s.providerIdx = (s.providerIdx - 1 + len(codeReviewProviders)) % len(codeReviewProviders)
			}
		case "right", "l":
			if s.cursor == rowCodeReviewProvider {
				s.providerIdx = (s.providerIdx + 1) % len(codeReviewProviders)
			}
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
	rec := &state.CodeReviewRecord{
		Enabled:    s.enabled,
		Provider:   codeReviewProviders[s.providerIdx],
		StrictMode: s.strict,
	}
	return func() tea.Msg {
		ws, err := codeReviewGetwd()
		if err != nil {
			return codeReviewAppliedMsg{err: err}
		}
		return codeReviewAppliedMsg{err: applyCodeReview(ws, svc.state, svc.backup, rec)}
	}
}

func (s *codeReviewScreen) View() string {
	var b strings.Builder
	b.WriteString(titleSty.Render("Configure code review") + "\n")
	b.WriteString(dimSty.Render("Wire Gentleman Guardian Angel (gga) into this project: AI review on every commit.") + "\n\n")

	if !s.ggaAvailable {
		b.WriteString(warnSty.Render("! gga is not installed — capiko configures it, but the hook needs gga on PATH.") + "\n")
		b.WriteString(dimSty.Render("  Install: brew install gentleman-programming/tap/gga") + "\n\n")
	}

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
		{"Provider", codeReviewProviders[s.providerIdx]},
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

	b.WriteString("\n" + dimSty.Render("↑/↓ move · ←/→ provider · space toggle · enter select · esc back") + "\n")
	return b.String()
}

// onOff renders a boolean as a styled on/off badge.
func onOff(v bool) string {
	if v {
		return okSty.Render("on")
	}
	return dimSty.Render("off")
}

// codereviewConfig merges a state record over capiko's defaults.
func codereviewConfig(rec *state.CodeReviewRecord) codereview.Config {
	c := codereview.DefaultConfig()
	if rec.Provider != "" {
		c.Provider = rec.Provider
	}
	if rec.RulesFile != "" {
		c.RulesFile = rec.RulesFile
	}
	if rec.FilePatterns != "" {
		c.FilePatterns = rec.FilePatterns
	}
	c.ExcludePatterns = rec.ExcludePatterns
	c.StrictMode = rec.StrictMode
	if rec.Timeout > 0 {
		c.Timeout = rec.Timeout
	}
	return c
}
