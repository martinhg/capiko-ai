package tui

import (
	"fmt"
	"path/filepath"

	"github.com/martinhg/capiko-ai/internal/backup"
	"github.com/martinhg/capiko-ai/internal/copilot"
	"github.com/martinhg/capiko-ai/internal/instructions"
	"github.com/martinhg/capiko-ai/internal/state"
	"github.com/martinhg/capiko-ai/internal/trigger"
)

// applyTriggerRules injects the trigger-rules block (default rules) into
// Copilot's instructions file, backing up only when it changes, then records
// enablement in state. Shared by the install flow and the post-sync re-apply.
func applyTriggerRules(host *copilot.Host, store *state.Store, bkp *backup.Store, enabled bool) error {
	if host == nil {
		return nil
	}

	var block string
	if enabled {
		block = trigger.Render(trigger.DefaultRules())
	}

	path := filepath.Join(host.ConfigDir, "copilot-instructions.md")
	content, changed, err := instructions.Render(path, trigger.MarkerStart, trigger.MarkerEnd, block)
	if err != nil {
		return err
	}
	if changed {
		if bkp != nil {
			if _, err := bkp.CreateFiles("trigger", Version, []string{path}); err != nil {
				return fmt.Errorf("backup failed, aborting: %w", err)
			}
		}
		if err := instructions.Write(path, content); err != nil {
			return err
		}
	}
	if store != nil {
		return store.SetTriggerRules(enabled)
	}
	return nil
}
