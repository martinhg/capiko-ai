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

func TestDetectLinuxDistro(t *testing.T) {
	tests := []struct {
		name       string
		osRelease  string
		wantDistro string
	}{
		{"empty", "", linuxDistroUnknown},
		{"ubuntu", "ID=ubuntu\nVERSION_ID=\"24.04\"", linuxDistroUbuntu},
		{"debian", "ID=debian", linuxDistroDebian},
		{"mint via id_like", "ID=linuxmint\nID_LIKE=ubuntu", linuxDistroUbuntu},
		{"arch", "ID=arch", linuxDistroArch},
		{"manjaro via id_like", "ID=manjaro\nID_LIKE=arch", linuxDistroArch},
		{"fedora", "ID=fedora", linuxDistroFedora},
		{"rocky", "ID=rocky", linuxDistroFedora},
		{"rhel via id_like", "ID=ol\nID_LIKE=\"rhel fedora\"", linuxDistroFedora},
		{"unknown", "ID=void", linuxDistroUnknown},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := detectLinuxDistro(tc.osRelease); got != tc.wantDistro {
				t.Errorf("detectLinuxDistro = %q, want %q", got, tc.wantDistro)
			}
		})
	}
}

func TestPackageManager(t *testing.T) {
	origLook, origOSRel := lookPath, osReleaseFn
	t.Cleanup(func() { lookPath, osReleaseFn = origLook, origOSRel })

	noBrew := func(name string) (string, error) { return "", errors.New("not found") }
	lookPath = noBrew

	if got := packageManager("darwin"); got != "brew" {
		t.Errorf("darwin pm = %q, want brew", got)
	}
	if got := packageManager("windows"); got != "winget" {
		t.Errorf("windows pm = %q, want winget", got)
	}

	cases := map[string]string{"ID=ubuntu": "apt", "ID=arch": "pacman", "ID=fedora": "dnf", "ID=void": ""}
	for osrel, want := range cases {
		osReleaseFn = func() string { return osrel }
		if got := packageManager("linux"); got != want {
			t.Errorf("linux %q pm = %q, want %q", osrel, got, want)
		}
	}

	// brew on Linux wins over the distro package manager.
	lookPath = func(name string) (string, error) {
		if name == "brew" {
			return "/home/linuxbrew/.linuxbrew/bin/brew", nil
		}
		return "", errors.New("not found")
	}
	osReleaseFn = func() string { return "ID=ubuntu" }
	if got := packageManager("linux"); got != "brew" {
		t.Errorf("linux with brew pm = %q, want brew", got)
	}
}

func TestInstallInfo(t *testing.T) {
	if cmd, auto := installInfo("node", "brew"); cmd != "brew install node" || !auto {
		t.Errorf("brew node = (%q, %v), want (brew install node, true)", cmd, auto)
	}
	if cmd, auto := installInfo("pnpm", "apt"); !strings.Contains(cmd, "pnpm") || !auto {
		t.Errorf("pnpm = (%q, %v), want a runnable pnpm install", cmd, auto)
	}
	// Each Linux package manager produces its own correct, non-auto command.
	for pm, want := range map[string]string{
		"apt":    "sudo apt-get install -y git",
		"pacman": "sudo pacman -S --noconfirm git",
		"dnf":    "sudo dnf install -y git",
	} {
		if cmd, auto := installInfo("git", pm); cmd != want || auto {
			t.Errorf("%s git = (%q, %v), want (%q, false)", pm, cmd, auto, want)
		}
	}
	// Windows uses winget, not apt (regression guard).
	if cmd, auto := installInfo("git", "winget"); !strings.Contains(cmd, "winget") || auto {
		t.Errorf("winget git = (%q, %v), want a winget command, not auto", cmd, auto)
	}
	// Node on apt uses the NodeSource setup, not a bare apt package.
	if cmd, _ := installInfo("node", "apt"); !strings.Contains(cmd, "nodesource") {
		t.Errorf("apt node = %q, want a NodeSource install", cmd)
	}
	// Unknown package manager falls back to a manual hint.
	if cmd, auto := installInfo("git", ""); auto || strings.HasPrefix(cmd, "sudo") {
		t.Errorf("unknown pm git = (%q, %v), want a non-auto manual hint", cmd, auto)
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
