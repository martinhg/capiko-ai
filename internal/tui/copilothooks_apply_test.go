package tui

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/martinhg/capiko-ai/internal/backup"
	"github.com/martinhg/capiko-ai/internal/copilot"
	"github.com/martinhg/capiko-ai/internal/copilothooks"
	"github.com/martinhg/capiko-ai/internal/state"
)

// TestApplyCopilotHooksWritesFileAndRecordsState covers both strict and warn
// postures: applyCopilotHooks must write the guardrails hook file and record
// Enabled:true with a non-empty checksum matching CombinedChecksum.
func TestApplyCopilotHooksWritesFileAndRecordsState(t *testing.T) {
	for _, posture := range []copilothooks.Posture{copilothooks.PostureStrict, copilothooks.PostureWarn} {
		t.Run(string(posture), func(t *testing.T) {
			hooksDir := filepath.Join(t.TempDir(), "hooks")
			host := &copilot.Host{HooksDir: hooksDir}
			store := state.NewStore(t.TempDir())

			rec := &state.CopilotHooksRecord{Enabled: true, Posture: string(posture)}
			if err := applyCopilotHooks(host, store, nil, rec); err != nil {
				t.Fatalf("applyCopilotHooks: %v", err)
			}

			target := filepath.Join(hooksDir, copilothooks.GuardrailsFile)
			if _, err := os.Stat(target); err != nil {
				t.Fatalf("guardrails hook file not written: %v", err)
			}

			st, err := store.Load()
			if err != nil {
				t.Fatal(err)
			}
			if st.CopilotHooks == nil || !st.CopilotHooks.Enabled {
				t.Fatalf("expected enabled record, got %+v", st.CopilotHooks)
			}
			if st.CopilotHooks.Posture != string(posture) {
				t.Errorf("posture = %q, want %q", st.CopilotHooks.Posture, posture)
			}
			if st.CopilotHooks.Checksum == "" {
				t.Error("expected non-empty checksum")
			}
			want := copilothooks.CombinedChecksum(hooksDir)
			if st.CopilotHooks.Checksum != want {
				t.Errorf("checksum = %q, want %q (CombinedChecksum)", st.CopilotHooks.Checksum, want)
			}
			if len(st.CopilotHooks.Presets) == 0 || st.CopilotHooks.Presets[0] != "guardrails" {
				t.Errorf("presets = %v, want [guardrails]", st.CopilotHooks.Presets)
			}
		})
	}
}

// TestApplyCopilotHooksOffRoutesToDisable asserts posture "off" removes any
// existing guardrails file and records Enabled:false + Checksum:"" (REQ-8.4).
func TestApplyCopilotHooksOffRoutesToDisable(t *testing.T) {
	hooksDir := filepath.Join(t.TempDir(), "hooks")
	host := &copilot.Host{HooksDir: hooksDir}
	store := state.NewStore(t.TempDir())

	// Enable first so there is a file to remove.
	if err := applyCopilotHooks(host, store, nil, &state.CopilotHooksRecord{Enabled: true, Posture: string(copilothooks.PostureStrict)}); err != nil {
		t.Fatal(err)
	}

	if err := applyCopilotHooks(host, store, nil, &state.CopilotHooksRecord{Enabled: true, Posture: string(copilothooks.PostureOff)}); err != nil {
		t.Fatalf("applyCopilotHooks(off): %v", err)
	}

	target := filepath.Join(hooksDir, copilothooks.GuardrailsFile)
	if _, err := os.Stat(target); !os.IsNotExist(err) {
		t.Errorf("guardrails hook file should be removed when posture is off, stat err = %v", err)
	}

	st, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	if st.CopilotHooks == nil || st.CopilotHooks.Enabled {
		t.Errorf("expected disabled record, got %+v", st.CopilotHooks)
	}
	if st.CopilotHooks.Checksum != "" {
		t.Errorf("checksum = %q, want empty on disable", st.CopilotHooks.Checksum)
	}
}

// TestDisableCopilotHooksRemovesFileAndClearsChecksum exercises
// disableCopilotHooks directly (not routed through applyCopilotHooks).
func TestDisableCopilotHooksRemovesFileAndClearsChecksum(t *testing.T) {
	hooksDir := filepath.Join(t.TempDir(), "hooks")
	host := &copilot.Host{HooksDir: hooksDir}
	store := state.NewStore(t.TempDir())

	if err := applyCopilotHooks(host, store, nil, &state.CopilotHooksRecord{Enabled: true, Posture: string(copilothooks.PostureWarn)}); err != nil {
		t.Fatal(err)
	}

	if err := disableCopilotHooks(host, store, nil); err != nil {
		t.Fatalf("disableCopilotHooks: %v", err)
	}

	target := filepath.Join(hooksDir, copilothooks.GuardrailsFile)
	if _, err := os.Stat(target); !os.IsNotExist(err) {
		t.Errorf("guardrails hook file should be removed, stat err = %v", err)
	}
	st, _ := store.Load()
	if st.CopilotHooks == nil || st.CopilotHooks.Enabled || st.CopilotHooks.Checksum != "" {
		t.Errorf("expected disabled record with empty checksum, got %+v", st.CopilotHooks)
	}
}

// TestApplyCopilotHooksPostureDowngradeLeavesConsistentState walks
// strict -> warn -> off and asserts state/file stay consistent at every step.
func TestApplyCopilotHooksPostureDowngradeLeavesConsistentState(t *testing.T) {
	hooksDir := filepath.Join(t.TempDir(), "hooks")
	host := &copilot.Host{HooksDir: hooksDir}
	store := state.NewStore(t.TempDir())
	target := filepath.Join(hooksDir, copilothooks.GuardrailsFile)

	for _, posture := range []copilothooks.Posture{copilothooks.PostureStrict, copilothooks.PostureWarn, copilothooks.PostureOff} {
		if err := applyCopilotHooks(host, store, nil, &state.CopilotHooksRecord{Enabled: true, Posture: string(posture)}); err != nil {
			t.Fatalf("applyCopilotHooks(%s): %v", posture, err)
		}
		st, err := store.Load()
		if err != nil {
			t.Fatal(err)
		}
		if st.CopilotHooks.Posture != string(posture) {
			t.Errorf("posture = %q, want %q", st.CopilotHooks.Posture, posture)
		}
		_, statErr := os.Stat(target)
		if posture == copilothooks.PostureOff {
			if !os.IsNotExist(statErr) {
				t.Errorf("posture off: expected file removed, stat err = %v", statErr)
			}
			if st.CopilotHooks.Enabled || st.CopilotHooks.Checksum != "" {
				t.Errorf("posture off: expected disabled + empty checksum, got %+v", st.CopilotHooks)
			}
		} else {
			if statErr != nil {
				t.Errorf("posture %s: expected file present: %v", posture, statErr)
			}
			if !st.CopilotHooks.Enabled || st.CopilotHooks.Checksum == "" {
				t.Errorf("posture %s: expected enabled + non-empty checksum, got %+v", posture, st.CopilotHooks)
			}
		}
	}
}

// TestApplyCopilotHooksBackupCalledBeforeWrite asserts a snapshot is created
// before an existing guardrails file is overwritten by a posture change.
func TestApplyCopilotHooksBackupCalledBeforeWrite(t *testing.T) {
	hooksDir := filepath.Join(t.TempDir(), "hooks")
	host := &copilot.Host{HooksDir: hooksDir}
	store := state.NewStore(t.TempDir())
	bkp := backup.NewStore(t.TempDir())

	if err := applyCopilotHooks(host, store, bkp, &state.CopilotHooksRecord{Enabled: true, Posture: string(copilothooks.PostureStrict)}); err != nil {
		t.Fatal(err)
	}
	manifests, err := bkp.List()
	if err != nil {
		t.Fatal(err)
	}
	if len(manifests) != 0 {
		t.Fatalf("first write has nothing to back up, got %d manifests", len(manifests))
	}

	// Second apply with a different posture changes the rendered bytes, so a
	// backup of the pre-existing file must be created before the overwrite.
	if err := applyCopilotHooks(host, store, bkp, &state.CopilotHooksRecord{Enabled: true, Posture: string(copilothooks.PostureWarn)}); err != nil {
		t.Fatal(err)
	}
	manifests, err = bkp.List()
	if err != nil {
		t.Fatal(err)
	}
	if len(manifests) == 0 {
		t.Error("expected a backup snapshot before overwriting the changed hook file")
	}
}

// TestApplyCopilotHooksAbortsOnBackupError asserts the write is aborted (and
// state is not recorded) when the backup step fails (SC-15).
func TestApplyCopilotHooksAbortsOnBackupError(t *testing.T) {
	hooksDir := filepath.Join(t.TempDir(), "hooks")
	host := &copilot.Host{HooksDir: hooksDir}
	store := state.NewStore(t.TempDir())

	// Seed an existing guardrails file so the change-gate triggers a backup.
	if err := applyCopilotHooks(host, store, nil, &state.CopilotHooksRecord{Enabled: true, Posture: string(copilothooks.PostureStrict)}); err != nil {
		t.Fatal(err)
	}

	// Point the backup store's parent at a file (not a dir), so
	// Store.CreateFiles's MkdirAll fails before any write happens.
	blocker := filepath.Join(t.TempDir(), "blocker")
	if err := os.WriteFile(blocker, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	bkp := backup.NewStore(filepath.Join(blocker, "backups"))

	before, err := os.ReadFile(filepath.Join(hooksDir, copilothooks.GuardrailsFile))
	if err != nil {
		t.Fatal(err)
	}

	err = applyCopilotHooks(host, store, bkp, &state.CopilotHooksRecord{Enabled: true, Posture: string(copilothooks.PostureWarn)})
	if err == nil {
		t.Fatal("expected backup failure to abort applyCopilotHooks")
	}

	after, err := os.ReadFile(filepath.Join(hooksDir, copilothooks.GuardrailsFile))
	if err != nil {
		t.Fatal(err)
	}
	if string(before) != string(after) {
		t.Error("hook file must be untouched when the backup step fails")
	}

	st, _ := store.Load()
	if st.CopilotHooks.Posture != string(copilothooks.PostureStrict) {
		t.Errorf("state must not advance to the new posture on backup failure, got %+v", st.CopilotHooks)
	}
}
