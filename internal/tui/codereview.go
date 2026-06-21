package tui

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/martinhg/capiko-ai/internal/backup"
	"github.com/martinhg/capiko-ai/internal/codereview"
	"github.com/martinhg/capiko-ai/internal/instructions"
	"github.com/martinhg/capiko-ai/internal/state"
)

// gga hook seams, swapped in tests so apply never shells out to a real gga binary
// or touches a real git repo. capiko configures gga; it never installs the binary.
var (
	ggaInstallHook   = func(workspace string) error { return runGGA(workspace, "install") }
	ggaUninstallHook = func(workspace string) error { return runGGA(workspace, "uninstall") }
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
