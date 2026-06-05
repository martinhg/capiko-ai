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
		{"plain go build falls back to base", "dev", "(devel)", baseVersion},
		{"empty build falls back to base", "dev", "", baseVersion},
		{"build version is trimmed", "dev", "  v2.0.0  ", "2.0.0"},
		{"pseudo-version falls back to base", "dev", "v1.0.1-0.20260605163409-8dd8ce81334f", baseVersion},
		{"pre-release falls back to base", "dev", "v1.2.0-rc1", baseVersion},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := resolveVersion(tc.injected, tc.build); got != tc.want {
				t.Errorf("resolveVersion(%q, %q) = %q, want %q", tc.injected, tc.build, got, tc.want)
			}
		})
	}
}
