package rdd

import "testing"

func TestResolveMode(t *testing.T) {
	managed := &ModeRecord{SchemaVersion: 1, Mode: ModeManaged, UpdatedAt: "2026-08-21T00:00:00Z"}
	disabled := &ModeRecord{SchemaVersion: 1, Mode: ModeDisabled, UpdatedAt: "2026-08-21T00:00:00Z"}
	corrupt := &ModeRecord{SchemaVersion: 1, Mode: ReviewMode("garbage"), UpdatedAt: "2026-08-21T00:00:00Z"}
	empty := &ModeRecord{}

	tests := []struct {
		name   string
		global *ModeRecord
		clone  *ModeRecord
		want   ReviewMode
	}{
		{
			name:   "both nil fails closed to managed",
			global: nil,
			clone:  nil,
			want:   ModeManaged,
		},
		{
			name:   "global managed, clone nil uses global",
			global: managed,
			clone:  nil,
			want:   ModeManaged,
		},
		{
			name:   "global disabled, clone nil uses global",
			global: disabled,
			clone:  nil,
			want:   ModeDisabled,
		},
		{
			name:   "clone overrides global: clone managed wins over global disabled",
			global: disabled,
			clone:  managed,
			want:   ModeManaged,
		},
		{
			name:   "clone overrides global: clone disabled wins over global managed",
			global: managed,
			clone:  disabled,
			want:   ModeDisabled,
		},
		{
			name:   "clone nil, global nil fails closed",
			global: nil,
			clone:  nil,
			want:   ModeManaged,
		},
		{
			name:   "corrupt clone mode falls back to global",
			global: disabled,
			clone:  corrupt,
			want:   ModeDisabled,
		},
		{
			name:   "corrupt clone mode falls back to managed when global absent",
			global: nil,
			clone:  corrupt,
			want:   ModeManaged,
		},
		{
			name:   "corrupt global mode ignored, clone still wins",
			global: corrupt,
			clone:  disabled,
			want:   ModeDisabled,
		},
		{
			name:   "both corrupt fails closed to managed",
			global: corrupt,
			clone:  corrupt,
			want:   ModeManaged,
		},
		{
			name:   "empty clone record (zero-value Mode) treated as absent",
			global: disabled,
			clone:  empty,
			want:   ModeDisabled,
		},
		{
			name:   "empty global record (zero-value Mode) treated as absent",
			global: empty,
			clone:  nil,
			want:   ModeManaged,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ResolveMode(tt.global, tt.clone)
			if got != tt.want {
				t.Errorf("ResolveMode(global=%+v, clone=%+v) = %q, want %q", tt.global, tt.clone, got, tt.want)
			}
		})
	}
}

func TestResolveMode_Determinism(t *testing.T) {
	global := &ModeRecord{SchemaVersion: 1, Mode: ModeManaged, UpdatedAt: "2026-08-21T00:00:00Z"}
	clone := &ModeRecord{SchemaVersion: 1, Mode: ModeDisabled, UpdatedAt: "2026-08-21T00:00:00Z"}

	first := ResolveMode(global, clone)
	for i := 0; i < 10; i++ {
		if got := ResolveMode(global, clone); got != first {
			t.Fatalf("ResolveMode not deterministic: run %d got %q, want %q", i, got, first)
		}
	}
}
