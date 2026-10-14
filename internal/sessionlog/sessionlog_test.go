package sessionlog_test

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/martinhg/capiko-ai/internal/sessionlog"
)

func writeFile(t *testing.T, dir, name, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", name, err)
	}
}

const validLines = `{"event":"prompt","ts":"2026-01-01T00:00:00Z","payload":{"sessionId":"a","prompt":"hi"}}
{"event":"tool_use","ts":"2026-01-01T00:00:01Z","payload":{"sessionId":"a","toolName":"bash"}}
`

func TestDir(t *testing.T) {
	want := filepath.Join("/h", ".capiko", "sessions")
	if got := sessionlog.Dir("/h"); got != want {
		t.Errorf("Dir = %q, want %q", got, want)
	}
}

func TestListSessions(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "b.jsonl", "")
	writeFile(t, dir, "a.jsonl", "")
	writeFile(t, dir, "notes.txt", "")
	if err := os.Mkdir(filepath.Join(dir, "sub.jsonl"), 0o755); err != nil {
		t.Fatal(err)
	}

	got, err := sessionlog.ListSessions(dir)
	if err != nil {
		t.Fatalf("ListSessions: %v", err)
	}
	if want := []string{"a", "b"}; !reflect.DeepEqual(got, want) {
		t.Errorf("ListSessions = %v, want %v", got, want)
	}
}

func TestListSessions_MissingDir(t *testing.T) {
	got, err := sessionlog.ListSessions(filepath.Join(t.TempDir(), "nope"))
	if err != nil {
		t.Fatalf("ListSessions: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("ListSessions = %v, want empty", got)
	}
}

func TestReadSession(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "a.jsonl", validLines)

	events, err := sessionlog.ReadSession(dir, "a")
	if err != nil {
		t.Fatalf("ReadSession: %v", err)
	}
	if len(events) != 2 {
		t.Fatalf("len(events) = %d, want 2", len(events))
	}
	if events[0].Type != sessionlog.EventPrompt || events[0].TS != "2026-01-01T00:00:00Z" {
		t.Errorf("events[0] = %+v", events[0])
	}
	if events[1].Type != sessionlog.EventToolUse {
		t.Errorf("events[1].Type = %q, want %q", events[1].Type, sessionlog.EventToolUse)
	}
	if want := `{"sessionId":"a","toolName":"bash"}`; string(events[1].Payload) != want {
		t.Errorf("events[1].Payload = %s, want %s", events[1].Payload, want)
	}
}

func TestReadSession_Empty(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "e.jsonl", "")

	events, err := sessionlog.ReadSession(dir, "e")
	if err != nil {
		t.Fatalf("ReadSession: %v", err)
	}
	if events == nil || len(events) != 0 {
		t.Errorf("events = %#v, want empty non-nil slice", events)
	}
}

func TestReadSession_Missing(t *testing.T) {
	if _, err := sessionlog.ReadSession(t.TempDir(), "nope"); err == nil {
		t.Fatal("want error for missing session file")
	}
}

func TestReadSession_SkipsMalformedLines(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "m.jsonl", "not json\n\n"+validLines+`{"event":"prompt","ts":`+"\n")

	events, err := sessionlog.ReadSession(dir, "m")
	if err != nil {
		t.Fatalf("ReadSession: %v", err)
	}
	if len(events) != 2 {
		t.Errorf("len(events) = %d, want 2 (malformed lines skipped)", len(events))
	}
}

func TestReadAll(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "a.jsonl", validLines)
	writeFile(t, dir, "b.jsonl", "")

	all, err := sessionlog.ReadAll(dir)
	if err != nil {
		t.Fatalf("ReadAll: %v", err)
	}
	if len(all) != 2 || len(all["a"]) != 2 || len(all["b"]) != 0 {
		t.Errorf("ReadAll = %v", all)
	}
}
