package sessionreport

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/martinhg/capiko-ai/internal/sessionlog"
)

func ev(typ, ts, payload string) sessionlog.Event {
	return sessionlog.Event{Type: typ, TS: ts, Payload: json.RawMessage(payload)}
}

func fixture() []sessionlog.Event {
	return []sessionlog.Event{
		ev("prompt", "2026-10-14T01:00:00Z", `{"userPrompt":"hello"}`),
		ev("tool_use", "2026-10-14T01:10:00Z", `{"toolName":"bash"}`),
		ev("tool_use", "2026-10-14T01:20:00Z", `{"toolName":"bash"}`),
		ev("tool_use", "2026-10-14T01:30:00Z", `{"toolName":"edit"}`),
		ev("prompt", "2026-10-14T02:00:00Z", `{"prompt":"hello world!"}`),
		ev("prompt", "not-a-date", `{"prompt":""}`),
	}
}

func TestAnalyze(t *testing.T) {
	r := Analyze(fixture())

	if r.Events != 6 || r.Prompts != 3 || r.ToolCalls != 3 {
		t.Fatalf("counts = %d/%d/%d", r.Events, r.Prompts, r.ToolCalls)
	}
	if r.DurationSeconds != 3600 {
		t.Errorf("duration = %d, want 3600", r.DurationSeconds)
	}
	if r.FirstEvent != "2026-10-14T01:00:00Z" || r.LastEvent != "2026-10-14T02:00:00Z" {
		t.Errorf("first/last = %s / %s", r.FirstEvent, r.LastEvent)
	}
	if len(r.Tools) != 2 || r.Tools[0].Name != "bash" || r.Tools[0].Count != 2 {
		t.Fatalf("tools = %+v", r.Tools)
	}
	if got := r.Tools[0].Percent; got < 66.6 || got > 66.7 {
		t.Errorf("bash percent = %f", got)
	}
	// lengths 5, 12, 0 -> avg 5, longest 12
	if r.AvgPromptLen != 5 || r.LongestPrompt != 12 {
		t.Errorf("avg/longest = %d/%d", r.AvgPromptLen, r.LongestPrompt)
	}
}

func TestAnalyze_Empty(t *testing.T) {
	r := Analyze(nil)
	if r.Events != 0 || r.Prompts != 0 || r.ToolCalls != 0 || r.DurationSeconds != 0 || r.AvgPromptLen != 0 {
		t.Errorf("expected zero report, got %+v", r)
	}
	if r.Tools == nil {
		t.Error("Tools should be non-nil for stable JSON")
	}
	_ = RenderText(r) // must not panic
}

func TestAnalyze_UnknownToolAndBadPayload(t *testing.T) {
	r := Analyze([]sessionlog.Event{
		ev("tool_use", "2026-10-14T01:00:00Z", `{}`),
		ev("tool_use", "2026-10-14T01:00:00Z", `not json`),
	})
	if len(r.Tools) != 1 || r.Tools[0].Name != "unknown" || r.Tools[0].Count != 2 {
		t.Errorf("tools = %+v", r.Tools)
	}
}

func TestAnalyzeAll(t *testing.T) {
	agg := AnalyzeAll(map[string][]sessionlog.Event{
		"a": fixture(),
		"b": {
			ev("prompt", "2026-10-15T01:00:00Z", `{"prompt":"x"}`),
			ev("tool_use", "2026-10-15T01:30:00Z", `{"toolName":"read"}`),
		},
	})

	if agg.Sessions != 2 || agg.TotalPrompts != 4 || agg.TotalToolCalls != 4 {
		t.Fatalf("agg = %+v", agg)
	}
	if agg.AvgPromptsPerSession != 2 || agg.AvgToolCallsPerSession != 2 {
		t.Errorf("avgs = %f / %f", agg.AvgPromptsPerSession, agg.AvgToolCallsPerSession)
	}
	if agg.AvgDurationSeconds != (3600+1800)/2 {
		t.Errorf("avg duration = %d", agg.AvgDurationSeconds)
	}
	if agg.Tools[0].Name != "bash" || agg.Tools[0].Count != 2 {
		t.Errorf("tools = %+v", agg.Tools)
	}
}

func TestAnalyzeAll_Empty(t *testing.T) {
	agg := AnalyzeAll(nil)
	if agg.Sessions != 0 || agg.AvgPromptsPerSession != 0 {
		t.Errorf("agg = %+v", agg)
	}
}

func TestRenderText(t *testing.T) {
	r := Analyze(fixture())
	r.SessionID = "abc-123"
	out := RenderText(r)

	for _, want := range []string{
		"Session Report: abc-123", "Overview", "Tool calls: 3", "Duration:   1h 0m",
		"Tool Usage", "bash", "66.7%", "Prompt Analysis", "Longest:    12 chars",
		"Efficiency Signals", "Behavior Patterns", "Feature Usage", "No data",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q:\n%s", want, out)
		}
	}
}

func TestRenderAggregateText(t *testing.T) {
	out := RenderAggregateText(AnalyzeAll(map[string][]sessionlog.Event{"a": fixture()}))
	for _, want := range []string{"Aggregate Session Report", "Sessions:", "Total prompts:", "Top Tools", "bash"} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q:\n%s", want, out)
		}
	}
}

func TestRenderJSON(t *testing.T) {
	r := Analyze(fixture())
	r.SessionID = "abc"
	s, err := RenderJSON(r)
	if err != nil {
		t.Fatal(err)
	}
	var back Report
	if err := json.Unmarshal([]byte(s), &back); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if back.SessionID != "abc" || back.ToolCalls != 3 {
		t.Errorf("round trip = %+v", back)
	}

	as, err := RenderAggregateJSON(AnalyzeAll(map[string][]sessionlog.Event{"a": fixture()}))
	if err != nil {
		t.Fatal(err)
	}
	var agg AggregateReport
	if err := json.Unmarshal([]byte(as), &agg); err != nil || agg.Sessions != 1 {
		t.Errorf("aggregate JSON: %v %+v", err, agg)
	}
}

func TestFormatDuration(t *testing.T) {
	cases := map[int64]string{0: "0s", 45: "45s", 125: "2m 5s", 4980: "1h 23m"}
	for in, want := range cases {
		if got := FormatDuration(in); got != want {
			t.Errorf("FormatDuration(%d) = %q, want %q", in, got, want)
		}
	}
}
