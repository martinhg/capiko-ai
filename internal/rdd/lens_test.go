package rdd

import (
	"reflect"
	"testing"
)

func TestSelectLenses(t *testing.T) {
	tests := []struct {
		name string
		tier RiskTier
		want []Lens
	}{
		{
			name: "tier 0 selects no lenses",
			tier: RiskTierStandard,
			want: nil,
		},
		{
			name: "tier 1 selects only the risk lens",
			tier: RiskTierElevated,
			want: []Lens{LensRisk},
		},
		{
			name: "tier 4 selects all four canonical lenses",
			tier: RiskTierCritical,
			want: []Lens{LensRisk, LensReadability, LensReliability, LensResilience},
		},
		{
			name: "unknown tier fails closed to all four lenses",
			tier: RiskTier(99),
			want: []Lens{LensRisk, LensReadability, LensReliability, LensResilience},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := SelectLenses(tt.tier)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("SelectLenses(%v) = %v, want %v", tt.tier, got, tt.want)
			}
		})
	}
}

func TestLookupMandate(t *testing.T) {
	tests := []struct {
		name            string
		lens            Lens
		wantTitle       string
		wantFocusSubstr string
	}{
		{
			name:            "R1 risk lens has title and inspection focus",
			lens:            LensRisk,
			wantTitle:       "Risk",
			wantFocusSubstr: "security",
		},
		{
			name:            "R2 readability lens has title and inspection focus",
			lens:            LensReadability,
			wantTitle:       "Readability",
			wantFocusSubstr: "naming",
		},
		{
			name:            "R3 reliability lens has title and inspection focus",
			lens:            LensReliability,
			wantTitle:       "Reliability",
			wantFocusSubstr: "test",
		},
		{
			name:            "R4 resilience lens has title and inspection focus",
			lens:            LensResilience,
			wantTitle:       "Resilience",
			wantFocusSubstr: "fallback",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := LookupMandate(tt.lens)
			if err != nil {
				t.Fatalf("LookupMandate(%v) returned unexpected error: %v", tt.lens, err)
			}
			if got.ID != tt.lens {
				t.Errorf("LookupMandate(%v).ID = %v, want %v", tt.lens, got.ID, tt.lens)
			}
			if got.Title != tt.wantTitle {
				t.Errorf("LookupMandate(%v).Title = %q, want %q", tt.lens, got.Title, tt.wantTitle)
			}
			if got.InspectionFocus == "" {
				t.Errorf("LookupMandate(%v).InspectionFocus is empty", tt.lens)
			}
		})
	}
}

func TestLookupMandate_UnknownLensFailsClosed(t *testing.T) {
	_, err := LookupMandate(Lens("R99"))
	if err == nil {
		t.Fatal("LookupMandate(\"R99\") expected an error for an unrecognized lens, got nil")
	}
}
