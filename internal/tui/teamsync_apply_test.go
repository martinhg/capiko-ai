package tui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/martinhg/capiko-ai/internal/backup"
	"github.com/martinhg/capiko-ai/internal/state"
)

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

// setupTeamSyncWorkspace creates a minimal fake git workspace and returns the
// root path and paths to both managed hook files.
func setupTeamSyncWorkspace(t *testing.T) (root, postMerge, prePush string) {
	t.Helper()
	root = t.TempDir()
	hooksDir := filepath.Join(root, ".git", "hooks")
	if err := os.MkdirAll(hooksDir, 0o755); err != nil {
		t.Fatal(err)
	}
	postMerge = filepath.Join(hooksDir, "post-merge")
	prePush = filepath.Join(hooksDir, "pre-push")
	return root, postMerge, prePush
}

// ---------------------------------------------------------------------------
// applyTeamSync — happy path: writes both hook files + records state
// ---------------------------------------------------------------------------

func TestApplyTeamSync_HappyPath_WritesBothHooks(t *testing.T) {
	root, postMerge, prePush := setupTeamSyncWorkspace(t)
	store := state.NewStore(t.TempDir())
	bkp := backup.NewStore(t.TempDir())

	// Stub the conflict-detection seam so no conflict is reported.
	orig := teamSyncDetectConflict
	teamSyncDetectConflict = func(_ string) string { return "" }
	t.Cleanup(func() { teamSyncDetectConflict = orig })

	rec := &state.TeamSyncRecord{Enabled: true, Workspace: root, Project: "my-proj"}
	if err := applyTeamSync(root, store, bkp, rec); err != nil {
		t.Fatalf("applyTeamSync: %v", err)
	}

	// Both hook files must exist and be executable.
	checkHookWritten(t, postMerge, "engram sync --import")
	checkHookWritten(t, prePush, "engram sync --project")

	// State must be persisted.
	st, err := store.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if st.TeamSync == nil {
		t.Fatal("TeamSync should be persisted after apply")
	}
	if !st.TeamSync.Enabled {
		t.Error("TeamSync.Enabled should be true")
	}
	if st.TeamSync.Workspace != root {
		t.Errorf("TeamSync.Workspace = %q, want %q", st.TeamSync.Workspace, root)
	}
	if st.TeamSync.Conflict != "" {
		t.Errorf("TeamSync.Conflict should be empty on clean apply, got %q", st.TeamSync.Conflict)
	}
}

// checkHookWritten asserts the hook file exists and contains the expected snippet.
func checkHookWritten(t *testing.T, path, wantSnippet string) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("hook file %q not written: %v", path, err)
	}
	if !strings.Contains(string(data), wantSnippet) {
		t.Errorf("hook file %q does not contain %q:\n%s", path, wantSnippet, data)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat %q: %v", path, err)
	}
	if info.Mode()&0o111 == 0 {
		t.Errorf("hook file %q should be executable, mode = %v", path, info.Mode())
	}
}

// ---------------------------------------------------------------------------
// applyTeamSync — conflict path: skips writes, records conflict, no error
// ---------------------------------------------------------------------------

func TestApplyTeamSync_ConflictPath_SkipsWritesRecordsConflict(t *testing.T) {
	root, postMerge, prePush := setupTeamSyncWorkspace(t)
	store := state.NewStore(t.TempDir())
	bkp := backup.NewStore(t.TempDir())

	const conflictReason = "husky is configured (.husky/ directory found)"

	// Stub the conflict-detection seam to return a conflict.
	orig := teamSyncDetectConflict
	teamSyncDetectConflict = func(_ string) string { return conflictReason }
	t.Cleanup(func() { teamSyncDetectConflict = orig })

	rec := &state.TeamSyncRecord{Enabled: true, Workspace: root, Project: "my-proj"}
	if err := applyTeamSync(root, store, bkp, rec); err != nil {
		t.Fatalf("applyTeamSync conflict path must return nil, got: %v", err)
	}

	// Hook files must NOT be written.
	if _, err := os.Stat(postMerge); err == nil {
		t.Error("post-merge hook must not be written when conflict detected")
	}
	if _, err := os.Stat(prePush); err == nil {
		t.Error("pre-push hook must not be written when conflict detected")
	}

	// State must be persisted with Conflict set.
	st, err := store.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if st.TeamSync == nil {
		t.Fatal("TeamSync should be persisted even on conflict")
	}
	if st.TeamSync.Conflict != conflictReason {
		t.Errorf("TeamSync.Conflict = %q, want %q", st.TeamSync.Conflict, conflictReason)
	}
}

// ---------------------------------------------------------------------------
// applyTeamSync — backup before write
// ---------------------------------------------------------------------------

func TestApplyTeamSync_BackupBeforeWrite(t *testing.T) {
	root, postMerge, _ := setupTeamSyncWorkspace(t)

	// Seed an existing post-merge hook so CreateFiles has something to back up.
	if err := os.WriteFile(postMerge, []byte("#!/bin/sh\n# user content\n"), 0o755); err != nil {
		t.Fatal(err)
	}

	bkpDir := t.TempDir()
	bkp := backup.NewStore(bkpDir)
	store := state.NewStore(t.TempDir())

	orig := teamSyncDetectConflict
	teamSyncDetectConflict = func(_ string) string { return "" }
	t.Cleanup(func() { teamSyncDetectConflict = orig })

	rec := &state.TeamSyncRecord{Enabled: true, Workspace: root, Project: "proj"}
	if err := applyTeamSync(root, store, bkp, rec); err != nil {
		t.Fatalf("applyTeamSync: %v", err)
	}

	// At least one backup manifest should have been created.
	manifests, err := bkp.List()
	if err != nil {
		t.Fatalf("bkp.List: %v", err)
	}
	if len(manifests) == 0 {
		t.Error("expected a backup to have been created before writing hooks")
	}
}

// ---------------------------------------------------------------------------
// disableTeamSync — removes both hooks and records Enabled:false
// ---------------------------------------------------------------------------

func TestDisableTeamSync_RemovesBothHooks(t *testing.T) {
	root, postMerge, prePush := setupTeamSyncWorkspace(t)
	store := state.NewStore(t.TempDir())
	bkp := backup.NewStore(t.TempDir())

	// First apply (no conflict), then disable.
	orig := teamSyncDetectConflict
	teamSyncDetectConflict = func(_ string) string { return "" }
	t.Cleanup(func() { teamSyncDetectConflict = orig })

	rec := &state.TeamSyncRecord{Enabled: true, Workspace: root, Project: "proj"}
	if err := applyTeamSync(root, store, bkp, rec); err != nil {
		t.Fatalf("applyTeamSync: %v", err)
	}
	// Both hooks should exist now.
	if _, err := os.Stat(postMerge); err != nil {
		t.Fatal("post-merge hook should exist after apply")
	}

	if err := disableTeamSync(root, store, bkp); err != nil {
		t.Fatalf("disableTeamSync: %v", err)
	}

	// Both hook files must be removed (only shebang would remain → deleted).
	if _, err := os.Stat(postMerge); err == nil {
		t.Error("post-merge hook should be removed after disable")
	}
	if _, err := os.Stat(prePush); err == nil {
		t.Error("pre-push hook should be removed after disable")
	}

	// State must reflect Enabled:false.
	st, err := store.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if st.TeamSync == nil {
		t.Fatal("TeamSync should be recorded after disable")
	}
	if st.TeamSync.Enabled {
		t.Error("TeamSync.Enabled should be false after disable")
	}
}

// ---------------------------------------------------------------------------
// idempotency — applying twice does not break anything
// ---------------------------------------------------------------------------

func TestApplyTeamSync_Idempotent(t *testing.T) {
	root, postMerge, prePush := setupTeamSyncWorkspace(t)
	store := state.NewStore(t.TempDir())
	bkp := backup.NewStore(t.TempDir())

	orig := teamSyncDetectConflict
	teamSyncDetectConflict = func(_ string) string { return "" }
	t.Cleanup(func() { teamSyncDetectConflict = orig })

	rec := &state.TeamSyncRecord{Enabled: true, Workspace: root, Project: "proj"}
	if err := applyTeamSync(root, store, bkp, rec); err != nil {
		t.Fatalf("first apply: %v", err)
	}
	if err := applyTeamSync(root, store, bkp, rec); err != nil {
		t.Fatalf("second apply: %v", err)
	}

	// Hooks should still be present and correct.
	checkHookWritten(t, postMerge, "engram sync --import")
	checkHookWritten(t, prePush, "engram sync --project")
}
