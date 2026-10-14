// Package sessionlog reads the per-session JSONL event files written by the
// capiko prompt-capture and tool-monitor hooks. It only reads and parses;
// filtering and aggregation belong to callers.
package sessionlog

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Event types written by the capture hooks.
const (
	EventPrompt  = "prompt"
	EventToolUse = "tool_use"
)

const fileExt = ".jsonl"

// maxLineBytes bounds a single JSONL line; tool results can be large.
const maxLineBytes = 8 << 20

// Event is one captured hook event.
type Event struct {
	Type    string          `json:"event"`
	TS      string          `json:"ts"`
	Payload json.RawMessage `json:"payload"`
}

// Dir returns the sessions directory under the given home directory.
func Dir(home string) string {
	return filepath.Join(home, ".capiko", "sessions")
}

// ListSessions returns the session IDs found in dir, sorted. A missing
// directory yields an empty list, not an error.
func ListSessions(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, fmt.Errorf("sessionlog: list sessions: %w", err)
	}
	var ids []string
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), fileExt) {
			continue
		}
		ids = append(ids, strings.TrimSuffix(e.Name(), fileExt))
	}
	sort.Strings(ids)
	return ids, nil
}

// ReadSession parses dir/<sessionID>.jsonl. Malformed lines are skipped.
func ReadSession(dir, sessionID string) ([]Event, error) {
	f, err := os.Open(filepath.Join(dir, sessionID+fileExt))
	if err != nil {
		return nil, fmt.Errorf("sessionlog: read session %q: %w", sessionID, err)
	}
	defer f.Close()

	events := []Event{}
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64*1024), maxLineBytes)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		var ev Event
		if err := json.Unmarshal([]byte(line), &ev); err != nil {
			continue
		}
		events = append(events, ev)
	}
	if err := sc.Err(); err != nil {
		return nil, fmt.Errorf("sessionlog: scan session %q: %w", sessionID, err)
	}
	return events, nil
}

// ReadAll reads every session in dir, keyed by session ID.
func ReadAll(dir string) (map[string][]Event, error) {
	ids, err := ListSessions(dir)
	if err != nil {
		return nil, err
	}
	all := make(map[string][]Event, len(ids))
	for _, id := range ids {
		events, err := ReadSession(dir, id)
		if err != nil {
			return nil, err
		}
		all[id] = events
	}
	return all, nil
}
