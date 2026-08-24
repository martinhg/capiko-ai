package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/martinhg/capiko-ai/internal/githooks"
	"github.com/martinhg/capiko-ai/internal/rdd"
	"github.com/martinhg/capiko-ai/internal/rddgate"
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

// --- review start / capture-result (PR5) ---

// withStubBuildCandidate stubs buildCandidate to return identity/paths/err,
// mirroring the other seam-var stub helpers above so `review start` and
// `review capture-result` tests never need a real git repository.
func withStubBuildCandidate(t *testing.T, identity rdd.CandidateIdentity, changedPaths []string, err error) {
	t.Helper()
	prev := buildCandidate
	buildCandidate = func(string) (rdd.CandidateIdentity, []string, error) {
		return identity, changedPaths, err
	}
	t.Cleanup(func() { buildCandidate = prev })
}

// testCandidateIdentity returns a fixed, deterministic CandidateIdentity
// fixture shared by review start/capture-result tests.
func testCandidateIdentity() rdd.CandidateIdentity {
	return rdd.CandidateIdentity{
		RepositoryID:            "capiko-ai",
		BaseTree:                "base-tree-hash",
		CandidateTree:           "candidate-tree-hash",
		ChangedPathsModesDigest: "digest-sha",
		PolicyHash:              "",
	}
}

// seedReviewingState directly writes a review-state.json under authorityDir
// with State=reviewing, bypassing `review start`, so capture-result tests
// can exercise capture-result in isolation.
func seedReviewingState(t *testing.T, authorityDir string, identity rdd.CandidateIdentity, selected []rdd.Lens, existing map[rdd.Lens]reviewstore.LensResult) *reviewstore.ReviewState {
	t.Helper()
	store := reviewstore.NewStore(authorityDir)
	state := &reviewstore.ReviewState{
		SchemaVersion:  1,
		Candidate:      identity,
		Tier:           rdd.RiskTierElevated,
		Lenses:         len(selected),
		State:          rdd.StateReviewing,
		SubjectHash:    computeSubjectHash(identity),
		SelectedLenses: selected,
		LensResults:    existing,
		CreatedAt:      "2026-08-21T00:00:00Z",
		UpdatedAt:      "2026-08-21T00:00:00Z",
	}
	if err := store.SaveState(state); err != nil {
		t.Fatalf("seedReviewingState: SaveState: %v", err)
	}
	return state
}

func TestReviewStart_MissingConsentFlag_FailsClosed(t *testing.T) {
	var buf bytes.Buffer
	handled, exitCode, err := reviewCommand("review", []string{"start"}, &buf)
	if !handled || exitCode != 1 || err == nil {
		t.Fatalf("handled=%v exitCode=%d err=%v, want handled=true exitCode=1 err!=nil", handled, exitCode, err)
	}
	if !strings.Contains(err.Error(), "--consent") {
		t.Errorf("err = %v, want it to mention --consent", err)
	}
}

func TestReviewStart_InvalidConsentValue_FailsClosed(t *testing.T) {
	var buf bytes.Buffer
	handled, exitCode, err := reviewCommand("review", []string{"start", "--consent", "bogus"}, &buf)
	if !handled || exitCode != 1 || err == nil {
		t.Fatalf("handled=%v exitCode=%d err=%v, want handled=true exitCode=1 err!=nil", handled, exitCode, err)
	}
}

func TestReviewStart_KillSwitchNotManaged_Errors(t *testing.T) {
	workspace := t.TempDir()
	commonDir := filepath.Join(workspace, ".git")
	home := t.TempDir()
	withStubResolveWorkspace(t, workspace, nil)
	withStubGitCommonDirFn(t, commonDir, nil)
	withStubUserHomeDir(t, home, nil)

	clonePath := cloneModePath(commonDir)
	if err := reviewstore.SaveModeRecord(clonePath, &rdd.ModeRecord{
		SchemaVersion: 1,
		Mode:          rdd.ModeDisabled,
		UpdatedAt:     "2026-08-21T00:00:00Z",
	}); err != nil {
		t.Fatal(err)
	}

	var buf bytes.Buffer
	handled, exitCode, err := reviewCommand("review", []string{"start", "--consent", "granted"}, &buf)
	if !handled || exitCode != 1 || err == nil {
		t.Fatalf("handled=%v exitCode=%d err=%v, want handled=true exitCode=1 err!=nil", handled, exitCode, err)
	}
	if !strings.Contains(err.Error(), "managed") {
		t.Errorf("err = %v, want it to mention managed", err)
	}
}

func TestReviewStart_Relay_OutputsConsentEnvelopeWithoutFreezing(t *testing.T) {
	workspace := t.TempDir()
	commonDir := filepath.Join(workspace, ".git")
	home := t.TempDir()
	authorityDir := reviewAuthorityDir(commonDir)
	withStubResolveWorkspace(t, workspace, nil)
	withStubGitCommonDirFn(t, commonDir, nil)
	withStubUserHomeDir(t, home, nil)
	withStubBuildCandidate(t, testCandidateIdentity(), []string{"internal/authz/handler.go"}, nil)

	var buf bytes.Buffer
	handled, exitCode, err := reviewCommand("review", []string{"start", "--consent", "relay"}, &buf)
	if !handled || exitCode != 0 || err != nil {
		t.Fatalf("handled=%v exitCode=%d err=%v, want handled=true exitCode=0 err=nil", handled, exitCode, err)
	}

	var envelope rdd.ConsentResult
	if err := json.Unmarshal(buf.Bytes(), &envelope); err != nil {
		t.Fatalf("output is not a ConsentResult envelope: %v (output=%q)", err, buf.String())
	}
	if envelope.Granted {
		t.Errorf("ConsentResult.Granted = true, want false for relay")
	}
	if envelope.Headline == "" {
		t.Errorf("ConsentResult.Headline is empty, want a description")
	}

	store := reviewstore.NewStore(authorityDir)
	state, err := store.LoadState()
	if err != nil {
		t.Fatalf("LoadState: %v", err)
	}
	if state != nil {
		t.Errorf("LoadState() = %+v, want nil (relay must not freeze or transition)", state)
	}
}

func TestReviewStart_Declined_DoesNotFreezeAndErrorsActionably(t *testing.T) {
	workspace := t.TempDir()
	commonDir := filepath.Join(workspace, ".git")
	home := t.TempDir()
	authorityDir := reviewAuthorityDir(commonDir)
	withStubResolveWorkspace(t, workspace, nil)
	withStubGitCommonDirFn(t, commonDir, nil)
	withStubUserHomeDir(t, home, nil)
	withStubBuildCandidate(t, testCandidateIdentity(), []string{"internal/authz/handler.go"}, nil)

	var buf bytes.Buffer
	handled, exitCode, err := reviewCommand("review", []string{"start", "--consent", "declined"}, &buf)
	if !handled || exitCode != 1 || err == nil {
		t.Fatalf("handled=%v exitCode=%d err=%v, want handled=true exitCode=1 err!=nil", handled, exitCode, err)
	}
	if !strings.Contains(err.Error(), "declined") {
		t.Errorf("err = %v, want it to mention declined", err)
	}

	store := reviewstore.NewStore(authorityDir)
	state, loadErr := store.LoadState()
	if loadErr != nil {
		t.Fatalf("LoadState: %v", loadErr)
	}
	if state != nil {
		t.Errorf("LoadState() = %+v, want nil (declined must not freeze or transition)", state)
	}
}

func TestReviewStart_Granted_FreezesClassifiesAndTransitions(t *testing.T) {
	workspace := t.TempDir()
	commonDir := filepath.Join(workspace, ".git")
	home := t.TempDir()
	authorityDir := reviewAuthorityDir(commonDir)
	identity := testCandidateIdentity()
	withStubResolveWorkspace(t, workspace, nil)
	withStubGitCommonDirFn(t, commonDir, nil)
	withStubUserHomeDir(t, home, nil)
	withStubReviewNow(t, time.Date(2026, 8, 21, 12, 0, 0, 0, time.UTC))
	withStubBuildCandidate(t, identity, []string{"internal/permissions/handler.go"}, nil)

	var buf bytes.Buffer
	handled, exitCode, err := reviewCommand("review", []string{"start", "--consent", "granted"}, &buf)
	if !handled || exitCode != 0 || err != nil {
		t.Fatalf("handled=%v exitCode=%d err=%v, want handled=true exitCode=0 err=nil", handled, exitCode, err)
	}

	store := reviewstore.NewStore(authorityDir)
	state, loadErr := store.LoadState()
	if loadErr != nil {
		t.Fatalf("LoadState: %v", loadErr)
	}
	if state == nil {
		t.Fatal("LoadState() = nil, want a frozen reviewing state")
	}
	if state.State != rdd.StateReviewing {
		t.Errorf("State = %q, want %q", state.State, rdd.StateReviewing)
	}
	if state.Tier != rdd.RiskTierElevated {
		t.Errorf("Tier = %v, want %v", state.Tier, rdd.RiskTierElevated)
	}
	if len(state.SelectedLenses) != 1 || state.SelectedLenses[0] != rdd.LensRisk {
		t.Errorf("SelectedLenses = %v, want [R1]", state.SelectedLenses)
	}
	if state.Candidate != identity {
		t.Errorf("Candidate = %+v, want %+v", state.Candidate, identity)
	}
	if state.SubjectHash == "" {
		t.Errorf("SubjectHash is empty, want a frozen digest")
	}
	if state.Consent == nil || !state.Consent.Granted {
		t.Errorf("Consent = %+v, want Granted=true", state.Consent)
	}
}

func TestReviewCaptureResult_ValidLens_StaysReviewingWhenLensesRemain(t *testing.T) {
	workspace := t.TempDir()
	commonDir := filepath.Join(workspace, ".git")
	authorityDir := reviewAuthorityDir(commonDir)
	identity := testCandidateIdentity()
	withStubResolveWorkspace(t, workspace, nil)
	withStubGitCommonDirFn(t, commonDir, nil)
	withStubReviewNow(t, time.Date(2026, 8, 21, 13, 0, 0, 0, time.UTC))
	withStubBuildCandidate(t, identity, nil, nil)

	seedReviewingState(t, authorityDir, identity, []rdd.Lens{rdd.LensRisk, rdd.LensReadability}, nil)

	resultPath := filepath.Join(t.TempDir(), "r1.json")
	if err := os.WriteFile(resultPath, []byte(`{"issues":[]}`), 0o644); err != nil {
		t.Fatal(err)
	}

	var buf bytes.Buffer
	handled, exitCode, err := reviewCommand("review", []string{"capture-result", "--lens", "R1", resultPath}, &buf)
	if !handled || exitCode != 0 || err != nil {
		t.Fatalf("handled=%v exitCode=%d err=%v, want handled=true exitCode=0 err=nil", handled, exitCode, err)
	}

	store := reviewstore.NewStore(authorityDir)
	state, loadErr := store.LoadState()
	if loadErr != nil {
		t.Fatalf("LoadState: %v", loadErr)
	}
	if state.State != rdd.StateReviewing {
		t.Errorf("State = %q, want %q (lenses remain)", state.State, rdd.StateReviewing)
	}
	lr, ok := state.LensResults[rdd.LensRisk]
	if !ok {
		t.Fatal("LensResults[R1] missing")
	}
	if lr.SubjectHash != state.SubjectHash {
		t.Errorf("LensResults[R1].SubjectHash = %q, want %q", lr.SubjectHash, state.SubjectHash)
	}
	if !jsonEqual(t, lr.Findings, []byte(`{"issues":[]}`)) {
		t.Errorf("LensResults[R1].Findings = %s, want the file contents", lr.Findings)
	}
}

// jsonEqual reports whether a and b marshal to the same JSON value,
// ignoring whitespace — SaveState re-indents nested json.RawMessage
// content, so byte-exact comparison against the original file contents is
// not meaningful.
func jsonEqual(t *testing.T, a, b []byte) bool {
	t.Helper()
	var av, bv interface{}
	if err := json.Unmarshal(a, &av); err != nil {
		t.Fatalf("jsonEqual: invalid JSON a: %v", err)
	}
	if err := json.Unmarshal(b, &bv); err != nil {
		t.Fatalf("jsonEqual: invalid JSON b: %v", err)
	}
	aBytes, _ := json.Marshal(av)
	bBytes, _ := json.Marshal(bv)
	return string(aBytes) == string(bBytes)
}

func TestReviewCaptureResult_LastMandatedLens_TransitionsToFindingsFrozen(t *testing.T) {
	workspace := t.TempDir()
	commonDir := filepath.Join(workspace, ".git")
	authorityDir := reviewAuthorityDir(commonDir)
	identity := testCandidateIdentity()
	withStubResolveWorkspace(t, workspace, nil)
	withStubGitCommonDirFn(t, commonDir, nil)
	withStubReviewNow(t, time.Date(2026, 8, 21, 13, 0, 0, 0, time.UTC))
	withStubBuildCandidate(t, identity, nil, nil)

	seedReviewingState(t, authorityDir, identity, []rdd.Lens{rdd.LensRisk}, nil)

	resultPath := filepath.Join(t.TempDir(), "r1.json")
	if err := os.WriteFile(resultPath, []byte(`{"issues":[]}`), 0o644); err != nil {
		t.Fatal(err)
	}

	var buf bytes.Buffer
	handled, exitCode, err := reviewCommand("review", []string{"capture-result", "--lens", "R1", resultPath}, &buf)
	if !handled || exitCode != 0 || err != nil {
		t.Fatalf("handled=%v exitCode=%d err=%v, want handled=true exitCode=0 err=nil", handled, exitCode, err)
	}

	store := reviewstore.NewStore(authorityDir)
	state, loadErr := store.LoadState()
	if loadErr != nil {
		t.Fatalf("LoadState: %v", loadErr)
	}
	if state.State != rdd.StateFindingsFrozen {
		t.Errorf("State = %q, want %q (last mandated lens captured)", state.State, rdd.StateFindingsFrozen)
	}
}

func TestReviewCaptureResult_DuplicateLens_RejectedStateUnchanged(t *testing.T) {
	workspace := t.TempDir()
	commonDir := filepath.Join(workspace, ".git")
	authorityDir := reviewAuthorityDir(commonDir)
	identity := testCandidateIdentity()
	withStubResolveWorkspace(t, workspace, nil)
	withStubGitCommonDirFn(t, commonDir, nil)
	withStubBuildCandidate(t, identity, nil, nil)

	existing := map[rdd.Lens]reviewstore.LensResult{
		rdd.LensRisk: {
			LensID:      rdd.LensRisk,
			SubjectHash: computeSubjectHash(identity),
			Findings:    []byte(`{"issues":[]}`),
			CapturedAt:  "2026-08-21T00:00:00Z",
		},
	}
	seeded := seedReviewingState(t, authorityDir, identity, []rdd.Lens{rdd.LensRisk, rdd.LensReadability}, existing)

	resultPath := filepath.Join(t.TempDir(), "r1.json")
	if err := os.WriteFile(resultPath, []byte(`{"issues":["new"]}`), 0o644); err != nil {
		t.Fatal(err)
	}

	var buf bytes.Buffer
	handled, exitCode, err := reviewCommand("review", []string{"capture-result", "--lens", "R1", resultPath}, &buf)
	if !handled || exitCode != 1 || err == nil {
		t.Fatalf("handled=%v exitCode=%d err=%v, want handled=true exitCode=1 err!=nil", handled, exitCode, err)
	}

	store := reviewstore.NewStore(authorityDir)
	state, loadErr := store.LoadState()
	if loadErr != nil {
		t.Fatalf("LoadState: %v", loadErr)
	}
	if state.Version != seeded.Version {
		t.Errorf("Version = %d, want %d (duplicate submission must not mutate state)", state.Version, seeded.Version)
	}
	if !jsonEqual(t, state.LensResults[rdd.LensRisk].Findings, []byte(`{"issues":[]}`)) {
		t.Errorf("LensResults[R1].Findings changed, want original preserved")
	}
}

func TestReviewCaptureResult_UnmandatedLens_Rejected(t *testing.T) {
	workspace := t.TempDir()
	commonDir := filepath.Join(workspace, ".git")
	authorityDir := reviewAuthorityDir(commonDir)
	identity := testCandidateIdentity()
	withStubResolveWorkspace(t, workspace, nil)
	withStubGitCommonDirFn(t, commonDir, nil)
	withStubBuildCandidate(t, identity, nil, nil)

	seedReviewingState(t, authorityDir, identity, []rdd.Lens{rdd.LensRisk}, nil)

	resultPath := filepath.Join(t.TempDir(), "r3.json")
	if err := os.WriteFile(resultPath, []byte(`{"issues":[]}`), 0o644); err != nil {
		t.Fatal(err)
	}

	var buf bytes.Buffer
	handled, exitCode, err := reviewCommand("review", []string{"capture-result", "--lens", "R3", resultPath}, &buf)
	if !handled || exitCode != 1 || err == nil {
		t.Fatalf("handled=%v exitCode=%d err=%v, want handled=true exitCode=1 err!=nil", handled, exitCode, err)
	}
	if !strings.Contains(err.Error(), "not mandated") {
		t.Errorf("err = %v, want it to mention not mandated", err)
	}
}

func TestReviewCaptureResult_SubjectHashMismatch_Rejected(t *testing.T) {
	workspace := t.TempDir()
	commonDir := filepath.Join(workspace, ".git")
	authorityDir := reviewAuthorityDir(commonDir)
	frozenIdentity := testCandidateIdentity()
	driftedIdentity := frozenIdentity
	driftedIdentity.CandidateTree = "different-candidate-tree-hash"
	withStubResolveWorkspace(t, workspace, nil)
	withStubGitCommonDirFn(t, commonDir, nil)
	withStubBuildCandidate(t, driftedIdentity, nil, nil)

	seedReviewingState(t, authorityDir, frozenIdentity, []rdd.Lens{rdd.LensRisk}, nil)

	resultPath := filepath.Join(t.TempDir(), "r1.json")
	if err := os.WriteFile(resultPath, []byte(`{"issues":[]}`), 0o644); err != nil {
		t.Fatal(err)
	}

	var buf bytes.Buffer
	handled, exitCode, err := reviewCommand("review", []string{"capture-result", "--lens", "R1", resultPath}, &buf)
	if !handled || exitCode != 1 || err == nil {
		t.Fatalf("handled=%v exitCode=%d err=%v, want handled=true exitCode=1 err!=nil", handled, exitCode, err)
	}
}

// --- review finalize / schema reviewer (PR6) ---

// withStubFinalizeRequiresFix stubs finalizeRequiresFix to return
// requiresFix, mirroring the other seam-var stub helpers above so tests can
// drive `review finalize` down the fix_required path without real evidence
// classification (R-2 has none yet).
func withStubFinalizeRequiresFix(t *testing.T, requiresFix bool) {
	t.Helper()
	prev := finalizeRequiresFix
	finalizeRequiresFix = func(*reviewstore.ReviewState) bool { return requiresFix }
	t.Cleanup(func() { finalizeRequiresFix = prev })
}

// withStubFinalizeVerificationOutcome stubs finalizeVerificationOutcome to
// return outcome, so tests can drive `review finalize` to a specific final
// verification result without real lens-result failure detection (R-2 has
// none yet).
func withStubFinalizeVerificationOutcome(t *testing.T, outcome rdd.LifecycleState) {
	t.Helper()
	prev := finalizeVerificationOutcome
	finalizeVerificationOutcome = func(*reviewstore.ReviewState) rdd.LifecycleState { return outcome }
	t.Cleanup(func() { finalizeVerificationOutcome = prev })
}

// seedStateAt directly writes a review-state.json with the given lifecycle
// state and lens results for every selected lens, bypassing the state
// machine, so finalize tests can start from an arbitrary state without
// driving `review start`/`review capture-result` first.
func seedStateAt(t *testing.T, authorityDir string, identity rdd.CandidateIdentity, state rdd.LifecycleState, selected []rdd.Lens) *reviewstore.ReviewState {
	t.Helper()
	store := reviewstore.NewStore(authorityDir)
	results := make(map[rdd.Lens]reviewstore.LensResult, len(selected))
	for _, l := range selected {
		results[l] = reviewstore.LensResult{
			LensID:      l,
			SubjectHash: computeSubjectHash(identity),
			Findings:    json.RawMessage(`{"issues":[]}`),
			CapturedAt:  "2026-08-21T00:00:00Z",
		}
	}
	st := &reviewstore.ReviewState{
		SchemaVersion:  1,
		Candidate:      identity,
		Tier:           rdd.RiskTierElevated,
		Lenses:         len(selected),
		State:          state,
		SubjectHash:    computeSubjectHash(identity),
		SelectedLenses: selected,
		LensResults:    results,
		CreatedAt:      "2026-08-21T00:00:00Z",
		UpdatedAt:      "2026-08-21T00:00:00Z",
	}
	if err := store.SaveState(st); err != nil {
		t.Fatalf("seedStateAt: SaveState: %v", err)
	}
	return st
}

func TestReviewFinalize_FromFindingsFrozen_DrivesToApprovedAndWritesReceipt(t *testing.T) {
	workspace := t.TempDir()
	commonDir := filepath.Join(workspace, ".git")
	authorityDir := reviewAuthorityDir(commonDir)
	identity := testCandidateIdentity()
	withStubResolveWorkspace(t, workspace, nil)
	withStubGitCommonDirFn(t, commonDir, nil)
	withStubReviewNow(t, time.Date(2026, 8, 21, 14, 0, 0, 0, time.UTC))

	seedStateAt(t, authorityDir, identity, rdd.StateFindingsFrozen, []rdd.Lens{rdd.LensRisk})

	var buf bytes.Buffer
	handled, exitCode, err := reviewCommand("review", []string{"finalize"}, &buf)
	if !handled || exitCode != 0 || err != nil {
		t.Fatalf("handled=%v exitCode=%d err=%v, want handled=true exitCode=0 err=nil", handled, exitCode, err)
	}

	store := reviewstore.NewStore(authorityDir)
	state, loadErr := store.LoadState()
	if loadErr != nil {
		t.Fatalf("LoadState: %v", loadErr)
	}
	if state.State != rdd.StateApproved {
		t.Errorf("State = %q, want %q", state.State, rdd.StateApproved)
	}
	if !rdd.IsTerminal(state.State) {
		t.Errorf("State = %q, want a terminal state", state.State)
	}

	receipt, receiptErr := store.LoadReceipt()
	if receiptErr != nil {
		t.Fatalf("LoadReceipt: %v", receiptErr)
	}
	if receipt == nil {
		t.Fatal("LoadReceipt() = nil, want a terminal receipt")
	}
	if receipt.Outcome != string(rdd.StateApproved) {
		t.Errorf("receipt.Outcome = %q, want %q", receipt.Outcome, rdd.StateApproved)
	}
}

func TestReviewFinalize_FixRequiredPath_EscalatesWithoutFixCLI(t *testing.T) {
	workspace := t.TempDir()
	commonDir := filepath.Join(workspace, ".git")
	authorityDir := reviewAuthorityDir(commonDir)
	identity := testCandidateIdentity()
	withStubResolveWorkspace(t, workspace, nil)
	withStubGitCommonDirFn(t, commonDir, nil)
	withStubReviewNow(t, time.Date(2026, 8, 21, 14, 0, 0, 0, time.UTC))
	withStubFinalizeRequiresFix(t, true)

	seedStateAt(t, authorityDir, identity, rdd.StateFindingsFrozen, []rdd.Lens{rdd.LensRisk})

	var buf bytes.Buffer
	handled, exitCode, err := reviewCommand("review", []string{"finalize"}, &buf)
	if !handled || exitCode != 0 || err != nil {
		t.Fatalf("handled=%v exitCode=%d err=%v, want handled=true exitCode=0 err=nil", handled, exitCode, err)
	}

	store := reviewstore.NewStore(authorityDir)
	state, loadErr := store.LoadState()
	if loadErr != nil {
		t.Fatalf("LoadState: %v", loadErr)
	}
	if state.State != rdd.StateEscalated {
		t.Errorf("State = %q, want %q (no fix CLI in R-2, fix_required must auto-escalate)", state.State, rdd.StateEscalated)
	}

	receipt, receiptErr := store.LoadReceipt()
	if receiptErr != nil {
		t.Fatalf("LoadReceipt: %v", receiptErr)
	}
	if receipt == nil {
		t.Fatal("LoadReceipt() = nil, want a terminal receipt")
	}
	if receipt.Outcome != string(rdd.StateEscalated) {
		t.Errorf("receipt.Outcome = %q, want %q", receipt.Outcome, rdd.StateEscalated)
	}
}

func TestReviewFinalize_FinalVerificationFailure_Escalates(t *testing.T) {
	workspace := t.TempDir()
	commonDir := filepath.Join(workspace, ".git")
	authorityDir := reviewAuthorityDir(commonDir)
	identity := testCandidateIdentity()
	withStubResolveWorkspace(t, workspace, nil)
	withStubGitCommonDirFn(t, commonDir, nil)
	withStubReviewNow(t, time.Date(2026, 8, 21, 14, 0, 0, 0, time.UTC))
	withStubFinalizeVerificationOutcome(t, rdd.StateEscalated)

	seedStateAt(t, authorityDir, identity, rdd.StateFindingsFrozen, []rdd.Lens{rdd.LensRisk})

	var buf bytes.Buffer
	handled, exitCode, err := reviewCommand("review", []string{"finalize"}, &buf)
	if !handled || exitCode != 0 || err != nil {
		t.Fatalf("handled=%v exitCode=%d err=%v, want handled=true exitCode=0 err=nil", handled, exitCode, err)
	}

	store := reviewstore.NewStore(authorityDir)
	state, loadErr := store.LoadState()
	if loadErr != nil {
		t.Fatalf("LoadState: %v", loadErr)
	}
	if state.State != rdd.StateEscalated {
		t.Errorf("State = %q, want %q", state.State, rdd.StateEscalated)
	}
}

func TestReviewFinalize_ReviewingState_Rejected(t *testing.T) {
	workspace := t.TempDir()
	commonDir := filepath.Join(workspace, ".git")
	authorityDir := reviewAuthorityDir(commonDir)
	identity := testCandidateIdentity()
	withStubResolveWorkspace(t, workspace, nil)
	withStubGitCommonDirFn(t, commonDir, nil)

	seedReviewingState(t, authorityDir, identity, []rdd.Lens{rdd.LensRisk}, nil)

	var buf bytes.Buffer
	handled, exitCode, err := reviewCommand("review", []string{"finalize"}, &buf)
	if !handled || exitCode != 1 || err == nil {
		t.Fatalf("handled=%v exitCode=%d err=%v, want handled=true exitCode=1 err!=nil", handled, exitCode, err)
	}
	if !strings.Contains(err.Error(), "findings_frozen") {
		t.Errorf("err = %v, want it to mention the required findings_frozen state", err)
	}
}

func TestReviewFinalize_TerminalState_Rejected(t *testing.T) {
	workspace := t.TempDir()
	commonDir := filepath.Join(workspace, ".git")
	authorityDir := reviewAuthorityDir(commonDir)
	identity := testCandidateIdentity()
	withStubResolveWorkspace(t, workspace, nil)
	withStubGitCommonDirFn(t, commonDir, nil)

	seedStateAt(t, authorityDir, identity, rdd.StateApproved, []rdd.Lens{rdd.LensRisk})

	var buf bytes.Buffer
	handled, exitCode, err := reviewCommand("review", []string{"finalize"}, &buf)
	if !handled || exitCode != 1 || err == nil {
		t.Fatalf("handled=%v exitCode=%d err=%v, want handled=true exitCode=1 err!=nil", handled, exitCode, err)
	}
}

func TestReviewFinalize_NoState_Rejected(t *testing.T) {
	workspace := t.TempDir()
	commonDir := filepath.Join(workspace, ".git")
	withStubResolveWorkspace(t, workspace, nil)
	withStubGitCommonDirFn(t, commonDir, nil)

	// No `review start` has run: no state file exists at all.
	var buf bytes.Buffer
	handled, exitCode, err := reviewCommand("review", []string{"finalize"}, &buf)
	if !handled || exitCode != 1 || err == nil {
		t.Fatalf("handled=%v exitCode=%d err=%v, want handled=true exitCode=1 err!=nil", handled, exitCode, err)
	}
}

func TestReviewSchemaReviewer_OutputsValidJSONSchema(t *testing.T) {
	var buf bytes.Buffer
	handled, exitCode, err := reviewCommand("review", []string{"schema", "reviewer"}, &buf)
	if !handled || exitCode != 0 || err != nil {
		t.Fatalf("handled=%v exitCode=%d err=%v, want handled=true exitCode=0 err=nil", handled, exitCode, err)
	}
	if !json.Valid(buf.Bytes()) {
		t.Fatalf("output is not valid JSON: %q", buf.String())
	}
	var schema map[string]interface{}
	if err := json.Unmarshal(buf.Bytes(), &schema); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}
	if _, ok := schema["properties"]; !ok {
		t.Errorf("schema = %v, want a \"properties\" key describing the findings shape", schema)
	}
}

func TestReviewSchemaReviewer_WorksInAnyStateWithoutMutatingState(t *testing.T) {
	workspace := t.TempDir()
	commonDir := filepath.Join(workspace, ".git")
	authorityDir := reviewAuthorityDir(commonDir)
	withStubResolveWorkspace(t, workspace, nil)
	withStubGitCommonDirFn(t, commonDir, nil)

	// No review state exists at all — schema reviewer must still succeed.
	var buf bytes.Buffer
	handled, exitCode, err := reviewCommand("review", []string{"schema", "reviewer"}, &buf)
	if !handled || exitCode != 0 || err != nil {
		t.Fatalf("handled=%v exitCode=%d err=%v, want handled=true exitCode=0 err=nil", handled, exitCode, err)
	}

	store := reviewstore.NewStore(authorityDir)
	state, loadErr := store.LoadState()
	if loadErr != nil {
		t.Fatalf("LoadState: %v", loadErr)
	}
	if state != nil {
		t.Errorf("LoadState() = %+v, want nil (schema reviewer must not mutate state)", state)
	}
}

func TestReviewCaptureResult_StateNotReviewing_Rejected(t *testing.T) {
	workspace := t.TempDir()
	commonDir := filepath.Join(workspace, ".git")
	identity := testCandidateIdentity()
	withStubResolveWorkspace(t, workspace, nil)
	withStubGitCommonDirFn(t, commonDir, nil)
	withStubBuildCandidate(t, identity, nil, nil)

	// No `review start` has run: no state file exists at all.
	resultPath := filepath.Join(t.TempDir(), "r1.json")
	if err := os.WriteFile(resultPath, []byte(`{"issues":[]}`), 0o644); err != nil {
		t.Fatal(err)
	}

	var buf bytes.Buffer
	handled, exitCode, err := reviewCommand("review", []string{"capture-result", "--lens", "R1", resultPath}, &buf)
	if !handled || exitCode != 1 || err == nil {
		t.Fatalf("handled=%v exitCode=%d err=%v, want handled=true exitCode=1 err!=nil", handled, exitCode, err)
	}
	if !strings.Contains(err.Error(), "reviewing") {
		t.Errorf("err = %v, want it to mention the required reviewing state", err)
	}
}

// --- review validate / review gate (PR2c) ---

// withStubBuildIdentity stubs buildIdentity to return identity (or err),
// mirroring the other seam-var stub helpers above so `review validate
// --gate pre-commit` tests never need a real git repository.
func withStubBuildIdentity(t *testing.T, identity rdd.CandidateIdentity, err error) {
	t.Helper()
	prev := buildIdentity
	buildIdentity = func(string) (rdd.CandidateIdentity, error) { return identity, err }
	t.Cleanup(func() { buildIdentity = prev })
}

// withStubBuildIdentityFromCommit stubs buildIdentityFromCommit to fn, so
// `review validate --gate pre-push` tests can vary the returned identity
// per commit SHA (e.g. one ref passes, another fails).
func withStubBuildIdentityFromCommit(t *testing.T, fn func(workspace, commitSHA string) (rdd.CandidateIdentity, error)) {
	t.Helper()
	prev := buildIdentityFromCommit
	buildIdentityFromCommit = fn
	t.Cleanup(func() { buildIdentityFromCommit = prev })
}

// withStubBuildGateAncestryEvidence stubs buildGateAncestryEvidence to fn,
// so gate tests can drive rdd.Compare to a specific Relation without a real
// git repository (BuildGateAncestryEvidence's real implementation shells
// out to `git diff` + `git patch-id`).
func withStubBuildGateAncestryEvidence(t *testing.T, fn func(workspace string, a, b rdd.CandidateIdentity) (*rdd.AncestryEvidence, error)) {
	t.Helper()
	prev := buildGateAncestryEvidence
	buildGateAncestryEvidence = fn
	t.Cleanup(func() { buildGateAncestryEvidence = prev })
}

// withStubReadPrePushStdin stubs readPrePushStdin to return refs (or err),
// so `review validate --gate pre-push` tests never need to feed real stdin.
func withStubReadPrePushStdin(t *testing.T, refs []rddgate.PushRef, err error) {
	t.Helper()
	prev := readPrePushStdin
	readPrePushStdin = func() ([]rddgate.PushRef, error) { return refs, err }
	t.Cleanup(func() { readPrePushStdin = prev })
}

// seedReceipt writes a terminal ReviewReceipt directly via WriteReceipt,
// bypassing `review finalize`, so gate tests can exercise validate in
// isolation.
func seedReceipt(t *testing.T, authorityDir string, identity rdd.CandidateIdentity, outcome rdd.LifecycleState) *reviewstore.ReviewReceipt {
	t.Helper()
	store := reviewstore.NewStore(authorityDir)
	receipt := &reviewstore.ReviewReceipt{
		SchemaVersion: 1,
		Candidate:     identity,
		Tier:          rdd.RiskTierElevated,
		Outcome:       string(outcome),
		IssuedAt:      "2026-08-21T00:00:00Z",
	}
	if err := store.WriteReceipt(receipt); err != nil {
		t.Fatalf("seedReceipt: WriteReceipt: %v", err)
	}
	return receipt
}

func TestReviewValidate_MissingGateFlag_FailsClosed(t *testing.T) {
	var buf bytes.Buffer
	handled, exitCode, err := reviewCommand("review", []string{"validate"}, &buf)
	if !handled || exitCode != 1 || err == nil {
		t.Fatalf("handled=%v exitCode=%d err=%v, want handled=true exitCode=1 err!=nil", handled, exitCode, err)
	}
	if !strings.Contains(err.Error(), "--gate") {
		t.Errorf("err = %v, want it to mention --gate", err)
	}
}

func TestReviewValidate_UnknownGateValue_FailsClosedBeforeIO(t *testing.T) {
	// resolveWorkspace must never be reached: unknown --gate values fail
	// closed before any receipt/candidate access (spec delivery-gates
	// "Unknown gate value").
	withStubResolveWorkspace(t, "", errors.New("resolveWorkspace should not be called"))

	var buf bytes.Buffer
	handled, exitCode, err := reviewCommand("review", []string{"validate", "--gate", "foo"}, &buf)
	if !handled || exitCode != 1 || err == nil {
		t.Fatalf("handled=%v exitCode=%d err=%v, want handled=true exitCode=1 err!=nil", handled, exitCode, err)
	}
	if !strings.Contains(err.Error(), "unknown gate") {
		t.Errorf("err = %v, want it to mention unknown gate", err)
	}
}

func TestReviewValidate_PreCommit_Disabled_ExitsZeroSilently(t *testing.T) {
	workspace := t.TempDir()
	commonDir := filepath.Join(workspace, ".git")
	home := t.TempDir()
	withStubResolveWorkspace(t, workspace, nil)
	withStubGitCommonDirFn(t, commonDir, nil)
	withStubUserHomeDir(t, home, nil)

	clonePath := cloneModePath(commonDir)
	if err := reviewstore.SaveModeRecord(clonePath, &rdd.ModeRecord{
		SchemaVersion: 1,
		Mode:          rdd.ModeDisabled,
		UpdatedAt:     "2026-08-21T00:00:00Z",
	}); err != nil {
		t.Fatal(err)
	}

	var buf bytes.Buffer
	handled, exitCode, err := reviewCommand("review", []string{"validate", "--gate", "pre-commit"}, &buf)
	if !handled || exitCode != 0 || err != nil {
		t.Fatalf("handled=%v exitCode=%d err=%v, want handled=true exitCode=0 err=nil", handled, exitCode, err)
	}
	if buf.String() != "" {
		t.Errorf("output = %q, want empty (silent skip)", buf.String())
	}
}

func TestReviewValidate_PrePush_Disabled_ExitsZeroSilently(t *testing.T) {
	workspace := t.TempDir()
	commonDir := filepath.Join(workspace, ".git")
	home := t.TempDir()
	withStubResolveWorkspace(t, workspace, nil)
	withStubGitCommonDirFn(t, commonDir, nil)
	withStubUserHomeDir(t, home, nil)

	clonePath := cloneModePath(commonDir)
	if err := reviewstore.SaveModeRecord(clonePath, &rdd.ModeRecord{
		SchemaVersion: 1,
		Mode:          rdd.ModeDisabled,
		UpdatedAt:     "2026-08-21T00:00:00Z",
	}); err != nil {
		t.Fatal(err)
	}

	var buf bytes.Buffer
	handled, exitCode, err := reviewCommand("review", []string{"validate", "--gate", "pre-push"}, &buf)
	if !handled || exitCode != 0 || err != nil {
		t.Fatalf("handled=%v exitCode=%d err=%v, want handled=true exitCode=0 err=nil", handled, exitCode, err)
	}
	if buf.String() != "" {
		t.Errorf("output = %q, want empty (silent skip)", buf.String())
	}
}

func TestReviewValidate_PreCommit_NoReceipt_FailsClosed(t *testing.T) {
	workspace := t.TempDir()
	commonDir := filepath.Join(workspace, ".git")
	home := t.TempDir()
	withStubResolveWorkspace(t, workspace, nil)
	withStubGitCommonDirFn(t, commonDir, nil)
	withStubUserHomeDir(t, home, nil)
	// No mode record: effective mode defaults to managed (fail-closed). No
	// receipt seeded either.

	var buf bytes.Buffer
	handled, exitCode, err := reviewCommand("review", []string{"validate", "--gate", "pre-commit"}, &buf)
	if !handled || exitCode != 1 || err == nil {
		t.Fatalf("handled=%v exitCode=%d err=%v, want handled=true exitCode=1 err!=nil", handled, exitCode, err)
	}
	if !strings.Contains(err.Error(), "no review receipt") {
		t.Errorf("err = %v, want it to mention no review receipt", err)
	}
}

func TestReviewValidate_PreCommit_CorruptReceipt_FailsClosed(t *testing.T) {
	workspace := t.TempDir()
	commonDir := filepath.Join(workspace, ".git")
	home := t.TempDir()
	authorityDir := reviewAuthorityDir(commonDir)
	withStubResolveWorkspace(t, workspace, nil)
	withStubGitCommonDirFn(t, commonDir, nil)
	withStubUserHomeDir(t, home, nil)

	if err := writeFileAll(filepath.Join(authorityDir, "review-receipt.json"), []byte("{not json")); err != nil {
		t.Fatal(err)
	}

	var buf bytes.Buffer
	handled, exitCode, err := reviewCommand("review", []string{"validate", "--gate", "pre-commit"}, &buf)
	if !handled || exitCode != 1 || err == nil {
		t.Fatalf("handled=%v exitCode=%d err=%v, want handled=true exitCode=1 err!=nil", handled, exitCode, err)
	}
}

func TestReviewValidate_PreCommit_ExactRelation_Passes(t *testing.T) {
	workspace := t.TempDir()
	commonDir := filepath.Join(workspace, ".git")
	home := t.TempDir()
	authorityDir := reviewAuthorityDir(commonDir)
	identity := testCandidateIdentity()
	withStubResolveWorkspace(t, workspace, nil)
	withStubGitCommonDirFn(t, commonDir, nil)
	withStubUserHomeDir(t, home, nil)
	withStubBuildIdentity(t, identity, nil)

	seedReceipt(t, authorityDir, identity, rdd.StateApproved)

	var buf bytes.Buffer
	handled, exitCode, err := reviewCommand("review", []string{"validate", "--gate", "pre-commit"}, &buf)
	if !handled || exitCode != 0 || err != nil {
		t.Fatalf("handled=%v exitCode=%d err=%v, want handled=true exitCode=0 err=nil", handled, exitCode, err)
	}
}

func TestReviewValidate_PreCommit_CompatibleBaseAdvance_Passes(t *testing.T) {
	workspace := t.TempDir()
	commonDir := filepath.Join(workspace, ".git")
	home := t.TempDir()
	authorityDir := reviewAuthorityDir(commonDir)
	receiptIdentity := testCandidateIdentity()
	currentIdentity := receiptIdentity
	currentIdentity.BaseTree = "advanced-base-tree-hash"
	withStubResolveWorkspace(t, workspace, nil)
	withStubGitCommonDirFn(t, commonDir, nil)
	withStubUserHomeDir(t, home, nil)
	withStubBuildIdentity(t, currentIdentity, nil)
	withStubBuildGateAncestryEvidence(t, func(string, rdd.CandidateIdentity, rdd.CandidateIdentity) (*rdd.AncestryEvidence, error) {
		return &rdd.AncestryEvidence{MergeBaseHash: "gate-evidence", APatchID: "patch-1", BPatchID: "patch-1"}, nil
	})

	seedReceipt(t, authorityDir, receiptIdentity, rdd.StateApproved)

	var buf bytes.Buffer
	handled, exitCode, err := reviewCommand("review", []string{"validate", "--gate", "pre-commit"}, &buf)
	if !handled || exitCode != 0 || err != nil {
		t.Fatalf("handled=%v exitCode=%d err=%v, want handled=true exitCode=0 err=nil", handled, exitCode, err)
	}
}

func TestReviewValidate_PreCommit_ChangedRelation_Blocks(t *testing.T) {
	workspace := t.TempDir()
	commonDir := filepath.Join(workspace, ".git")
	home := t.TempDir()
	authorityDir := reviewAuthorityDir(commonDir)
	receiptIdentity := testCandidateIdentity()
	currentIdentity := receiptIdentity
	currentIdentity.CandidateTree = "different-candidate-tree-hash"
	withStubResolveWorkspace(t, workspace, nil)
	withStubGitCommonDirFn(t, commonDir, nil)
	withStubUserHomeDir(t, home, nil)
	withStubBuildIdentity(t, currentIdentity, nil)
	withStubBuildGateAncestryEvidence(t, func(string, rdd.CandidateIdentity, rdd.CandidateIdentity) (*rdd.AncestryEvidence, error) {
		return &rdd.AncestryEvidence{MergeBaseHash: "gate-evidence", APatchID: "patch-1", BPatchID: "patch-2"}, nil
	})

	seedReceipt(t, authorityDir, receiptIdentity, rdd.StateApproved)

	var buf bytes.Buffer
	handled, exitCode, err := reviewCommand("review", []string{"validate", "--gate", "pre-commit"}, &buf)
	if !handled || exitCode != 1 || err == nil {
		t.Fatalf("handled=%v exitCode=%d err=%v, want handled=true exitCode=1 err!=nil", handled, exitCode, err)
	}
	if !strings.Contains(err.Error(), "does not cover") {
		t.Errorf("err = %v, want it to mention the receipt does not cover the candidate", err)
	}
}

func TestReviewValidate_PrePush_AllRefsCovered_ExitsZero(t *testing.T) {
	workspace := t.TempDir()
	commonDir := filepath.Join(workspace, ".git")
	home := t.TempDir()
	authorityDir := reviewAuthorityDir(commonDir)
	identity := testCandidateIdentity()
	withStubResolveWorkspace(t, workspace, nil)
	withStubGitCommonDirFn(t, commonDir, nil)
	withStubUserHomeDir(t, home, nil)
	withStubReadPrePushStdin(t, []rddgate.PushRef{
		{LocalRef: "refs/heads/main", LocalSHA: "sha-main", RemoteRef: "refs/heads/main", RemoteSHA: "remote-main"},
		{LocalRef: "refs/heads/feature", LocalSHA: "sha-feature", RemoteRef: "refs/heads/feature", RemoteSHA: "remote-feature"},
	}, nil)
	withStubBuildIdentityFromCommit(t, func(workspace, commitSHA string) (rdd.CandidateIdentity, error) {
		return identity, nil
	})

	seedReceipt(t, authorityDir, identity, rdd.StateApproved)

	var buf bytes.Buffer
	handled, exitCode, err := reviewCommand("review", []string{"validate", "--gate", "pre-push"}, &buf)
	if !handled || exitCode != 0 || err != nil {
		t.Fatalf("handled=%v exitCode=%d err=%v, want handled=true exitCode=0 err=nil", handled, exitCode, err)
	}
}

func TestReviewValidate_PrePush_OneRefUncovered_BlocksEntirePush(t *testing.T) {
	workspace := t.TempDir()
	commonDir := filepath.Join(workspace, ".git")
	home := t.TempDir()
	authorityDir := reviewAuthorityDir(commonDir)
	receiptIdentity := testCandidateIdentity()
	uncoveredIdentity := receiptIdentity
	uncoveredIdentity.CandidateTree = "uncovered-candidate-tree-hash"
	withStubResolveWorkspace(t, workspace, nil)
	withStubGitCommonDirFn(t, commonDir, nil)
	withStubUserHomeDir(t, home, nil)
	withStubReadPrePushStdin(t, []rddgate.PushRef{
		{LocalRef: "refs/heads/main", LocalSHA: "sha-good", RemoteRef: "refs/heads/main", RemoteSHA: "remote-main"},
		{LocalRef: "refs/heads/feature", LocalSHA: "sha-bad", RemoteRef: "refs/heads/feature", RemoteSHA: "remote-feature"},
	}, nil)
	withStubBuildIdentityFromCommit(t, func(workspace, commitSHA string) (rdd.CandidateIdentity, error) {
		if commitSHA == "sha-bad" {
			return uncoveredIdentity, nil
		}
		return receiptIdentity, nil
	})
	withStubBuildGateAncestryEvidence(t, func(string, rdd.CandidateIdentity, rdd.CandidateIdentity) (*rdd.AncestryEvidence, error) {
		return nil, nil
	})

	seedReceipt(t, authorityDir, receiptIdentity, rdd.StateApproved)

	var buf bytes.Buffer
	handled, exitCode, err := reviewCommand("review", []string{"validate", "--gate", "pre-push"}, &buf)
	if !handled || exitCode != 1 || err == nil {
		t.Fatalf("handled=%v exitCode=%d err=%v, want handled=true exitCode=1 err!=nil", handled, exitCode, err)
	}
	if !strings.Contains(err.Error(), "refs/heads/feature") {
		t.Errorf("err = %v, want it to identify the blocking ref refs/heads/feature", err)
	}
}

func TestReviewValidate_PrePush_DeleteRefSkipped(t *testing.T) {
	workspace := t.TempDir()
	commonDir := filepath.Join(workspace, ".git")
	home := t.TempDir()
	authorityDir := reviewAuthorityDir(commonDir)
	identity := testCandidateIdentity()
	deletedSHA := strings.Repeat("0", 40)
	withStubResolveWorkspace(t, workspace, nil)
	withStubGitCommonDirFn(t, commonDir, nil)
	withStubUserHomeDir(t, home, nil)
	withStubReadPrePushStdin(t, []rddgate.PushRef{
		{LocalRef: "refs/heads/deleted", LocalSHA: deletedSHA, RemoteRef: "refs/heads/deleted", RemoteSHA: "remote-deleted"},
		{LocalRef: "refs/heads/main", LocalSHA: "sha-good", RemoteRef: "refs/heads/main", RemoteSHA: "remote-main"},
	}, nil)
	called := make(map[string]bool)
	withStubBuildIdentityFromCommit(t, func(workspace, commitSHA string) (rdd.CandidateIdentity, error) {
		called[commitSHA] = true
		return identity, nil
	})

	seedReceipt(t, authorityDir, identity, rdd.StateApproved)

	var buf bytes.Buffer
	handled, exitCode, err := reviewCommand("review", []string{"validate", "--gate", "pre-push"}, &buf)
	if !handled || exitCode != 0 || err != nil {
		t.Fatalf("handled=%v exitCode=%d err=%v, want handled=true exitCode=0 err=nil", handled, exitCode, err)
	}
	if called[deletedSHA] {
		t.Errorf("buildIdentityFromCommit called for delete-ref %q, want it skipped", deletedSHA)
	}
	if !called["sha-good"] {
		t.Errorf("buildIdentityFromCommit not called for non-delete ref, want it validated")
	}
}

func TestReviewValidate_PrePush_StdinParseError_FailsClosed(t *testing.T) {
	workspace := t.TempDir()
	commonDir := filepath.Join(workspace, ".git")
	home := t.TempDir()
	authorityDir := reviewAuthorityDir(commonDir)
	withStubResolveWorkspace(t, workspace, nil)
	withStubGitCommonDirFn(t, commonDir, nil)
	withStubUserHomeDir(t, home, nil)
	withStubReadPrePushStdin(t, nil, errors.New("malformed pre-push stdin"))

	seedReceipt(t, authorityDir, testCandidateIdentity(), rdd.StateApproved)

	var buf bytes.Buffer
	handled, exitCode, err := reviewCommand("review", []string{"validate", "--gate", "pre-push"}, &buf)
	if !handled || exitCode != 1 || err == nil {
		t.Fatalf("handled=%v exitCode=%d err=%v, want handled=true exitCode=1 err!=nil", handled, exitCode, err)
	}
}

func TestReviewGate_RequiresVerb(t *testing.T) {
	var buf bytes.Buffer
	handled, exitCode, err := reviewCommand("review", []string{"gate"}, &buf)
	if !handled || exitCode != 1 || err == nil {
		t.Fatalf("handled=%v exitCode=%d err=%v, want handled=true exitCode=1 err!=nil", handled, exitCode, err)
	}
}

func TestReviewGate_UnknownVerb(t *testing.T) {
	var buf bytes.Buffer
	handled, exitCode, _ := reviewCommand("review", []string{"gate", "bogus"}, &buf)
	if !handled || exitCode != 1 {
		t.Fatalf("handled=%v exitCode=%d, want handled=true exitCode=1", handled, exitCode)
	}
	if !strings.Contains(buf.String(), "unknown gate verb") {
		t.Errorf("output = %q, want it to mention unknown gate verb", buf.String())
	}
}

func TestReviewGateInstall_WritesBothHooksWithRDDMarkersAndHookBody(t *testing.T) {
	workspace := t.TempDir()
	withStubResolveWorkspace(t, workspace, nil)

	var buf bytes.Buffer
	handled, exitCode, err := reviewCommand("review", []string{"gate", "install"}, &buf)
	if !handled || exitCode != 0 || err != nil {
		t.Fatalf("handled=%v exitCode=%d err=%v, want handled=true exitCode=0 err=nil", handled, exitCode, err)
	}

	preCommit, err := os.ReadFile(filepath.Join(workspace, ".git", "hooks", "pre-commit"))
	if err != nil {
		t.Fatalf("reading pre-commit hook: %v", err)
	}
	preCommitContent := string(preCommit)
	if !strings.Contains(preCommitContent, rddPreCommitMarkerStart) || !strings.Contains(preCommitContent, rddPreCommitMarkerEnd) {
		t.Errorf("pre-commit hook = %q, want it to contain the RDD pre-commit markers", preCommitContent)
	}
	if !strings.Contains(preCommitContent, "capiko-ai review validate --gate pre-commit") {
		t.Errorf("pre-commit hook = %q, want it to invoke review validate --gate pre-commit", preCommitContent)
	}

	prePush, err := os.ReadFile(filepath.Join(workspace, ".git", "hooks", "pre-push"))
	if err != nil {
		t.Fatalf("reading pre-push hook: %v", err)
	}
	prePushContent := string(prePush)
	if !strings.Contains(prePushContent, rddPrePushMarkerStart) || !strings.Contains(prePushContent, rddPrePushMarkerEnd) {
		t.Errorf("pre-push hook = %q, want it to contain the RDD pre-push markers", prePushContent)
	}
	if !strings.Contains(prePushContent, "capiko-ai review validate --gate pre-push") {
		t.Errorf("pre-push hook = %q, want it to invoke review validate --gate pre-push", prePushContent)
	}
}

func TestReviewGateInstall_ResolveWorkspaceError(t *testing.T) {
	withStubResolveWorkspace(t, "", errors.New("boom"))

	var buf bytes.Buffer
	handled, exitCode, err := reviewCommand("review", []string{"gate", "install"}, &buf)
	if !handled || exitCode != 1 || err == nil {
		t.Fatalf("handled=%v exitCode=%d err=%v, want handled=true exitCode=1 err!=nil", handled, exitCode, err)
	}
}

func TestReviewGateUninstall_RemovesOnlyRDDBlock(t *testing.T) {
	workspace := t.TempDir()
	withStubResolveWorkspace(t, workspace, nil)

	if _, _, err := reviewCommand("review", []string{"gate", "install"}, &bytes.Buffer{}); err != nil {
		t.Fatalf("seeding gate install: %v", err)
	}
	// Seed an unrelated block that must survive uninstall.
	if err := githooks.WriteBlock(workspace, "pre-commit", "# >>> capiko:cga:pre-commit >>>", "# <<< capiko:cga:pre-commit <<<", "echo cga"); err != nil {
		t.Fatalf("seeding unrelated pre-commit block: %v", err)
	}

	var buf bytes.Buffer
	handled, exitCode, err := reviewCommand("review", []string{"gate", "uninstall"}, &buf)
	if !handled || exitCode != 0 || err != nil {
		t.Fatalf("handled=%v exitCode=%d err=%v, want handled=true exitCode=0 err=nil", handled, exitCode, err)
	}

	preCommit, err := os.ReadFile(filepath.Join(workspace, ".git", "hooks", "pre-commit"))
	if err != nil {
		t.Fatalf("reading pre-commit hook: %v", err)
	}
	preCommitContent := string(preCommit)
	if strings.Contains(preCommitContent, rddPreCommitMarkerStart) {
		t.Errorf("pre-commit hook = %q, want the RDD block removed", preCommitContent)
	}
	if !strings.Contains(preCommitContent, "capiko:cga:pre-commit") {
		t.Errorf("pre-commit hook = %q, want the unrelated block preserved", preCommitContent)
	}

	// The pre-push hook had only the RDD block, so RemoveBlock deletes the
	// file entirely rather than leaving an inert capiko-only hook behind
	// (githooks.RemoveBlock: "delete the file when only the capiko-seeded
	// shebang would remain").
	prePush, err := os.ReadFile(filepath.Join(workspace, ".git", "hooks", "pre-push"))
	if err == nil {
		t.Errorf("pre-push hook = %q, want the file removed (no content left after RDD block removal)", string(prePush))
	} else if !os.IsNotExist(err) {
		t.Fatalf("reading pre-push hook: %v", err)
	}
}

func TestReviewGateUninstall_ResolveWorkspaceError(t *testing.T) {
	withStubResolveWorkspace(t, "", errors.New("boom"))

	var buf bytes.Buffer
	handled, exitCode, err := reviewCommand("review", []string{"gate", "uninstall"}, &buf)
	if !handled || exitCode != 1 || err == nil {
		t.Fatalf("handled=%v exitCode=%d err=%v, want handled=true exitCode=1 err!=nil", handled, exitCode, err)
	}
}
