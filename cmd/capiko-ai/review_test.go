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
	withStubBuildCandidate(t, identity, []string{"internal/authz/handler.go"}, nil)

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
	if string(lr.Findings) != `{"issues":[]}` {
		t.Errorf("LensResults[R1].Findings = %s, want the file contents", lr.Findings)
	}
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
	if string(state.LensResults[rdd.LensRisk].Findings) != `{"issues":[]}` {
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

func TestReviewCaptureResult_StateNotReviewing_Rejected(t *testing.T) {
	workspace := t.TempDir()
	commonDir := filepath.Join(workspace, ".git")
	authorityDir := reviewAuthorityDir(commonDir)
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
