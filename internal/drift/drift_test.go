package drift

import (
	"reflect"
	"testing"

	"github.com/martinhg/capiko-ai/internal/skill"
	"github.com/martinhg/capiko-ai/internal/state"
)

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
