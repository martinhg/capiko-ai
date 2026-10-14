package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseReportArgs(t *testing.T) {
	cases := []struct {
		name    string
		args    []string
		wantErr bool
		check   func(reportOpts) bool
	}{
		{"none", nil, false, func(o reportOpts) bool { return !o.all && !o.asJSON && o.sessionID == "" }},
		{"session", []string{"--session", "s1"}, false, func(o reportOpts) bool { return o.sessionID == "s1" }},
		{"all json", []string{"--all", "--json"}, false, func(o reportOpts) bool { return o.all && o.asJSON }},
		{"since", []string{"--since", "2026-10-01"}, false, func(o reportOpts) bool { return o.hasSince && o.since.Day() == 1 }},
		{"bad since", []string{"--since", "yesterday"}, true, nil},
		{"missing value", []string{"--session"}, true, nil},
		{"unknown", []string{"--bogus"}, true, nil},
		{"session and all", []string{"--session", "x", "--all"}, true, nil},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			o, err := parseReportArgs(c.args)
			if (err != nil) != c.wantErr {
				t.Fatalf("err = %v, wantErr %v", err, c.wantErr)
			}
			if !c.wantErr && !c.check(o) {
				t.Errorf("unexpected opts %+v", o)
			}
		})
	}
}

func setupSessions(t *testing.T) {
	t.Helper()
	home := t.TempDir()
	orig := userHomeDirFn
	userHomeDirFn = func() (string, error) { return home, nil }
	t.Cleanup(func() { userHomeDirFn = orig })

	dir := filepath.Join(home, ".capiko", "sessions")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	old := `{"event":"prompt","ts":"2026-09-01T10:00:00Z","payload":{"prompt":"old"}}` + "\n"
	recent := `{"event":"prompt","ts":"2026-10-14T01:00:00Z","payload":{"userPrompt":"hello"}}` + "\n" +
		`{"event":"tool_use","ts":"2026-10-14T01:30:00Z","payload":{"toolName":"bash"}}` + "\n"
	for name, body := range map[string]string{"old.jsonl": old, "recent.jsonl": recent} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

func run(t *testing.T, args ...string) (string, int, error) {
	t.Helper()
	var buf bytes.Buffer
	handled, code, err := reportCommand("report", args, &buf)
	if !handled {
		t.Fatal("report not handled")
	}
	return buf.String(), code, err
}

func TestReportCommand_NotHandled(t *testing.T) {
	handled, _, _ := reportCommand("doctor", nil, &bytes.Buffer{})
	if handled {
		t.Error("should not handle other commands")
	}
}

func TestReportCommand_LatestSession(t *testing.T) {
	setupSessions(t)
	out, code, err := run(t)
	if err != nil || code != 0 {
		t.Fatalf("code=%d err=%v", code, err)
	}
	if !strings.Contains(out, "Session Report: recent") || !strings.Contains(out, "Tool calls: 1") {
		t.Errorf("unexpected output:\n%s", out)
	}
}

func TestReportCommand_SessionJSON(t *testing.T) {
	setupSessions(t)
	out, _, err := run(t, "--session", "old", "--json")
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	if err := json.Unmarshal([]byte(out), &m); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, out)
	}
	if m["session_id"] != "old" || m["prompts"] != float64(1) {
		t.Errorf("unexpected JSON: %v", m)
	}
}

func TestReportCommand_AllAndSince(t *testing.T) {
	setupSessions(t)

	out, _, err := run(t, "--all")
	if err != nil || !strings.Contains(out, "Sessions:             2") {
		t.Fatalf("--all: err=%v\n%s", err, out)
	}

	out, _, err = run(t, "--since", "2026-10-01", "--json")
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	if err := json.Unmarshal([]byte(out), &m); err != nil {
		t.Fatal(err)
	}
	if m["sessions"] != float64(1) {
		t.Errorf("since should keep 1 session, got %v", m["sessions"])
	}
}

func TestReportCommand_Errors(t *testing.T) {
	setupSessions(t)
	if _, code, err := run(t, "--session", "missing"); err == nil || code != 1 {
		t.Errorf("missing session: code=%d err=%v", code, err)
	}
	if _, code, err := run(t, "--bogus"); err == nil || code != 1 {
		t.Errorf("unknown flag: code=%d err=%v", code, err)
	}
	if _, code, err := run(t, "--since", "2030-01-01"); err == nil || code != 1 {
		t.Errorf("empty since: code=%d err=%v", code, err)
	}
}

func TestReportCommand_NoSessions(t *testing.T) {
	home := t.TempDir()
	orig := userHomeDirFn
	userHomeDirFn = func() (string, error) { return home, nil }
	t.Cleanup(func() { userHomeDirFn = orig })
	if _, code, err := run(t); err == nil || code != 1 {
		t.Errorf("code=%d err=%v", code, err)
	}
}
