package tui

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/martinhg/capiko-ai/internal/cga"
	"github.com/martinhg/capiko-ai/internal/copilot"
	"github.com/martinhg/capiko-ai/internal/copilothooks"
	"github.com/martinhg/capiko-ai/internal/efficiency"
	"github.com/martinhg/capiko-ai/internal/engram"
	"github.com/martinhg/capiko-ai/internal/githooks"
	"github.com/martinhg/capiko-ai/internal/headroom"
	"github.com/martinhg/capiko-ai/internal/instructions"
	"github.com/martinhg/capiko-ai/internal/memory"
	"github.com/martinhg/capiko-ai/internal/persona"
	"github.com/martinhg/capiko-ai/internal/scoped"
	"github.com/martinhg/capiko-ai/internal/sdd"
	"github.com/martinhg/capiko-ai/internal/state"
	"github.com/martinhg/capiko-ai/internal/trigger"
)

// RDD git hook markers — duplicated here because the originals in cmd/capiko-ai
// are in package main and inaccessible.
const (
	rddPreCommitMarkerStart = "# >>> capiko:rdd:pre-commit >>>"
	rddPreCommitMarkerEnd   = "# <<< capiko:rdd:pre-commit <<<"
	rddPrePushMarkerStart   = "# >>> capiko:rdd:pre-push >>>"
	rddPrePushMarkerEnd     = "# <<< capiko:rdd:pre-push <<<"
)

// PurgeConfig holds everything PurgeAll needs from the outside world.
type PurgeConfig struct {
	Host      *copilot.Host
	Store     *state.Store
	Workspace string
}

// PurgeReport tracks what was removed per category.
type PurgeReport struct {
	Skills             []string
	Agents             []string
	InstructionBlocks  []string
	MCPEntries         []string
	CopilotHooks       []string
	GitHooks           []string
	ScopedInstructions []string
	WorkspaceArtifacts []string
	HomeArtifacts      []string
	VSCodeEntries      []string
	Warnings           []string
}

// TotalRemoved returns the total number of items removed across all categories.
func (r PurgeReport) TotalRemoved() int {
	return len(r.Skills) + len(r.Agents) + len(r.InstructionBlocks) +
		len(r.MCPEntries) + len(r.CopilotHooks) + len(r.GitHooks) +
		len(r.ScopedInstructions) + len(r.WorkspaceArtifacts) +
		len(r.HomeArtifacts) + len(r.VSCodeEntries)
}

var (
	purgeGitDir          = defaultPurgeGitDir
	purgeGitCommonDir    = defaultPurgeGitCommonDir
	purgeVSCodeUserPath  = engram.VSCodeUserMCPPath
	purgeHomeDir         = os.UserHomeDir
	purgeRemoveAll       = os.RemoveAll
	purgeRemove          = os.Remove
	purgeScopedLoad      = scoped.Load
	purgeCustomInstrDirs = copilot.CustomInstructionDirs
)

func defaultPurgeGitDir(workspace string) (string, error) {
	cmd := exec.Command("git", "-C", workspace, "rev-parse", "--git-dir")
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	dir := strings.TrimSpace(string(out))
	if !filepath.IsAbs(dir) {
		dir = filepath.Join(workspace, dir)
	}
	return dir, nil
}

func defaultPurgeGitCommonDir(workspace string) (string, error) {
	cmd := exec.Command("git", "-C", workspace, "rev-parse", "--git-common-dir")
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	dir := strings.TrimSpace(string(out))
	if !filepath.IsAbs(dir) {
		dir = filepath.Join(workspace, dir)
	}
	return dir, nil
}

// PurgeAll removes every trace of capiko from the system. It is best-effort:
// individual category failures produce warnings, not errors. Only a truly
// unrecoverable failure (e.g. cannot determine home directory) returns an error.
func PurgeAll(cfg PurgeConfig) (PurgeReport, error) {
	var rpt PurgeReport

	purgeSkillsAndAgents(cfg, &rpt)
	purgeInstructionBlocks(cfg, &rpt)
	purgeMCPEntries(cfg, &rpt)
	purgeCopilotHooks(cfg, &rpt)
	purgeVerifiedFile(cfg, &rpt)
	purgeScopedInstructions(cfg, &rpt)
	purgeVSCodeMCPEntries(cfg, &rpt)
	purgeWorkspaceGitHooks(cfg, &rpt)
	purgeWorkspaceCapikoDir(cfg, &rpt)
	purgeEngramConfig(cfg, &rpt)
	if err := purgeCapikoHome(&rpt); err != nil {
		return rpt, err
	}

	return rpt, nil
}

func purgeSkillsAndAgents(cfg PurgeConfig, rpt *PurgeReport) {
	if cfg.Store == nil {
		rpt.Warnings = append(rpt.Warnings, "state store unavailable: skipping skill/agent removal")
		return
	}

	skills, agents, err := managedItems(cfg.Store)
	if err != nil {
		rpt.Warnings = append(rpt.Warnings, "loading managed items: "+err.Error())
		return
	}

	for _, name := range skills {
		if err := cfg.Host.UninstallSkill(name); err != nil {
			rpt.Warnings = append(rpt.Warnings, fmt.Sprintf("removing skill %s: %s", name, err))
			continue
		}
		rpt.Skills = append(rpt.Skills, name)
	}
	for _, name := range agents {
		if err := cfg.Host.UninstallAgent(name); err != nil {
			rpt.Warnings = append(rpt.Warnings, fmt.Sprintf("removing agent %s: %s", name, err))
			continue
		}
		rpt.Agents = append(rpt.Agents, name)
	}
}

type instructionMarker struct {
	name  string
	start string
	end   string
}

var allInstructionMarkers = []instructionMarker{
	{"persona", persona.MarkerStart, persona.MarkerEnd},
	{"sdd", sdd.MarkerStart, sdd.MarkerEnd},
	{"memory", memory.MarkerStart, memory.MarkerEnd},
	{"trigger", trigger.MarkerStart, trigger.MarkerEnd},
	{"headroom", headroom.GuidanceMarkerStart, headroom.GuidanceMarkerEnd},
	{"efficiency", efficiency.MarkerStart, efficiency.MarkerEnd},
}

func purgeInstructionBlocks(cfg PurgeConfig, rpt *PurgeReport) {
	path := filepath.Join(cfg.Host.ConfigDir, "copilot-instructions.md")
	for _, m := range allInstructionMarkers {
		content, changed, err := instructions.Render(path, m.start, m.end, "")
		if err != nil {
			rpt.Warnings = append(rpt.Warnings, fmt.Sprintf("removing %s block: %s", m.name, err))
			continue
		}
		if !changed {
			continue
		}
		if err := instructions.Write(path, content); err != nil {
			rpt.Warnings = append(rpt.Warnings, fmt.Sprintf("writing instructions after removing %s: %s", m.name, err))
			continue
		}
		rpt.InstructionBlocks = append(rpt.InstructionBlocks, m.name)
	}
}

func purgeMCPEntries(cfg PurgeConfig, rpt *PurgeReport) {
	entries := []struct {
		topKey string
		name   string
	}{
		{"mcpServers", "engram"},
		{"mcpServers", headroom.ServerName},
	}
	for _, e := range entries {
		if err := engram.RemoveMCPEntry(cfg.Host.MCPConfigPath, e.topKey, e.name); err != nil {
			rpt.Warnings = append(rpt.Warnings, fmt.Sprintf("removing MCP entry %s: %s", e.name, err))
			continue
		}
		rpt.MCPEntries = append(rpt.MCPEntries, e.name)
	}
}

func purgeCopilotHooks(cfg PurgeConfig, rpt *PurgeReport) {
	hookFiles := []string{
		copilothooks.GuardrailsFile,
		copilothooks.SessionCheckFile,
		copilothooks.SessionProbeFile,
	}
	for _, name := range hookFiles {
		if err := copilothooks.RemoveHookFile(cfg.Host.HooksDir, name); err != nil {
			rpt.Warnings = append(rpt.Warnings, fmt.Sprintf("removing hook %s: %s", name, err))
			continue
		}
		rpt.CopilotHooks = append(rpt.CopilotHooks, name)
	}
}

func purgeVerifiedFile(cfg PurgeConfig, rpt *PurgeReport) {
	path := filepath.Join(cfg.Host.ConfigDir, ".capiko-verified.json")
	if err := purgeRemove(path); err != nil && !os.IsNotExist(err) {
		rpt.Warnings = append(rpt.Warnings, "removing .capiko-verified.json: "+err.Error())
		return
	}
	rpt.WorkspaceArtifacts = append(rpt.WorkspaceArtifacts, ".capiko-verified.json")
}

func purgeScopedInstructions(cfg PurgeConfig, rpt *PurgeReport) {
	ins, err := purgeScopedLoad()
	if err != nil {
		rpt.Warnings = append(rpt.Warnings, "loading scoped instructions: "+err.Error())
		return
	}

	dirs := []string{scoped.Dir(cfg.Host.ConfigDir)}
	dirs = append(dirs, purgeCustomInstrDirs()...)

	for _, dir := range dirs {
		for _, in := range ins {
			path := scoped.Path(dir, in)
			if err := purgeRemove(path); err != nil {
				if !os.IsNotExist(err) {
					rpt.Warnings = append(rpt.Warnings, fmt.Sprintf("removing scoped instruction %s: %s", path, err))
				}
				continue
			}
			rpt.ScopedInstructions = append(rpt.ScopedInstructions, in.Name)
		}
	}
}

func purgeVSCodeMCPEntries(cfg PurgeConfig, rpt *PurgeReport) {
	// Workspace-level
	if cfg.Workspace != "" {
		wsPath := filepath.Join(cfg.Workspace, ".vscode", "mcp.json")
		if err := engram.RemoveMCPEntry(wsPath, "servers", "engram"); err != nil {
			rpt.Warnings = append(rpt.Warnings, "removing VS Code workspace MCP entry: "+err.Error())
		} else {
			rpt.VSCodeEntries = append(rpt.VSCodeEntries, "workspace")
		}
	}

	// User-level
	userPath, err := purgeVSCodeUserPath()
	if err != nil {
		rpt.Warnings = append(rpt.Warnings, "resolving VS Code user MCP path: "+err.Error())
		return
	}
	if err := engram.RemoveMCPEntry(userPath, "servers", "engram"); err != nil {
		rpt.Warnings = append(rpt.Warnings, "removing VS Code user MCP entry: "+err.Error())
		return
	}
	rpt.VSCodeEntries = append(rpt.VSCodeEntries, "user")
}

type gitHookRemoval struct {
	hookName    string
	markerStart string
	markerEnd   string
	label       string
}

var allGitHookRemovals = []gitHookRemoval{
	{"pre-commit", cga.MarkerStart, cga.MarkerEnd, "cga:pre-commit"},
	{"post-commit", cga.PostCommitMarkerStart, cga.PostCommitMarkerEnd, "cga:post-commit"},
	{"post-merge", teamSyncPostMergeMarkerStart, teamSyncPostMergeMarkerEnd, "team-sync:post-merge"},
	{"pre-push", teamSyncPrePushMarkerStart, teamSyncPrePushMarkerEnd, "team-sync:pre-push"},
	{"pre-commit", rddPreCommitMarkerStart, rddPreCommitMarkerEnd, "rdd:pre-commit"},
	{"pre-push", rddPrePushMarkerStart, rddPrePushMarkerEnd, "rdd:pre-push"},
}

func purgeWorkspaceGitHooks(cfg PurgeConfig, rpt *PurgeReport) {
	if cfg.Workspace == "" {
		return
	}
	for _, h := range allGitHookRemovals {
		if err := githooks.RemoveBlock(cfg.Workspace, h.hookName, h.markerStart, h.markerEnd); err != nil {
			rpt.Warnings = append(rpt.Warnings, fmt.Sprintf("removing git hook %s: %s", h.label, err))
			continue
		}
		rpt.GitHooks = append(rpt.GitHooks, h.label)
	}
}

func purgeWorkspaceCapikoDir(cfg PurgeConfig, rpt *PurgeReport) {
	if cfg.Workspace == "" {
		return
	}

	// .git/capiko/ (per-worktree)
	gitDir, err := purgeGitDir(cfg.Workspace)
	if err == nil {
		capikoDir := filepath.Join(gitDir, "capiko")
		if err := purgeRemoveAll(capikoDir); err != nil && !os.IsNotExist(err) {
			rpt.Warnings = append(rpt.Warnings, "removing .git/capiko: "+err.Error())
		} else {
			rpt.WorkspaceArtifacts = append(rpt.WorkspaceArtifacts, ".git/capiko")
		}
	}

	// git-common-dir/capiko/rdd/ (shared across worktrees)
	commonDir, err := purgeGitCommonDir(cfg.Workspace)
	if err == nil && commonDir != gitDir {
		rddDir := filepath.Join(commonDir, "capiko", "rdd")
		if err := purgeRemoveAll(rddDir); err != nil && !os.IsNotExist(err) {
			rpt.Warnings = append(rpt.Warnings, "removing common-dir capiko/rdd: "+err.Error())
		} else {
			rpt.WorkspaceArtifacts = append(rpt.WorkspaceArtifacts, "common-dir/capiko/rdd")
		}
	}
}

func purgeEngramConfig(cfg PurgeConfig, rpt *PurgeReport) {
	if cfg.Workspace == "" {
		return
	}
	path := filepath.Join(cfg.Workspace, ".engram", "config.json")
	if err := purgeRemove(path); err != nil {
		if !os.IsNotExist(err) {
			rpt.Warnings = append(rpt.Warnings, "removing .engram/config.json: "+err.Error())
		}
		return
	}
	rpt.WorkspaceArtifacts = append(rpt.WorkspaceArtifacts, ".engram/config.json")
}

func purgeCapikoHome(rpt *PurgeReport) error {
	home, err := purgeHomeDir()
	if err != nil {
		return fmt.Errorf("cannot determine home directory: %w", err)
	}
	capikoHome := filepath.Join(home, ".capiko")
	entries, err := os.ReadDir(capikoHome)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		rpt.Warnings = append(rpt.Warnings, "reading ~/.capiko: "+err.Error())
		return nil
	}

	names := make([]string, 0, len(entries))
	for _, e := range entries {
		names = append(names, e.Name())
	}
	sort.Strings(names)

	if err := purgeRemoveAll(capikoHome); err != nil {
		rpt.Warnings = append(rpt.Warnings, "removing ~/.capiko: "+err.Error())
		return nil
	}
	for _, n := range names {
		rpt.HomeArtifacts = append(rpt.HomeArtifacts, n)
	}
	return nil
}
