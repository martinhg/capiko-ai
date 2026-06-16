package tui

import "testing"

func TestReconcileResultTotalChanged(t *testing.T) {
	tests := []struct {
		name string
		r    ReconcileResult
		want int
	}{
		{
			name: "empty",
			r:    ReconcileResult{},
			want: 0,
		},
		{
			name: "installed only",
			r: ReconcileResult{
				InstalledSkills: []string{"capiko-dev", "go-testing"},
				InstalledAgents: []string{"capiko-onboard"},
			},
			want: 3,
		},
		{
			name: "removed only",
			r: ReconcileResult{
				RemovedSkills: []string{"capiko-dev"},
				RemovedAgents: []string{"capiko-onboard", "capiko-review"},
			},
			want: 3,
		},
		{
			name: "mixed install and remove",
			r: ReconcileResult{
				InstalledSkills: []string{"a"},
				InstalledAgents: []string{"b"},
				RemovedSkills:   []string{"c"},
				RemovedAgents:   []string{"d", "e"},
			},
			want: 5,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.r.TotalChanged(); got != tt.want {
				t.Errorf("TotalChanged() = %d, want %d", got, tt.want)
			}
		})
	}
}
