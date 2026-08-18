package cga

import (
	"bufio"
	"bytes"
	"encoding/json"
	"io"
)

// FindingsLogName is the filename of the append-only CGA findings log,
// stored under the repository's git directory (e.g. ".git/capiko/<FindingsLogName>").
const FindingsLogName = "cga-findings.jsonl"

// LogEntry is a single persisted CGA review result, one per JSONL line.
type LogEntry struct {
	Timestamp string    `json:"timestamp"`
	ParentSHA string    `json:"parent_sha"`
	CommitSHA string    `json:"commit_sha"`
	Verdict   string    `json:"verdict"`
	Reason    string    `json:"reason,omitempty"`
	Findings  []Finding `json:"findings,omitempty"`
}

// ParseLog reads JSONL from r, returning one LogEntry per well-formed line
// in file order. Malformed lines (invalid JSON) and blank lines are silently
// skipped — they never cause ParseLog to return an error. An error is
// returned only when the underlying reader fails.
func ParseLog(r io.Reader) ([]LogEntry, error) {
	var entries []LogEntry

	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		line := bytes.TrimSpace(scanner.Bytes())
		if len(line) == 0 {
			continue
		}

		var entry LogEntry
		if err := json.Unmarshal(line, &entry); err != nil {
			continue
		}
		entries = append(entries, entry)
	}

	if err := scanner.Err(); err != nil {
		return entries, err
	}
	return entries, nil
}
