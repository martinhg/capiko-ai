// Package githooks injects and removes marker-delimited blocks in
// .git/hooks/<name> files. It is mechanism-only: no engram processes, no git
// processes, no policy. All marker semantics are delegated to
// internal/instructions.Inject so the block format stays identical to the
// persona, SDD, and code-review managed blocks.
package githooks

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/martinhg/capiko-ai/internal/instructions"
)

// WriteBlock injects a marker-delimited block into
// <workspace>/.git/hooks/<hookName>. When the file is absent or contains only
// whitespace it is created with a "#!/bin/sh\n" first line. Content outside
// the markers is always preserved. The file is made executable (mode 0o755).
// The operation is idempotent: identical arguments produce identical bytes.
func WriteBlock(workspace, hookName, markerStart, markerEnd, block string) error {
	hookPath := filepath.Join(workspace, ".git", "hooks", hookName)
	if err := os.MkdirAll(filepath.Dir(hookPath), 0o755); err != nil {
		return err
	}

	existingBytes, err := os.ReadFile(hookPath)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	existing := string(existingBytes)
	if strings.TrimSpace(existing) == "" {
		// Seed the shebang only when the file is absent or empty.
		existing = "#!/bin/sh\n"
	}

	updated := instructions.Inject(existing, markerStart, markerEnd, strings.TrimRight(block, "\n"))

	tmp := hookPath + ".tmp"
	if err := os.WriteFile(tmp, []byte(updated), 0o644); err != nil {
		return err
	}
	// Explicitly chmod before rename so the exec bit is guaranteed regardless
	// of the process umask.
	if err := os.Chmod(tmp, 0o755); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	return os.Rename(tmp, hookPath)
}

// RemoveBlock removes the marker-delimited block from
// <workspace>/.git/hooks/<hookName>. A missing file or absent markers is a
// no-op. If removing the block leaves only the capiko-seeded shebang (and
// nothing else of substance), the hook file is deleted so disable leaves no
// inert capiko-only hook behind.
func RemoveBlock(workspace, hookName, markerStart, markerEnd string) error {
	hookPath := filepath.Join(workspace, ".git", "hooks", hookName)

	existingBytes, err := os.ReadFile(hookPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	existing := string(existingBytes)
	remaining := instructions.Inject(existing, markerStart, markerEnd, "")
	if remaining == existing {
		// Markers were not present — leave the file untouched.
		return nil
	}

	// Delete the file when only the capiko-seeded shebang would remain.
	if remaining == "" || strings.TrimSpace(remaining) == "#!/bin/sh" {
		return os.Remove(hookPath)
	}

	// Preserve the original file mode (exec bit) when rewriting.
	var mode os.FileMode = 0o755
	if info, err := os.Stat(hookPath); err == nil {
		mode = info.Mode()
	}
	tmp := hookPath + ".tmp"
	if err := os.WriteFile(tmp, []byte(remaining), 0o644); err != nil {
		return err
	}
	if err := os.Chmod(tmp, mode); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	return os.Rename(tmp, hookPath)
}
