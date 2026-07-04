package copilothooks_test

import (
	"bytes"
	"encoding/json"
	"flag"
	"os"
	"path/filepath"
	"testing"

	"github.com/martinhg/capiko-ai/internal/copilothooks"
)

var updateGolden = flag.Bool("update", false, "update golden files")

// golden compares got against testdata/<name>, or writes it when -update is
// passed. Shared by the schema tests here and the renderer tests in
// render_test.go.
func golden(t *testing.T, name string, got []byte) {
	t.Helper()
	path := filepath.Join("testdata", name)
	if *updateGolden {
		if err := os.MkdirAll("testdata", 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, got, 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read golden %q: %v (run: go test ./internal/copilothooks -update)", name, err)
	}
	if !bytes.Equal(got, want) {
		t.Errorf("%s mismatch\n--- got ---\n%s\n--- want ---\n%s", name, got, want)
	}
}

func TestMarshal_TrailingNewlineAndRoundTrip(t *testing.T) {
	f := copilothooks.HookFile{
		Version: copilothooks.SchemaVersion,
		Hooks: map[string][]copilothooks.Hook{
			copilothooks.EventPreToolUse: {{
				Type:       "command",
				Matcher:    copilothooks.MatcherBash,
				Bash:       "echo bash",
				PowerShell: "echo ps",
				TimeoutSec: copilothooks.TimeoutSeconds,
			}},
		},
	}

	data, err := copilothooks.Marshal(f)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	if len(data) == 0 || data[len(data)-1] != '\n' {
		t.Fatalf("Marshal: want trailing newline, got %q", data)
	}
	if len(data) >= 2 && data[len(data)-2] == '\n' {
		t.Fatalf("Marshal: want exactly one trailing newline, got %q", data)
	}

	var round copilothooks.HookFile
	if err := json.Unmarshal(data, &round); err != nil {
		t.Fatalf("Unmarshal round trip: %v", err)
	}
	if round.Version != 1 {
		t.Errorf("Version = %d, want 1", round.Version)
	}
	hooks, ok := round.Hooks["preToolUse"]
	if !ok || len(hooks) != 1 {
		t.Fatalf("Hooks[preToolUse] = %v, want exactly one entry", hooks)
	}
	h := hooks[0]
	if h.Matcher != "bash" {
		t.Errorf("Matcher = %q, want bash", h.Matcher)
	}
	if h.Bash == "" || h.PowerShell == "" {
		t.Errorf("want both Bash and PowerShell populated, got Bash=%q PowerShell=%q", h.Bash, h.PowerShell)
	}
}

func TestHookFile_EventIsMapKeyNotField(t *testing.T) {
	// REQ-2.1 / ADR-2: the event name is the KEY of the Hooks map, not a
	// field on Hook. This guards against reintroducing the rejected flat
	// "hooks":[{event:...}] array shape.
	h := copilothooks.Hook{Type: "command", Matcher: copilothooks.MatcherBash}
	f := copilothooks.HookFile{Hooks: map[string][]copilothooks.Hook{
		copilothooks.EventPreToolUse: {h},
	}}
	if _, ok := f.Hooks[copilothooks.EventPreToolUse]; !ok {
		t.Fatal("want event name as map key")
	}
}

func TestUnmarshal_VerifiedSchemaShape(t *testing.T) {
	// The exact REQ-2.1 example JSON, verified against the real Copilot CLI
	// schema (engram #395), must unmarshal cleanly into HookFile.
	const raw = `{
  "version": 1,
  "hooks": {
    "preToolUse": [
      {
        "type": "command",
        "matcher": "bash",
        "bash": "echo bash-script",
        "powershell": "echo ps-script",
        "timeoutSec": 5
      }
    ]
  }
}`
	var f copilothooks.HookFile
	if err := json.Unmarshal([]byte(raw), &f); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if f.Version != 1 {
		t.Errorf("Version = %d, want 1", f.Version)
	}
	entries, ok := f.Hooks["preToolUse"]
	if !ok || len(entries) != 1 {
		t.Fatalf("Hooks[preToolUse] = %v, want exactly one entry", entries)
	}
	got := entries[0]
	want := copilothooks.Hook{
		Type:       "command",
		Matcher:    "bash",
		Bash:       "echo bash-script",
		PowerShell: "echo ps-script",
		TimeoutSec: 5,
	}
	if got != want {
		t.Errorf("Hooks[preToolUse][0] = %+v, want %+v", got, want)
	}
}
