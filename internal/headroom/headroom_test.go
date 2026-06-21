package headroom

import (
	"errors"
	"testing"
)

func TestCopilotCLIEntryShape(t *testing.T) {
	e := CopilotCLIEntry()
	if e["type"] != "local" {
		t.Errorf(`type = %v, want "local"`, e["type"])
	}
	if e["command"] != "headroom" {
		t.Errorf(`command = %v, want "headroom"`, e["command"])
	}
	args, ok := e["args"].([]string)
	if !ok || len(args) != 2 || args[0] != "mcp" || args[1] != "serve" {
		t.Errorf(`args = %v, want ["mcp" "serve"]`, e["args"])
	}
	tools, ok := e["tools"].([]string)
	if !ok || len(tools) != 1 || tools[0] != "*" {
		t.Errorf(`tools = %v, want ["*"]`, e["tools"])
	}
}

func TestDetectedReportsLookPath(t *testing.T) {
	prev := lookPath
	t.Cleanup(func() { lookPath = prev })

	lookPath = func(string) (string, error) { return "/usr/local/bin/headroom", nil }
	if !Detected() {
		t.Error("Detected should be true when headroom is on PATH")
	}

	lookPath = func(string) (string, error) { return "", errors.New("not found") }
	if Detected() {
		t.Error("Detected should be false when headroom is absent")
	}
}
