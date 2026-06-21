// Package clilog emits structured JSON-lines diagnostics for CLI subcommands,
// gated by --verbose. Lines go to a sink (stderr in production) so stdout stays
// clean for scripting. A nil sink makes the logger a no-op, so callers can build
// one unconditionally and let the flag decide whether anything is written.
package clilog

import (
	"encoding/json"
	"fmt"
	"io"
	"time"
)

// Entry is one structured log line: when it happened, which command emitted it,
// the event/check name, its result, and (for timed steps) how long it took.
type Entry struct {
	Time       string `json:"time"`
	Command    string `json:"command"`
	Event      string `json:"event"`
	Result     string `json:"result,omitempty"`
	DurationMs int64  `json:"duration_ms,omitempty"`
	Detail     string `json:"detail,omitempty"`
}

// Logger writes JSON-lines entries to its sink. A nil sink disables it.
type Logger struct {
	w   io.Writer
	cmd string
	now func() time.Time
}

// New returns a logger that writes to w, tagging every entry with command. A nil
// w yields a disabled (no-op) logger.
func New(w io.Writer, command string) *Logger {
	return &Logger{w: w, cmd: command, now: time.Now}
}

// Enabled reports whether the logger writes anything.
func (l *Logger) Enabled() bool { return l != nil && l.w != nil }

// Event logs a point-in-time event with a result (no duration).
func (l *Logger) Event(event, result string) {
	l.write(Entry{Event: event, Result: result})
}

// Detailf logs an event with a formatted detail string and no result.
func (l *Logger) Detailf(event, format string, args ...any) {
	l.write(Entry{Event: event, Detail: fmt.Sprintf(format, args...)})
}

// Step starts timing an operation. Call the returned function with the result
// when the operation completes to emit a single entry carrying its duration.
func (l *Logger) Step(event string) func(result string) {
	if !l.Enabled() {
		return func(string) {}
	}
	start := l.now()
	return func(result string) {
		l.write(Entry{Event: event, Result: result, DurationMs: l.now().Sub(start).Milliseconds()})
	}
}

func (l *Logger) write(e Entry) {
	if !l.Enabled() {
		return
	}
	e.Time = l.now().UTC().Format(time.RFC3339)
	e.Command = l.cmd
	b, err := json.Marshal(e)
	if err != nil {
		return
	}
	fmt.Fprintln(l.w, string(b))
}
