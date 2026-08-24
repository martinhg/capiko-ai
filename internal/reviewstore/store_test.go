package reviewstore

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"

	"github.com/martinhg/capiko-ai/internal/rdd"
)

func testCandidate() rdd.CandidateIdentity {
	return rdd.CandidateIdentity{
		RepositoryID:            "capiko-ai",
		BaseTree:                "base-tree-sha",
		CandidateTree:           "candidate-tree-sha",
		ChangedPathsModesDigest: "digest-sha",
		PolicyHash:              "policy-sha",
	}
}

func TestStore_LoadState_MissingFile(t *testing.T) {
	s := NewStore(t.TempDir())

	got, err := s.LoadState()
	if err != nil {
		t.Fatalf("LoadState() error = %v, want nil for missing file", err)
	}
	if got != nil {
		t.Errorf("LoadState() = %+v, want nil for missing file", got)
	}
}

func TestStore_LoadState_CorruptJSON(t *testing.T) {
	dir := t.TempDir()
	s := NewStore(dir)

	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("failed to seed dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "review-state.json"), []byte("{not valid json"), 0o644); err != nil {
		t.Fatalf("failed to seed corrupt file: %v", err)
	}

	got, err := s.LoadState()
	if got != nil {
		t.Errorf("LoadState() state = %+v, want nil on corrupt input", got)
	}
	if !errors.Is(err, ErrCorruptState) {
		t.Errorf("LoadState() error = %v, want errors.Is(err, ErrCorruptState)", err)
	}
}

func TestStore_SaveState_FirstWrite_RoundTrip(t *testing.T) {
	s := NewStore(t.TempDir())

	state := &ReviewState{
		SchemaVersion: 1,
		Version:       0,
		Candidate:     testCandidate(),
		Tier:          rdd.RiskTierStandard,
		Lenses:        0,
		State:         "pending",
		CreatedAt:     "2026-08-21T00:00:00Z",
		UpdatedAt:     "2026-08-21T00:00:00Z",
	}

	if err := s.SaveState(state); err != nil {
		t.Fatalf("SaveState() error = %v, want nil", err)
	}
	if state.Version != 1 {
		t.Errorf("SaveState() left Version = %d, want 1 after first write", state.Version)
	}

	got, err := s.LoadState()
	if err != nil {
		t.Fatalf("LoadState() error = %v, want nil", err)
	}
	if got == nil {
		t.Fatal("LoadState() = nil, want round-tripped state")
	}
	if got.Version != 1 {
		t.Errorf("LoadState().Version = %d, want 1", got.Version)
	}
	if got.Candidate != state.Candidate {
		t.Errorf("LoadState().Candidate = %+v, want %+v", got.Candidate, state.Candidate)
	}
	if got.State != "pending" {
		t.Errorf("LoadState().State = %q, want %q", got.State, "pending")
	}
}

func TestStore_SaveState_SecondWrite_IncrementsVersion(t *testing.T) {
	s := NewStore(t.TempDir())

	first := &ReviewState{Version: 0, Candidate: testCandidate(), State: "pending"}
	if err := s.SaveState(first); err != nil {
		t.Fatalf("first SaveState() error = %v", err)
	}

	second := &ReviewState{Version: first.Version, Candidate: testCandidate(), State: "approved"}
	if err := s.SaveState(second); err != nil {
		t.Fatalf("second SaveState() error = %v, want nil", err)
	}
	if second.Version != 2 {
		t.Errorf("second SaveState() left Version = %d, want 2", second.Version)
	}

	got, err := s.LoadState()
	if err != nil {
		t.Fatalf("LoadState() error = %v", err)
	}
	if got.State != "approved" {
		t.Errorf("LoadState().State = %q, want %q", got.State, "approved")
	}
}

func TestStore_SaveState_StaleWriteRejected(t *testing.T) {
	s := NewStore(t.TempDir())

	first := &ReviewState{Version: 0, Candidate: testCandidate(), State: "pending"}
	if err := s.SaveState(first); err != nil {
		t.Fatalf("first SaveState() error = %v", err)
	}

	// A second writer commits version 2 first.
	second := &ReviewState{Version: first.Version, Candidate: testCandidate(), State: "approved"}
	if err := s.SaveState(second); err != nil {
		t.Fatalf("second SaveState() error = %v", err)
	}

	// A stale writer still holding version 1 tries to commit.
	stale := &ReviewState{Version: 1, Candidate: testCandidate(), State: "rejected"}
	err := s.SaveState(stale)
	if !errors.Is(err, ErrStaleWrite) {
		t.Fatalf("stale SaveState() error = %v, want errors.Is(err, ErrStaleWrite)", err)
	}

	got, loadErr := s.LoadState()
	if loadErr != nil {
		t.Fatalf("LoadState() error = %v", loadErr)
	}
	if got.State != "approved" {
		t.Errorf("LoadState().State = %q after rejected stale write, want %q (unchanged)", got.State, "approved")
	}
}

func TestStore_SaveState_AtomicWrite_NoTempFileLeftBehind(t *testing.T) {
	dir := t.TempDir()
	s := NewStore(dir)

	state := &ReviewState{Version: 0, Candidate: testCandidate(), State: "pending"}
	if err := s.SaveState(state); err != nil {
		t.Fatalf("SaveState() error = %v", err)
	}

	if _, err := os.Stat(filepath.Join(dir, "review-state.json.tmp")); !os.IsNotExist(err) {
		t.Errorf("expected tmp file to be renamed away, stat error = %v", err)
	}
}

func TestStore_SaveState_CrashSafety_PartialWriteLeavesOriginalIntact(t *testing.T) {
	dir := t.TempDir()
	s := NewStore(dir)

	state := &ReviewState{Version: 0, Candidate: testCandidate(), State: "pending"}
	if err := s.SaveState(state); err != nil {
		t.Fatalf("SaveState() error = %v", err)
	}

	// Simulate a crash mid-write: a .tmp file exists with different content,
	// but the rename into place never happened.
	tmpPath := filepath.Join(dir, "review-state.json.tmp")
	if err := os.WriteFile(tmpPath, []byte(`{"version":99,"state":"corrupted-mid-write"}`), 0o644); err != nil {
		t.Fatalf("failed to seed partial-write tmp file: %v", err)
	}

	got, err := s.LoadState()
	if err != nil {
		t.Fatalf("LoadState() error = %v, want nil", err)
	}
	if got.Version != 1 || got.State != "pending" {
		t.Errorf("LoadState() = %+v, want the original committed state unaffected by an orphaned tmp file", got)
	}
}

func TestStore_SaveState_ConcurrentGoroutines_NoCorruption(t *testing.T) {
	s := NewStore(t.TempDir())

	first := &ReviewState{Version: 0, Candidate: testCandidate(), State: "pending"}
	if err := s.SaveState(first); err != nil {
		t.Fatalf("seed SaveState() error = %v", err)
	}

	const n = 8
	var wg sync.WaitGroup
	results := make([]error, n)
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			// Every goroutine races with the same stale-read version,
			// simulating concurrent read-modify-write attempts.
			state := &ReviewState{Version: first.Version, Candidate: testCandidate(), State: "approved"}
			results[i] = s.SaveState(state)
		}(i)
	}
	wg.Wait()

	successes := 0
	for _, err := range results {
		if err == nil {
			successes++
			continue
		}
		if !errors.Is(err, ErrStaleWrite) {
			t.Errorf("goroutine SaveState() error = %v, want nil or ErrStaleWrite", err)
		}
	}
	if successes != 1 {
		t.Errorf("got %d successful concurrent SaveState() calls from the same stale version, want exactly 1", successes)
	}

	got, err := s.LoadState()
	if err != nil {
		t.Fatalf("LoadState() error = %v", err)
	}
	if got.Version != 2 {
		t.Errorf("LoadState().Version = %d after one accepted concurrent write, want 2", got.Version)
	}
}

func TestStore_LoadReceipt_MissingFile(t *testing.T) {
	s := NewStore(t.TempDir())

	got, err := s.LoadReceipt()
	if err != nil {
		t.Fatalf("LoadReceipt() error = %v, want nil for missing file", err)
	}
	if got != nil {
		t.Errorf("LoadReceipt() = %+v, want nil for missing file", got)
	}
}

func TestStore_WriteReceipt_FirstWrite_RoundTrip(t *testing.T) {
	s := NewStore(t.TempDir())

	receipt := &ReviewReceipt{
		SchemaVersion: 1,
		Candidate:     testCandidate(),
		Tier:          rdd.RiskTierElevated,
		Outcome:       "approved",
		IssuedAt:      "2026-08-21T00:00:00Z",
	}

	if err := s.WriteReceipt(receipt); err != nil {
		t.Fatalf("WriteReceipt() error = %v, want nil", err)
	}
	if receipt.Version != 1 {
		t.Errorf("WriteReceipt() left Version = %d, want 1", receipt.Version)
	}

	got, err := s.LoadReceipt()
	if err != nil {
		t.Fatalf("LoadReceipt() error = %v, want nil", err)
	}
	if got == nil {
		t.Fatal("LoadReceipt() = nil, want round-tripped receipt")
	}
	if got.Version != 1 {
		t.Errorf("LoadReceipt().Version = %d, want 1", got.Version)
	}
	if got.Outcome != "approved" {
		t.Errorf("LoadReceipt().Outcome = %q, want %q", got.Outcome, "approved")
	}
}

func TestStore_WriteReceipt_SecondWrite_Rejected(t *testing.T) {
	s := NewStore(t.TempDir())

	first := &ReviewReceipt{Candidate: testCandidate(), Outcome: "approved", IssuedAt: "2026-08-21T00:00:00Z"}
	if err := s.WriteReceipt(first); err != nil {
		t.Fatalf("first WriteReceipt() error = %v, want nil", err)
	}

	second := &ReviewReceipt{Candidate: testCandidate(), Outcome: "rejected", IssuedAt: "2026-08-21T01:00:00Z"}
	err := s.WriteReceipt(second)
	if !errors.Is(err, ErrReceiptExists) {
		t.Fatalf("second WriteReceipt() error = %v, want errors.Is(err, ErrReceiptExists)", err)
	}

	got, loadErr := s.LoadReceipt()
	if loadErr != nil {
		t.Fatalf("LoadReceipt() error = %v", loadErr)
	}
	if got.Outcome != "approved" {
		t.Errorf("LoadReceipt().Outcome = %q after rejected second write, want %q (unchanged)", got.Outcome, "approved")
	}
}

func TestStore_LoadReceipt_CorruptJSON(t *testing.T) {
	dir := t.TempDir()
	s := NewStore(dir)

	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("failed to seed dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "review-receipt.json"), []byte("{not valid json"), 0o644); err != nil {
		t.Fatalf("failed to seed corrupt file: %v", err)
	}

	got, err := s.LoadReceipt()
	if got != nil {
		t.Errorf("LoadReceipt() receipt = %+v, want nil on corrupt input", got)
	}
	if !errors.Is(err, ErrCorruptState) {
		t.Errorf("LoadReceipt() error = %v, want errors.Is(err, ErrCorruptState)", err)
	}
}

// --- R-2 lifecycle state + additive field tests ---

func TestStore_LoadState_BackfillsEmptyStateToUnreviewed(t *testing.T) {
	dir := t.TempDir()
	s := NewStore(dir)

	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("failed to seed dir: %v", err)
	}
	// Pre-R-2 record: no "state" field at all.
	preR2 := `{"schema_version":1,"version":1,"candidate":{"repository_id":"capiko-ai"},"tier":0,"lenses":0,"created_at":"2026-08-21T00:00:00Z","updated_at":"2026-08-21T00:00:00Z"}`
	if err := os.WriteFile(filepath.Join(dir, "review-state.json"), []byte(preR2), 0o644); err != nil {
		t.Fatalf("failed to seed pre-R-2 record: %v", err)
	}

	got, err := s.LoadState()
	if err != nil {
		t.Fatalf("LoadState() error = %v, want nil", err)
	}
	if got == nil {
		t.Fatal("LoadState() = nil, want backfilled state")
	}
	if got.State != rdd.StateUnreviewed {
		t.Errorf("LoadState().State = %q, want %q (backfilled)", got.State, rdd.StateUnreviewed)
	}
}

func TestStore_LoadState_PreR2Record_ZeroValueDefaultsForNewFields(t *testing.T) {
	dir := t.TempDir()
	s := NewStore(dir)

	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("failed to seed dir: %v", err)
	}
	preR2 := `{"schema_version":1,"version":1,"candidate":{"repository_id":"capiko-ai"},"tier":0,"lenses":0,"state":"approved","created_at":"2026-08-21T00:00:00Z","updated_at":"2026-08-21T00:00:00Z"}`
	if err := os.WriteFile(filepath.Join(dir, "review-state.json"), []byte(preR2), 0o644); err != nil {
		t.Fatalf("failed to seed pre-R-2 record: %v", err)
	}

	got, err := s.LoadState()
	if err != nil {
		t.Fatalf("LoadState() error = %v, want nil", err)
	}
	if got.State != rdd.StateApproved {
		t.Errorf("LoadState().State = %q, want %q (present, no backfill)", got.State, rdd.StateApproved)
	}
	if got.SubjectHash != "" {
		t.Errorf("LoadState().SubjectHash = %q, want zero value for pre-R-2 record", got.SubjectHash)
	}
	if got.SelectedLenses != nil {
		t.Errorf("LoadState().SelectedLenses = %v, want nil for pre-R-2 record", got.SelectedLenses)
	}
	if got.LensResults != nil {
		t.Errorf("LoadState().LensResults = %v, want nil for pre-R-2 record", got.LensResults)
	}
	if got.Consent != nil {
		t.Errorf("LoadState().Consent = %v, want nil for pre-R-2 record", got.Consent)
	}
}

func TestStore_SaveState_NewFields_RoundTrip(t *testing.T) {
	s := NewStore(t.TempDir())

	state := &ReviewState{
		Version:        0,
		Candidate:      testCandidate(),
		State:          rdd.StateReviewing,
		SubjectHash:    "subject-sha",
		SelectedLenses: []rdd.Lens{rdd.LensRisk, rdd.LensReadability},
		LensResults: map[rdd.Lens]LensResult{
			rdd.LensRisk: {
				LensID:      rdd.LensRisk,
				SubjectHash: "subject-sha",
				Findings:    json.RawMessage(`{"issues":[]}`),
				CapturedAt:  "2026-08-21T00:00:00Z",
			},
		},
		Consent: &rdd.ConsentResult{
			Headline: "consent granted",
			Reason:   "user approved via --consent granted",
			Evidence: "cli flag --consent=granted",
			Granted:  true,
		},
	}

	if err := s.SaveState(state); err != nil {
		t.Fatalf("SaveState() error = %v, want nil", err)
	}

	got, err := s.LoadState()
	if err != nil {
		t.Fatalf("LoadState() error = %v, want nil", err)
	}
	if got.State != rdd.StateReviewing {
		t.Errorf("LoadState().State = %q, want %q", got.State, rdd.StateReviewing)
	}
	if got.SubjectHash != "subject-sha" {
		t.Errorf("LoadState().SubjectHash = %q, want %q", got.SubjectHash, "subject-sha")
	}
	if !reflect.DeepEqual(got.SelectedLenses, state.SelectedLenses) {
		t.Errorf("LoadState().SelectedLenses = %v, want %v", got.SelectedLenses, state.SelectedLenses)
	}
	lr, ok := got.LensResults[rdd.LensRisk]
	if !ok {
		t.Fatal("LoadState().LensResults[LensRisk] missing, want round-tripped entry")
	}
	if lr.LensID != rdd.LensRisk || lr.SubjectHash != "subject-sha" || lr.CapturedAt != "2026-08-21T00:00:00Z" {
		t.Errorf("LoadState().LensResults[LensRisk] = %+v, want matching round-trip", lr)
	}
	// Findings is compared semantically, not byte-for-byte: SaveState persists
	// via json.MarshalIndent, which re-indents embedded json.RawMessage —
	// the round-tripped bytes are legitimately reformatted, not corrupted.
	var gotFindings, wantFindings map[string]any
	if err := json.Unmarshal(lr.Findings, &gotFindings); err != nil {
		t.Fatalf("LoadState().LensResults[LensRisk].Findings is not valid JSON: %v", err)
	}
	if err := json.Unmarshal([]byte(`{"issues":[]}`), &wantFindings); err != nil {
		t.Fatalf("failed to parse expected findings: %v", err)
	}
	if !reflect.DeepEqual(gotFindings, wantFindings) {
		t.Errorf("LoadState().LensResults[LensRisk].Findings = %s, want semantically equal to %s", lr.Findings, `{"issues":[]}`)
	}
	if got.Consent == nil {
		t.Fatal("LoadState().Consent = nil, want round-tripped consent")
	}
	if !got.Consent.Granted || got.Consent.Headline != "consent granted" {
		t.Errorf("LoadState().Consent = %+v, want Granted=true and matching Headline", got.Consent)
	}
}

// TestStore_SaveState_CorrectionFields_RoundTrip covers the R-4a additions
// to ReviewState (design "Data Model Changes"): CorrectionConsumed and
// CorrectionAttemptHash round-trip through SaveState/LoadState like the
// other additive R-2 fields.
func TestStore_SaveState_CorrectionFields_RoundTrip(t *testing.T) {
	s := NewStore(t.TempDir())

	state := &ReviewState{
		Version:               0,
		Candidate:             testCandidate(),
		State:                 rdd.StateFixValidating,
		CorrectionConsumed:    true,
		CorrectionAttemptHash: "attempt-sha-1",
	}

	if err := s.SaveState(state); err != nil {
		t.Fatalf("SaveState() error = %v, want nil", err)
	}

	got, err := s.LoadState()
	if err != nil {
		t.Fatalf("LoadState() error = %v, want nil", err)
	}
	if !got.CorrectionConsumed {
		t.Errorf("LoadState().CorrectionConsumed = false, want true")
	}
	if got.CorrectionAttemptHash != "attempt-sha-1" {
		t.Errorf("LoadState().CorrectionAttemptHash = %q, want %q", got.CorrectionAttemptHash, "attempt-sha-1")
	}
}

func TestStore_SaveState_NewFields_OmittedWhenZeroValue(t *testing.T) {
	dir := t.TempDir()
	s := NewStore(dir)

	state := &ReviewState{Version: 0, Candidate: testCandidate(), State: rdd.StateUnreviewed}
	if err := s.SaveState(state); err != nil {
		t.Fatalf("SaveState() error = %v, want nil", err)
	}

	data, err := os.ReadFile(filepath.Join(dir, "review-state.json"))
	if err != nil {
		t.Fatalf("failed to read persisted state: %v", err)
	}
	for _, field := range []string{"subject_hash", "selected_lenses", "lens_results", "consent", "correction_consumed", "correction_attempt_hash"} {
		if strings.Contains(string(data), field) {
			t.Errorf("persisted state contains %q with zero value, want omitempty to drop it", field)
		}
	}
}

// --- Freeze invariant tests (W-1/S-1) ---
//
// Once ReviewState.State has left rdd.StateUnreviewed, Tier, SelectedLenses,
// and SubjectHash are frozen (spec review-lifecycle, design #780 "Freeze
// invariant"). SaveState is the last line of defense for this invariant: the
// R-2 verify report (W-1) flagged that it was previously enforced only at
// the CLI layer (cmd/capiko-ai/review.go), so a direct store consumer could
// violate it. These tests pin store-layer enforcement.

func TestStore_SaveState_FreezeInvariant_TierChangeBlockedAfterUnreviewed(t *testing.T) {
	s := NewStore(t.TempDir())

	first := &ReviewState{
		Version:   0,
		Candidate: testCandidate(),
		State:     rdd.StateReviewing,
		Tier:      rdd.RiskTierStandard,
	}
	if err := s.SaveState(first); err != nil {
		t.Fatalf("seed SaveState() error = %v, want nil", err)
	}

	attempt := &ReviewState{
		Version:   first.Version,
		Candidate: testCandidate(),
		State:     rdd.StateFindingsFrozen,
		Tier:      rdd.RiskTierElevated,
	}
	err := s.SaveState(attempt)
	if !errors.Is(err, ErrFreezeViolation) {
		t.Fatalf("SaveState() error = %v, want errors.Is(err, ErrFreezeViolation)", err)
	}

	got, loadErr := s.LoadState()
	if loadErr != nil {
		t.Fatalf("LoadState() error = %v", loadErr)
	}
	if got.Tier != rdd.RiskTierStandard {
		t.Errorf("LoadState().Tier = %v after rejected write, want %v (unchanged)", got.Tier, rdd.RiskTierStandard)
	}
	if got.Version != first.Version {
		t.Errorf("LoadState().Version = %d after rejected write, want %d (unchanged)", got.Version, first.Version)
	}
}

func TestStore_SaveState_FreezeInvariant_SelectedLensesChangeBlockedAfterUnreviewed(t *testing.T) {
	s := NewStore(t.TempDir())

	first := &ReviewState{
		Version:        0,
		Candidate:      testCandidate(),
		State:          rdd.StateReviewing,
		SelectedLenses: []rdd.Lens{rdd.LensRisk, rdd.LensReadability},
	}
	if err := s.SaveState(first); err != nil {
		t.Fatalf("seed SaveState() error = %v, want nil", err)
	}

	attempt := &ReviewState{
		Version:        first.Version,
		Candidate:      testCandidate(),
		State:          rdd.StateFindingsFrozen,
		SelectedLenses: []rdd.Lens{rdd.LensRisk},
	}
	err := s.SaveState(attempt)
	if !errors.Is(err, ErrFreezeViolation) {
		t.Fatalf("SaveState() error = %v, want errors.Is(err, ErrFreezeViolation)", err)
	}

	got, loadErr := s.LoadState()
	if loadErr != nil {
		t.Fatalf("LoadState() error = %v", loadErr)
	}
	if !reflect.DeepEqual(got.SelectedLenses, first.SelectedLenses) {
		t.Errorf("LoadState().SelectedLenses = %v after rejected write, want %v (unchanged)", got.SelectedLenses, first.SelectedLenses)
	}
}

func TestStore_SaveState_FreezeInvariant_SubjectHashChangeBlockedAfterUnreviewed(t *testing.T) {
	s := NewStore(t.TempDir())

	first := &ReviewState{
		Version:     0,
		Candidate:   testCandidate(),
		State:       rdd.StateReviewing,
		SubjectHash: "subject-sha-1",
	}
	if err := s.SaveState(first); err != nil {
		t.Fatalf("seed SaveState() error = %v, want nil", err)
	}

	attempt := &ReviewState{
		Version:     first.Version,
		Candidate:   testCandidate(),
		State:       rdd.StateFindingsFrozen,
		SubjectHash: "subject-sha-2",
	}
	err := s.SaveState(attempt)
	if !errors.Is(err, ErrFreezeViolation) {
		t.Fatalf("SaveState() error = %v, want errors.Is(err, ErrFreezeViolation)", err)
	}

	got, loadErr := s.LoadState()
	if loadErr != nil {
		t.Fatalf("LoadState() error = %v", loadErr)
	}
	if got.SubjectHash != "subject-sha-1" {
		t.Errorf("LoadState().SubjectHash = %q after rejected write, want %q (unchanged)", got.SubjectHash, "subject-sha-1")
	}
}

func TestStore_SaveState_FreezeInvariant_ChangeAllowedWhileStillUnreviewed(t *testing.T) {
	s := NewStore(t.TempDir())

	first := &ReviewState{
		Version:     0,
		Candidate:   testCandidate(),
		State:       rdd.StateUnreviewed,
		Tier:        rdd.RiskTierStandard,
		SubjectHash: "subject-sha-1",
	}
	if err := s.SaveState(first); err != nil {
		t.Fatalf("seed SaveState() error = %v, want nil", err)
	}

	// Still unreviewed: freeze fields are not yet frozen, any value is
	// allowed (spec review-lifecycle: candidate identity and tier/lenses
	// freeze only on entering "reviewing").
	second := &ReviewState{
		Version:     first.Version,
		Candidate:   testCandidate(),
		State:       rdd.StateUnreviewed,
		Tier:        rdd.RiskTierElevated,
		SubjectHash: "subject-sha-2",
	}
	if err := s.SaveState(second); err != nil {
		t.Fatalf("SaveState() error = %v, want nil while state is still unreviewed", err)
	}

	got, err := s.LoadState()
	if err != nil {
		t.Fatalf("LoadState() error = %v", err)
	}
	if got.Tier != rdd.RiskTierElevated || got.SubjectHash != "subject-sha-2" {
		t.Errorf("LoadState() = %+v, want updated Tier/SubjectHash allowed pre-freeze", got)
	}
}

func TestStore_SaveState_FreezeInvariant_UnchangedFreezeFieldsSucceedAfterUnreviewed(t *testing.T) {
	s := NewStore(t.TempDir())

	first := &ReviewState{
		Version:        0,
		Candidate:      testCandidate(),
		State:          rdd.StateReviewing,
		Tier:           rdd.RiskTierElevated,
		SelectedLenses: []rdd.Lens{rdd.LensRisk, rdd.LensReadability},
		SubjectHash:    "subject-sha-1",
	}
	if err := s.SaveState(first); err != nil {
		t.Fatalf("seed SaveState() error = %v, want nil", err)
	}

	// Advances lifecycle state and touches an unrelated field (LensResults)
	// but leaves all three frozen fields byte-for-byte identical.
	second := &ReviewState{
		Version:        first.Version,
		Candidate:      testCandidate(),
		State:          rdd.StateFindingsFrozen,
		Tier:           rdd.RiskTierElevated,
		SelectedLenses: []rdd.Lens{rdd.LensRisk, rdd.LensReadability},
		SubjectHash:    "subject-sha-1",
		LensResults: map[rdd.Lens]LensResult{
			rdd.LensRisk: {LensID: rdd.LensRisk, SubjectHash: "subject-sha-1"},
		},
	}
	if err := s.SaveState(second); err != nil {
		t.Fatalf("SaveState() error = %v, want nil when frozen fields are unchanged", err)
	}

	got, err := s.LoadState()
	if err != nil {
		t.Fatalf("LoadState() error = %v", err)
	}
	if got.State != rdd.StateFindingsFrozen {
		t.Errorf("LoadState().State = %q, want %q", got.State, rdd.StateFindingsFrozen)
	}
}

// TestStore_SaveState_FreezeInvariant_CorrectionConsumedMonotonic covers the
// R-4a monotonic freeze (spec bounded-correction "single-consumption
// budget"; design "Freeze behavior"): once CorrectionConsumed is true,
// reverting to false is rejected. Unlike Tier/SelectedLenses/SubjectHash,
// this freeze is on the field's own value, not on leaving StateUnreviewed.
func TestStore_SaveState_FreezeInvariant_CorrectionConsumedMonotonic(t *testing.T) {
	s := NewStore(t.TempDir())

	first := &ReviewState{
		Version:            0,
		Candidate:          testCandidate(),
		State:              rdd.StateFixValidating,
		CorrectionConsumed: true,
	}
	if err := s.SaveState(first); err != nil {
		t.Fatalf("seed SaveState() error = %v, want nil", err)
	}

	attempt := &ReviewState{
		Version:            first.Version,
		Candidate:          testCandidate(),
		State:              rdd.StateEscalated,
		CorrectionConsumed: false,
	}
	err := s.SaveState(attempt)
	if !errors.Is(err, ErrFreezeViolation) {
		t.Fatalf("SaveState() error = %v, want errors.Is(err, ErrFreezeViolation)", err)
	}

	got, loadErr := s.LoadState()
	if loadErr != nil {
		t.Fatalf("LoadState() error = %v", loadErr)
	}
	if !got.CorrectionConsumed {
		t.Errorf("LoadState().CorrectionConsumed = false after rejected write, want true (unchanged)")
	}
	if got.Version != first.Version {
		t.Errorf("LoadState().Version = %d after rejected write, want %d (unchanged)", got.Version, first.Version)
	}
}

// TestStore_SaveState_FreezeInvariant_CorrectionConsumedTrueToTrueAllowed
// covers the exact-replay path (design "Exact replay"): re-saving with
// CorrectionConsumed already true and staying true is not a freeze
// violation.
func TestStore_SaveState_FreezeInvariant_CorrectionConsumedTrueToTrueAllowed(t *testing.T) {
	s := NewStore(t.TempDir())

	first := &ReviewState{
		Version:            0,
		Candidate:          testCandidate(),
		State:              rdd.StateFixValidating,
		CorrectionConsumed: true,
	}
	if err := s.SaveState(first); err != nil {
		t.Fatalf("seed SaveState() error = %v, want nil", err)
	}

	second := &ReviewState{
		Version:            first.Version,
		Candidate:          testCandidate(),
		State:              rdd.StateReadyFinalVerification,
		CorrectionConsumed: true,
	}
	if err := s.SaveState(second); err != nil {
		t.Fatalf("SaveState() error = %v, want nil when CorrectionConsumed stays true", err)
	}
}

// TestStore_SaveState_FreezeInvariant_CorrectionConsumedFalseToTrueAllowed
// covers the normal consumption path: false -> true is the expected
// transition when a correction is first consumed, and must not be rejected.
func TestStore_SaveState_FreezeInvariant_CorrectionConsumedFalseToTrueAllowed(t *testing.T) {
	s := NewStore(t.TempDir())

	first := &ReviewState{
		Version:            0,
		Candidate:          testCandidate(),
		State:              rdd.StateFixRequired,
		CorrectionConsumed: false,
	}
	if err := s.SaveState(first); err != nil {
		t.Fatalf("seed SaveState() error = %v, want nil", err)
	}

	second := &ReviewState{
		Version:            first.Version,
		Candidate:          testCandidate(),
		State:              rdd.StateFixValidating,
		CorrectionConsumed: true,
	}
	if err := s.SaveState(second); err != nil {
		t.Fatalf("SaveState() error = %v, want nil when CorrectionConsumed transitions false -> true", err)
	}
}

func TestStore_WriteReceipt_AtomicWrite_NoTempFileLeftBehind(t *testing.T) {
	dir := t.TempDir()
	s := NewStore(dir)

	receipt := &ReviewReceipt{Candidate: testCandidate(), Outcome: "approved", IssuedAt: "2026-08-21T00:00:00Z"}
	if err := s.WriteReceipt(receipt); err != nil {
		t.Fatalf("WriteReceipt() error = %v", err)
	}

	if _, err := os.Stat(filepath.Join(dir, "review-receipt.json.tmp")); !os.IsNotExist(err) {
		t.Errorf("expected tmp file to be renamed away, stat error = %v", err)
	}
}
