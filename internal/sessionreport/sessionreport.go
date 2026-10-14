// Package sessionreport turns captured session events into metrics and renders
// them as text or JSON. It is pure analysis: reading session files belongs to
// internal/sessionlog.
package sessionreport

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/martinhg/capiko-ai/internal/sessionlog"
)

const noDataMsg = "No data — requires additional hooks (H-3 through H-8)"

// ToolCount is the call count for one tool.
type ToolCount struct {
	Name    string  `json:"name"`
	Count   int     `json:"count"`
	Percent float64 `json:"percent"`
}

// Report is the analysis of a single session.
type Report struct {
	SessionID       string      `json:"session_id,omitempty"`
	Events          int         `json:"events"`
	Prompts         int         `json:"prompts"`
	ToolCalls       int         `json:"tool_calls"`
	DurationSeconds int64       `json:"duration_seconds"`
	FirstEvent      string      `json:"first_event,omitempty"`
	LastEvent       string      `json:"last_event,omitempty"`
	Tools           []ToolCount `json:"tools"`
	AvgPromptLen    int         `json:"avg_prompt_length"`
	LongestPrompt   int         `json:"longest_prompt_length"`
}

// AggregateReport summarizes several sessions.
type AggregateReport struct {
	Sessions               int         `json:"sessions"`
	TotalPrompts           int         `json:"total_prompts"`
	TotalToolCalls         int         `json:"total_tool_calls"`
	AvgPromptsPerSession   float64     `json:"avg_prompts_per_session"`
	AvgToolCallsPerSession float64     `json:"avg_tool_calls_per_session"`
	AvgDurationSeconds     int64       `json:"avg_duration_seconds"`
	Tools                  []ToolCount `json:"tools"`
}

// Analyze computes the report for one session's events.
func Analyze(events []sessionlog.Event) Report {
	r := Report{Events: len(events), Tools: []ToolCount{}}
	tools := map[string]int{}
	var first, last time.Time
	var firstRaw, lastRaw string
	totalLen := 0

	for _, ev := range events {
		if t, err := time.Parse(time.RFC3339, ev.TS); err == nil {
			if first.IsZero() || t.Before(first) {
				first, firstRaw = t, ev.TS
			}
			if last.IsZero() || t.After(last) {
				last, lastRaw = t, ev.TS
			}
		}
		switch ev.Type {
		case sessionlog.EventPrompt:
			r.Prompts++
			n := utf8.RuneCountInString(promptText(ev.Payload))
			totalLen += n
			if n > r.LongestPrompt {
				r.LongestPrompt = n
			}
		case sessionlog.EventToolUse:
			r.ToolCalls++
			name := toolName(ev.Payload)
			if name == "" {
				name = "unknown"
			}
			tools[name]++
		}
	}

	if r.Prompts > 0 {
		r.AvgPromptLen = totalLen / r.Prompts
	}
	if !first.IsZero() {
		r.FirstEvent, r.LastEvent = firstRaw, lastRaw
		r.DurationSeconds = int64(last.Sub(first).Seconds())
	}
	r.Tools = rankTools(tools, r.ToolCalls)
	return r
}

// AnalyzeAll aggregates metrics across sessions keyed by session ID.
func AnalyzeAll(sessions map[string][]sessionlog.Event) AggregateReport {
	agg := AggregateReport{Sessions: len(sessions), Tools: []ToolCount{}}
	tools := map[string]int{}
	var totalDur int64

	for _, events := range sessions {
		r := Analyze(events)
		agg.TotalPrompts += r.Prompts
		agg.TotalToolCalls += r.ToolCalls
		totalDur += r.DurationSeconds
		for _, t := range r.Tools {
			tools[t.Name] += t.Count
		}
	}

	if agg.Sessions > 0 {
		n := float64(agg.Sessions)
		agg.AvgPromptsPerSession = float64(agg.TotalPrompts) / n
		agg.AvgToolCallsPerSession = float64(agg.TotalToolCalls) / n
		agg.AvgDurationSeconds = totalDur / int64(agg.Sessions)
	}
	agg.Tools = rankTools(tools, agg.TotalToolCalls)
	return agg
}

func rankTools(counts map[string]int, total int) []ToolCount {
	out := make([]ToolCount, 0, len(counts))
	for name, c := range counts {
		pct := 0.0
		if total > 0 {
			pct = float64(c) * 100 / float64(total)
		}
		out = append(out, ToolCount{Name: name, Count: c, Percent: pct})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Count != out[j].Count {
			return out[i].Count > out[j].Count
		}
		return out[i].Name < out[j].Name
	})
	return out
}

func stringField(payload json.RawMessage, keys ...string) string {
	var m map[string]json.RawMessage
	if json.Unmarshal(payload, &m) != nil {
		return ""
	}
	for _, k := range keys {
		var s string
		if raw, ok := m[k]; ok && json.Unmarshal(raw, &s) == nil && s != "" {
			return s
		}
	}
	return ""
}

func promptText(payload json.RawMessage) string {
	return stringField(payload, "userPrompt", "prompt")
}

func toolName(payload json.RawMessage) string {
	return stringField(payload, "toolName")
}

// FormatDuration renders seconds as a compact "1h 23m" style string.
func FormatDuration(seconds int64) string {
	d := time.Duration(seconds) * time.Second
	switch {
	case d >= time.Hour:
		return fmt.Sprintf("%dh %dm", int(d.Hours()), int(d.Minutes())%60)
	case d >= time.Minute:
		return fmt.Sprintf("%dm %ds", int(d.Minutes()), int(d.Seconds())%60)
	default:
		return fmt.Sprintf("%ds", int(d.Seconds()))
	}
}

func writeTools(b *strings.Builder, tools []ToolCount) {
	if len(tools) == 0 {
		b.WriteString("  No tool calls\n")
		return
	}
	for _, t := range tools {
		fmt.Fprintf(b, "  %-12s %4d (%.1f%%)\n", t.Name, t.Count, t.Percent)
	}
}

func title(b *strings.Builder, s string) {
	b.WriteString(s + "\n")
	b.WriteString(strings.Repeat("═", utf8.RuneCountInString(s)) + "\n\n")
}

// RenderText renders a single-session report as plain text.
func RenderText(r Report) string {
	var b strings.Builder
	title(&b, "Session Report: "+r.SessionID)

	b.WriteString("Overview\n")
	fmt.Fprintf(&b, "  Events:     %d\n", r.Events)
	fmt.Fprintf(&b, "  Prompts:    %d\n", r.Prompts)
	fmt.Fprintf(&b, "  Tool calls: %d\n", r.ToolCalls)
	fmt.Fprintf(&b, "  Duration:   %s\n", FormatDuration(r.DurationSeconds))
	if r.FirstEvent != "" {
		fmt.Fprintf(&b, "  First:      %s\n", r.FirstEvent)
		fmt.Fprintf(&b, "  Last:       %s\n", r.LastEvent)
	}

	b.WriteString("\nTool Usage\n")
	writeTools(&b, r.Tools)

	b.WriteString("\nPrompt Analysis\n")
	fmt.Fprintf(&b, "  Count:      %d\n", r.Prompts)
	fmt.Fprintf(&b, "  Avg length: %d chars\n", r.AvgPromptLen)
	fmt.Fprintf(&b, "  Longest:    %d chars\n", r.LongestPrompt)

	for _, sec := range []string{"Efficiency Signals", "Behavior Patterns", "Feature Usage"} {
		fmt.Fprintf(&b, "\n%s\n  %s\n", sec, noDataMsg)
	}
	return b.String()
}

// RenderJSON renders a single-session report as indented JSON.
func RenderJSON(r Report) (string, error) {
	return marshal(r)
}

// RenderAggregateText renders an aggregate report as plain text.
func RenderAggregateText(a AggregateReport) string {
	var b strings.Builder
	title(&b, "Aggregate Session Report")

	b.WriteString("Overview\n")
	fmt.Fprintf(&b, "  Sessions:             %d\n", a.Sessions)
	fmt.Fprintf(&b, "  Total prompts:        %d\n", a.TotalPrompts)
	fmt.Fprintf(&b, "  Total tool calls:     %d\n", a.TotalToolCalls)
	fmt.Fprintf(&b, "  Avg prompts/session:  %.1f\n", a.AvgPromptsPerSession)
	fmt.Fprintf(&b, "  Avg tools/session:    %.1f\n", a.AvgToolCallsPerSession)
	fmt.Fprintf(&b, "  Avg duration:         %s\n", FormatDuration(a.AvgDurationSeconds))

	b.WriteString("\nTop Tools\n")
	writeTools(&b, a.Tools)
	return b.String()
}

// RenderAggregateJSON renders an aggregate report as indented JSON.
func RenderAggregateJSON(a AggregateReport) (string, error) {
	return marshal(a)
}

func marshal(v any) (string, error) {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return "", fmt.Errorf("sessionreport: marshal: %w", err)
	}
	return string(data) + "\n", nil
}
