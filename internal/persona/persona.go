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

// ByID returns the persona with the given id.
func ByID(id ID) (Persona, bool) {
	for _, p := range Available() {
		if p.ID == id {
			return p, true
		}
	}
	return Persona{}, false
}

// Render computes the instructions file content with the persona's marker block
// injected (or removed, for None), without writing. changed reports whether it
// differs from the current file, so the caller can back up only when needed.
func Render(instructionsPath string, p Persona) (content string, changed bool, err error) {
	existing, err := os.ReadFile(instructionsPath)
	if err != nil && !os.IsNotExist(err) {
		return "", false, err
	}
	updated := injectSection(string(existing), strings.TrimRight(p.Content, "\n"))
	return updated, updated != string(existing), nil
}

// Write atomically writes the rendered instructions content.
func Write(instructionsPath, content string) error {
	if err := os.MkdirAll(filepath.Dir(instructionsPath), 0o755); err != nil {
		return err
	}
	tmp := instructionsPath + ".tmp"
	if err := os.WriteFile(tmp, []byte(content), 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, instructionsPath)
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
