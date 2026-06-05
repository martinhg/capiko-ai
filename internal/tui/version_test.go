package tui

import "testing"

func TestResolveVersion(t *testing.T) {
	tests := []struct {
		name            string
		injected, build string
		want            string
	}{
		{"ldflags injected wins", "1.0.0", "v9.9.9", "1.0.0"},
		{"ldflags wins with empty build", "1.2.3", "", "1.2.3"},
		{"go install recovers from build info", "dev", "v1.4.0", "1.4.0"},
		{"plain go build stays dev", "dev", "(devel)", "dev"},
		{"empty build stays dev", "dev", "", "dev"},
		{"build version is trimmed", "dev", "  v2.0.0  ", "2.0.0"},
		{"pseudo-version stays dev", "dev", "v1.0.1-0.20260605163409-8dd8ce81334f", "dev"},
		{"pre-release stays dev", "dev", "v1.2.0-rc1", "dev"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := resolveVersion(tc.injected, tc.build); got != tc.want {
				t.Errorf("resolveVersion(%q, %q) = %q, want %q", tc.injected, tc.build, got, tc.want)
			}
		})
	}
}
