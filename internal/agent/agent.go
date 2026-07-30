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

// WithRouting returns a new slice with model: frontmatter injected, replaced,
// or stripped per the models map. Keys are SDD phase names (e.g. "explore"),
// matched by stripping the "capiko-sdd-" prefix from each agent's Name.
// "default" or "" strips any existing model: line. Agents whose derived key
// has no entry in models (including "coordinator", which is never an SDD
// phase key) pass through with byte-identical Content. WithRouting is a pure
// transform: it does not mutate agents or perform I/O.
func WithRouting(agents []Agent, models map[string]string) []Agent {
	out := make([]Agent, len(agents))
	for i, a := range agents {
		phase := strings.TrimPrefix(a.Name, "capiko-sdd-")
		model, ok := models[phase]
		if !ok {
			out[i] = a
			continue
		}
		out[i] = Agent{Name: a.Name, Description: a.Description, Content: routeModel(a.Content, model)}
	}
	return out
}

// routeModel rewrites the model: line in content's frontmatter. When model is
// "default" or "", any existing model: line is removed. Otherwise the
// existing model: line is replaced (or a new one inserted before the closing
// --- delimiter) with the given value. All other frontmatter fields and the
// body are left untouched. Content without a well-formed --- ... --- header
// is returned unchanged.
func routeModel(content, model string) string {
	lines := strings.Split(content, "\n")
	if len(lines) == 0 || strings.TrimSpace(lines[0]) != "---" {
		return content
	}
	end := -1
	for i := 1; i < len(lines); i++ {
		if strings.TrimSpace(lines[i]) == "---" {
			end = i
			break
		}
	}
	if end == -1 {
		return content
	}

	strip := model == "" || model == "default"
	modelLine := "model: " + model

	newLines := make([]string, 0, len(lines)+1)
	newLines = append(newLines, lines[0])
	found := false
	for i := 1; i < end; i++ {
		if strings.HasPrefix(strings.TrimSpace(lines[i]), "model:") {
			found = true
			if !strip {
				newLines = append(newLines, modelLine)
			}
			continue
		}
		newLines = append(newLines, lines[i])
	}
	if !strip && !found {
		newLines = append(newLines, modelLine)
	}
	newLines = append(newLines, lines[end:]...)
	return strings.Join(newLines, "\n")
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
