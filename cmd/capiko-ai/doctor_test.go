package main

import (
	"bytes"
	"strings"
	"testing"

	"github.com/martinhg/capiko-ai/internal/agent"
	"github.com/martinhg/capiko-ai/internal/backup"
	"github.com/martinhg/capiko-ai/internal/copilot"
	"github.com/martinhg/capiko-ai/internal/doctor"
	"github.com/martinhg/capiko-ai/internal/skill"
	"github.com/martinhg/capiko-ai/internal/state"
	"github.com/martinhg/capiko-ai/internal/sysinfo"
)

// withStubInputs swaps the gather seam for the duration of a test.
func withStubInputs(t *testing.T, in doctor.Inputs) {
	t.Helper()
	prev := gatherDoctorInputs
	gatherDoctorInputs = func() doctor.Inputs { return in }
	t.Cleanup(func() { gatherDoctorInputs = prev })
}

func healthyInputs() doctor.Inputs {
	return doctor.Inputs{
		Env: sysinfo.Report{OS: "darwin", Arch: "arm64", Supported: true, Dependencies: []sysinfo.Dependency{
			{Name: "copilot", Required: true, Found: true, Version: "0.0.1"},
			{Name: "node", Required: true, Found: true, Version: "22"},
			{Name: "npm", Required: true, Found: true, Version: "10"},
			{Name: "pnpm", Required: true, Found: true, Version: "9"},
			{Name: "git", Required: true, Found: true, Version: "2.44"},
			{Name: "curl", Required: true, Found: true, Version: "8"},
		}},
		CopilotHost: &copilot.Host{BinPath: "/b/copilot", ConfigDir: "/h/.copilot"},
		State:       &state.State{Version: "1.2.1"},
	}
}

func TestDoctorCommandNotHandledForOtherName(t *testing.T) {
	var buf bytes.Buffer
	handled, _, err := doctorCommand("sync", nil, &buf)
	if handled || err != nil {
		t.Fatalf("doctor should not handle %q (handled=%v err=%v)", "sync", handled, err)
	}
}

func TestDoctorCommandTextOutput(t *testing.T) {
	withStubInputs(t, healthyInputs())
	var buf bytes.Buffer
	handled, healthy, err := doctorCommand("doctor", nil, &buf)
	if !handled || err != nil {
		t.Fatalf("handled=%v err=%v", handled, err)
	}
	if !healthy {
		t.Error("healthy inputs should yield a healthy report")
	}
	out := buf.String()
	if !strings.Contains(out, "Copilot CLI") || !strings.Contains(out, "pass") {
		t.Errorf("unexpected text output:\n%s", out)
	}
}

func TestDoctorCommandJSONFlag(t *testing.T) {
	withStubInputs(t, healthyInputs())
	var buf bytes.Buffer
	if _, _, err := doctorCommand("doctor", []string{"--json"}, &buf); err != nil {
		t.Fatalf("err=%v", err)
	}
	out := buf.String()
	if !strings.HasPrefix(strings.TrimSpace(out), "{") {
		t.Errorf("--json should emit JSON, got:\n%s", out)
	}
	if !strings.Contains(out, `"status": "pass"`) {
		t.Errorf("--json missing string statuses:\n%s", out)
	}
}

func TestDoctorCommandReportsUnhealthy(t *testing.T) {
	in := healthyInputs()
	in.Env.Dependencies[0] = sysinfo.Dependency{Name: "copilot", Required: true, Found: false, Install: "install it"}
	withStubInputs(t, in)

	var buf bytes.Buffer
	_, healthy, err := doctorCommand("doctor", nil, &buf)
	if err != nil {
		t.Fatalf("err=%v", err)
	}
	if healthy {
		t.Error("missing copilot must report unhealthy (drives exit code 1)")
	}
}

func TestDoctorCommandRejectsUnknownFlag(t *testing.T) {
	withStubInputs(t, healthyInputs())
	var buf bytes.Buffer
	if _, _, err := doctorCommand("doctor", []string{"--nope"}, &buf); err == nil {
		t.Error("an unknown flag should error")
	}
}

// driftInputs is a healthy environment with skill drift, so `doctor --repair`
// has something to fix while the report itself stays healthy (drift is a Warn).
func driftInputs() doctor.Inputs {
	in := healthyInputs()
	in.SkillDrift = []string{"capiko-hello"}
	return in
}

func TestDoctorRepairWithDriftRunsSync(t *testing.T) {
	withStubInputs(t, driftInputs())
	withStubSyncInputs(t, readySyncInputs(t))
	called := false
	withStubRunSync(t, func(_ *copilot.Host, _ []skill.Skill, _ []agent.Agent, _ *state.Store, _ *backup.Store) (int, error) {
		called = true
		return 3, nil
	})

	var buf bytes.Buffer
	handled, healthy, err := doctorCommand("doctor", []string{"--repair"}, &buf)
	if !handled || err != nil {
		t.Fatalf("handled=%v err=%v", handled, err)
	}
	if !healthy {
		t.Error("drift is a warning, not a failure; healthy must stay true after repair")
	}
	if !called {
		t.Error("RunSync MUST be called when --repair finds drift")
	}
	out := buf.String()
	if !strings.Contains(out, "Repair") || !strings.Contains(out, "repaired 3 item") {
		t.Errorf("expected repair summary naming 3 repaired items, got:\n%s", out)
	}
}

func TestDoctorRepairNoDriftSkipsSync(t *testing.T) {
	withStubInputs(t, healthyInputs()) // no drift
	withStubSyncInputs(t, readySyncInputs(t))
	withStubRunSync(t, func(_ *copilot.Host, _ []skill.Skill, _ []agent.Agent, _ *state.Store, _ *backup.Store) (int, error) {
		t.Error("RunSync must NOT be called when there is no drift")
		return 0, nil
	})

	var buf bytes.Buffer
	_, healthy, err := doctorCommand("doctor", []string{"--repair"}, &buf)
	if err != nil {
		t.Fatalf("err=%v", err)
	}
	if !healthy {
		t.Error("healthy inputs should stay healthy")
	}
	if !strings.Contains(buf.String(), "nothing to repair") {
		t.Errorf("expected 'nothing to repair' note, got:\n%s", buf.String())
	}
}

func TestDoctorRepairJSONEmitsSingleObject(t *testing.T) {
	withStubInputs(t, driftInputs())
	withStubSyncInputs(t, readySyncInputs(t))
	withStubRunSync(t, func(_ *copilot.Host, _ []skill.Skill, _ []agent.Agent, _ *state.Store, _ *backup.Store) (int, error) {
		return 2, nil
	})

	var buf bytes.Buffer
	if _, _, err := doctorCommand("doctor", []string{"--json", "--repair"}, &buf); err != nil {
		t.Fatalf("err=%v", err)
	}
	out := strings.TrimSpace(buf.String())
	if !strings.HasPrefix(out, "{") || strings.Count(out, "{") != 1 {
		t.Errorf("--json --repair must emit a single JSON object, got:\n%s", out)
	}
	if !strings.Contains(out, `"drift_detected": true`) || !strings.Contains(out, `"repaired": 2`) {
		t.Errorf("repair JSON missing expected fields:\n%s", out)
	}
}

func TestDoctorRepairCopilotNotFound(t *testing.T) {
	withStubInputs(t, driftInputs())
	withStubSyncInputs(t, syncInputs{hostExitCode: 2})
	withStubRunSync(t, func(_ *copilot.Host, _ []skill.Skill, _ []agent.Agent, _ *state.Store, _ *backup.Store) (int, error) {
		t.Error("RunSync must NOT run when Copilot is not found")
		return 0, nil
	})

	var buf bytes.Buffer
	_, healthy, err := doctorCommand("doctor", []string{"--repair"}, &buf)
	if err != nil {
		t.Fatalf("err=%v", err)
	}
	if healthy {
		t.Error("cannot-repair (no Copilot) must be unhealthy so it exits non-zero")
	}
	if !strings.Contains(buf.String(), "Copilot") {
		t.Errorf("expected a Copilot-not-found note, got:\n%s", buf.String())
	}
}
