// Package instructions injects capiko-managed, marker-bound blocks into Copilot's
// instruction files (e.g. ~/.copilot/copilot-instructions.md). Each managed block
// lives between a start/end marker so capiko only ever touches its own sections
// and never clobbers content the user keeps outside them. Shared by the persona
// and SDD orchestrator features.
package instructions

import (
	"os"
	"path/filepath"
	"strings"
)

// Inject replaces the block delimited by start/end markers in existing with
// block, inserting it when absent. An empty block removes the section. Content
// outside the markers is always preserved.
func Inject(existing, start, end, block string) string {
	var section string
	if block != "" {
		section = start + "\n" + block + "\n" + end
	}

	si := strings.Index(existing, start)
	ei := strings.Index(existing, end)
	if si >= 0 && ei > si {
		before := strings.TrimRight(existing[:si], "\n")
		after := strings.TrimLeft(existing[ei+len(end):], "\n")
		parts := make([]string, 0, 3)
		if before != "" {
			parts = append(parts, before)
		}
		if section != "" {
			parts = append(parts, section)
		}
		if after != "" {
			parts = append(parts, after)
		}
		joined := strings.Join(parts, "\n\n")
		if joined == "" {
			return ""
		}
		return joined + "\n"
	}

	if section == "" {
		return existing
	}
	if strings.TrimSpace(existing) == "" {
		return section + "\n"
	}
	return strings.TrimRight(existing, "\n") + "\n\n" + section + "\n"
}

// Render reads path (treating a missing file as empty), injects block between the
// markers, and reports whether the result differs from the current file.
func Render(path, start, end, block string) (content string, changed bool, err error) {
	existing, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return "", false, err
	}
	updated := Inject(string(existing), start, end, strings.TrimRight(block, "\n"))
	return updated, updated != string(existing), nil
}

// Write atomically writes content to path.
func Write(path, content string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, []byte(content), 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}
