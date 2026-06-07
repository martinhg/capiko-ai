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

// File is one bundled file inside a skill, written relative to the skill dir.
type File struct {
	Path    string // relative to <skillsDir>/<name>, e.g. "references/strict-tdd.md"
	Content string
}

// Skill is a single capiko capability. Content is the full SKILL.md, written
// verbatim on install, so what is authored is exactly what Copilot loads. Extra
// carries any additional bundled files (reference docs, shared contracts) so a
// skill can ship as a multi-file directory, not just a lone SKILL.md.
type Skill struct {
	Name        string // directory name under ~/.copilot/skills
	Description string // parsed from the frontmatter, shown in the configurator
	Content     string // full SKILL.md text
	Extra       []File // additional bundled files, relative to the skill dir
}

// Install writes the skill bundle under <skillsDir>/<name>: SKILL.md plus every
// Extra file (creating subdirectories as needed). It returns the SKILL.md path.
func (s Skill) Install(skillsDir string) (string, error) {
	dir := filepath.Join(skillsDir, s.Name)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("creating skill dir: %w", err)
	}
	p := filepath.Join(dir, "SKILL.md")
	if err := os.WriteFile(p, []byte(s.Content), 0o644); err != nil {
		return "", fmt.Errorf("writing SKILL.md: %w", err)
	}
	for _, f := range s.Extra {
		fp := filepath.Join(dir, filepath.FromSlash(f.Path))
		if err := os.MkdirAll(filepath.Dir(fp), 0o755); err != nil {
			return "", fmt.Errorf("creating dir for %s: %w", f.Path, err)
		}
		if err := os.WriteFile(fp, []byte(f.Content), 0o644); err != nil {
			return "", fmt.Errorf("writing %s: %w", f.Path, err)
		}
	}
	return p, nil
}

// CanonicalContent returns a deterministic representation of the whole bundle for
// checksumming. For a single-file skill it is exactly Content, so checksums
// recorded before multi-file support stay stable (no spurious drift). Extra files
// are folded in sorted by path, independent of their slice order.
func (s Skill) CanonicalContent() string {
	if len(s.Extra) == 0 {
		return s.Content
	}
	extra := append([]File(nil), s.Extra...)
	sort.Slice(extra, func(i, j int) bool { return extra[i].Path < extra[j].Path })
	var b strings.Builder
	b.WriteString(s.Content)
	for _, f := range extra {
		b.WriteString("\x00")
		b.WriteString(f.Path)
		b.WriteString("\x00")
		b.WriteString(f.Content)
	}
	return b.String()
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
		s, err := Parse(e.Name(), string(data))
		if err != nil {
			return nil, fmt.Errorf("%s: %w", e.Name(), err)
		}
		extra, err := loadExtra(fsys, e.Name())
		if err != nil {
			return nil, fmt.Errorf("%s: %w", e.Name(), err)
		}
		s.Extra = extra
		skills = append(skills, s)
	}
	sort.Slice(skills, func(i, j int) bool { return skills[i].Name < skills[j].Name })
	return skills, nil
}

// loadExtra collects every file under the skill dir except its SKILL.md, with
// forward-slash paths relative to the skill dir, sorted for determinism.
func loadExtra(fsys fs.FS, name string) ([]File, error) {
	var extra []File
	err := fs.WalkDir(fsys, name, func(p string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			return nil
		}
		rel := strings.TrimPrefix(p, name+"/")
		if rel == "SKILL.md" {
			return nil
		}
		data, err := fs.ReadFile(fsys, p)
		if err != nil {
			return err
		}
		extra = append(extra, File{Path: rel, Content: string(data)})
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Slice(extra, func(i, j int) bool { return extra[i].Path < extra[j].Path })
	return extra, nil
}

// Parse builds a Skill from a directory name and SKILL.md content. It is the
// exported single-skill parser, so callers that scan untrusted skill
// directories (e.g. the skill-registry engine) can parse and tolerate one
// SKILL.md at a time instead of failing a whole catalog on one bad file.
func Parse(name, content string) (Skill, error) {
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
