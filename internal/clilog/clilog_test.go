package clilog

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
	"time"
)

// fixedClock returns monotonically increasing timestamps so duration assertions
// are deterministic.
func fixedClock(times ...time.Time) func() time.Time {
	i := 0
	return func() time.Time {
		t := times[i]
		if i < len(times)-1 {
			i++
		}
		return t
	}
}

func TestDisabledLoggerWritesNothing(t *testing.T) {
	// A nil sink means --verbose was off: the logger must be a no-op.
	l := New(nil, "doctor")
	if l.Enabled() {
		t.Fatal("logger with a nil sink should be disabled")
	}
	l.Event("anything", "ok")
	l.Step("step")("done") // must not panic
}

func TestEventEmitsJSONLine(t *testing.T) {
	var buf bytes.Buffer
	l := New(&buf, "doctor")
	l.now = fixedClock(time.Date(2026, 6, 21, 10, 0, 0, 0, time.UTC))

	l.Event("engram-check", "pass")

	line := strings.TrimSpace(buf.String())
	var e Entry
	if err := json.Unmarshal([]byte(line), &e); err != nil {
		t.Fatalf("output is not valid JSON: %v\n%s", err, line)
	}
	if e.Command != "doctor" || e.Event != "engram-check" || e.Result != "pass" {
		t.Errorf("entry = %+v, want command=doctor event=engram-check result=pass", e)
	}
	if e.Time == "" {
		t.Error("entry should carry a timestamp")
	}
}

func TestStepRecordsDuration(t *testing.T) {
	var buf bytes.Buffer
	l := New(&buf, "install")
	start := time.Date(2026, 6, 21, 10, 0, 0, 0, time.UTC)
	l.now = fixedClock(start, start.Add(250*time.Millisecond))

	done := l.Step("write-skills")
	done("ok")

	var e Entry
	if err := json.Unmarshal([]byte(strings.TrimSpace(buf.String())), &e); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if e.Event != "write-skills" || e.Result != "ok" {
		t.Errorf("entry = %+v, want event=write-skills result=ok", e)
	}
	if e.DurationMs != 250 {
		t.Errorf("duration = %d ms, want 250", e.DurationMs)
	}
}

func TestEachEventIsOneLine(t *testing.T) {
	var buf bytes.Buffer
	l := New(&buf, "sync")
	l.Event("a", "ok")
	l.Event("b", "ok")
	if got := strings.Count(strings.TrimSpace(buf.String()), "\n"); got != 1 {
		t.Errorf("two events should be two JSON lines (one newline between), got %d newlines", got)
	}
}
