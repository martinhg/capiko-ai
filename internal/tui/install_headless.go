package tui

import (
	"fmt"

	"github.com/martinhg/capiko-ai/internal/agent"
	"github.com/martinhg/capiko-ai/internal/backup"
	"github.com/martinhg/capiko-ai/internal/copilot"
	"github.com/martinhg/capiko-ai/internal/skill"
	"github.com/martinhg/capiko-ai/internal/state"
)

// InstallAll installs every catalog skill and agent that is not already
// present on disk, records the result in state, and backs up the affected
// skills and agent files first (in a single snapshot via CreateWithAgents).
// It is additive-only: already-installed items are left untouched, and nothing
// is ever removed. A nil store or backup degrades gracefully (changes still
// apply, but are not recorded/snapshotted).
//
// Already-installed skills/agents are discovered from state when store is
// non-nil; otherwise they are discovered straight from disk via
// host.InstalledSkills/host.InstalledAgents.
func InstallAll(host *copilot.Host, catalog []skill.Skill, agentCatalog []agent.Agent, store *state.Store, bkp *backup.Store) (ReconcileResult, error) {
	installedSkills, err := alreadyInstalledSkills(host, store)
	if err != nil {
		return ReconcileResult{}, err
	}
	installedAgents, err := alreadyInstalledAgents(host, store)
	if err != nil {
		return ReconcileResult{}, err
	}

	var toInstall []skill.Skill
	for _, sk := range catalog {
		if !installedSkills[sk.Name] {
			toInstall = append(toInstall, sk)
		}
	}
	var agentsToInstall []agent.Agent
	for _, a := range agentCatalog {
		if !installedAgents[a.Name] {
			agentsToInstall = append(agentsToInstall, a)
		}
	}

	if len(toInstall) == 0 && len(agentsToInstall) == 0 {
		return ReconcileResult{}, nil
	}

	if bkp != nil && (len(toInstall) > 0 || len(agentsToInstall) > 0) {
		skillNames := make([]string, len(toInstall))
		for i, sk := range toInstall {
			skillNames[i] = sk.Name
		}
		agentNames := make([]string, len(agentsToInstall))
		for i, a := range agentsToInstall {
			agentNames[i] = a.Name
		}
		if _, err := bkp.CreateWithAgents(host.SkillsDir, host.AgentsDir, "install", Version, skillNames, agentNames); err != nil {
			return ReconcileResult{}, fmt.Errorf("backup failed, aborting: %w", err)
		}
	}

	var res ReconcileResult
	recorded := make([]state.Installed, 0, len(toInstall))
	for _, sk := range toInstall {
		if _, err := sk.Install(host.SkillsDir); err != nil {
			return ReconcileResult{}, fmt.Errorf("installing %s: %w", sk.Name, err)
		}
		res.InstalledSkills = append(res.InstalledSkills, sk.Name)
		recorded = append(recorded, state.Installed{Name: sk.Name, Checksum: state.Checksum(sk.CanonicalContent())})
	}

	agentRecorded := make([]state.Installed, 0, len(agentsToInstall))
	for _, a := range agentsToInstall {
		if _, err := a.Install(host.AgentsDir); err != nil {
			return ReconcileResult{}, fmt.Errorf("installing agent %s: %w", a.Name, err)
		}
		res.InstalledAgents = append(res.InstalledAgents, a.Name)
		agentRecorded = append(agentRecorded, state.Installed{Name: a.Name, Checksum: state.Checksum(a.CanonicalContent())})
	}

	if store != nil {
		if len(recorded) > 0 {
			if err := store.Apply(Version, recorded, nil); err != nil {
				return ReconcileResult{}, fmt.Errorf("recording state: %w", err)
			}
		}
		if len(agentRecorded) > 0 {
			if err := store.ApplyAgents(Version, agentRecorded, nil); err != nil {
				return ReconcileResult{}, fmt.Errorf("recording agent state: %w", err)
			}
		}
	}

	return res, nil
}

// alreadyInstalledSkills reports which catalog skills are already present,
// preferring state (when available) over a disk scan.
func alreadyInstalledSkills(host *copilot.Host, store *state.Store) (map[string]bool, error) {
	if store != nil {
		st, err := store.Load()
		if err != nil {
			return nil, fmt.Errorf("loading state: %w", err)
		}
		installed := make(map[string]bool, len(st.Skills))
		for name := range st.Skills {
			installed[name] = true
		}
		return installed, nil
	}
	installed, err := host.InstalledSkills()
	if err != nil {
		return nil, fmt.Errorf("scanning installed skills: %w", err)
	}
	return installed, nil
}

// alreadyInstalledAgents reports which catalog agents are already present,
// preferring state (when available) over a disk scan.
func alreadyInstalledAgents(host *copilot.Host, store *state.Store) (map[string]bool, error) {
	if store != nil {
		st, err := store.Load()
		if err != nil {
			return nil, fmt.Errorf("loading state: %w", err)
		}
		installed := make(map[string]bool, len(st.Agents))
		for name := range st.Agents {
			installed[name] = true
		}
		return installed, nil
	}
	installed, err := host.InstalledAgents()
	if err != nil {
		return nil, fmt.Errorf("scanning installed agents: %w", err)
	}
	return installed, nil
}
