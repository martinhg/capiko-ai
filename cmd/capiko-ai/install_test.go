package main

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"github.com/martinhg/capiko-ai/internal/agent"
	"github.com/martinhg/capiko-ai/internal/backup"
	"github.com/martinhg/capiko-ai/internal/copilot"
	"github.com/martinhg/capiko-ai/internal/skill"
	"github.com/martinhg/capiko-ai/internal/state"
	"github.com/martinhg/capiko-ai/internal/tui"
)

// withStubInstallInputs swaps the gather seam for the duration of a test.
func withStubInstallInputs(t *testing.T, in installInputs) {
	t.Helper()
	prev := gatherInstallInputs
	gatherInstallInputs = func() (installInputs, error) { return in, nil }
	t.Cleanup(func() { gatherInstallInputs = prev })
}

func withStubInstallInputsErr(t *testing.T, err error) {
	t.Helper()
	prev := gatherInstallInputs
	gatherInstallInputs = func() (installInputs, error) { return installInputs{}, err }
	t.Cleanup(func() { gatherInstallInputs = prev })
}

func readyInstallInputs(t *testing.T) installInputs {
	t.Helper()
	return installInputs{
		host:    &copilot.Host{SkillsDir: t.TempDir(), AgentsDir: t.TempDir()},
		catalog: []skill.Skill{{Name: "capiko-hello", Description: "smoke test", Content: "---\nname: capiko-hello\n---\nx"}},
		agents:  []agent.Agent{{Name: "capiko-sdd-explore", Description: "explore", Content: "---\ndescription: explore\n---\nx"}},
		store:   state.NewStore(t.TempDir()),
		bkp:     backup.NewStore(t.TempDir()),
	}
}

func TestInstallCommandNotHandledForOtherName(t *testing.T) {
	var buf bytes.Buffer
	handled, exitCode, err := installCommand("sync", nil, &buf)
	if handled || exitCode != 0 || err != nil {
		t.Fatalf("install should not handle %q (handled=%v exitCode=%d err=%v)", "sync", handled, exitCode, err)
	}
}

func TestInstallCommandTextOutput(t *testing.T) {
	withStubInstallInputs(t, readyInstallInputs(t))
	var buf bytes.Buffer
	handled, exitCode, err := installCommand("install", nil, &buf)
	if !handled || exitCode != 0 || err != nil {
		t.Fatalf("handled=%v exitCode=%d err=%v", handled, exitCode, err)
	}
	out := buf.String()
	if !strings.Contains(out, "capiko-hello") || !strings.Contains(out, "capiko-sdd-explore") {
		t.Errorf("unexpected text output:\n%s", out)
	}
	if !strings.Contains(out, "installed") {
		t.Errorf("text output missing install summary:\n%s", out)
	}
}

func TestInstallCommandJSONOutput(t *testing.T) {
	withStubInstallInputs(t, readyInstallInputs(t))
	var buf bytes.Buffer
	handled, exitCode, err := installCommand("install", []string{"--json"}, &buf)
	if !handled || exitCode != 0 || err != nil {
		t.Fatalf("handled=%v exitCode=%d err=%v", handled, exitCode, err)
	}
	out := buf.String()
	if !strings.HasPrefix(strings.TrimSpace(out), "{") {
		t.Errorf("--json should emit JSON, got:\n%s", out)
	}
	if !strings.Contains(out, `"ok": true`) {
		t.Errorf("--json missing ok:true:\n%s", out)
	}
	if !strings.Contains(out, "capiko-hello") {
		t.Errorf("--json missing installed skill name:\n%s", out)
	}
}

func TestInstallCommandAllFlagParity(t *testing.T) {
	withStubInstallInputs(t, readyInstallInputs(t))
	var buf bytes.Buffer
	handled, exitCode, err := installCommand("install", []string{"--all"}, &buf)
	if !handled || exitCode != 0 || err != nil {
		t.Fatalf("handled=%v exitCode=%d err=%v", handled, exitCode, err)
	}
	if !strings.Contains(buf.String(), "capiko-hello") {
		t.Errorf("--all should behave like bare install:\n%s", buf.String())
	}
}

func TestInstallCommandCopilotNotFound(t *testing.T) {
	withStubInstallInputs(t, installInputs{hostExitCode: 2})
	var buf bytes.Buffer
	handled, exitCode, err := installCommand("install", nil, &buf)
	if !handled || exitCode != 2 {
		t.Fatalf("handled=%v exitCode=%d err=%v, want exitCode=2", handled, exitCode, err)
	}
	if !strings.Contains(buf.String(), "not found") {
		t.Errorf("expected 'not found' message, got:\n%s", buf.String())
	}
}

func TestInstallCommandDetectError(t *testing.T) {
	detectErr := errors.New("no home dir")
	withStubInstallInputs(t, installInputs{hostExitCode: 1, hostErr: detectErr})
	var buf bytes.Buffer
	handled, exitCode, err := installCommand("install", nil, &buf)
	if !handled || exitCode != 1 || err != detectErr {
		t.Fatalf("handled=%v exitCode=%d err=%v, want exitCode=1 err=%v", handled, exitCode, err, detectErr)
	}
}

func TestInstallCommandGatherError(t *testing.T) {
	withStubInstallInputsErr(t, errors.New("loading catalog: boom"))
	var buf bytes.Buffer
	handled, exitCode, err := installCommand("install", nil, &buf)
	if !handled || exitCode != 1 || err == nil {
		t.Fatalf("handled=%v exitCode=%d err=%v, want exitCode=1 with error", handled, exitCode, err)
	}
}

func TestInstallCommandRejectsUnknownFlag(t *testing.T) {
	withStubInstallInputs(t, readyInstallInputs(t))
	var buf bytes.Buffer
	handled, exitCode, err := installCommand("install", []string{"--nope"}, &buf)
	if !handled || exitCode != 1 || err == nil {
		t.Error("an unknown flag should error with exitCode=1")
	}
}

func TestInstallCommandInstallErrorRendersAndExitsNonZero(t *testing.T) {
	// SkillsDir nested under a file makes every install fail.
	in := readyInstallInputs(t)
	in.host = &copilot.Host{SkillsDir: in.host.SkillsDir + "/\x00impossible", AgentsDir: in.host.AgentsDir}
	withStubInstallInputs(t, in)

	var buf bytes.Buffer
	handled, exitCode, err := installCommand("install", nil, &buf)
	if !handled || exitCode != 1 {
		t.Fatalf("handled=%v exitCode=%d err=%v, want exitCode=1", handled, exitCode, err)
	}
	if !strings.Contains(buf.String(), `"ok": false`) && !strings.Contains(buf.String(), "error") {
		// text mode: just assert something was rendered describing failure
		if !strings.Contains(buf.String(), "capiko-ai install") {
			t.Errorf("expected rendered error output, got:\n%s", buf.String())
		}
	}
}

// Sanity: gatherInstallInputs default var must exist and be callable (smoke,
// not asserting real environment behavior — install_test stubs it everywhere
// else).
func TestGatherInstallInputsIsAFunctionVar(t *testing.T) {
	if gatherInstallInputs == nil {
		t.Fatal("gatherInstallInputs must be set")
	}
}

var _ = tui.ReconcileResult{} // keep tui import if test bodies are trimmed later
