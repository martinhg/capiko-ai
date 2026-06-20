package tui

import (
	"fmt"
	"path/filepath"

	"github.com/martinhg/capiko-ai/internal/backup"
	"github.com/martinhg/capiko-ai/internal/copilot"
	"github.com/martinhg/capiko-ai/internal/efficiency"
	"github.com/martinhg/capiko-ai/internal/instructions"
	"github.com/martinhg/capiko-ai/internal/state"
)

// applyOutputEfficiency injects (enabled) or removes (disabled) the
// output-efficiency block in Copilot's instructions file, backing up only when it
// changes, then records the choice in state. Shared by the persona screen and the
// post-sync re-apply, mirroring applyTriggerRules.
func applyOutputEfficiency(host *copilot.Host, store *state.Store, bkp *backup.Store, enabled bool) error {
	if host == nil {
		return nil
	}

	var block string
	if enabled {
		block = efficiency.Render()
	}

	path := filepath.Join(host.ConfigDir, "copilot-instructions.md")
	content, changed, err := instructions.Render(path, efficiency.MarkerStart, efficiency.MarkerEnd, block)
	if err != nil {
		return err
	}
	if changed {
		if bkp != nil {
			if _, err := bkp.CreateFiles("efficiency", Version, []string{path}); err != nil {
				return fmt.Errorf("backup failed, aborting: %w", err)
			}
		}
		if err := instructions.Write(path, content); err != nil {
			return err
		}
	}
	if store != nil {
		return store.SetOutputEfficiency(enabled)
	}
	return nil
}
