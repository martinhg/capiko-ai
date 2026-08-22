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
			// "auth" is also a tier-4 hot-path substring (auth-critical), and
			// tier 4 takes precedence over tier 1 (spec risk-classifier-extension).
			// This case documents that precedence rather than a pure tier-1 result;
			// see TestClassify_Tier4PrecedenceOverTier1 for the dedicated test.
			name:        "auth path is tier 4 due to hot-path precedence over tier 1",
			paths:       []string{"internal/auth/middleware.go"},
			wantTier:    RiskTierCritical,
			wantLenses:  4,
			wantReasons: []string{"changed path matches auth-critical pattern: internal/auth/middleware.go"},
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
			// Uses "internal/auth/session.go" deliberately: mixed with unrelated
			// paths, the tier-4 auth-critical match still wins over tier 1.
			name:       "mixed hot-path and unrelated paths yields tier 4 with only hot-path reasons",
			paths:      []string{"README.md", "internal/auth/session.go", "internal/tui/view.go"},
			wantTier:   RiskTierCritical,
			wantLenses: 4,
			wantReasons: []string{
				"changed path matches auth-critical pattern: internal/auth/session.go",
			},
		},
		{
			// Uses permissions + crypto (neither overlaps tier4Patterns) to
			// exercise pure tier-1 multi-category accumulation without tripping
			// tier-4 precedence.
			name:       "multiple distinct sensitive patterns produce multiple named reasons",
			paths:      []string{"internal/permissions/acl.go", "internal/crypto/hash.go"},
			wantTier:   RiskTierElevated,
			wantLenses: 1,
			wantReasons: []string{
				"changed path matches cryptography-sensitive pattern: internal/crypto/hash.go",
				"changed path matches permissions-sensitive pattern: internal/permissions/acl.go",
			},
		},
		{
			// Uses two crypto paths (no tier4 overlap) to exercise pure tier-1
			// dedup-per-category behavior.
			name:       "duplicate matches against same pattern deduplicate reasons",
			paths:      []string{"internal/crypto/sign.go", "internal/crypto/hash.go"},
			wantTier:   RiskTierElevated,
			wantLenses: 1,
			wantReasons: []string{
				"changed path matches cryptography-sensitive pattern: internal/crypto/sign.go",
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

func TestClassify_Tier4HotPathPatterns(t *testing.T) {
	tests := []struct {
		name  string
		paths []string
	}{
		{name: "auth-critical path", paths: []string{"internal/auth/critical.go"}},
		{name: "security-critical path", paths: []string{"internal/security/policy.go"}},
		{name: "webhook path", paths: []string{"internal/webhook/handler.go"}},
		{name: "payments path", paths: []string{"internal/payment/charge.go"}},
		{name: "review-system code under internal/rdd/", paths: []string{"internal/rdd/classifier.go"}},
		{name: "review-system code under internal/reviewstore/", paths: []string{"internal/reviewstore/store.go"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Classify(tt.paths)
			if got.Tier != RiskTierCritical {
				t.Errorf("Classify(%v).Tier = %v, want %v", tt.paths, got.Tier, RiskTierCritical)
			}
			if got.Lenses != 4 {
				t.Errorf("Classify(%v).Lenses = %v, want 4", tt.paths, got.Lenses)
			}
			if len(got.Reasons) == 0 {
				t.Errorf("Classify(%v).Reasons is empty, want at least one named reason", tt.paths)
			}
		})
	}
}

func TestClassify_Tier4PrecedenceOverTier1(t *testing.T) {
	// internal/auth/session.go matches BOTH the tier-1 "authentication-sensitive"
	// pattern (riskPatterns) and the tier-4 "auth-critical" hot-path pattern
	// (tier4Patterns). Tier 4 must win.
	paths := []string{"internal/auth/session.go"}

	got := Classify(paths)

	if got.Tier != RiskTierCritical {
		t.Errorf("Classify(%v).Tier = %v, want %v (tier 4 must take precedence over tier 1)", paths, got.Tier, RiskTierCritical)
	}
	if got.Lenses != 4 {
		t.Errorf("Classify(%v).Lenses = %v, want 4", paths, got.Lenses)
	}
}

func TestClassify_Tier4SingleMatchIsEnough(t *testing.T) {
	// A single tier-4 match, even among many unrelated paths, is enough to
	// assign tier 4 — tier does not depend on accumulation of matches.
	paths := []string{"README.md", "docs/page.md", "internal/webhook/receiver.go"}

	got := Classify(paths)

	if got.Tier != RiskTierCritical {
		t.Errorf("Classify(%v).Tier = %v, want %v", paths, got.Tier, RiskTierCritical)
	}
	if got.Lenses != 4 {
		t.Errorf("Classify(%v).Lenses = %v, want 4", paths, got.Lenses)
	}
}

func TestClassify_Tier4NeverDependsOnPathCount(t *testing.T) {
	// Volume/size/file-count must never affect tier: a large batch of tier-4
	// matching paths still yields exactly tier 4, not some higher tier.
	paths := make([]string, 500)
	for i := range paths {
		paths[i] = "internal/webhook/handler.go"
	}

	got := Classify(paths)
	if got.Tier != RiskTierCritical {
		t.Errorf("Classify(500 webhook paths).Tier = %v, want %v (size must never drive tier)", got.Tier, RiskTierCritical)
	}
	if got.Lenses != 4 {
		t.Errorf("Classify(500 webhook paths).Lenses = %v, want 4", got.Lenses)
	}
}
