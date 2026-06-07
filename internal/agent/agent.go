// Package agent is the capiko domain model for a Copilot CLI custom agent.
//
// An agent is a single .agent.md file (unlike a skill, which is a directory).
// The catalog is loaded from any fs.FS — an embedded filesystem at build time,
// a real directory in tests.
package agent

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/martinhg/capiko-ai/internal/state"
	"gopkg.in/yaml.v3"
)

// Agent is a single Copilot custom agent. Content is the full .agent.md text,
// written verbatim on install. Name is the filename stem (without .agent.md).
type Agent struct {
	Name        string // filename stem, e.g. "capiko-sdd-explore"
	Description string // parsed from frontmatter
	Content     string // full .agent.md text
}

// CanonicalContent returns a deterministic representation of the agent for
// checksumming. For agents (single-file model) this is exactly Content.
func (a Agent) CanonicalContent() string {
	return a.Content
}

// Install writes the agent file as <name>.agent.md under agentsDir, creating
// the directory if necessary. If an existing file has identical content (same
// checksum) the write is skipped (no-op). It returns the path written.
func (a Agent) Install(agentsDir string) (string, error) {
	if err := os.MkdirAll(agentsDir, 0o755); err != nil {
		return "", fmt.Errorf("creating agents dir: %w", err)
	}
	p := filepath.Join(agentsDir, a.Name+".agent.md")

	// Skip write when checksum matches (idempotency).
	if existing, err := os.ReadFile(p); err == nil {
		if state.Checksum(string(existing)) == state.Checksum(a.CanonicalContent()) {
			return p, nil
		}
	}

	if err := os.WriteFile(p, []byte(a.Content), 0o644); err != nil {
		return "", fmt.Errorf("writing agent file: %w", err)
	}
	return p, nil
}

// LoadCatalog reads every *.agent.md file under fsys into an Agent, using the
// filename stem as the agent Name and the frontmatter for the description. The
// result is sorted by name. An invalid frontmatter is a fatal authoring error.
func LoadCatalog(fsys fs.FS) ([]Agent, error) {
	entries, err := fs.Glob(fsys, "*.agent.md")
	if err != nil {
		return nil, err
	}
	var agents []Agent
	for _, entry := range entries {
		data, err := fs.ReadFile(fsys, entry)
		if err != nil {
			return nil, fmt.Errorf("reading %s: %w", entry, err)
		}
		name := strings.TrimSuffix(entry, ".agent.md")
		a, err := parse(name, string(data))
		if err != nil {
			return nil, fmt.Errorf("%s: %w", entry, err)
		}
		agents = append(agents, a)
	}
	sort.Slice(agents, func(i, j int) bool { return agents[i].Name < agents[j].Name })
	return agents, nil
}

// parse builds an Agent from a filename stem and raw .agent.md content.
func parse(name, content string) (Agent, error) {
	m, err := parseFrontmatter(content)
	if err != nil {
		return Agent{}, err
	}
	return Agent{Name: name, Description: m.Description, Content: content}, nil
}

// Frontmatter is the parsed YAML header of a .agent.md file, exposing all
// fields the tests and install logic need. It is exported so catalog-level
// tests can validate field constraints without re-parsing.
type Frontmatter struct {
	Description   string   `yaml:"description"`
	Tools         []string `yaml:"tools"`
	UserInvocable bool     `yaml:"user-invocable"`
	Agents        []string `yaml:"agents"`
	Target        string   `yaml:"target"`
	Model         string   `yaml:"model"`
}

// ParseFrontmatter extracts and parses the YAML block delimited by --- lines.
// Exported so tests can validate catalog agents' fields.
func ParseFrontmatter(content string) (Frontmatter, error) {
	return parseFrontmatter(content)
}

func parseFrontmatter(content string) (Frontmatter, error) {
	lines := strings.Split(content, "\n")
	if len(lines) == 0 || strings.TrimSpace(lines[0]) != "---" {
		return Frontmatter{}, errors.New("missing frontmatter")
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
		return Frontmatter{}, errors.New("unterminated frontmatter")
	}
	var m Frontmatter
	if err := yaml.Unmarshal([]byte(strings.Join(body, "\n")), &m); err != nil {
		return Frontmatter{}, fmt.Errorf("invalid frontmatter: %w", err)
	}
	if m.Description == "" {
		return Frontmatter{}, errors.New("frontmatter missing required field: description")
	}
	return m, nil
}
