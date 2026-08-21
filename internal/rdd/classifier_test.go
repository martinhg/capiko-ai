package rdd

import (
	"reflect"
	"testing"
)

func TestClassify(t *testing.T) {
	tests := []struct {
		name        string
		paths       []string
		wantTier    RiskTier
		wantLenses  int
		wantReasons []string
	}{
		{
			name:        "unrelated path defaults to tier 0",
			paths:       []string{"internal/tui/copilot-instructions.md"},
			wantTier:    RiskTierStandard,
			wantLenses:  0,
			wantReasons: nil,
		},
		{
			name:        "no changed paths defaults to tier 0",
			paths:       nil,
			wantTier:    RiskTierStandard,
			wantLenses:  0,
			wantReasons: nil,
		},
		{
			name:        "auth path is tier 1 with a named reason",
			paths:       []string{"internal/auth/middleware.go"},
			wantTier:    RiskTierElevated,
			wantLenses:  1,
			wantReasons: []string{"changed path matches authentication-sensitive pattern: internal/auth/middleware.go"},
		},
		{
			name:        "crypto path is tier 1 with a named reason",
			paths:       []string{"internal/crypto/sign.go"},
			wantTier:    RiskTierElevated,
			wantLenses:  1,
			wantReasons: []string{"changed path matches cryptography-sensitive pattern: internal/crypto/sign.go"},
		},
		{
			name:        "permissions path is tier 1 with a named reason",
			paths:       []string{"internal/permissions/acl.go"},
			wantTier:    RiskTierElevated,
			wantLenses:  1,
			wantReasons: []string{"changed path matches permissions-sensitive pattern: internal/permissions/acl.go"},
		},
		{
			name:        "secrets path is tier 1 with a named reason",
			paths:       []string{"config/secrets.yaml"},
			wantTier:    RiskTierElevated,
			wantLenses:  1,
			wantReasons: []string{"changed path matches secrets-sensitive pattern: config/secrets.yaml"},
		},
		{
			name:       "mixed sensitive and unrelated paths still tier 1 with only sensitive reasons",
			paths:      []string{"README.md", "internal/auth/session.go", "internal/tui/view.go"},
			wantTier:   RiskTierElevated,
			wantLenses: 1,
			wantReasons: []string{
				"changed path matches authentication-sensitive pattern: internal/auth/session.go",
			},
		},
		{
			name:       "multiple distinct sensitive patterns produce multiple named reasons",
			paths:      []string{"internal/auth/session.go", "internal/crypto/hash.go"},
			wantTier:   RiskTierElevated,
			wantLenses: 1,
			wantReasons: []string{
				"changed path matches authentication-sensitive pattern: internal/auth/session.go",
				"changed path matches cryptography-sensitive pattern: internal/crypto/hash.go",
			},
		},
		{
			name:       "duplicate matches against same pattern deduplicate reasons",
			paths:      []string{"internal/auth/session.go", "internal/auth/token.go"},
			wantTier:   RiskTierElevated,
			wantLenses: 1,
			wantReasons: []string{
				"changed path matches authentication-sensitive pattern: internal/auth/session.go",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Classify(tt.paths)
			if got.Tier != tt.wantTier {
				t.Errorf("Classify(%v).Tier = %v, want %v", tt.paths, got.Tier, tt.wantTier)
			}
			if got.Lenses != tt.wantLenses {
				t.Errorf("Classify(%v).Lenses = %v, want %v", tt.paths, got.Lenses, tt.wantLenses)
			}
			if !reflect.DeepEqual(got.Reasons, tt.wantReasons) {
				t.Errorf("Classify(%v).Reasons = %v, want %v", tt.paths, got.Reasons, tt.wantReasons)
			}
		})
	}
}

func TestClassify_Determinism(t *testing.T) {
	paths := []string{"internal/crypto/hash.go", "internal/auth/session.go", "README.md"}

	first := Classify(paths)
	second := Classify(paths)

	if !reflect.DeepEqual(first, second) {
		t.Errorf("Classify is not deterministic across repeated calls: %+v != %+v", first, second)
	}
}

func TestClassify_NeverDependsOnPathCount(t *testing.T) {
	// A large batch of unrelated paths must still be tier 0 — classification
	// depends only on path evidence, never on how many paths changed.
	paths := make([]string, 500)
	for i := range paths {
		paths[i] = "docs/page.md"
	}

	got := Classify(paths)
	if got.Tier != RiskTierStandard {
		t.Errorf("Classify(500 unrelated paths).Tier = %v, want %v (size must never drive tier)", got.Tier, RiskTierStandard)
	}
}
