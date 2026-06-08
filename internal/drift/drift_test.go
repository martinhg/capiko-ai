package drift

import (
	"path/filepath"
	"reflect"
	"testing"

	"github.com/martinhg/capiko-ai/internal/agent"
	"github.com/martinhg/capiko-ai/internal/engram"
	"github.com/martinhg/capiko-ai/internal/skill"
	"github.com/martinhg/capiko-ai/internal/state"
)

func TestStaleEngram(t *testing.T) {
	path := filepath.Join(t.TempDir(), "mcp-config.json")

	if StaleEngram(path, &state.State{}) {
		t.Error("unmanaged engram should not be stale")
	}
	if StaleEngram(path, &state.State{Engram: &state.EngramRecord{Enabled: false}}) {
		t.Error("disabled engram should not be stale")
	}

	rec := &state.EngramRecord{Enabled: true, Checksum: engram.EntryChecksum(engram.CopilotCLIEntry(""))}
	if !StaleEngram(path, &state.State{Engram: rec}) {
		t.Error("enabled engram with no on-disk entry should be stale")
	}

	if err := engram.MergeMCPEntry(path, "mcpServers", "engram", engram.CopilotCLIEntry("")); err != nil {
		t.Fatal(err)
	}
	if StaleEngram(path, &state.State{Engram: rec}) {
		t.Error("a matching on-disk entry should not be stale")
	}

	if !StaleEngram(path, &state.State{Engram: &state.EngramRecord{Enabled: true, Checksum: "different"}}) {
		t.Error("a diverged recorded checksum should be stale")
	}
}

func catalog() []skill.Skill {
	return []skill.Skill{
		{Name: "alpha", Content: "alpha-v2"},
		{Name: "beta", Content: "beta-v1"},
		{Name: "gamma", Content: "gamma-v1"},
	}
}

func stateWith(records map[string]string) *state.State {
	skills := map[string]state.SkillRecord{}
	for name, checksum := range records {
		skills[name] = state.SkillRecord{Checksum: checksum}
	}
	return &state.State{Skills: skills}
}

func TestStale(t *testing.T) {
	tests := []struct {
		name    string
		records map[string]string
		want    []string
	}{
		{
			name:    "all up to date",
			records: map[string]string{"alpha": state.Checksum("alpha-v2"), "beta": state.Checksum("beta-v1")},
			want:    nil,
		},
		{
			name:    "one drifted",
			records: map[string]string{"alpha": state.Checksum("alpha-v1"), "beta": state.Checksum("beta-v1")},
			want:    []string{"alpha"},
		},
		{
			name:    "stale reported in catalog order",
			records: map[string]string{"beta": "old", "alpha": "old"},
			want:    []string{"alpha", "beta"},
		},
		{
			name:    "uninstalled skills are not stale",
			records: map[string]string{},
			want:    nil,
		},
		{
			name:    "state skill missing from catalog is ignored",
			records: map[string]string{"alpha": state.Checksum("alpha-v2"), "deleted": "whatever"},
			want:    nil,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := Stale(catalog(), stateWith(tc.records))
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("Stale = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestStaleNilState(t *testing.T) {
	if got := Stale(catalog(), nil); got != nil {
		t.Errorf("Stale(nil) = %v, want nil", got)
	}
}

// agentCatalog returns a small catalog of agent.Agent for drift tests.
func agentCatalog() []agent.Agent {
	return []agent.Agent{
		{Name: "capiko-sdd-apply", Content: "apply-content-v1"},
		{Name: "capiko-sdd-spec", Content: "spec-content-v1"},
		{Name: "capiko-sdd-verify", Content: "verify-content-v1"},
	}
}

// agentStateWith builds a *state.State carrying AgentRecords for the given
// name→checksum map, so tests don't have to import state internals directly.
func agentStateWith(records map[string]string) *state.State {
	agents := map[string]state.AgentRecord{}
	for name, checksum := range records {
		agents[name] = state.AgentRecord{Checksum: checksum}
	}
	return &state.State{
		Skills: map[string]state.SkillRecord{},
		Agents: agents,
	}
}

func TestStaleAgents_AllInSync(t *testing.T) {
	cat := agentCatalog()
	st := agentStateWith(map[string]string{
		"capiko-sdd-apply":  state.Checksum("apply-content-v1"),
		"capiko-sdd-spec":   state.Checksum("spec-content-v1"),
		"capiko-sdd-verify": state.Checksum("verify-content-v1"),
	})

	got := StaleAgents(cat, st)
	if len(got) != 0 {
		t.Errorf("StaleAgents = %v, want nil (all in sync)", got)
	}
}

func TestStaleAgents_MissingAgent(t *testing.T) {
	cat := agentCatalog()
	// capiko-sdd-spec is absent from state.
	st := agentStateWith(map[string]string{
		"capiko-sdd-apply":  state.Checksum("apply-content-v1"),
		"capiko-sdd-verify": state.Checksum("verify-content-v1"),
	})

	got := StaleAgents(cat, st)
	if !reflect.DeepEqual(got, []string{"capiko-sdd-spec"}) {
		t.Errorf("StaleAgents = %v, want [capiko-sdd-spec]", got)
	}
}

func TestStaleAgents_ChangedContent(t *testing.T) {
	cat := agentCatalog()
	// capiko-sdd-apply has a stale checksum.
	st := agentStateWith(map[string]string{
		"capiko-sdd-apply":  state.Checksum("apply-content-OLD"),
		"capiko-sdd-spec":   state.Checksum("spec-content-v1"),
		"capiko-sdd-verify": state.Checksum("verify-content-v1"),
	})

	got := StaleAgents(cat, st)
	if !reflect.DeepEqual(got, []string{"capiko-sdd-apply"}) {
		t.Errorf("StaleAgents = %v, want [capiko-sdd-apply]", got)
	}
}
