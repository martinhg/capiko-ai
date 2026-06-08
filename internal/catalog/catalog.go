// Package catalog provides the built-in capiko skill and agent catalogs,
// embedded in the binary at build time. Edit the SKILL.md files under skills/
// or the .agent.md files under agents/ and rebuild to change what the
// configurator offers.
package catalog

import (
	"embed"
	"io/fs"

	"github.com/martinhg/capiko-ai/internal/agent"
	"github.com/martinhg/capiko-ai/internal/skill"
)

//go:embed skills
//go:embed agents
var files embed.FS

// Load parses the embedded skill catalog into skills. It fails only if an
// embedded SKILL.md is malformed — an authoring error caught by tests, not at
// runtime.
func Load() ([]skill.Skill, error) {
	sub, err := fs.Sub(files, "skills")
	if err != nil {
		return nil, err
	}
	return skill.LoadCatalog(sub)
}

// LoadAgents parses the embedded agent catalog into agents. It fails only if
// an embedded .agent.md is malformed — an authoring error caught by tests, not
// at runtime.
func LoadAgents() ([]agent.Agent, error) {
	sub, err := fs.Sub(files, "agents")
	if err != nil {
		return nil, err
	}
	return agent.LoadCatalog(sub)
}
