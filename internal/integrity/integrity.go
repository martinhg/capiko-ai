// Package integrity computes and compares SHA-256 checksums for a declared
// set of protected-surface files (the "manifest"). capiko records the
// manifest in state.json; drift detection re-hashes the files and compares
// against the recorded values to catch changes made outside an authorized
// flow.
package integrity

import (
	"os"
	"sort"
	"strings"

	"github.com/martinhg/capiko-ai/internal/state"
)

// Manifest computes per-file SHA-256 checksums and a combined checksum for the
// given file paths. Missing files get an empty checksum entry. The combined
// checksum is deterministic: files are sorted by path, then concatenated as
// path+"\n"+content before hashing. This is the SINGLE source of both the
// stored IntegrityRecord.Checksum and the drift-comparison value.
func Manifest(files []string) (checksums map[string]string, combined string) {
	sorted := make([]string, len(files))
	copy(sorted, files)
	sort.Strings(sorted)

	checksums = make(map[string]string, len(sorted))
	var buf strings.Builder
	for _, f := range sorted {
		data, err := os.ReadFile(f)
		if err != nil {
			checksums[f] = ""
			continue
		}
		checksums[f] = state.Checksum(string(data))
		buf.WriteString(f)
		buf.WriteByte('\n')
		buf.Write(data)
	}
	combined = state.Checksum(buf.String())
	return checksums, combined
}

// CombinedChecksum returns only the combined checksum for a set of files.
func CombinedChecksum(files []string) string {
	_, combined := Manifest(files)
	return combined
}

// Drifted returns file paths from the record whose current on-disk checksum
// no longer matches the recorded value. An absent file counts as drifted if
// it had a non-empty recorded checksum (file was deleted). A newly-appearing
// file is drifted if the recorded checksum was empty. Returns nil when
// nothing changed.
func Drifted(rec *state.IntegrityRecord) []string {
	if rec == nil || !rec.Enabled || len(rec.Files) == 0 {
		return nil
	}
	current, _ := Manifest(rec.Files)
	var out []string
	sorted := make([]string, len(rec.Files))
	copy(sorted, rec.Files)
	sort.Strings(sorted)
	for _, f := range sorted {
		if current[f] != rec.Checksums[f] {
			out = append(out, f)
		}
	}
	return out
}
