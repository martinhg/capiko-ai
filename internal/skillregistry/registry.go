// Package skillregistry is capiko's native skill-registry engine. It scans the
// Copilot skill directories on disk and indexes every installed skill by its
// trigger/description and path, so an orchestrator can resolve the exact
// SKILL.md paths to inject into a sub-agent before delegating work.
//
// It mirrors the native SDD engine (internal/sddstatus): deterministic Go that
// the agent shells out for (`capiko-ai skill-registry`) instead of inferring
// from loose markdown. The registry is an index, not a summary — it lists
// paths so callers load the full SKILL.md and preserve author intent.
package skillregistry

import (
	"os"
	"path/filepath"
	"sort"

	"github.com/martinhg/capiko-ai/internal/skill"
)

// Entry is one indexed skill: its name, the frontmatter description (which
// embeds the trigger), the scope it was found in, and the absolute path to its
// SKILL.md.
type Entry struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Scope       string `json:"scope"` // "user" | "project"
	Path        string `json:"path"`  // absolute path to the SKILL.md
}

// Registry is the resolved index: the project label, every candidate source
// directory that was scanned (whether or not it existed), and the entries found
// across them, sorted by name then scope.
type Registry struct {
	Project string   `json:"project"`
	Sources []string `json:"sources"`
	Entries []Entry  `json:"entries"`
}

// ResolveOptions selects where to scan. Empty fields fall back to the process
// working directory and the user's home directory; tests override both.
type ResolveOptions struct {
	Cwd  string
	Home string
}

// source pairs a candidate skills directory with the scope its skills carry.
type source struct {
	dir   string
	scope string
}

// Resolve scans the user and project Copilot skill directories and returns the
// indexed registry. Missing directories are skipped, not errors — the same
// friendly contract as the rest of capiko's host adapter.
func Resolve(opts ResolveOptions) (Registry, error) {
	cwd := opts.Cwd
	if cwd == "" {
		wd, err := os.Getwd()
		if err != nil {
			return Registry{}, err
		}
		cwd = wd
	}
	home := opts.Home
	if home == "" {
		h, err := os.UserHomeDir()
		if err != nil {
			return Registry{}, err
		}
		home = h
	}

	sources := []source{
		{dir: filepath.Join(home, ".copilot", "skills"), scope: "user"},
		{dir: filepath.Join(cwd, ".copilot", "skills"), scope: "project"},
	}

	reg := Registry{Project: filepath.Base(cwd)}
	for _, s := range sources {
		reg.Sources = append(reg.Sources, s.dir)
		entries, err := scan(s)
		if err != nil {
			return Registry{}, err
		}
		reg.Entries = append(reg.Entries, entries...)
	}

	sort.Slice(reg.Entries, func(i, j int) bool {
		if reg.Entries[i].Name != reg.Entries[j].Name {
			return reg.Entries[i].Name < reg.Entries[j].Name
		}
		return reg.Entries[i].Scope < reg.Entries[j].Scope
	})
	return reg, nil
}

// scan reads every skill under one source directory. A missing directory yields
// no entries. Each skill is parsed independently and a malformed SKILL.md is
// skipped, not fatal — the directory is user-controlled, so one bad third-party
// skill must not break resolution of all the others.
func scan(s source) ([]Entry, error) {
	entries, err := os.ReadDir(s.dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var out []Entry
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		skillPath := filepath.Join(s.dir, e.Name(), "SKILL.md")
		data, err := os.ReadFile(skillPath)
		if err != nil {
			continue // not a skill directory (no SKILL.md)
		}
		sk, err := skill.Parse(e.Name(), string(data))
		if err != nil {
			continue // malformed frontmatter — skip this one, keep the rest
		}
		out = append(out, Entry{
			Name:        sk.Name,
			Description: sk.Description,
			Scope:       s.scope,
			Path:        skillPath,
		})
	}
	return out, nil
}
