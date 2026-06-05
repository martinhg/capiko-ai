// Package scoped manages capiko's curated scoped instruction files. Unlike the
// always-on persona/SDD blocks in copilot-instructions.md, these are
// `*.instructions.md` files written under ~/.copilot/instructions/, which Copilot
// applies only when working on files matching each file's `applyTo` glob.
package scoped

import (
	"embed"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

//go:embed files/*.instructions.md
var files embed.FS

const suffix = ".instructions.md"

// Instruction is one curated scoped instruction file, embedded in the binary.
type Instruction struct {
	Name    string // base name without the .instructions.md suffix, e.g. "go"
	Content string // full file content, including frontmatter, written verbatim
}

// Load returns the embedded curated instructions, sorted by name.
func Load() ([]Instruction, error) {
	entries, err := fs.ReadDir(files, "files")
	if err != nil {
		return nil, err
	}
	var out []Instruction
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), suffix) {
			continue
		}
		data, err := files.ReadFile("files/" + e.Name())
		if err != nil {
			return nil, err
		}
		out = append(out, Instruction{
			Name:    strings.TrimSuffix(e.Name(), suffix),
			Content: string(data),
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

// Dir returns the instructions directory under the given Copilot config dir.
func Dir(configDir string) string { return filepath.Join(configDir, "instructions") }

// Path returns where an instruction is written under dir.
func Path(dir string, ins Instruction) string {
	return filepath.Join(dir, ins.Name+suffix)
}

// Install writes the instruction verbatim to its path under dir, returning the path.
func Install(dir string, ins Instruction) (string, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	p := Path(dir, ins)
	if err := os.WriteFile(p, []byte(ins.Content), 0o644); err != nil {
		return "", err
	}
	return p, nil
}
