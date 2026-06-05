// Package skill is the capiko domain model for a Copilot CLI skill.
//
// A skill is a SKILL.md file. The catalog is loaded from any fs.FS (an embedded
// filesystem today, a real directory tomorrow) — the domain does not know or
// care where the bytes come from.
package skill

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

// Skill is a single capiko capability. Content is the full SKILL.md, written
// verbatim on install, so what is authored is exactly what Copilot loads.
type Skill struct {
	Name        string // directory name under ~/.copilot/skills
	Description string // parsed from the frontmatter, shown in the configurator
	Content     string // full SKILL.md text
}

// Install writes the skill to <skillsDir>/<name>/SKILL.md and returns the path.
func (s Skill) Install(skillsDir string) (string, error) {
	dir := filepath.Join(skillsDir, s.Name)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("creating skill dir: %w", err)
	}
	p := filepath.Join(dir, "SKILL.md")
	if err := os.WriteFile(p, []byte(s.Content), 0o644); err != nil {
		return "", fmt.Errorf("writing SKILL.md: %w", err)
	}
	return p, nil
}

// LoadCatalog reads every <name>/SKILL.md under fsys into a Skill, using the
// directory name as the skill name and the frontmatter for the description.
// The result is sorted by name; directories without a SKILL.md are skipped.
func LoadCatalog(fsys fs.FS) ([]Skill, error) {
	entries, err := fs.ReadDir(fsys, ".")
	if err != nil {
		return nil, err
	}
	var skills []Skill
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		data, err := fs.ReadFile(fsys, path.Join(e.Name(), "SKILL.md"))
		if err != nil {
			if errors.Is(err, fs.ErrNotExist) {
				continue // not a skill directory
			}
			return nil, err
		}
		s, err := parse(e.Name(), string(data))
		if err != nil {
			return nil, fmt.Errorf("%s: %w", e.Name(), err)
		}
		skills = append(skills, s)
	}
	sort.Slice(skills, func(i, j int) bool { return skills[i].Name < skills[j].Name })
	return skills, nil
}

// parse builds a Skill from a directory name and SKILL.md content.
func parse(name, content string) (Skill, error) {
	m, err := frontmatter(content)
	if err != nil {
		return Skill{}, err
	}
	return Skill{Name: name, Description: m.Description, Content: content}, nil
}

type meta struct {
	Description string `yaml:"description"`
}

// frontmatter extracts and parses the leading YAML block delimited by --- lines.
func frontmatter(content string) (meta, error) {
	lines := strings.Split(content, "\n")
	if len(lines) == 0 || strings.TrimSpace(lines[0]) != "---" {
		return meta{}, errors.New("missing frontmatter")
	}
	var body []string
	closed := false
	for _, l := range lines[1:] {
		if strings.TrimSpace(l) == "---" {
			closed = true
			break
		}
		body = append(body, l)
	}
	if !closed {
		return meta{}, errors.New("unterminated frontmatter")
	}
	var m meta
	if err := yaml.Unmarshal([]byte(strings.Join(body, "\n")), &m); err != nil {
		return meta{}, fmt.Errorf("invalid frontmatter: %w", err)
	}
	return m, nil
}
