package copilothooks

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/martinhg/capiko-ai/internal/state"
)

// WriteHookFile atomically writes data to <hooksDir>/<name>: it creates
// hooksDir if missing, writes to a "<name>.tmp" sibling, chmod 0o644, then
// renames it into place (ADR-8). 0o644, not 0o755: the JSON file is read by
// Copilot, not executed — the bash/powershell fields inside it are the
// executable content, unlike githooks.WriteBlock which needs 0o755 because
// the hook file itself IS the script.
func WriteHookFile(hooksDir, name string, data []byte) error {
	if err := guardName(hooksDir, name); err != nil {
		return err
	}
	if err := os.MkdirAll(hooksDir, 0o755); err != nil {
		return fmt.Errorf("copilothooks: create hooks dir: %w", err)
	}

	target := filepath.Join(hooksDir, name)
	tmp := target + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return fmt.Errorf("copilothooks: write temp hook file: %w", err)
	}
	if err := os.Rename(tmp, target); err != nil {
		return fmt.Errorf("copilothooks: rename hook file into place: %w", err)
	}
	return nil
}

// RemoveHookFile removes <hooksDir>/<name>. It is idempotent: removing a file
// that is not present is not an error (mirrors copilot.Host.UninstallAgent).
func RemoveHookFile(hooksDir, name string) error {
	if err := guardName(hooksDir, name); err != nil {
		return err
	}
	target := filepath.Join(hooksDir, name)
	if err := os.Remove(target); err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("copilothooks: remove hook file: %w", err)
	}
	return nil
}

// guardName refuses any name that resolves outside hooksDir or into a
// subdirectory (path traversal), mirroring copilot.go's UninstallAgent guard
// (copilot.go:132-149). name is always a capiko constant in v1, but the guard
// makes a bad name incapable of deleting or writing arbitrary files —
// defense in depth for future dynamic preset names.
func guardName(hooksDir, name string) error {
	target := filepath.Join(hooksDir, name)
	rel, err := filepath.Rel(hooksDir, target)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) || strings.ContainsRune(rel, os.PathSeparator) {
		return fmt.Errorf("copilothooks: refusing %q: resolves outside hooks directory", name)
	}
	return nil
}

// HookFileChecksum returns state.Checksum of the file at path's bytes, or ""
// if the file is absent. It is used for the backup-only-when-changed
// optimization (compare on-disk bytes to freshly rendered bytes before
// deciding to snapshot+write) — it is NOT the value stored in
// CopilotHooksRecord.Checksum; see CombinedChecksum for that (ADR-6).
func HookFileChecksum(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return state.Checksum(string(data))
}

// CombinedChecksum hashes every capiko-owned hook file (FilePrefix-prefixed,
// ".json"-suffixed) in hooksDir, sorted by name, as name+"\n"+bytes
// concatenations (ADR-6). A missing hooksDir or a directory with no
// capiko-owned files returns "". This is the SINGLE source of both the
// stored CopilotHooksRecord.Checksum and the drift-comparison value —
// applyCopilotHooks and drift.StaleCopilotHooks both call this exact
// function so the two values can never diverge.
func CombinedChecksum(hooksDir string) string {
	entries, err := os.ReadDir(hooksDir)
	if err != nil {
		return ""
	}

	var names []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		n := e.Name()
		if !strings.HasPrefix(n, FilePrefix) || !strings.HasSuffix(n, ".json") {
			continue
		}
		names = append(names, n)
	}
	if len(names) == 0 {
		return ""
	}
	sort.Strings(names)

	var buf strings.Builder
	for _, n := range names {
		data, err := os.ReadFile(filepath.Join(hooksDir, n))
		if err != nil {
			return ""
		}
		buf.WriteString(n)
		buf.WriteByte('\n')
		buf.Write(data)
	}
	return state.Checksum(buf.String())
}
