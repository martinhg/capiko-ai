package sysinfo

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
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
	if d, ok := deps["pnpm"]; !ok || d.Found {
		t.Errorf("pnpm should be detected as a not-found dependency, got %+v", d)
	}
	if d := deps["node"]; d.Install == "" {
		t.Error("a missing dependency should carry an install hint")
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

func TestCustomInstructionDirsInConfigs(t *testing.T) {
	origHome := userHomeDir
	t.Cleanup(func() { userHomeDir = origHome })

	existing := t.TempDir()
	userHomeDir = func() (string, error) { return t.TempDir(), nil }
	t.Setenv("COPILOT_CUSTOM_INSTRUCTIONS_DIRS", " "+existing+" , /does/not/exist ")

	exists := map[string]bool{}
	for _, c := range detectConfigs() {
		exists[c.Name] = c.Exists
	}
	if !exists[existing] {
		t.Errorf("configured dir %q should be reported present, got %v", existing, exists)
	}
	if _, ok := exists["/does/not/exist"]; !ok || exists["/does/not/exist"] {
		t.Error("a non-existent configured dir should be listed as missing")
	}
}

func TestInstallInfo(t *testing.T) {
	if cmd, auto := installInfo("node", "darwin"); cmd != "brew install node" || !auto {
		t.Errorf("darwin node = (%q, %v), want (brew install node, true)", cmd, auto)
	}
	if cmd, auto := installInfo("pnpm", "linux"); !strings.Contains(cmd, "pnpm") || !auto {
		t.Errorf("linux pnpm = (%q, %v), want a runnable pnpm install", cmd, auto)
	}
	if _, auto := installInfo("git", "linux"); auto {
		t.Error("sudo apt install should not be auto-runnable")
	}
}

func TestInstall(t *testing.T) {
	orig := runInstall
	t.Cleanup(func() { runInstall = orig })

	var ran string
	runInstall = func(cmd string) error { ran = cmd; return nil }

	if err := Install(Dependency{Name: "node", Install: "brew install node", Auto: true}); err != nil {
		t.Fatalf("Install: %v", err)
	}
	if ran != "brew install node" {
		t.Errorf("ran %q, want brew install node", ran)
	}

	if err := Install(Dependency{Name: "git", Install: "sudo apt install git", Auto: false}); err == nil {
		t.Error("a non-auto dependency must not be run")
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
