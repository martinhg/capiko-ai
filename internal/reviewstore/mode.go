// Package reviewstore holds Review Driven Development's persistence and git
// access: CAS-protected authority storage, flock locking, git shell-outs
// behind function-var seams, and kill-switch mode records. It imports
// internal/rdd for pure logic and types; internal/rdd never imports back.
package reviewstore

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/martinhg/capiko-ai/internal/rdd"
)

// ErrCorruptMode is returned by LoadModeRecord when the mode record file at
// path exists but cannot be parsed as JSON. Per the kill-switch fail-closed
// invariant (design "Corruption fails closed"), callers resolving the
// effective mode should treat this the same as an absent record — pass a
// nil *rdd.ModeRecord for this scope into rdd.ResolveMode, which fails
// closed to rdd.ModeManaged rather than fabricating a default.
var ErrCorruptMode = errors.New("reviewstore: mode record is corrupt")

// LoadModeRecord reads the kill-switch mode record at path. A missing file
// returns (nil, nil) — normal first-use state, resolved as absent by
// rdd.ResolveMode. A file that exists but fails to parse as JSON returns
// (nil, error) wrapping ErrCorruptMode via errors.Is.
func LoadModeRecord(path string) (*rdd.ModeRecord, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var rec rdd.ModeRecord
	if err := json.Unmarshal(data, &rec); err != nil {
		return nil, fmt.Errorf("%w: %s: %v", ErrCorruptMode, path, err)
	}
	return &rec, nil
}

// SaveModeRecord writes rec to path atomically: a temp file in the same
// directory is written first, then renamed into place, so a crash mid-write
// can never leave a corrupt or partially-written mode record (design
// "Atomic writes"). SaveModeRecord only ever touches path — toggling the
// kill switch never fabricates or destroys any other file, including
// receipts (spec rdd-kill-switch "No fabrication, no destruction").
func SaveModeRecord(path string, rec *rdd.ModeRecord) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}

	data, err := json.MarshalIndent(rec, "", "  ")
	if err != nil {
		return err
	}

	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}
