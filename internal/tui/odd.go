package tui

import (
	"fmt"
	"path/filepath"

	"github.com/martinhg/capiko-ai/internal/backup"
	"github.com/martinhg/capiko-ai/internal/copilot"
	"github.com/martinhg/capiko-ai/internal/instructions"
	"github.com/martinhg/capiko-ai/internal/odd"
	"github.com/martinhg/capiko-ai/internal/sdd"
	"github.com/martinhg/capiko-ai/internal/state"
)

// applyODD injects the ODD routing block into Copilot's instructions file,
// backing the file up only when it changes, then records ODD as enabled in
// state. rddEnabled controls whether review-trigger integration clauses are
// included.
func applyODD(host *copilot.Host, store *state.Store, bkp *backup.Store, rddEnabled bool) error {
	if host == nil {
		return nil
	}
	path := filepath.Join(host.ConfigDir, "copilot-instructions.md")
	content, changed, err := instructions.Render(path, odd.MarkerStart, odd.MarkerEnd, odd.Render(rddEnabled))
	if err != nil {
		return err
	}
	if changed {
		if bkp != nil {
			if _, err := bkp.CreateFiles("odd", Version, []string{path}); err != nil {
				return fmt.Errorf("backup failed, aborting: %w", err)
			}
		}
		if err := instructions.Write(path, content); err != nil {
			return err
		}
	}
	if store != nil {
		return store.SetODD(true)
	}
	return nil
}

// migrateSddToOdd removes the SDD marker block from the instructions file so
// it can be replaced by the ODD block. Called once on the first sync after
// upgrading to an ODD-capable version when state still has SDD models set.
func migrateSddToOdd(host *copilot.Host, store *state.Store, bkp *backup.Store) error {
	if host == nil {
		return nil
	}
	path := filepath.Join(host.ConfigDir, "copilot-instructions.md")
	content, changed, err := instructions.Render(path, sdd.MarkerStart, sdd.MarkerEnd, "")
	if err != nil {
		return err
	}
	if changed {
		if bkp != nil {
			if _, err := bkp.CreateFiles("odd-migrate", Version, []string{path}); err != nil {
				return fmt.Errorf("backup failed, aborting: %w", err)
			}
		}
		if err := instructions.Write(path, content); err != nil {
			return err
		}
	}
	return nil
}
