package drift

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/martinhg/capiko-ai/internal/agent"
	"github.com/martinhg/capiko-ai/internal/copilothooks"
	"github.com/martinhg/capiko-ai/internal/engram"
	"github.com/martinhg/capiko-ai/internal/headroom"
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

func TestStaleHeadroom(t *testing.T) {
	path := filepath.Join(t.TempDir(), "mcp-config.json")

	if StaleHeadroom(path, &state.State{}) {
		t.Error("unmanaged headroom should not be stale")
	}
	if StaleHeadroom(path, &state.State{Headroom: &state.HeadroomRecord{Enabled: false}}) {
		t.Error("disabled headroom should not be stale")
	}

	rec := &state.HeadroomRecord{Enabled: true, Checksum: engram.EntryChecksum(headroom.CopilotCLIEntry())}
	if !StaleHeadroom(path, &state.State{Headroom: rec}) {
		t.Error("enabled headroom with no on-disk entry should be stale")
	}

	if err := engram.MergeMCPEntry(path, "mcpServers", headroom.ServerName, headroom.CopilotCLIEntry()); err != nil {
		t.Fatal(err)
	}
	if StaleHeadroom(path, &state.State{Headroom: rec}) {
		t.Error("a matching on-disk entry should not be stale")
	}

	if !StaleHeadroom(path, &state.State{Headroom: &state.HeadroomRecord{Enabled: true, Checksum: "different"}}) {
		t.Error("a diverged recorded checksum should be stale")
	}
}

// TestStaleCopilotHooks covers unmanaged/disabled -> false, a matching
// checksum -> false, a hand-edited file -> true, and a missing file while
// enabled -> true (REQ-7.3, SC-08/SC-09).
func TestStaleCopilotHooks(t *testing.T) {
	hooksDir := filepath.Join(t.TempDir(), "hooks")

	if StaleCopilotHooks(hooksDir, nil) {
		t.Error("nil state should not be stale")
	}
	if StaleCopilotHooks(hooksDir, &state.State{}) {
		t.Error("unmanaged copilot hooks should not be stale")
	}
	if StaleCopilotHooks(hooksDir, &state.State{CopilotHooks: &state.CopilotHooksRecord{Enabled: false}}) {
		t.Error("disabled copilot hooks should not be stale")
	}

	// A recorded (non-empty) checksum with nothing on disk is stale, regardless
	// of the checksum's exact value — CombinedChecksum on a missing dir is "".
	rec := &state.CopilotHooksRecord{Enabled: true, Posture: string(copilothooks.PostureStrict), Checksum: "expected-checksum"}
	if !StaleCopilotHooks(hooksDir, &state.State{CopilotHooks: rec}) {
		t.Error("enabled copilot hooks with no on-disk file should be stale")
	}

	// Write the file capiko would render and record its combined checksum.
	hf, err := copilothooks.RenderGuardrails(copilothooks.PostureStrict)
	if err != nil {
		t.Fatal(err)
	}
	data, err := copilothooks.Marshal(hf)
	if err != nil {
		t.Fatal(err)
	}
	if err := copilothooks.WriteHookFile(hooksDir, copilothooks.GuardrailsFile, data); err != nil {
		t.Fatal(err)
	}
	rec.Checksum = copilothooks.CombinedChecksum(hooksDir)
	if StaleCopilotHooks(hooksDir, &state.State{CopilotHooks: rec}) {
		t.Error("a matching on-disk file should not be stale")
	}

	// Hand-edit the file: checksum diverges.
	target := filepath.Join(hooksDir, copilothooks.GuardrailsFile)
	if err := os.WriteFile(target, append(data, []byte("tampered")...), 0o644); err != nil {
		t.Fatal(err)
	}
	if !StaleCopilotHooks(hooksDir, &state.State{CopilotHooks: rec}) {
		t.Error("a hand-edited file should be stale")
	}

	// Remove the file entirely while still enabled.
	if err := os.Remove(target); err != nil {
		t.Fatal(err)
	}
	if !StaleCopilotHooks(hooksDir, &state.State{CopilotHooks: rec}) {
		t.Error("a missing file while enabled should be stale")
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
