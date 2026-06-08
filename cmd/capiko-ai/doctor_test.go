package main

import (
	"bytes"
	"strings"
	"testing"

	"github.com/martinhg/capiko-ai/internal/copilot"
	"github.com/martinhg/capiko-ai/internal/doctor"
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
