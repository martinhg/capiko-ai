package sysinfo

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestDetect(t *testing.T) {
	origLook, origEnv, origRun, origHome := lookPath, getenv, runVersion, userHomeDir
	t.Cleanup(func() { lookPath, getenv, runVersion, userHomeDir = origLook, origEnv, origRun, origHome })

	lookPath = func(name string) (string, error) {
		if name == "git" || name == "brew" {
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
	runVersion = func(name string, _ ...string) (string, error) {
		switch name {
		case "copilot":
			return "1.0.59\n", nil
		case "git":
			return "git version 2.43.0", nil
		}
		return "", errors.New("not found")
	}
	home := t.TempDir()
	if err := os.MkdirAll(filepath.Join(home, ".copilot"), 0o755); err != nil {
		t.Fatal(err)
	}
	userHomeDir = func() (string, error) { return home, nil }

	r := Detect()

	if r.OS != runtime.GOOS || r.Arch != runtime.GOARCH {
		t.Errorf("OS/Arch = %s/%s, want %s/%s", r.OS, r.Arch, runtime.GOOS, runtime.GOARCH)
	}
	if r.Shell != "fish" {
		t.Errorf("Shell = %q, want fish", r.Shell)
	}
	if r.Supported != IsSupportedOS(runtime.GOOS) {
		t.Errorf("Supported = %v", r.Supported)
	}

	tools := map[string]bool{}
	for _, tl := range r.Tools {
		tools[tl.Name] = tl.Found
	}
	if !tools["git"] || !tools["brew"] || tools["curl"] || tools["node"] || tools["go"] {
		t.Errorf("tools presence wrong: %+v", tools)
	}

	deps := map[string]Dependency{}
	for _, d := range r.Dependencies {
		deps[d.Name] = d
	}
	if d := deps["copilot"]; !d.Found || d.Version != "1.0.59" || !d.Required {
		t.Errorf("copilot dep = %+v, want found 1.0.59 required", d)
	}
	if d := deps["git"]; !d.Found || d.Version != "2.43.0" {
		t.Errorf("git dep = %+v, want found 2.43.0", d)
	}
	if d := deps["node"]; d.Found {
		t.Errorf("node dep should be not found, got %+v", d)
	}

	cfgs := map[string]bool{}
	for _, c := range r.Configs {
		cfgs[c.Name] = c.Exists
	}
	if !cfgs["~/.copilot"] {
		t.Error("~/.copilot should be detected as present")
	}
	if cfgs["settings.json"] {
		t.Error("settings.json should be missing")
	}
}

func TestIsSupportedOS(t *testing.T) {
	for _, os := range []string{"darwin", "linux", "windows"} {
		if !IsSupportedOS(os) {
			t.Errorf("%s should be supported", os)
		}
	}
	if IsSupportedOS("plan9") {
		t.Error("plan9 should not be supported")
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
