package reviewstore

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/martinhg/capiko-ai/internal/rdd"
)

// ErrCorruptState is returned by LoadState/LoadReceipt when the
// corresponding file exists but cannot be parsed as JSON. Per the
// authority store's fail-closed invariant (design "Corruption fails
// closed"; spec rdd-authority-store "Corruption fails closed"), callers
// MUST treat this as blocking, never as a default-allow state.
var ErrCorruptState = errors.New("reviewstore: authority state is corrupt")

// ErrStaleWrite is returned by SaveState when the caller's ReviewState.Version
// does not match the version currently on disk — another writer has already
// committed a newer version and the caller's read is stale (spec
// rdd-authority-store "Stale write rejected").
var ErrStaleWrite = errors.New("reviewstore: stale write rejected")

// ErrReceiptExists is returned by WriteReceipt when a receipt has already
// been written for this store. Terminal receipts are immutable and
// write-once (spec rdd-authority-store "Receipt write-once").
var ErrReceiptExists = errors.New("reviewstore: receipt already exists")

// ReviewState is the mutable authority record for an in-progress review. It
// is CAS-protected: every write must present the Version it read, and
// SaveState increments Version on success (design "CAS atomicity").
type ReviewState struct {
	SchemaVersion int                   `json:"schema_version"`
	Version       int                   `json:"version"`
	Candidate     rdd.CandidateIdentity `json:"candidate"`
	Tier          rdd.RiskTier          `json:"tier"`
	Lenses        int                   `json:"lenses"`
	State         rdd.LifecycleState    `json:"state"`

	// SubjectHash, SelectedLenses, LensResults, and Consent are additive R-2
	// fields (spec review-lifecycle, lens-mandates, review-consent). They are
	// omitempty so pre-R-2 records serialize and deserialize unchanged, and
	// SchemaVersion stays 1 (design #780 "Migration": additive fields only).
	SubjectHash    string                  `json:"subject_hash,omitempty"`
	SelectedLenses []rdd.Lens              `json:"selected_lenses,omitempty"`
	LensResults    map[rdd.Lens]LensResult `json:"lens_results,omitempty"`
	Consent        *rdd.ConsentResult      `json:"consent,omitempty"`

	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
}

// LensResult is one lens's captured findings, bound to the subject hash that
// was frozen when the review started (spec review-cli-commands:
// "capture-result ... reject unmandated lens IDs; reject duplicate
// submission per lens"). Findings is opaque JSON — the reviewstore package
// persists it without interpreting its shape; internal/rdd/cmd own the
// findings schema.
type LensResult struct {
	LensID      rdd.Lens        `json:"lens_id"`
	SubjectHash string          `json:"subject_hash"`
	Findings    json.RawMessage `json:"findings"`
	CapturedAt  string          `json:"captured_at"`
}

// ReviewReceipt is the immutable terminal record issued once a review
// reaches a final outcome. It carries a Version field from its first write
// (always 1, since a receipt is written exactly once) and MUST NOT be
// mutated afterward (spec rdd-authority-store "Immutable receipt with
// schema version").
type ReviewReceipt struct {
	SchemaVersion int                   `json:"schema_version"`
	Version       int                   `json:"version"`
	Candidate     rdd.CandidateIdentity `json:"candidate"`
	Tier          rdd.RiskTier          `json:"tier"`
	Outcome       string                `json:"outcome"`
	IssuedAt      string                `json:"issued_at"`
}

// Store is the authority store for a single RDD scope: it persists
// review-state.json (mutable, CAS-protected) and review-receipt.json
// (immutable, write-once) under dir, which callers resolve to
// <git-common-dir>/capiko/rdd/ so all worktrees of a repository share one
// authority (design "Storage Layout"; spec rdd-authority-store
// "Worktree-shared storage").
type Store struct {
	dir string
}

// NewStore returns a Store rooted at dir. dir is created lazily on first
// write; NewStore itself performs no I/O.
func NewStore(dir string) *Store {
	return &Store{dir: dir}
}

func (s *Store) statePath() string {
	return filepath.Join(s.dir, "review-state.json")
}

func (s *Store) receiptPath() string {
	return filepath.Join(s.dir, "review-receipt.json")
}

func (s *Store) lockPath() string {
	return filepath.Join(s.dir, ".lock")
}

// LoadState reads review-state.json. A missing file returns (nil, nil) —
// normal first-use state, callers treat this as version 0. A file that
// exists but fails to parse as JSON returns (nil, error) wrapping
// ErrCorruptState via errors.Is.
func (s *Store) LoadState() (*ReviewState, error) {
	data, err := os.ReadFile(s.statePath())
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var st ReviewState
	if err := json.Unmarshal(data, &st); err != nil {
		return nil, fmt.Errorf("%w: %s: %v", ErrCorruptState, s.statePath(), err)
	}
	// Pre-R-2 records have no "state" field. Backfill to the initial
	// lifecycle state rather than leaving the zero value (empty string),
	// which is not itself a valid rdd.LifecycleState (design #780 "State
	// Field Type": "empty backfills as unreviewed").
	if st.State == "" {
		st.State = rdd.StateUnreviewed
	}
	return &st, nil
}

// SaveState commits state as the new authority record, enforcing
// single-writer CAS semantics (design "Locking Mechanism"; spec
// rdd-authority-store "CAS atomicity", "Single-writer locking"):
//
//  1. Acquire an exclusive flock on the store's lock file, blocking until
//     available — this serializes concurrent read-modify-write attempts.
//  2. Read the current on-disk version (0 if no state file exists yet).
//  3. Compare state.Version against the current version. If they differ,
//     the caller's read was stale: return ErrStaleWrite without writing.
//  4. Increment state.Version to the new committed version.
//  5. Write the new state to a temp file and atomically rename it into
//     place, so a crash mid-write can never corrupt the prior state.
//  6. Release the flock.
//
// On success, state.Version is mutated in place to reflect the newly
// committed version.
func (s *Store) SaveState(state *ReviewState) error {
	if err := os.MkdirAll(s.dir, 0o755); err != nil {
		return err
	}

	lockFile, err := flockAcquire(s.lockPath())
	if err != nil {
		return err
	}
	defer func() { _ = flockRelease(lockFile) }()

	current, err := s.LoadState()
	if err != nil {
		return err
	}

	currentVersion := 0
	if current != nil {
		currentVersion = current.Version
	}

	if state.Version != currentVersion {
		return ErrStaleWrite
	}

	state.Version = currentVersion + 1

	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}

	tmp := s.statePath() + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, s.statePath())
}

// LoadReceipt reads review-receipt.json. A missing file returns (nil, nil)
// — no terminal decision has been issued yet. A file that exists but fails
// to parse as JSON returns (nil, error) wrapping ErrCorruptState via
// errors.Is.
func (s *Store) LoadReceipt() (*ReviewReceipt, error) {
	data, err := os.ReadFile(s.receiptPath())
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var r ReviewReceipt
	if err := json.Unmarshal(data, &r); err != nil {
		return nil, fmt.Errorf("%w: %s: %v", ErrCorruptState, s.receiptPath(), err)
	}
	return &r, nil
}

// WriteReceipt writes receipt as the store's terminal record, write-once:
// if a receipt already exists, it returns ErrReceiptExists and leaves the
// existing receipt untouched (spec rdd-authority-store "Receipt
// write-once"). receipt.Version is set to 1, since a receipt has exactly
// one write in its lifetime. Guarded by the same flock as SaveState so a
// concurrent WriteReceipt/WriteReceipt race cannot produce two receipts.
func (s *Store) WriteReceipt(receipt *ReviewReceipt) error {
	if err := os.MkdirAll(s.dir, 0o755); err != nil {
		return err
	}

	lockFile, err := flockAcquire(s.lockPath())
	if err != nil {
		return err
	}
	defer func() { _ = flockRelease(lockFile) }()

	if _, err := os.Stat(s.receiptPath()); err == nil {
		return ErrReceiptExists
	} else if !os.IsNotExist(err) {
		return err
	}

	receipt.Version = 1

	data, err := json.MarshalIndent(receipt, "", "  ")
	if err != nil {
		return err
	}

	tmp := s.receiptPath() + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, s.receiptPath())
}
