package tui

import (
	"fmt"
	"path/filepath"

	"github.com/martinhg/capiko-ai/internal/backup"
	"github.com/martinhg/capiko-ai/internal/copilot"
	"github.com/martinhg/capiko-ai/internal/instructions"
	"github.com/martinhg/capiko-ai/internal/memory"
	"github.com/martinhg/capiko-ai/internal/state"
)

// applyMemoryProtocol injects (enabled) or removes (disabled) the engram
// proactive-memory protocol block in Copilot's instructions file, backing up
// only when it changes. Presence is fully derived from engram's enabled state,
// so unlike applyOutputEfficiency it records no flag of its own. The store
// param is kept for call-site parity with the other appliers.
func applyMemoryProtocol(host *copilot.Host, store *state.Store, bkp *backup.Store, enabled bool) error {
	if host == nil {
		return nil
	}

	var block string
	if enabled {
		block = memory.Render()
	}

	path := filepath.Join(host.ConfigDir, "copilot-instructions.md")
	content, changed, err := instructions.Render(path, memory.MarkerStart, memory.MarkerEnd, block)
	if err != nil {
		return err
	}
	if changed {
		if bkp != nil {
			if _, err := bkp.CreateFiles("memory", Version, []string{path}); err != nil {
				return fmt.Errorf("backup failed, aborting: %w", err)
			}
		}
		if err := instructions.Write(path, content); err != nil {
			return err
		}
	}
	_ = store // no new state field; gating derives from EngramRecord.Enabled
	return nil
}
