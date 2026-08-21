package reviewstore

import (
	"errors"
	"os"
	"path/filepath"
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
