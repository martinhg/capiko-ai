package main

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/martinhg/capiko-ai/internal/rdd"
	"github.com/martinhg/capiko-ai/internal/reviewstore"
)

// writeFileAll writes data to path, creating parent directories as needed —
// used to seed a corrupt mode-record file directly (bypassing
// reviewstore.SaveModeRecord, which always writes valid JSON).
func writeFileAll(path string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

// withStubResolveWorkspace stubs resolveWorkspace to return dir (or err),
// mirroring cga_test.go's withStubGitDir pattern.
func withStubResolveWorkspace(t *testing.T, dir string, err error) {
	t.Helper()
	prev := resolveWorkspace
	resolveWorkspace = func() (string, error) { return dir, err }
	t.Cleanup(func() { resolveWorkspace = prev })
}

// withStubUserHomeDir stubs userHomeDirFn to return dir (or err).
func withStubUserHomeDir(t *testing.T, dir string, err error) {
	t.Helper()
	prev := userHomeDirFn
	userHomeDirFn = func() (string, error) { return dir, err }
	t.Cleanup(func() { userHomeDirFn = prev })
}

// withStubGitCommonDirFn stubs reviewstore.GitCommonDir to return dir (or err).
func withStubGitCommonDirFn(t *testing.T, dir string, err error) {
	t.Helper()
	prev := reviewstore.GitCommonDir
	reviewstore.GitCommonDir = func(string) (string, error) { return dir, err }
	t.Cleanup(func() { reviewstore.GitCommonDir = prev })
}

// withStubReviewNow stubs reviewNow to a fixed instant for deterministic
// UpdatedAt assertions.
func withStubReviewNow(t *testing.T, when time.Time) {
	t.Helper()
	prev := reviewNow
	reviewNow = func() time.Time { return when }
	t.Cleanup(func() { reviewNow = prev })
}

func TestReviewCommandNotHandledForOtherName(t *testing.T) {
	var buf bytes.Buffer
	handled, exitCode, err := reviewCommand("cga", nil, &buf)
	if handled || exitCode != 0 || err != nil {
		t.Fatalf("review should not handle %q", "cga")
	}
}

func TestReviewCommandRequiresSubcommand(t *testing.T) {
	var buf bytes.Buffer
	handled, exitCode, err := reviewCommand("review", nil, &buf)
	if !handled || exitCode != 1 || err == nil {
		t.Fatalf("handled=%v exitCode=%d err=%v, want handled=true exitCode=1 err!=nil", handled, exitCode, err)
	}
}

func TestReviewCommandUnknownSubcommand(t *testing.T) {
	var buf bytes.Buffer
	handled, exitCode, _ := reviewCommand("review", []string{"bogus"}, &buf)
	if !handled || exitCode != 1 {
		t.Fatalf("handled=%v exitCode=%d, want handled=true exitCode=1", handled, exitCode)
	}
	if !strings.Contains(buf.String(), "unknown subcommand") {
		t.Errorf("output = %q, want it to mention unknown subcommand", buf.String())
	}
}

func TestReviewCommandModeRequiresVerb(t *testing.T) {
	var buf bytes.Buffer
	handled, exitCode, err := reviewCommand("review", []string{"mode"}, &buf)
	if !handled || exitCode != 1 || err == nil {
		t.Fatalf("handled=%v exitCode=%d err=%v, want handled=true exitCode=1 err!=nil", handled, exitCode, err)
	}
}

func TestReviewCommandModeUnknownVerb(t *testing.T) {
	var buf bytes.Buffer
	handled, exitCode, _ := reviewCommand("review", []string{"mode", "bogus"}, &buf)
	if !handled || exitCode != 1 {
		t.Fatalf("handled=%v exitCode=%d, want handled=true exitCode=1", handled, exitCode)
	}
	if !strings.Contains(buf.String(), "unknown mode verb") {
		t.Errorf("output = %q, want it to mention unknown mode verb", buf.String())
	}
}

func TestReviewCommandModeEnable_SavesClonModeManaged(t *testing.T) {
	workspace := t.TempDir()
	commonDir := filepath.Join(workspace, ".git")
	withStubResolveWorkspace(t, workspace, nil)
	withStubGitCommonDirFn(t, commonDir, nil)
	withStubReviewNow(t, time.Date(2026, 8, 21, 12, 0, 0, 0, time.UTC))

	var buf bytes.Buffer
	handled, exitCode, err := reviewCommand("review", []string{"mode", "enable"}, &buf)
	if !handled || exitCode != 0 || err != nil {
		t.Fatalf("handled=%v exitCode=%d err=%v, want handled=true exitCode=0 err=nil", handled, exitCode, err)
	}

	clonePath := filepath.Join(commonDir, "capiko", "rdd", "review-mode.json")
	rec, err := reviewstore.LoadModeRecord(clonePath)
	if err != nil {
		t.Fatalf("LoadModeRecord: %v", err)
	}
	if rec == nil || rec.Mode != rdd.ModeManaged {
		t.Fatalf("clone mode record = %+v, want Mode=managed", rec)
	}
	if rec.UpdatedAt != "2026-08-21T12:00:00Z" {
		t.Errorf("UpdatedAt = %q, want 2026-08-21T12:00:00Z", rec.UpdatedAt)
	}
	if !strings.Contains(buf.String(), "managed") {
		t.Errorf("output = %q, want it to mention managed", buf.String())
	}
}

func TestReviewCommandModeDisable_SavesCloneModeDisabled(t *testing.T) {
	workspace := t.TempDir()
	commonDir := filepath.Join(workspace, ".git")
	withStubResolveWorkspace(t, workspace, nil)
	withStubGitCommonDirFn(t, commonDir, nil)
	withStubReviewNow(t, time.Date(2026, 8, 21, 12, 0, 0, 0, time.UTC))

	var buf bytes.Buffer
	handled, exitCode, err := reviewCommand("review", []string{"mode", "disable"}, &buf)
	if !handled || exitCode != 0 || err != nil {
		t.Fatalf("handled=%v exitCode=%d err=%v, want handled=true exitCode=0 err=nil", handled, exitCode, err)
	}

	clonePath := filepath.Join(commonDir, "capiko", "rdd", "review-mode.json")
	rec, err := reviewstore.LoadModeRecord(clonePath)
	if err != nil {
		t.Fatalf("LoadModeRecord: %v", err)
	}
	if rec == nil || rec.Mode != rdd.ModeDisabled {
		t.Fatalf("clone mode record = %+v, want Mode=disabled", rec)
	}
	if !strings.Contains(buf.String(), "disabled") {
		t.Errorf("output = %q, want it to mention disabled", buf.String())
	}
}

func TestReviewCommandModeEnable_ResolveWorkspaceError(t *testing.T) {
	withStubResolveWorkspace(t, "", errors.New("boom"))

	var buf bytes.Buffer
	handled, exitCode, err := reviewCommand("review", []string{"mode", "enable"}, &buf)
	if !handled || exitCode != 1 || err == nil {
		t.Fatalf("handled=%v exitCode=%d err=%v, want handled=true exitCode=1 err!=nil", handled, exitCode, err)
	}
}

func TestReviewCommandModeEnable_GitCommonDirError(t *testing.T) {
	withStubResolveWorkspace(t, t.TempDir(), nil)
	withStubGitCommonDirFn(t, "", errors.New("not a git repo"))

	var buf bytes.Buffer
	handled, exitCode, err := reviewCommand("review", []string{"mode", "enable"}, &buf)
	if !handled || exitCode != 1 || err == nil {
		t.Fatalf("handled=%v exitCode=%d err=%v, want handled=true exitCode=1 err!=nil", handled, exitCode, err)
	}
}

func TestReviewCommandModeStatus_NoRecordsDefaultsToManaged(t *testing.T) {
	workspace := t.TempDir()
	commonDir := filepath.Join(workspace, ".git")
	home := t.TempDir()
	withStubResolveWorkspace(t, workspace, nil)
	withStubGitCommonDirFn(t, commonDir, nil)
	withStubUserHomeDir(t, home, nil)

	var buf bytes.Buffer
	handled, exitCode, err := reviewCommand("review", []string{"mode", "status"}, &buf)
	if !handled || exitCode != 0 || err != nil {
		t.Fatalf("handled=%v exitCode=%d err=%v, want handled=true exitCode=0 err=nil", handled, exitCode, err)
	}

	out := buf.String()
	if !strings.Contains(out, "effective mode: managed") {
		t.Errorf("output = %q, want it to report effective mode: managed", out)
	}
	if !strings.Contains(out, "clone mode:     not set") {
		t.Errorf("output = %q, want it to report clone mode: not set", out)
	}
	if !strings.Contains(out, "global mode:    not set") {
		t.Errorf("output = %q, want it to report global mode: not set", out)
	}
	if !strings.Contains(out, commonDir) {
		t.Errorf("output = %q, want it to include the git common dir %q", out, commonDir)
	}
}

func TestReviewCommandModeStatus_CloneOverridesGlobal(t *testing.T) {
	workspace := t.TempDir()
	commonDir := filepath.Join(workspace, ".git")
	home := t.TempDir()
	withStubResolveWorkspace(t, workspace, nil)
	withStubGitCommonDirFn(t, commonDir, nil)
	withStubUserHomeDir(t, home, nil)
	withStubReviewNow(t, time.Date(2026, 8, 21, 12, 0, 0, 0, time.UTC))

	// global = disabled, clone = enabled -> effective = managed (clone wins).
	globalPath := filepath.Join(home, ".capiko", "review-mode.json")
	if err := reviewstore.SaveModeRecord(globalPath, &rdd.ModeRecord{
		SchemaVersion: 1,
		Mode:          rdd.ModeDisabled,
		UpdatedAt:     "2026-08-21T00:00:00Z",
	}); err != nil {
		t.Fatal(err)
	}
	clonePath := filepath.Join(commonDir, "capiko", "rdd", "review-mode.json")
	if err := reviewstore.SaveModeRecord(clonePath, &rdd.ModeRecord{
		SchemaVersion: 1,
		Mode:          rdd.ModeManaged,
		UpdatedAt:     "2026-08-21T00:00:00Z",
	}); err != nil {
		t.Fatal(err)
	}

	var buf bytes.Buffer
	handled, exitCode, err := reviewCommand("review", []string{"mode", "status"}, &buf)
	if !handled || exitCode != 0 || err != nil {
		t.Fatalf("handled=%v exitCode=%d err=%v, want handled=true exitCode=0 err=nil", handled, exitCode, err)
	}

	out := buf.String()
	if !strings.Contains(out, "effective mode: managed") {
		t.Errorf("output = %q, want effective mode: managed (clone overrides global)", out)
	}
	if !strings.Contains(out, "clone mode:     managed") {
		t.Errorf("output = %q, want clone mode: managed", out)
	}
	if !strings.Contains(out, "global mode:    disabled") {
		t.Errorf("output = %q, want global mode: disabled", out)
	}
}

func TestReviewCommandModeStatus_CorruptCloneRecordReportedButNotFatal(t *testing.T) {
	workspace := t.TempDir()
	commonDir := filepath.Join(workspace, ".git")
	home := t.TempDir()
	withStubResolveWorkspace(t, workspace, nil)
	withStubGitCommonDirFn(t, commonDir, nil)
	withStubUserHomeDir(t, home, nil)

	clonePath := filepath.Join(commonDir, "capiko", "rdd", "review-mode.json")
	if err := writeFileAll(clonePath, []byte("{not json")); err != nil {
		t.Fatal(err)
	}

	var buf bytes.Buffer
	handled, exitCode, err := reviewCommand("review", []string{"mode", "status"}, &buf)
	if !handled || exitCode != 0 || err != nil {
		t.Fatalf("handled=%v exitCode=%d err=%v, want handled=true exitCode=0 err=nil (fail-closed, not fatal)", handled, exitCode, err)
	}

	out := buf.String()
	// Corrupt clone record must resolve as absent (fail-closed to managed),
	// per spec rdd-kill-switch "Fail-closed default".
	if !strings.Contains(out, "effective mode: managed") {
		t.Errorf("output = %q, want effective mode: managed (fail-closed on corrupt record)", out)
	}
	if !strings.Contains(out, "clone mode:     corrupt") {
		t.Errorf("output = %q, want clone mode to be reported as corrupt", out)
	}
}

func TestReviewCommandModeStatus_UserHomeDirError(t *testing.T) {
	workspace := t.TempDir()
	commonDir := filepath.Join(workspace, ".git")
	withStubResolveWorkspace(t, workspace, nil)
	withStubGitCommonDirFn(t, commonDir, nil)
	withStubUserHomeDir(t, "", errors.New("no home"))

	var buf bytes.Buffer
	handled, exitCode, err := reviewCommand("review", []string{"mode", "status"}, &buf)
	if !handled || exitCode != 1 || err == nil {
		t.Fatalf("handled=%v exitCode=%d err=%v, want handled=true exitCode=1 err!=nil", handled, exitCode, err)
	}
}
