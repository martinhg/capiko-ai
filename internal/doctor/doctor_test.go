package doctor

import (
	"strings"
	"testing"
	"time"

	"github.com/martinhg/capiko-ai/internal/copilot"
	"github.com/martinhg/capiko-ai/internal/state"
	"github.com/martinhg/capiko-ai/internal/sysinfo"
)

// healthyEnv is a sysinfo.Report where every prerequisite capiko needs is
// present, used as the baseline the failing cases mutate.
func healthyEnv() sysinfo.Report {
	deps := []sysinfo.Dependency{
		{Name: "copilot", Required: true, Found: true, Version: "0.0.1"},
		{Name: "node", Required: true, Found: true, Version: "22.0.0"},
		{Name: "npm", Required: true, Found: true, Version: "10.0.0"},
		{Name: "pnpm", Required: true, Found: true, Version: "9.0.0"},
		{Name: "git", Required: true, Found: true, Version: "2.44.0"},
		{Name: "curl", Required: true, Found: true, Version: "8.0.0"},
	}
	return sysinfo.Report{OS: "darwin", Arch: "arm64", Supported: true, Dependencies: deps}
}

// find returns the check with the given name, failing the test when absent.
func find(t *testing.T, r Report, name string) Check {
	t.Helper()
	for _, c := range r.Checks {
		if c.Name == name {
			return c
		}
	}
	t.Fatalf("no check named %q in report; have %v", name, names(r))
	return Check{}
}

func names(r Report) []string {
	out := make([]string, 0, len(r.Checks))
	for _, c := range r.Checks {
		out = append(out, c.Name)
	}
	return out
}

func TestEvaluateAllHealthy(t *testing.T) {
	in := Inputs{
		Env:         healthyEnv(),
		CopilotHost: &copilot.Host{BinPath: "/usr/local/bin/copilot", ConfigDir: "/home/u/.copilot"},
		State:       &state.State{Version: "1.2.1"},
	}
	r := Evaluate(in)

	if !r.Healthy() {
		t.Fatalf("expected healthy report, got %v", names(r))
	}
	_, _, fail := r.Counts()
	if fail != 0 {
		t.Errorf("expected 0 fail checks, got %d", fail)
	}
	if c := find(t, r, "Copilot CLI"); c.Status != Pass {
		t.Errorf("Copilot CLI: want Pass, got %v", c.Status)
	}
}

func TestEvaluateCopilotMissingFails(t *testing.T) {
	env := healthyEnv()
	env.Dependencies[0] = sysinfo.Dependency{
		Name: "copilot", Required: true, Found: false,
		Install: "install copilot from https://github.com/github/copilot-cli",
	}
	r := Evaluate(Inputs{Env: env})

	if r.Healthy() {
		t.Fatal("expected unhealthy report when copilot is missing")
	}
	c := find(t, r, "Copilot CLI")
	if c.Status != Fail {
		t.Errorf("Copilot CLI: want Fail, got %v", c.Status)
	}
	if c.Remedy == "" {
		t.Error("a failed required-dependency check must carry a remedy")
	}
}

func TestEvaluateCopilotInstalledButNotInitialized(t *testing.T) {
	// copilot binary is found, but Detect returned no host (~/.copilot absent).
	r := Evaluate(Inputs{Env: healthyEnv(), CopilotHost: nil})

	c := find(t, r, "Copilot config")
	if c.Status != Warn {
		t.Errorf("Copilot config: want Warn when binary present but not initialized, got %v", c.Status)
	}
	// A warn must not break overall health.
	if !r.Healthy() {
		t.Error("an un-initialized config is a warning, not a failure")
	}
}

func TestEvaluateCorruptStateFails(t *testing.T) {
	r := Evaluate(Inputs{
		Env:         healthyEnv(),
		CopilotHost: &copilot.Host{BinPath: "/b/copilot"},
		StateErr:    errString("unexpected end of JSON input"),
	})
	c := find(t, r, "State file")
	if c.Status != Fail {
		t.Errorf("State file: want Fail on corrupt state, got %v", c.Status)
	}
	if r.Healthy() {
		t.Error("a corrupt state file must make the report unhealthy")
	}
}

func TestEvaluateSkillDriftWarnsNotFails(t *testing.T) {
	r := Evaluate(Inputs{
		Env:         healthyEnv(),
		CopilotHost: &copilot.Host{BinPath: "/b/copilot"},
		State:       &state.State{Version: "1.2.1"},
		SkillDrift:  []string{"sdd-apply", "sdd-spec"},
	})
	c := find(t, r, "Skill drift")
	if c.Status != Warn {
		t.Errorf("Skill drift: want Warn, got %v", c.Status)
	}
	if !r.Healthy() {
		t.Error("drift is a warning (run sync), not a hard failure")
	}
	if c.Remedy == "" {
		t.Error("drift warning should suggest running sync")
	}
}

func TestEvaluateEngramManagedButBinaryMissing(t *testing.T) {
	env := healthyEnv() // no engram dep entry → treated as not found
	r := Evaluate(Inputs{
		Env:         env,
		CopilotHost: &copilot.Host{BinPath: "/b/copilot"},
		State: &state.State{
			Version: "1.2.1",
			Engram:  &state.EngramRecord{Enabled: true, ArtifactMode: "hybrid"},
		},
	})
	c := find(t, r, "Engram backend")
	if c.Status != Warn {
		t.Errorf("Engram backend: want Warn when managed but binary missing, got %v", c.Status)
	}
}

// engramEnv is a healthy environment with the engram binary present at version.
func engramEnv(version string) sysinfo.Report {
	env := healthyEnv()
	env.Dependencies = append(env.Dependencies, sysinfo.Dependency{Name: "engram", Found: true, Version: version})
	return env
}

// managedEngramState is a managed install with engram enabled.
func managedEngramState() *state.State {
	return &state.State{Version: "1.2.1", Engram: &state.EngramRecord{Enabled: true, ArtifactMode: "hybrid"}}
}

func TestEvaluateEngramOutdated(t *testing.T) {
	r := Evaluate(Inputs{
		Env:               engramEnv("1.16.3"),
		CopilotHost:       &copilot.Host{BinPath: "/b/copilot"},
		State:             managedEngramState(),
		RecommendedEngram: "1.17.0",
	})
	c := find(t, r, "Engram version")
	if c.Status != Warn {
		t.Errorf("Engram version: want Warn when behind recommended, got %v", c.Status)
	}
	if c.Remedy == "" {
		t.Error("outdated engram should suggest an upgrade")
	}
	if !strings.Contains(c.Detail, "1.16.3") || !strings.Contains(c.Detail, "1.17.0") {
		t.Errorf("detail should name installed and recommended versions, got %q", c.Detail)
	}
}

func TestEvaluateEngramCurrentIsPass(t *testing.T) {
	r := Evaluate(Inputs{
		Env:               engramEnv("1.17.0"),
		CopilotHost:       &copilot.Host{BinPath: "/b/copilot"},
		State:             managedEngramState(),
		RecommendedEngram: "1.17.0",
	})
	c := find(t, r, "Engram version")
	if c.Status != Pass {
		t.Errorf("Engram version: want Pass when at recommended, got %v", c.Status)
	}
}

func TestEvaluateEngramVersionSkippedWhenUnmanaged(t *testing.T) {
	r := Evaluate(Inputs{
		Env:               engramEnv("1.16.3"),
		CopilotHost:       &copilot.Host{BinPath: "/b/copilot"},
		State:             &state.State{Version: "1.2.1"}, // engram unmanaged
		RecommendedEngram: "1.17.0",
	})
	for _, c := range r.Checks {
		if c.Name == "Engram version" {
			t.Error("Engram version check must not appear when engram is unmanaged")
		}
	}
}

func TestEvaluateEngramVersionSkippedWhenMissing(t *testing.T) {
	// Managed but the engram binary is absent: engramCheck warns about the missing
	// binary; no version line is added.
	r := Evaluate(Inputs{
		Env:               healthyEnv(), // no engram dep
		CopilotHost:       &copilot.Host{BinPath: "/b/copilot"},
		State:             managedEngramState(),
		RecommendedEngram: "1.17.0",
	})
	for _, c := range r.Checks {
		if c.Name == "Engram version" {
			t.Error("Engram version check must not appear when the binary is missing")
		}
	}
}

func TestEvaluateEngramUnmanagedIsPass(t *testing.T) {
	r := Evaluate(Inputs{
		Env:         healthyEnv(),
		CopilotHost: &copilot.Host{BinPath: "/b/copilot"},
		State:       &state.State{Version: "1.2.1"}, // Engram nil
	})
	c := find(t, r, "Engram backend")
	if c.Status != Pass {
		t.Errorf("Engram backend: want Pass when unmanaged (optional), got %v", c.Status)
	}
}

func TestEvaluateDriftIgnoredWhenUnmanaged(t *testing.T) {
	// No managed install (empty Version): drift slices are meaningless because
	// there is no recorded baseline, so the checks must not cry wolf.
	r := Evaluate(Inputs{
		Env:         healthyEnv(),
		CopilotHost: &copilot.Host{BinPath: "/b/copilot"},
		State:       &state.State{}, // Version "" = unmanaged
		SkillDrift:  []string{"sdd-apply"},
		AgentDrift:  []string{"capiko-sdd-apply"},
	})
	if c := find(t, r, "Skill drift"); c.Status != Pass {
		t.Errorf("skill drift on an unmanaged install should be Pass (n/a), got %v", c.Status)
	}
	if c := find(t, r, "Agent drift"); c.Status != Pass {
		t.Errorf("agent drift on an unmanaged install should be Pass (n/a), got %v", c.Status)
	}
	if !r.Healthy() {
		t.Error("an unmanaged install with no real problems is healthy")
	}
}

func TestEvaluateUpdateCheckNeverChecked(t *testing.T) {
	// A managed install that has never recorded a successful release check still
	// reports a Pass line — "never checked" is informational, not a problem.
	r := Evaluate(Inputs{Env: healthyEnv(), State: &state.State{Version: "1.2.1"}})

	c := find(t, r, "Update check")
	if c.Status != Pass {
		t.Errorf("Update check: want Pass, got %v", c.Status)
	}
	if !strings.Contains(c.Detail, "no successful update check") {
		t.Errorf("Update check: want never-checked detail, got %q", c.Detail)
	}
	if !r.Healthy() {
		t.Error("a never-checked update state must not break health")
	}
}

func TestEvaluateUpdateCheckReportsLastTime(t *testing.T) {
	last := time.Date(2026, 6, 20, 14, 3, 0, 0, time.UTC)
	now := last.Add(2 * time.Hour)
	r := Evaluate(Inputs{
		Env:   healthyEnv(),
		State: &state.State{Version: "1.2.1", LastUpdateCheck: &last},
		Now:   now,
	})

	c := find(t, r, "Update check")
	if c.Status != Pass {
		t.Errorf("Update check: want Pass, got %v", c.Status)
	}
	if !strings.Contains(c.Detail, "2026-06-20 14:03") {
		t.Errorf("Update check: want the last-check timestamp in detail, got %q", c.Detail)
	}
	if !strings.Contains(c.Detail, "2h0m0s ago") {
		t.Errorf("Update check: want the relative age in detail, got %q", c.Detail)
	}
}

// errString is a tiny error helper so tests don't import errors for one literal.
type errString string

func (e errString) Error() string { return string(e) }
