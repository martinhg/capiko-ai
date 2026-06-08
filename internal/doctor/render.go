package doctor

import (
	"encoding/json"
	"fmt"
	"strings"
)

// marker is the leading glyph for each status in the text report.
func marker(s Status) string {
	switch s {
	case Pass:
		return "✓"
	case Warn:
		return "!"
	case Fail:
		return "✗"
	default:
		return "?"
	}
}

// RenderText formats the report for a terminal: one line per check with its
// status marker and detail, an indented remedy arrow for anything not passing,
// and a closing tally.
func RenderText(r Report) string {
	var b strings.Builder
	b.WriteString("capiko-ai doctor\n\n")
	for _, c := range r.Checks {
		fmt.Fprintf(&b, "  %s %-18s %s\n", marker(c.Status), c.Name, c.Detail)
		if c.Status != Pass && c.Remedy != "" {
			fmt.Fprintf(&b, "      → %s\n", c.Remedy)
		}
	}
	pass, warn, fail := r.Counts()
	fmt.Fprintf(&b, "\n%d pass · %d warn · %d fail\n", pass, warn, fail)
	return b.String()
}

// RenderJSON returns the report as indented JSON (statuses as strings).
func RenderJSON(r Report) (string, error) {
	b, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return "", err
	}
	return string(b) + "\n", nil
}
