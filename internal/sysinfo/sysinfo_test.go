package sysinfo

import (
	"errors"
	"runtime"
	"testing"
)

func TestDetect(t *testing.T) {
	origLook, origEnv := lookPath, getenv
	t.Cleanup(func() { lookPath, getenv = origLook, origEnv })

	lookPath = func(name string) (string, error) {
		if name == "copilot" || name == "git" {
			return "/usr/bin/" + name, nil
		}
		return "", errors.New("not found")
	}
	getenv = func(key string) string {
		if key == "SHELL" {
			return "/opt/homebrew/bin/fish"
		}
		return ""
	}

	r := Detect()
	if r.OS != runtime.GOOS || r.Arch != runtime.GOARCH {
		t.Errorf("OS/Arch = %s/%s, want %s/%s", r.OS, r.Arch, runtime.GOOS, runtime.GOARCH)
	}
	if r.Shell != "fish" {
		t.Errorf("Shell = %q, want fish", r.Shell)
	}
	if len(r.Tools) != len(probed) {
		t.Fatalf("tools = %d, want %d", len(r.Tools), len(probed))
	}

	got := map[string]bool{}
	for _, tool := range r.Tools {
		got[tool.Name] = tool.Found
	}
	if !got["copilot"] || !got["git"] {
		t.Error("copilot and git should be found")
	}
	if got["node"] || got["npm"] {
		t.Error("node and npm should not be found")
	}
}

func TestShellFallsBackToUnknown(t *testing.T) {
	origEnv := getenv
	t.Cleanup(func() { getenv = origEnv })
	getenv = func(string) string { return "" }

	if s := shell(); s != "unknown" {
		t.Errorf("shell = %q, want unknown", s)
	}
}
