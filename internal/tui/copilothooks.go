// Package tui — copilothooks.go: apply/disable/backup orchestration plus the
// posture-dropdown configure screen for capiko's user-level GitHub Copilot CLI
// hook guardrails. The policy/UX layer (backup → write → record) and the
// Bubbletea screen (posture FSM) both live here, mirroring headroom/team-sync.
package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/martinhg/capiko-ai/internal/backup"
	"github.com/martinhg/capiko-ai/internal/copilot"
	"github.com/martinhg/capiko-ai/internal/copilothooks"
	"github.com/martinhg/capiko-ai/internal/state"
)

// applyCopilotHooks renders the guardrails hook file for rec.Posture and
// writes it to host.HooksDir, backing up the existing file first (snapshot-
// before-mutate) but only when the rendered bytes differ from what's already
// on disk — mirroring applyHeadroom's checksum-gated write. rec.Posture ==
// "off" (or rec.Enabled == false) routes to disableCopilotHooks instead of
// rendering (REQ-4.4/REQ-8.4). On success rec.Checksum is set to
// copilothooks.CombinedChecksum(host.HooksDir) — the single source shared
// with drift.StaleCopilotHooks (ADR-6) — and the record is persisted.
func applyCopilotHooks(host *copilot.Host, store *state.Store, bkp *backup.Store, rec *state.CopilotHooksRecord) error {
	if host == nil || rec == nil {
		return nil
	}
	if !rec.Enabled || rec.Posture == string(copilothooks.PostureOff) {
		return disableCopilotHooks(host, store, bkp)
	}

	hf, err := copilothooks.RenderGuardrails(copilothooks.Posture(rec.Posture))
	if err != nil {
		return err
	}
	data, err := copilothooks.Marshal(hf)
	if err != nil {
		return err
	}

	target := filepath.Join(host.HooksDir, copilothooks.GuardrailsFile)
	want := state.Checksum(string(data))
	if copilothooks.HookFileChecksum(target) != want {
		if err := backupCopilotHooks(bkp, host.HooksDir); err != nil {
			return err
		}
		if err := copilothooks.WriteHookFile(host.HooksDir, copilothooks.GuardrailsFile, data); err != nil {
			return err
		}
	}

	// Session verification hook — always rendered when hooks are enabled.
	scHf := copilothooks.RenderSessionCheck()
	scData, err := copilothooks.Marshal(scHf)
	if err != nil {
		return err
	}
	scTarget := filepath.Join(host.HooksDir, copilothooks.SessionCheckFile)
	scWant := state.Checksum(string(scData))
	if copilothooks.HookFileChecksum(scTarget) != scWant {
		if err := copilothooks.WriteHookFile(host.HooksDir, copilothooks.SessionCheckFile, scData); err != nil {
			return err
		}
	}

	// Session probe hook — captures stdin + env vars at session end for reporting.
	spHf := copilothooks.RenderSessionProbe()
	spData, err := copilothooks.Marshal(spHf)
	if err != nil {
		return err
	}
	spTarget := filepath.Join(host.HooksDir, copilothooks.SessionProbeFile)
	spWant := state.Checksum(string(spData))
	if copilothooks.HookFileChecksum(spTarget) != spWant {
		if err := copilothooks.WriteHookFile(host.HooksDir, copilothooks.SessionProbeFile, spData); err != nil {
			return err
		}
	}

	rec.Checksum = copilothooks.CombinedChecksum(host.HooksDir)
	if len(rec.Presets) == 0 {
		rec.Presets = []string{"guardrails"}
	}

	if store != nil {
		return store.SetCopilotHooks(rec)
	}
	return nil
}

// disableCopilotHooks removes the guardrails hook file, backing it up first,
// and records Enabled:false + Checksum:"" (zero value) so RunSync does not
// re-apply it (REQ-8.4). Mirrors disableHeadroom/disableTeamSync.
func disableCopilotHooks(host *copilot.Host, store *state.Store, bkp *backup.Store) error {
	if host == nil {
		return nil
	}
	if err := backupCopilotHooks(bkp, host.HooksDir); err != nil {
		return err
	}
	if err := copilothooks.RemoveHookFile(host.HooksDir, copilothooks.GuardrailsFile); err != nil {
		return err
	}
	if err := copilothooks.RemoveHookFile(host.HooksDir, copilothooks.SessionCheckFile); err != nil {
		return err
	}
	if err := copilothooks.RemoveHookFile(host.HooksDir, copilothooks.SessionProbeFile); err != nil {
		return err
	}
	if store != nil {
		return store.SetCopilotHooks(&state.CopilotHooksRecord{Enabled: false, Posture: string(copilothooks.PostureOff)})
	}
	return nil
}

// backupCopilotHooks snapshots the guardrails hook file before a mutation, if
// it currently exists. A first write has nothing to back up and is silently
// skipped. Mirrors backupTeamSyncHooks/backupMCPConfig (ADR-8).
func backupCopilotHooks(bkp *backup.Store, hooksDir string) error {
	if bkp == nil {
		return nil
	}
	path := filepath.Join(hooksDir, copilothooks.GuardrailsFile)
	if _, err := os.Stat(path); err != nil {
		return nil
	}
	if _, err := bkp.CreateFiles("copilot-hooks", Version, []string{path}); err != nil {
		return fmt.Errorf("backup failed, aborting: %w", err)
	}
	return nil
}

// Row indices for the Copilot hooks configure screen (ADR-9).
const (
	rowCopilotHooksPosture = iota // dropdown: off / warn / strict
	rowCopilotHooksApply          // Apply action
	rowCopilotHooksBack           // Back to main menu
	copilotHooksRows              // sentinel — count of rows
)

type copilotHooksState int

const (
	copilotHooksEditing copilotHooksState = iota
	copilotHooksApplying
	copilotHooksDone
	copilotHooksFailed
)

// postureCycle is the ordered posture ring the dropdown steps through
// (off → warn → strict → off). cyclePosture wraps in either direction.
var postureCycle = []copilothooks.Posture{
	copilothooks.PostureOff,
	copilothooks.PostureWarn,
	copilothooks.PostureStrict,
}

func cyclePosture(p copilothooks.Posture, delta int) copilothooks.Posture {
	idx := 0
	for i, c := range postureCycle {
		if c == p {
			idx = i
			break
		}
	}
	n := len(postureCycle)
	return postureCycle[((idx+delta)%n+n)%n]
}

// copilotHooksAppliedMsg is emitted by applyCmd when the goroutine completes.
// A nil err means success; a non-nil err means the write failed.
type copilotHooksAppliedMsg struct{ err error }

// copilotHooksScreen configures capiko's managed user-level Copilot CLI hook
// guardrails. A single posture dropdown (off/warn/strict) drives the whole
// wiring: off removes the hook file, warn/strict render and write it. It
// mirrors headroomScreen's editing→applying→done/failed FSM; there is no ack
// gate because guardrails only make the session safer (ADR-9).
type copilotHooksScreen struct {
	svc     services
	posture copilothooks.Posture
	cursor  int
	state   copilotHooksState
	err     error
}

// newCopilotHooks builds the screen, seeding the posture from a recorded
// CopilotHooksRecord (default off when unmanaged). Construction touches only
// state, so View() is deterministic with no filesystem access.
func newCopilotHooks(svc services) screen {
	s := &copilotHooksScreen{svc: svc, posture: copilothooks.PostureOff}
	if svc.state != nil {
		if st, err := svc.state.Load(); err == nil && st.CopilotHooks != nil && st.CopilotHooks.Posture != "" {
			s.posture = copilothooks.Posture(st.CopilotHooks.Posture)
		}
	}
	return s
}

func (s *copilotHooksScreen) Update(msg tea.Msg) (screen, tea.Cmd) {
	switch msg := msg.(type) {
	case copilotHooksAppliedMsg:
		if msg.err != nil {
			s.state, s.err = copilotHooksFailed, msg.err
			return s, nil
		}
		s.state = copilotHooksDone
		return s, nil
	case tea.KeyPressMsg:
		if s.state == copilotHooksApplying {
			return s, nil
		}
		switch msg.String() {
		case "q", "esc":
			return s, back
		}
		if s.state == copilotHooksDone || s.state == copilotHooksFailed {
			return s, back
		}
		switch msg.String() {
		case "up", "k":
			if s.cursor > 0 {
				s.cursor--
			}
		case "down", "j":
			if s.cursor < copilotHooksRows-1 {
				s.cursor++
			}
		case "left", "h":
			if s.cursor == rowCopilotHooksPosture {
				s.posture = cyclePosture(s.posture, -1)
			}
		case "right", "l", " ", "space":
			if s.cursor == rowCopilotHooksPosture {
				s.posture = cyclePosture(s.posture, +1)
			}
		case "enter":
			switch s.cursor {
			case rowCopilotHooksApply:
				s.state = copilotHooksApplying
				return s, s.applyCmd()
			case rowCopilotHooksBack:
				return s, back
			}
		}
	}
	return s, nil
}

// applyCmd runs applyCopilotHooks off the render loop and reports the outcome.
// posture "off" routes to disable inside applyCopilotHooks; warn/strict render
// and write with the guardrails preset.
func (s *copilotHooksScreen) applyCmd() tea.Cmd {
	host, store, bkp, posture := s.svc.host, s.svc.state, s.svc.backup, s.posture
	return func() tea.Msg {
		rec := &state.CopilotHooksRecord{
			Enabled: posture != copilothooks.PostureOff,
			Posture: string(posture),
		}
		return copilotHooksAppliedMsg{err: applyCopilotHooks(host, store, bkp, rec)}
	}
}

func (s *copilotHooksScreen) View() string {
	var b strings.Builder
	b.WriteString(titleSty.Render("Configure Copilot hooks") + "\n\n")

	switch s.state {
	case copilotHooksApplying:
		b.WriteString("Applying Copilot hooks…\n")
		return b.String()
	case copilotHooksDone:
		if s.posture == copilothooks.PostureOff {
			b.WriteString(okSty.Render("Copilot guardrails disabled ✓") + "\n\n")
		} else {
			b.WriteString(okSty.Render("Copilot guardrails "+string(s.posture)+" ✓") + "\n\n")
		}
		b.WriteString(dimSty.Render("any key to go back") + "\n")
		return b.String()
	case copilotHooksFailed:
		b.WriteString(errSty.Render("Error: "+s.err.Error()) + "\n\n")
		b.WriteString(dimSty.Render("any key to go back") + "\n")
		return b.String()
	}

	b.WriteString(dimSty.Render("Guardrail hooks for the Copilot CLI — block or ask before dangerous") + "\n")
	b.WriteString(dimSty.Render("commands (rm -rf /, curl | sh, chmod 777, git push --force to main).") + "\n\n")

	rows := []struct{ label, value string }{
		{"Posture", postureValue(s.posture)},
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
			b.WriteString("  " + r.value)
		}
		b.WriteString("\n")
	}

	// Static informational notes — scope + policy caveats (ADR-9). Constant
	// text keeps View() deterministic with no filesystem access.
	b.WriteString("\n")
	b.WriteString(dimSty.Render("Manages user-level CLI hooks. Repo-level cloud-agent enforcement") + "\n")
	b.WriteString(dimSty.Render("(.github/hooks) is a future feature.") + "\n")
	b.WriteString(dimSty.Render("Org policy hooks (/etc/github-copilot/policy.d/) can override these.") + "\n")

	b.WriteString("\n" + dimSty.Render("↑/↓ move · ←/→/space cycle posture · enter select · esc back") + "\n")
	return b.String()
}

// postureValue renders the posture with a severity-tinted style: off dim,
// warn amber, strict green (the strongest guardrail).
func postureValue(p copilothooks.Posture) string {
	switch p {
	case copilothooks.PostureWarn:
		return warnSty.Render("warn (ask)")
	case copilothooks.PostureStrict:
		return okSty.Render("strict (deny)")
	default:
		return dimSty.Render("off")
	}
}
