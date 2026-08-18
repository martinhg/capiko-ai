package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/martinhg/capiko-ai/internal/backup"
	"github.com/martinhg/capiko-ai/internal/cga"
	"github.com/martinhg/capiko-ai/internal/state"
)

// ---------------------------------------------------------------------------
// applyCGA
// ---------------------------------------------------------------------------

func TestApplyCGAWritesHookAndRecordsState(t *testing.T) {
	ws := t.TempDir()
	store := state.NewStore(t.TempDir())
	rec := &state.CGARecord{Enabled: true, StrictMode: true, Timeout: 60}

	if err := applyCGA(ws, store, nil, rec); err != nil {
		t.Fatalf("applyCGA: %v", err)
	}

	hook, err := os.ReadFile(filepath.Join(ws, ".git", "hooks", "pre-commit"))
	if err != nil {
		t.Fatalf("pre-commit hook not written: %v", err)
	}
	if !strings.Contains(string(hook), cga.MarkerStart) || !strings.Contains(string(hook), cga.MarkerEnd) {
		t.Errorf("pre-commit hook missing CGA markers:\n%s", hook)
	}
	if !strings.Contains(string(hook), "copilot -p") {
		t.Errorf("pre-commit hook should invoke copilot -p:\n%s", hook)
	}

	st, _ := store.Load()
	if st.CGA == nil || !st.CGA.Enabled {
		t.Error("apply should record the enabled CGA state")
	}
	if st.CGA.Workspace != ws {
		t.Errorf("apply should record the workspace, got %q", st.CGA.Workspace)
	}
	if st.CGA.Checksum == "" {
		t.Error("apply should record a checksum of the rendered hook")
	}
}

func TestApplyCGAIncludesActivePersona(t *testing.T) {
	ws := t.TempDir()
	store := state.NewStore(t.TempDir())
	if err := store.SetPersona("capiko"); err != nil {
		t.Fatal(err)
	}

	if err := applyCGA(ws, store, nil, &state.CGARecord{Enabled: true}); err != nil {
		t.Fatalf("applyCGA: %v", err)
	}

	hook, _ := os.ReadFile(filepath.Join(ws, ".git", "hooks", "pre-commit"))
	if !strings.Contains(string(hook), "capiko") {
		t.Errorf("pre-commit hook should reference the active persona:\n%s", hook)
	}
}

func TestApplyCGADisableRemovesHookAndRecordsState(t *testing.T) {
	ws := t.TempDir()
	store := state.NewStore(t.TempDir())

	// Enable first so there is a managed hook to remove.
	if err := applyCGA(ws, store, nil, &state.CGARecord{Enabled: true}); err != nil {
		t.Fatalf("enable: %v", err)
	}
	if err := applyCGA(ws, store, nil, &state.CGARecord{Enabled: false}); err != nil {
		t.Fatalf("disable: %v", err)
	}

	hook, err := os.ReadFile(filepath.Join(ws, ".git", "hooks", "pre-commit"))
	if err == nil && strings.Contains(string(hook), cga.MarkerStart) {
		t.Errorf("disable should remove the CGA hook block:\n%s", hook)
	}

	st, _ := store.Load()
	if st.CGA == nil || st.CGA.Enabled {
		t.Error("disable should record CGA as off")
	}
}

// ---------------------------------------------------------------------------
// post-commit hook lifecycle (CGA Phase 2 PR3)
// ---------------------------------------------------------------------------

// withFakeGitDir overrides gitRevParseGitDir for the duration of the test so
// findings-log path resolution does not require a real git repository.
func withFakeGitDir(t *testing.T, gitDir string) {
	t.Helper()
	prev := gitRevParseGitDir
	gitRevParseGitDir = func(string) (string, error) { return gitDir, nil }
	t.Cleanup(func() { gitRevParseGitDir = prev })
}

func TestApplyCGAWritesPostCommitHook(t *testing.T) {
	ws := t.TempDir()
	withFakeGitDir(t, filepath.Join(ws, ".git"))
	store := state.NewStore(t.TempDir())

	if err := applyCGA(ws, store, nil, &state.CGARecord{Enabled: true}); err != nil {
		t.Fatalf("applyCGA: %v", err)
	}

	hook, err := os.ReadFile(filepath.Join(ws, ".git", "hooks", "post-commit"))
	if err != nil {
		t.Fatalf("post-commit hook not written: %v", err)
	}
	if !strings.Contains(string(hook), cga.PostCommitMarkerStart) || !strings.Contains(string(hook), cga.PostCommitMarkerEnd) {
		t.Errorf("post-commit hook missing CGA markers:\n%s", hook)
	}
	if !strings.Contains(string(hook), "commit_sha") {
		t.Errorf("post-commit hook should patch commit_sha:\n%s", hook)
	}
}

func TestApplyCGAEmbedsLogPathFromGitDir(t *testing.T) {
	ws := t.TempDir()
	gitDir := filepath.Join(ws, ".git")
	withFakeGitDir(t, gitDir)
	store := state.NewStore(t.TempDir())

	if err := applyCGA(ws, store, nil, &state.CGARecord{Enabled: true}); err != nil {
		t.Fatalf("applyCGA: %v", err)
	}

	wantLogPath := filepath.Join(gitDir, "capiko", "cga-findings.jsonl")

	pre, err := os.ReadFile(filepath.Join(ws, ".git", "hooks", "pre-commit"))
	if err != nil {
		t.Fatalf("pre-commit hook not written: %v", err)
	}
	if !strings.Contains(string(pre), wantLogPath) {
		t.Errorf("pre-commit hook should embed the git-dir-resolved log path %q:\n%s", wantLogPath, pre)
	}

	post, err := os.ReadFile(filepath.Join(ws, ".git", "hooks", "post-commit"))
	if err != nil {
		t.Fatalf("post-commit hook not written: %v", err)
	}
	if !strings.Contains(string(post), wantLogPath) {
		t.Errorf("post-commit hook should embed the git-dir-resolved log path %q:\n%s", wantLogPath, post)
	}
}

func TestApplyCGADisablesLoggingWhenNotAGitRepo(t *testing.T) {
	ws := t.TempDir()
	prev := gitRevParseGitDir
	gitRevParseGitDir = func(string) (string, error) { return "", fmt.Errorf("not a git repository") }
	t.Cleanup(func() { gitRevParseGitDir = prev })
	store := state.NewStore(t.TempDir())

	if err := applyCGA(ws, store, nil, &state.CGARecord{Enabled: true}); err != nil {
		t.Fatalf("applyCGA: %v", err)
	}

	pre, err := os.ReadFile(filepath.Join(ws, ".git", "hooks", "pre-commit"))
	if err != nil {
		t.Fatalf("pre-commit hook not written: %v", err)
	}
	if strings.Contains(string(pre), "cga: append findings log entry") {
		t.Errorf("pre-commit hook should not append findings when git-dir resolution fails:\n%s", pre)
	}
}

func TestApplyCGADisableRemovesBothHooks(t *testing.T) {
	ws := t.TempDir()
	withFakeGitDir(t, filepath.Join(ws, ".git"))
	store := state.NewStore(t.TempDir())

	if err := applyCGA(ws, store, nil, &state.CGARecord{Enabled: true}); err != nil {
		t.Fatalf("enable: %v", err)
	}
	if err := applyCGA(ws, store, nil, &state.CGARecord{Enabled: false}); err != nil {
		t.Fatalf("disable: %v", err)
	}

	preHook, err := os.ReadFile(filepath.Join(ws, ".git", "hooks", "pre-commit"))
	if err == nil && strings.Contains(string(preHook), cga.MarkerStart) {
		t.Errorf("disable should remove the CGA pre-commit hook block:\n%s", preHook)
	}
	postHook, err := os.ReadFile(filepath.Join(ws, ".git", "hooks", "post-commit"))
	if err == nil && strings.Contains(string(postHook), cga.PostCommitMarkerStart) {
		t.Errorf("disable should remove the CGA post-commit hook block:\n%s", postHook)
	}
}

func TestBackupCGAHookSnapshotsPostCommit(t *testing.T) {
	ws := t.TempDir()
	if err := os.MkdirAll(filepath.Join(ws, ".git", "hooks"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(ws, ".git", "hooks", "post-commit"), []byte("#!/bin/sh\necho old\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	bkp := backup.NewStore(t.TempDir())

	if err := backupCGAHook(bkp, ws); err != nil {
		t.Fatalf("backupCGAHook: %v", err)
	}

	manifests, err := bkp.List()
	if err != nil {
		t.Fatal(err)
	}
	if len(manifests) == 0 {
		t.Fatal("backupCGAHook should snapshot the existing post-commit hook")
	}
	found := false
	for _, m := range manifests {
		for _, f := range m.Files {
			if strings.Contains(f.Path, "post-commit") {
				found = true
			}
		}
	}
	if !found {
		t.Errorf("backup manifest should reference post-commit, got %+v", manifests)
	}
}

func TestApplyCGABacksUpExistingHook(t *testing.T) {
	ws := t.TempDir()
	if err := os.MkdirAll(filepath.Join(ws, ".git", "hooks"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(ws, ".git", "hooks", "pre-commit"), []byte("#!/bin/sh\necho old\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	bkp := backup.NewStore(t.TempDir())

	if err := applyCGA(ws, state.NewStore(t.TempDir()), bkp, &state.CGARecord{Enabled: true}); err != nil {
		t.Fatalf("applyCGA: %v", err)
	}

	manifests, err := bkp.List()
	if err != nil {
		t.Fatal(err)
	}
	if len(manifests) == 0 {
		t.Error("apply should snapshot the existing hook before overwriting")
	}
}

func TestApplyCGANilRecordIsNoop(t *testing.T) {
	ws := t.TempDir()
	if err := applyCGA(ws, state.NewStore(t.TempDir()), nil, nil); err != nil {
		t.Fatalf("applyCGA with nil record should be a no-op, got: %v", err)
	}
	if _, err := os.Stat(filepath.Join(ws, ".git", "hooks", "pre-commit")); !os.IsNotExist(err) {
		t.Error("applyCGA with nil record should not write a hook")
	}
}

func TestApplyCGACleansUpGGARemnants(t *testing.T) {
	ws := t.TempDir()
	writeGGARemnants(t, ws)

	if err := applyCGA(ws, state.NewStore(t.TempDir()), nil, &state.CGARecord{Enabled: true}); err != nil {
		t.Fatalf("applyCGA: %v", err)
	}

	if _, err := os.Stat(filepath.Join(ws, ".gga")); !os.IsNotExist(err) {
		t.Error("applyCGA should remove the .gga remnant before installing its own hook")
	}
	agents, _ := os.ReadFile(filepath.Join(ws, "AGENTS.md"))
	if strings.Contains(string(agents), "capiko:review") {
		t.Errorf("applyCGA should remove the gga AGENTS.md block:\n%s", agents)
	}
	hook, err := os.ReadFile(filepath.Join(ws, ".git", "hooks", "pre-commit"))
	if err != nil {
		t.Fatalf("pre-commit hook not written: %v", err)
	}
	if strings.Contains(string(hook), "gga run") {
		t.Errorf("applyCGA should have removed gga's hook content:\n%s", hook)
	}
	if !strings.Contains(string(hook), cga.MarkerStart) {
		t.Errorf("applyCGA should still write its own hook after cleanup:\n%s", hook)
	}
}

// ---------------------------------------------------------------------------
// cleanupGGA
// ---------------------------------------------------------------------------

// writeGGARemnants seeds a workspace with the three artifacts GGA leaves
// behind: the .gga config, a capiko:review AGENTS.md block, and a
// non-marker-delimited pre-commit hook GGA installed directly.
func writeGGARemnants(t *testing.T, ws string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(ws, ".gga"), []byte(`PROVIDER="claude"`+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	agents := "<!-- capiko:review:start -->\n# Code Review Rules\n<!-- capiko:review:end -->\n"
	if err := os.WriteFile(filepath.Join(ws, "AGENTS.md"), []byte(agents), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(ws, ".git", "hooks"), 0o755); err != nil {
		t.Fatal(err)
	}
	hook := "#!/bin/sh\n# installed by gga\ngga run \"$@\"\n"
	if err := os.WriteFile(filepath.Join(ws, ".git", "hooks", "pre-commit"), []byte(hook), 0o755); err != nil {
		t.Fatal(err)
	}
}

func TestCleanupGGARemovesAllRemnants(t *testing.T) {
	ws := t.TempDir()
	writeGGARemnants(t, ws)

	if err := cleanupGGA(ws); err != nil {
		t.Fatalf("cleanupGGA: %v", err)
	}

	if _, err := os.Stat(filepath.Join(ws, ".gga")); !os.IsNotExist(err) {
		t.Error("cleanupGGA should remove the .gga file")
	}
	agents, err := os.ReadFile(filepath.Join(ws, "AGENTS.md"))
	if err != nil {
		t.Fatalf("AGENTS.md should still exist (only the block is removed): %v", err)
	}
	if strings.Contains(string(agents), "capiko:review") {
		t.Errorf("cleanupGGA should remove the capiko:review AGENTS.md block:\n%s", agents)
	}
	if _, err := os.Stat(filepath.Join(ws, ".git", "hooks", "pre-commit")); !os.IsNotExist(err) {
		t.Error("cleanupGGA should remove gga's pre-commit hook file")
	}
}

func TestCleanupGGAPreservesUserAgentsContent(t *testing.T) {
	ws := t.TempDir()
	writeGGARemnants(t, ws)
	agentsPath := filepath.Join(ws, "AGENTS.md")
	existing, _ := os.ReadFile(agentsPath)
	withUserContent := "# My own rules\n\n- REQUIRE: keep this line\n\n" + string(existing)
	if err := os.WriteFile(agentsPath, []byte(withUserContent), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := cleanupGGA(ws); err != nil {
		t.Fatalf("cleanupGGA: %v", err)
	}

	agents, _ := os.ReadFile(agentsPath)
	if !strings.Contains(string(agents), "keep this line") {
		t.Errorf("cleanupGGA must not clobber user-authored AGENTS.md content:\n%s", agents)
	}
}

func TestCleanupGGANoRemnantsIsNoop(t *testing.T) {
	ws := t.TempDir()

	if err := cleanupGGA(ws); err != nil {
		t.Fatalf("cleanupGGA on a clean workspace should be a no-op, got: %v", err)
	}
	if _, err := os.Stat(filepath.Join(ws, ".gga")); !os.IsNotExist(err) {
		t.Error("cleanupGGA should not create a .gga file")
	}
	if _, err := os.Stat(filepath.Join(ws, "AGENTS.md")); !os.IsNotExist(err) {
		t.Error("cleanupGGA should not create AGENTS.md")
	}
}

func TestCleanupGGALeavesNonGGAHookUntouched(t *testing.T) {
	ws := t.TempDir()
	if err := os.MkdirAll(filepath.Join(ws, ".git", "hooks"), 0o755); err != nil {
		t.Fatal(err)
	}
	hook := "#!/bin/sh\necho custom user hook\n"
	if err := os.WriteFile(filepath.Join(ws, ".git", "hooks", "pre-commit"), []byte(hook), 0o755); err != nil {
		t.Fatal(err)
	}

	if err := cleanupGGA(ws); err != nil {
		t.Fatalf("cleanupGGA: %v", err)
	}

	got, err := os.ReadFile(filepath.Join(ws, ".git", "hooks", "pre-commit"))
	if err != nil {
		t.Fatalf("cleanupGGA should not remove a hook without a gga signature: %v", err)
	}
	if string(got) != hook {
		t.Errorf("cleanupGGA modified a non-gga hook:\ngot:  %q\nwant: %q", got, hook)
	}
}

func TestCleanupGGAIdempotent(t *testing.T) {
	ws := t.TempDir()
	writeGGARemnants(t, ws)

	if err := cleanupGGA(ws); err != nil {
		t.Fatalf("first cleanupGGA: %v", err)
	}
	if err := cleanupGGA(ws); err != nil {
		t.Fatalf("second cleanupGGA should be a no-op, got: %v", err)
	}
}
