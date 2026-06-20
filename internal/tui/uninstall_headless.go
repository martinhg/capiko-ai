package tui

import (
	"fmt"
	"sort"

	"github.com/martinhg/capiko-ai/internal/backup"
	"github.com/martinhg/capiko-ai/internal/copilot"
	"github.com/martinhg/capiko-ai/internal/state"
)

// UninstallAll removes every capiko-managed skill and agent from the host, backs
// up affected skill names first, and clears them from state. It is the inverse
// of InstallAll: only items recorded in state are touched, in deterministic
// (sorted) order.
//
// A nil store is REFUSED, not degraded. Uninstall is destructive, and without
// state capiko cannot distinguish the skills/agents it installed from
// user-authored ones on disk. A nil store means state.DefaultStore() failed
// (e.g. an undetectable home dir), NOT "remove everything found on disk" —
// refusing avoids deleting unmanaged files. A nil backup still degrades
// gracefully (removal proceeds without a snapshot).
//
// Backup scope: only skills are snapshotted (via backup.Store.Create, which is
// built for the skill-directory layout). Agent files are NOT snapshotted — the
// same limitation InstallAll has — but agents are catalog-derived and
// recoverable via `capiko-ai install`. Symmetric agent backup is a known
// follow-up.
//
// Scope guard: UninstallAll only calls UninstallSkill, UninstallAgent,
// store.Apply, and store.ApplyAgents. It never touches persona, SDDModels,
// Engram, or any other state field — so copilot-instructions.md persona/SDD
// blocks and the engram MCP entry are naturally preserved.
//
// Consistency: removals are recorded in state even on a mid-loop failure — the
// items already removed are flushed before the error is returned — so state
// never claims an item is installed after it has been deleted from disk.
func UninstallAll(host *copilot.Host, store *state.Store, bkp *backup.Store) (ReconcileResult, error) {
	if store == nil {
		return ReconcileResult{}, fmt.Errorf("state store unavailable: cannot determine capiko-managed items to uninstall")
	}

	skillNames, agentNames, err := managedItems(store)
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
			_ = recordRemovals(store, res) // best-effort: keep state consistent with disk
			return res, fmt.Errorf("uninstalling %s: %w", name, err)
		}
		res.RemovedSkills = append(res.RemovedSkills, name)
	}
	for _, name := range agentNames {
		if err := host.UninstallAgent(name); err != nil {
			_ = recordRemovals(store, res)
			return res, fmt.Errorf("uninstalling agent %s: %w", name, err)
		}
		res.RemovedAgents = append(res.RemovedAgents, name)
	}

	if err := recordRemovals(store, res); err != nil {
		return res, fmt.Errorf("recording state: %w", err)
	}
	return res, nil
}

// recordRemovals persists the removed skill/agent names to state. The caller
// guarantees store is non-nil.
func recordRemovals(store *state.Store, res ReconcileResult) error {
	if len(res.RemovedSkills) > 0 {
		if err := store.Apply(Version, nil, res.RemovedSkills); err != nil {
			return err
		}
	}
	if len(res.RemovedAgents) > 0 {
		if err := store.ApplyAgents(Version, nil, res.RemovedAgents); err != nil {
			return err
		}
	}
	return nil
}

// managedItems returns the skill and agent names recorded in state — the items
// capiko installed and may remove — each sorted for deterministic removal order.
// The caller guarantees store is non-nil.
func managedItems(store *state.Store) (skills, agents []string, err error) {
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
