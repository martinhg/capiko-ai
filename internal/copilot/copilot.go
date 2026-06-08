// Package copilot is the adapter to the GitHub Copilot CLI host.
//
// It locates the Copilot CLI on the system and exposes the paths capiko writes
// its layer into. It does not know about skills or the UI.
package copilot

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// customInstructionDirsEnv is the env var Copilot reads to load extra instruction
// directories beyond ~/.copilot/instructions/.
const customInstructionDirsEnv = "COPILOT_CUSTOM_INSTRUCTIONS_DIRS"

// Host describes a detected and initialized Copilot CLI installation.
type Host struct {
	BinPath       string // absolute path to the copilot binary
	ConfigDir     string // ~/.copilot
	SkillsDir     string // ~/.copilot/skills
	AgentsDir     string // ~/.copilot/agents
	MCPConfigPath string // ~/.copilot/mcp-config.json
}

// Test seams: swapped in tests so detection does not depend on the real PATH or
// home directory.
var (
	lookPath    = exec.LookPath
	userHomeDir = os.UserHomeDir
)

// Detect locates the Copilot CLI. It returns (nil, nil) when Copilot is not
// installed or has not been initialized yet (no ~/.copilot), so callers can
// show a friendly message instead of treating it as a hard error. A non-nil
// error means something unexpected went wrong (e.g. no home directory).
func Detect() (*Host, error) {
	bin, err := lookPath("copilot")
	if err != nil {
		return nil, nil // not installed
	}
	home, err := userHomeDir()
	if err != nil {
		return nil, err
	}
	cfg := filepath.Join(home, ".copilot")
	if _, err := os.Stat(cfg); err != nil {
		return nil, nil // installed but never logged in
	}
	return &Host{
		BinPath:       bin,
		ConfigDir:     cfg,
		SkillsDir:     filepath.Join(cfg, "skills"),
		AgentsDir:     filepath.Join(cfg, "agents"),
		MCPConfigPath: filepath.Join(cfg, "mcp-config.json"),
	}, nil
}

// CustomInstructionDirs returns the extra instruction directories Copilot loads
// via COPILOT_CUSTOM_INSTRUCTIONS_DIRS (comma-separated), trimmed and with empty
// entries dropped. It is the single source of truth for that env var.
func CustomInstructionDirs() []string {
	raw := os.Getenv(customInstructionDirsEnv)
	if raw == "" {
		return nil
	}
	var out []string
	for _, d := range strings.Split(raw, ",") {
		if d = strings.TrimSpace(d); d != "" {
			out = append(out, d)
		}
	}
	return out
}

// InstalledSkills returns the set of skill names already present in the host's
// skills directory — that is, every subdirectory containing a SKILL.md. A
// missing skills directory is treated as "none installed", not an error.
func (h *Host) InstalledSkills() (map[string]bool, error) {
	entries, err := os.ReadDir(h.SkillsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return map[string]bool{}, nil
		}
		return nil, err
	}
	installed := make(map[string]bool)
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		if _, err := os.Stat(filepath.Join(h.SkillsDir, e.Name(), "SKILL.md")); err == nil {
			installed[e.Name()] = true
		}
	}
	return installed, nil
}

// InstalledAgents returns the set of agent names already present in the host's
// agents directory — every file matching *.agent.md (by filename stem, not
// directory structure). A missing agents directory is treated as "none
// installed", not an error.
func (h *Host) InstalledAgents() (map[string]bool, error) {
	entries, err := os.ReadDir(h.AgentsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return map[string]bool{}, nil
		}
		return nil, err
	}
	installed := make(map[string]bool)
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if strings.HasSuffix(name, ".agent.md") {
			stem := strings.TrimSuffix(name, ".agent.md")
			installed[stem] = true
		}
	}
	return installed, nil
}

// UninstallAgent removes the agent file <name>.agent.md from the host's agents
// directory. It is idempotent: removing an agent that is not present is not an
// error. As a safety guard it refuses any name that resolves outside the agents
// directory (path traversal) or into a subdirectory, and refuses to remove a
// directory — so a bad name can never delete arbitrary files.
func (h *Host) UninstallAgent(name string) error {
	target := filepath.Join(h.AgentsDir, name+".agent.md")
	rel, err := filepath.Rel(h.AgentsDir, target)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) || strings.ContainsRune(rel, os.PathSeparator) {
		return fmt.Errorf("refusing to remove %q: resolves outside the agents directory", name)
	}
	info, err := os.Stat(target)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // already gone
		}
		return err
	}
	if info.IsDir() {
		return fmt.Errorf("%q is not an agent file", name)
	}
	return os.Remove(target)
}

// UninstallSkill removes the skill directory <name> from the host's skills
// directory. It is idempotent: removing a skill that is not present is not an
// error. As a safety guard it refuses to remove anything that does not look
// like a skill (a directory containing a SKILL.md), so a bad name can never
// delete arbitrary files.
func (h *Host) UninstallSkill(name string) error {
	dir := filepath.Join(h.SkillsDir, name)
	info, err := os.Stat(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // already gone
		}
		return err
	}
	if !info.IsDir() {
		return fmt.Errorf("%q is not a skill directory", name)
	}
	if _, err := os.Stat(filepath.Join(dir, "SKILL.md")); err != nil {
		return fmt.Errorf("refusing to remove %q: no SKILL.md", name)
	}
	return os.RemoveAll(dir)
}
