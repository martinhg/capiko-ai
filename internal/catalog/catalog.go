// Package catalog provides the built-in capiko skill catalog, embedded in the
// binary at build time (the same approach gentle-ai uses). Edit the SKILL.md
// files under skills/ and rebuild to change what the configurator offers.
package catalog

import (
	"embed"
	"io/fs"

	"github.com/martinhg/capiko-ai/internal/skill"
)

//go:embed skills
var files embed.FS

// Load parses the embedded catalog into skills. It fails only if an embedded
// SKILL.md is malformed — an authoring error caught by tests, not at runtime.
func Load() ([]skill.Skill, error) {
	sub, err := fs.Sub(files, "skills")
	if err != nil {
		return nil, err
	}
	return skill.LoadCatalog(sub)
}
