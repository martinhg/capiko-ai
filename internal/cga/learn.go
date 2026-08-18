package cga

import "strings"

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
