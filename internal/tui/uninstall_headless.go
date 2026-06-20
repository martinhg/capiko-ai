package tui

import (
	"fmt"

	"github.com/martinhg/capiko-ai/internal/backup"
	"github.com/martinhg/capiko-ai/internal/copilot"
	"github.com/martinhg/capiko-ai/internal/state"
)

// UninstallAll removes every managed skill and agent from the host, backs up
// affected skill names first, and clears them from state. It is the inverse of
// InstallAll: only previously-recorded items are touched (or, when store is
// nil, everything discovered on disk). A nil store or backup degrades
// gracefully: changes still apply, but are not recorded/snapshotted.
//
// Scope guard: UninstallAll only calls UninstallSkill, UninstallAgent,
// store.Apply, and store.ApplyAgents. It never touches persona, SDDModels,
// Engram, or any other state fields — so copilot-instructions.md persona/SDD
// blocks and the engram MCP entry are naturally preserved.
func UninstallAll(host *copilot.Host, store *state.Store, bkp *backup.Store) (ReconcileResult, error) {
	skillNames, err := managedSkills(host, store)
	if err != nil {
		return ReconcileResult{}, err
	}
	agentNames, err := managedAgents(host, store)
	if err != nil {
		return ReconcileResult{}, err
	}

	if len(skillNames) == 0 && len(agentNames) == 0 {
		return ReconcileResult{}, nil
	}

	if bkp != nil && len(skillNames) > 0 {
		if _, err := bkp.Create(host.SkillsDir, "uninstall", Version, skillNames); err != nil {
			return ReconcileResult{}, fmt.Errorf("backup failed, aborting: %w", err)
		}
	}

	var res ReconcileResult
	for _, name := range skillNames {
		if err := host.UninstallSkill(name); err != nil {
			return ReconcileResult{}, fmt.Errorf("uninstalling %s: %w", name, err)
		}
		res.RemovedSkills = append(res.RemovedSkills, name)
	}
	for _, name := range agentNames {
		if err := host.UninstallAgent(name); err != nil {
			return ReconcileResult{}, fmt.Errorf("uninstalling agent %s: %w", name, err)
		}
		res.RemovedAgents = append(res.RemovedAgents, name)
	}

	if store != nil {
		if len(res.RemovedSkills) > 0 {
			if err := store.Apply(Version, nil, res.RemovedSkills); err != nil {
				return ReconcileResult{}, fmt.Errorf("recording state: %w", err)
			}
		}
		if len(res.RemovedAgents) > 0 {
			if err := store.ApplyAgents(Version, nil, res.RemovedAgents); err != nil {
				return ReconcileResult{}, fmt.Errorf("recording agent state: %w", err)
			}
		}
	}

	return res, nil
}

// managedSkills returns the names of skills to uninstall: from state when
// store is non-nil, or by scanning the host's skills directory otherwise.
func managedSkills(host *copilot.Host, store *state.Store) ([]string, error) {
	if store != nil {
		st, err := store.Load()
		if err != nil {
			return nil, fmt.Errorf("loading state: %w", err)
		}
		names := make([]string, 0, len(st.Skills))
		for name := range st.Skills {
			names = append(names, name)
		}
		return names, nil
	}
	installed, err := host.InstalledSkills()
	if err != nil {
		return nil, fmt.Errorf("scanning installed skills: %w", err)
	}
	names := make([]string, 0, len(installed))
	for name := range installed {
		names = append(names, name)
	}
	return names, nil
}

// managedAgents returns the names of agents to uninstall: from state when
// store is non-nil, or by scanning the host's agents directory otherwise.
func managedAgents(host *copilot.Host, store *state.Store) ([]string, error) {
	if store != nil {
		st, err := store.Load()
		if err != nil {
			return nil, fmt.Errorf("loading state: %w", err)
		}
		names := make([]string, 0, len(st.Agents))
		for name := range st.Agents {
			names = append(names, name)
		}
		return names, nil
	}
	installed, err := host.InstalledAgents()
	if err != nil {
		return nil, fmt.Errorf("scanning installed agents: %w", err)
	}
	names := make([]string, 0, len(installed))
	for name := range installed {
		names = append(names, name)
	}
	return names, nil
}
