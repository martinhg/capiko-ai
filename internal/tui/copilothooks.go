// Package tui — copilothooks.go: apply/disable/backup orchestration for
// capiko's user-level GitHub Copilot CLI hook guardrails (WU-6). The TUI
// screen (posture dropdown FSM, menu wiring) is a separate, later work unit —
// this file only owns the non-screen policy/UX layer described in the
// design's "Architecture approach": backup → write → record.
package tui

import (
	"fmt"
	"os"
	"path/filepath"

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
