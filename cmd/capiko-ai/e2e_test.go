//go:build e2e

// End-to-end tests for the headless CLI (install / sync / uninstall). They build
// the real capiko-ai binary and run it against an isolated, fake Copilot host —
// no real Copilot install required — asserting exit codes and on-disk state via
// the --json contract. Behind the `e2e` build tag so the normal `go test ./...`
// gate stays fast and self-contained; the dedicated e2e workflow runs them.
package main

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
)

// e2eResult mirrors internal/headless.CommandResult for decoding --json output.
type e2eResult struct {
	OK      bool   `json:"ok"`
	Command string `json:"command"`
	Items   struct {
		InstalledSkills []string `json:"installed_skills"`
		InstalledAgents []string `json:"installed_agents"`
		RemovedSkills   []string `json:"removed_skills"`
		RemovedAgents   []string `json:"removed_agents"`
	} `json:"items"`
	TotalChanged int    `json:"total_changed"`
	Error        string `json:"error"`
}

// buildCapiko compiles the capiko-ai binary into a temp dir and returns its path.
// The test's working directory is this package, so "." is the main package.
func buildCapiko(t *testing.T) string {
	t.Helper()
	bin := filepath.Join(t.TempDir(), "capiko-ai")
	cmd := exec.Command("go", "build", "-o", bin, ".")
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("build capiko-ai: %v", err)
	}
	return bin
}

// fakeHost builds an isolated environment that capiko detects as an initialized
// Copilot host: a temp HOME with a ~/.copilot directory, and a stub `copilot`
// executable on PATH. capiko only needs the binary to EXIST (it never runs it),
// so an empty executable file satisfies detection. Returns the home dir and the
// env the capiko binary should run with.
func fakeHost(t *testing.T) (home string, env []string) {
	t.Helper()
	home = t.TempDir()
	if err := os.MkdirAll(filepath.Join(home, ".copilot"), 0o755); err != nil {
		t.Fatal(err)
	}
	binDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(binDir, "copilot"), []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	env = append(os.Environ(),
		"HOME="+home,
		"PATH="+binDir+string(os.PathListSeparator)+os.Getenv("PATH"),
	)
	return home, env
}

// runCapiko runs the binary with the given env and args, returning the decoded
// JSON result and the process exit code. A non-ExitError failure (e.g. binary
// missing) fails the test.
func runCapiko(t *testing.T, bin string, env []string, args ...string) (e2eResult, int) {
	t.Helper()
	cmd := exec.Command(bin, args...)
	cmd.Env = env
	out, err := cmd.Output()
	exit := 0
	if ee, ok := err.(*exec.ExitError); ok {
		exit = ee.ExitCode()
	} else if err != nil {
		t.Fatalf("run %v: %v", args, err)
	}
	var r e2eResult
	if len(out) > 0 {
		if jerr := json.Unmarshal(out, &r); jerr != nil {
			t.Fatalf("decode --json from %v: %v\noutput: %s", args, jerr, out)
		}
	}
	return r, exit
}

// TestE2EHeadlessLifecycle drives the full install → re-install (idempotent) →
// sync → uninstall lifecycle against a fake host, asserting exit codes and the
// resulting on-disk state.
func TestE2EHeadlessLifecycle(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("e2e host stub is POSIX-only; Windows e2e is a future extension")
	}
	bin := buildCapiko(t)
	home, env := fakeHost(t)
	skillsDir := filepath.Join(home, ".copilot", "skills")
	probe := filepath.Join(skillsDir, "sdd-apply", "SKILL.md")

	// install --all: succeeds, installs skills, writes files to disk.
	r, code := runCapiko(t, bin, env, "install", "--all", "--json")
	if code != 0 {
		t.Fatalf("install exit = %d, want 0", code)
	}
	if !r.OK || r.Command != "install" {
		t.Fatalf("install result = %+v", r)
	}
	if len(r.Items.InstalledSkills) == 0 {
		t.Fatal("install reported no skills installed")
	}
	if r.TotalChanged == 0 {
		t.Fatal("install TotalChanged = 0, want > 0")
	}
	if _, err := os.Stat(probe); err != nil {
		t.Fatalf("expected %s on disk after install: %v", probe, err)
	}

	// install again: idempotent — everything already present, nothing changes.
	r2, code2 := runCapiko(t, bin, env, "install", "--all", "--json")
	if code2 != 0 {
		t.Fatalf("second install exit = %d, want 0", code2)
	}
	if r2.TotalChanged != 0 {
		t.Fatalf("second install TotalChanged = %d, want 0 (idempotent)", r2.TotalChanged)
	}

	// sync: succeeds against the installed baseline.
	rs, codeS := runCapiko(t, bin, env, "sync", "--json")
	if codeS != 0 {
		t.Fatalf("sync exit = %d, want 0", codeS)
	}
	if !rs.OK {
		t.Fatalf("sync result not OK: %+v", rs)
	}

	// uninstall --all: succeeds, removes the managed skills from disk.
	ru, codeU := runCapiko(t, bin, env, "uninstall", "--all", "--json")
	if codeU != 0 {
		t.Fatalf("uninstall exit = %d, want 0", codeU)
	}
	if !ru.OK {
		t.Fatalf("uninstall result not OK: %+v", ru)
	}
	if len(ru.Items.RemovedSkills) == 0 {
		t.Fatal("uninstall reported no skills removed")
	}
	if _, err := os.Stat(filepath.Join(skillsDir, "sdd-apply")); !os.IsNotExist(err) {
		t.Fatalf("expected sdd-apply removed after uninstall, stat err = %v", err)
	}
}

// TestE2ECopilotNotFoundExits2 asserts the exit-code contract: when Copilot is
// not on PATH, headless commands exit 2 (not found), not 1 (error).
func TestE2ECopilotNotFoundExits2(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("e2e host stub is POSIX-only; Windows e2e is a future extension")
	}
	bin := buildCapiko(t)
	// HOME with no ~/.copilot, and a PATH that contains no `copilot` binary.
	home := t.TempDir()
	emptyBin := t.TempDir()
	env := append(os.Environ(), "HOME="+home, "PATH="+emptyBin)

	_, code := runCapiko(t, bin, env, "install", "--all", "--json")
	if code != 2 {
		t.Fatalf("install with no Copilot on PATH: exit = %d, want 2", code)
	}
}
