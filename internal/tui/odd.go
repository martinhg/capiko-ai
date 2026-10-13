package tui

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/martinhg/capiko-ai/internal/backup"
	"github.com/martinhg/capiko-ai/internal/copilot"
	"github.com/martinhg/capiko-ai/internal/instructions"
	"github.com/martinhg/capiko-ai/internal/odd"
	"github.com/martinhg/capiko-ai/internal/scoped"
	"github.com/martinhg/capiko-ai/internal/sdd"
	"github.com/martinhg/capiko-ai/internal/state"
)

// applyODD injects the ODD routing block into Copilot's instructions file,
// backing the file up only when it changes, then records ODD as enabled in
// state. rddEnabled controls whether review-trigger integration clauses are
// included. When modular is true only the core block is injected and the
// routing and detail modules are written to the instructions directory;
// otherwise the full block is injected and any module files are removed.
func applyODD(host *copilot.Host, store *state.Store, bkp *backup.Store, rddEnabled, modular bool) error {
	if host == nil {
		return nil
	}
	path := filepath.Join(host.ConfigDir, "copilot-instructions.md")
	block := odd.Render(rddEnabled)
	if modular {
		block = odd.RenderCore(rddEnabled)
	}
	content, changed, err := instructions.Render(path, odd.MarkerStart, odd.MarkerEnd, block)
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
	dir := scoped.Dir(host.ConfigDir)
	if modular {
		if err := writeModules(dir, odd.RenderModules(rddEnabled)); err != nil {
			return err
		}
	} else if err := cleanModules(dir); err != nil {
		return err
	}
	if store != nil {
		if err := store.SetODD(true); err != nil {
			return err
		}
		return store.SetModularODD(modular)
	}
	return nil
}

const moduleSuffix = ".instructions.md"

// moduleFileName returns the filename for an ODD module.
func moduleFileName(name string) string { return name + moduleSuffix }

// writeModules writes each module file under dir.
func writeModules(dir string, mods []odd.Module) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	for _, m := range mods {
		if err := os.WriteFile(filepath.Join(dir, moduleFileName(m.Name)), []byte(m.Content), 0o644); err != nil {
			return err
		}
	}
	return nil
}

// cleanModules removes capiko ODD module files from the instructions dir.
func cleanModules(dir string) error {
	for _, name := range odd.ModuleNames() {
		err := os.Remove(filepath.Join(dir, moduleFileName(name)))
		if err != nil && !errors.Is(err, fs.ErrNotExist) {
			return err
		}
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
