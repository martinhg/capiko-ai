// Package drift detects when installed skills or agents have fallen behind the
// embedded catalog. After a capiko-ai upgrade the binary may ship newer content;
// comparing each installed item's recorded checksum (state.json) against the
// current catalog content reveals which ones a sync would refresh.
package drift

import (
	"github.com/martinhg/capiko-ai/internal/agent"
	"github.com/martinhg/capiko-ai/internal/skill"
	"github.com/martinhg/capiko-ai/internal/state"
)

// StaleAgents returns the names of catalog agents that are either missing from
// state (never installed) or whose recorded checksum no longer matches the
// catalog content. Results are in catalog order. Agents tracked in state but
// absent from the catalog are ignored (nothing to upgrade them to).
func StaleAgents(catalog []agent.Agent, st *state.State) []string {
	if st == nil {
		return nil
	}
	var out []string
	for _, a := range catalog {
		rec, ok := st.Agents[a.Name]
		if !ok {
			// Absent from state — missing install.
			out = append(out, a.Name)
			continue
		}
		if rec.Checksum != state.Checksum(a.CanonicalContent()) {
			out = append(out, a.Name)
		}
	}
	return out
}

// Stale returns the names of installed skills whose recorded checksum no longer
// matches the current catalog content, in catalog order. Skills that are not
// tracked in state (never installed by capiko) and installed skills that are
// unchanged are not stale. A catalog skill missing from state is simply not
// installed; a state skill missing from the catalog is ignored (nothing to
// upgrade it to).
func Stale(catalog []skill.Skill, st *state.State) []string {
	if st == nil {
		return nil
	}
	var out []string
	for _, sk := range catalog {
		rec, ok := st.Skills[sk.Name]
		if !ok {
			continue
		}
		if rec.Checksum != state.Checksum(sk.CanonicalContent()) {
			out = append(out, sk.Name)
		}
	}
	return out
}
