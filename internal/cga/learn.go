package cga

import (
	"sort"
	"strings"
)

// trailingPunctuation is the set of trailing characters NormalizeDescription
// strips once whitespace has been collapsed.
const trailingPunctuation = ".,;:!?"

// NormalizeDescription reduces a finding description to a comparable form:
// lowercase, internal whitespace collapsed to single spaces, leading/
// trailing whitespace removed, and trailing punctuation stripped. It is a
// pure function: no I/O.
func NormalizeDescription(d string) string {
	normalized := strings.Join(strings.Fields(strings.ToLower(d)), " ")
	return strings.TrimRight(normalized, trailingPunctuation)
}

// Pattern is a cluster of recurring findings sharing severity and a
// normalized description, as produced by DetectPatterns.
type Pattern struct {
	Severity      Severity
	Description   string   // normalized representative description
	EvidenceCount int      // distinct log entries containing this pattern
	Files         []string // deduplicated file paths, in first-seen order
	FirstSeen     string   // earliest timestamp from evidence
	LastSeen      string   // latest timestamp from evidence
}

// patternKey groups findings by severity + normalized description.
type patternKey struct {
	severity    Severity
	description string
}

// patternEvidence accumulates evidence for a single patternKey while
// DetectPatterns walks the log in order.
type patternEvidence struct {
	entrySeen map[int]struct{}
	fileSeen  map[string]struct{}
	files     []string
	firstSeen string
	lastSeen  string
}

// DetectPatterns groups findings across entries by severity + normalized
// description, returning patterns whose evidence count reaches threshold,
// sorted by evidence count descending (ties broken by most recent, then by
// description for full determinism). Evidence count is the number of
// distinct log entries containing at least one matching finding — repeated
// findings within a single entry count once. It is a pure function: no I/O.
func DetectPatterns(entries []LogEntry, threshold int) []Pattern {
	evidence := make(map[patternKey]*patternEvidence)
	var order []patternKey

	for i, entry := range entries {
		for _, finding := range entry.Findings {
			desc := NormalizeDescription(finding.Description)
			if desc == "" {
				continue
			}
			key := patternKey{severity: finding.Severity, description: desc}

			ev, ok := evidence[key]
			if !ok {
				ev = &patternEvidence{
					entrySeen: make(map[int]struct{}),
					fileSeen:  make(map[string]struct{}),
				}
				evidence[key] = ev
				order = append(order, key)
			}

			ev.entrySeen[i] = struct{}{}
			if finding.File != "" {
				if _, seen := ev.fileSeen[finding.File]; !seen {
					ev.fileSeen[finding.File] = struct{}{}
					ev.files = append(ev.files, finding.File)
				}
			}
			if ev.firstSeen == "" || entry.Timestamp < ev.firstSeen {
				ev.firstSeen = entry.Timestamp
			}
			if entry.Timestamp > ev.lastSeen {
				ev.lastSeen = entry.Timestamp
			}
		}
	}

	var patterns []Pattern
	for _, key := range order {
		ev := evidence[key]
		count := len(ev.entrySeen)
		if count < threshold {
			continue
		}
		patterns = append(patterns, Pattern{
			Severity:      key.severity,
			Description:   key.description,
			EvidenceCount: count,
			Files:         ev.files,
			FirstSeen:     ev.firstSeen,
			LastSeen:      ev.lastSeen,
		})
	}

	sort.Slice(patterns, func(i, j int) bool {
		if patterns[i].EvidenceCount != patterns[j].EvidenceCount {
			return patterns[i].EvidenceCount > patterns[j].EvidenceCount
		}
		if patterns[i].LastSeen != patterns[j].LastSeen {
			return patterns[i].LastSeen > patterns[j].LastSeen
		}
		return patterns[i].Description < patterns[j].Description
	})

	return patterns
}
