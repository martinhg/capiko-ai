package reconcile

import (
	"fmt"
	"sort"

	"github.com/martinhg/capiko-ai/internal/agent"
	"github.com/martinhg/capiko-ai/internal/backup"
	"github.com/martinhg/capiko-ai/internal/copilot"
	"github.com/martinhg/capiko-ai/internal/skill"
	"github.com/martinhg/capiko-ai/internal/state"
)

// ReconcileResult is the outcome of a headless install or uninstall
// operation: which skills and agents were installed or removed. A single
// command only ever populates one side (installed for `install`, removed for
// `uninstall`); `sync` populates installed only, mirroring the catalog it
// just wrote.
type ReconcileResult struct {
	InstalledSkills []string
	InstalledAgents []string
	RemovedSkills   []string
	RemovedAgents   []string
}

// TotalChanged returns the total number of items installed or removed.
func (r ReconcileResult) TotalChanged() int {
	return len(r.InstalledSkills) + len(r.InstalledAgents) +
		len(r.RemovedSkills) + len(r.RemovedAgents)
}

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
//
// version is recorded in state and backups. postInstall, when non-nil, runs
// after state is recorded and only if something was installed.
func InstallAll(host *copilot.Host, catalog []skill.Skill, agentCatalog []agent.Agent, store *state.Store, bkp *backup.Store, version string, postInstall func() error) (ReconcileResult, error) {
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
		if _, err := bkp.CreateWithAgents(host.SkillsDir, host.AgentsDir, "install", version, skillNames, agentNames); err != nil {
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
			if err := store.Apply(version, recorded, nil); err != nil {
				return ReconcileResult{}, fmt.Errorf("recording state: %w", err)
			}
		}
		if len(agentRecorded) > 0 {
			if err := store.ApplyAgents(version, agentRecorded, nil); err != nil {
				return ReconcileResult{}, fmt.Errorf("recording agent state: %w", err)
			}
		}
	}

	if postInstall != nil {
		if err := postInstall(); err != nil {
			return ReconcileResult{}, fmt.Errorf("post-install: %w", err)
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

// UninstallAll removes every capiko-managed skill and agent from the host, backs
// up the affected skills and agent files first (in a single snapshot via
// CreateWithAgents), and clears them from state. It is the inverse of
// InstallAll: only items recorded in state are touched, in deterministic
// (sorted) order.
//
// A nil store is REFUSED, not degraded. Uninstall is destructive, and without
// state capiko cannot distinguish the skills/agents it installed from
// user-authored ones on disk. A nil store means state.DefaultStore() failed
// (e.g. an undetectable home dir), NOT "remove everything found on disk" —
// refusing avoids deleting unmanaged files. A nil backup still degrades
// gracefully (removal proceeds without a snapshot).
//
// Backup scope: both skills (directory layout) and agent files (flat
// <name>.agent.md layout) are snapshotted in a single backup via
// CreateWithAgents, so a restore reinstates whatever this op removed.
//
// Scope guard: UninstallAll only calls UninstallSkill, UninstallAgent,
// store.Apply, and store.ApplyAgents. It never touches persona, SDDModels,
// Engram, or any other state field — so copilot-instructions.md persona/SDD
// blocks and the engram MCP entry are naturally preserved.
//
// Consistency: removals are recorded in state even on a mid-loop failure — the
// items already removed are flushed before the error is returned — so state
// never claims an item is installed after it has been deleted from disk.
func UninstallAll(host *copilot.Host, store *state.Store, bkp *backup.Store, version string) (ReconcileResult, error) {
	if store == nil {
		return ReconcileResult{}, fmt.Errorf("state store unavailable: cannot determine capiko-managed items to uninstall")
	}

	skillNames, agentNames, err := ManagedItems(store)
	if err != nil {
		return ReconcileResult{}, err
	}

	if len(skillNames) == 0 && len(agentNames) == 0 {
		return ReconcileResult{}, nil
	}

	if bkp != nil && (len(skillNames) > 0 || len(agentNames) > 0) {
		if _, err := bkp.CreateWithAgents(host.SkillsDir, host.AgentsDir, "uninstall", version, skillNames, agentNames); err != nil {
			return ReconcileResult{}, fmt.Errorf("backup failed, aborting: %w", err)
		}
	}

	var res ReconcileResult
	for _, name := range skillNames {
		if err := host.UninstallSkill(name); err != nil {
			_ = recordRemovals(store, version, res) // best-effort: keep state consistent with disk
			return res, fmt.Errorf("uninstalling %s: %w", name, err)
		}
		res.RemovedSkills = append(res.RemovedSkills, name)
	}
	for _, name := range agentNames {
		if err := host.UninstallAgent(name); err != nil {
			_ = recordRemovals(store, version, res)
			return res, fmt.Errorf("uninstalling agent %s: %w", name, err)
		}
		res.RemovedAgents = append(res.RemovedAgents, name)
	}

	if err := recordRemovals(store, version, res); err != nil {
		return res, fmt.Errorf("recording state: %w", err)
	}
	return res, nil
}

// recordRemovals persists the removed skill/agent names to state. The caller
// guarantees store is non-nil.
func recordRemovals(store *state.Store, version string, res ReconcileResult) error {
	if len(res.RemovedSkills) > 0 {
		if err := store.Apply(version, nil, res.RemovedSkills); err != nil {
			return err
		}
	}
	if len(res.RemovedAgents) > 0 {
		if err := store.ApplyAgents(version, nil, res.RemovedAgents); err != nil {
			return err
		}
	}
	return nil
}

// ManagedItems returns the skill and agent names recorded in state — the items
// capiko installed and may remove — each sorted for deterministic removal order.
// The caller guarantees store is non-nil.
func ManagedItems(store *state.Store) (skills, agents []string, err error) {
	st, err := store.Load()
	if err != nil {
		return nil, nil, fmt.Errorf("loading state: %w", err)
	}
	skills = make([]string, 0, len(st.Skills))
	for name := range st.Skills {
		skills = append(skills, name)
	}
	agents = make([]string, 0, len(st.Agents))
	for name := range st.Agents {
		agents = append(agents, name)
	}
	sort.Strings(skills)
	sort.Strings(agents)
	return skills, agents, nil
}
