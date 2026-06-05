// Package persona manages the capiko persona written into Copilot's global
// custom-instructions file (~/.copilot/copilot-instructions.md), which Copilot
// CLI always loads. The persona lives in a marker-bound block so capiko only
// ever touches its own section and never clobbers the user's other instructions.
package persona

import (
	"embed"
	"os"
	"path/filepath"
	"strings"
	"time"
)

//go:embed content/*.md
var contentFS embed.FS

// ID identifies a persona choice.
type ID string

const (
	Capiko  ID = "capiko"
	Neutral ID = "neutral"
	None    ID = "none"
)

// Marker delimiters for capiko's block inside the instructions file.
const (
	MarkerStart = "<!-- capiko:persona:start -->"
	MarkerEnd   = "<!-- capiko:persona:end -->"
)

// Persona is a selectable persona with its instruction content.
type Persona struct {
	ID          ID
	Name        string
	Description string
	Content     string // markdown injected into the block; empty for None
}

// Available returns the personas offered in the configurator.
func Available() []Persona {
	return []Persona{
		{Capiko, "Capiko", "Teaching-first mentor, warm Rioplatense tone", read("content/capiko.md")},
		{Neutral, "Neutral", "Same guidance, professional tone, no regional slang", read("content/neutral.md")},
		{None, "None", "Leave Copilot instructions untouched", ""},
	}
}

func read(p string) string {
	b, err := contentFS.ReadFile(p)
	if err != nil {
		panic(err) // embedded at build time; cannot fail at runtime
	}
	return string(b)
}

// Apply writes the persona's block into the instructions file, snapshotting the
// prior file under backupRoot first. None removes capiko's block, leaving the
// rest of the file intact. The write is atomic.
func Apply(instructionsPath, backupRoot string, p Persona) error {
	existing, err := os.ReadFile(instructionsPath)
	if err != nil && !os.IsNotExist(err) {
		return err
	}

	if len(existing) > 0 && backupRoot != "" {
		if err := snapshot(backupRoot, existing); err != nil {
			return err
		}
	}

	updated := injectSection(string(existing), strings.TrimRight(p.Content, "\n"))
	if updated == string(existing) {
		return nil // nothing changed (e.g. None with no existing block)
	}

	if err := os.MkdirAll(filepath.Dir(instructionsPath), 0o755); err != nil {
		return err
	}
	tmp := instructionsPath + ".tmp"
	if err := os.WriteFile(tmp, []byte(updated), 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, instructionsPath)
}

func snapshot(root string, content []byte) error {
	dir := filepath.Join(root, "persona-"+time.Now().UTC().Format("20060102T150405.000000000"))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, "copilot-instructions.md"), content, 0o644)
}

// injectSection replaces capiko's marker-bound block in existing with block, or
// inserts it when absent. An empty block removes the section. Content outside the
// markers is always preserved.
func injectSection(existing, block string) string {
	var section string
	if block != "" {
		section = MarkerStart + "\n" + block + "\n" + MarkerEnd
	}

	start := strings.Index(existing, MarkerStart)
	end := strings.Index(existing, MarkerEnd)

	if start >= 0 && end > start {
		before := strings.TrimRight(existing[:start], "\n")
		after := strings.TrimLeft(existing[end+len(MarkerEnd):], "\n")
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

	// No existing block.
	if section == "" {
		return existing
	}
	if strings.TrimSpace(existing) == "" {
		return section + "\n"
	}
	return strings.TrimRight(existing, "\n") + "\n\n" + section + "\n"
}
