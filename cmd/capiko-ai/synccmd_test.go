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
)

// withStubSyncInputs swaps the gather seam for the duration of a test.
func withStubSyncInputs(t *testing.T, in syncInputs) {
	t.Helper()
	prev := gatherSyncInputs
	gatherSyncInputs = func() (syncInputs, error) { return in, nil }
	t.Cleanup(func() { gatherSyncInputs = prev })
}

func withStubSyncInputsErr(t *testing.T, err error) {
	t.Helper()
	prev := gatherSyncInputs
	gatherSyncInputs = func() (syncInputs, error) { return syncInputs{}, err }
	t.Cleanup(func() { gatherSyncInputs = prev })
}

func readySyncInputs(t *testing.T) syncInputs {
	t.Helper()
	return syncInputs{
		host:    &copilot.Host{SkillsDir: t.TempDir(), AgentsDir: t.TempDir()},
		catalog: []skill.Skill{{Name: "capiko-hello", Description: "smoke test", Content: "---\nname: capiko-hello\n---\nx"}},
		agents:  []agent.Agent{{Name: "capiko-sdd-explore", Description: "explore", Content: "---\ndescription: explore\n---\nx"}},
		store:   state.NewStore(t.TempDir()),
		bkp:     backup.NewStore(t.TempDir()),
	}
}

// withStubRunSync replaces the runSync seam and restores it on cleanup.
func withStubRunSync(t *testing.T, fn func(*copilot.Host, []skill.Skill, []agent.Agent, *state.Store, *backup.Store) (int, error)) {
	t.Helper()
	prev := runSync
	runSync = fn
	t.Cleanup(func() { runSync = prev })
}

func TestSyncCommandNotHandledForOtherName(t *testing.T) {
	var buf bytes.Buffer
	handled, exitCode, err := syncCommand("install", nil, &buf)
	if handled || exitCode != 0 || err != nil {
		t.Fatalf("sync should not handle %q (handled=%v exitCode=%d err=%v)", "install", handled, exitCode, err)
	}
}

func TestSyncCommandTextOutput(t *testing.T) {
	withStubSyncInputs(t, readySyncInputs(t))
	var buf bytes.Buffer
	handled, exitCode, err := syncCommand("sync", nil, &buf)
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

func TestSyncCommandJSONOutput(t *testing.T) {
	withStubSyncInputs(t, readySyncInputs(t))
	var buf bytes.Buffer
	handled, exitCode, err := syncCommand("sync", []string{"--json"}, &buf)
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

func TestSyncCommandCopilotNotFound(t *testing.T) {
	withStubSyncInputs(t, syncInputs{hostExitCode: 2})
	var buf bytes.Buffer
	handled, exitCode, err := syncCommand("sync", nil, &buf)
	if !handled || exitCode != 2 {
		t.Fatalf("handled=%v exitCode=%d err=%v, want exitCode=2", handled, exitCode, err)
	}
	if !strings.Contains(buf.String(), "not found") {
		t.Errorf("expected 'not found' message, got:\n%s", buf.String())
	}
}

func TestSyncCommandDetectError(t *testing.T) {
	detectErr := errors.New("no home dir")
	withStubSyncInputs(t, syncInputs{hostExitCode: 1, hostErr: detectErr})
	var buf bytes.Buffer
	handled, exitCode, err := syncCommand("sync", nil, &buf)
	if !handled || exitCode != 1 || err != detectErr {
		t.Fatalf("handled=%v exitCode=%d err=%v, want exitCode=1 err=%v", handled, exitCode, err, detectErr)
	}
}

func TestSyncCommandGatherError(t *testing.T) {
	withStubSyncInputsErr(t, errors.New("loading catalog: boom"))
	var buf bytes.Buffer
	handled, exitCode, err := syncCommand("sync", nil, &buf)
	if !handled || exitCode != 1 || err == nil {
		t.Fatalf("handled=%v exitCode=%d err=%v, want exitCode=1 with error", handled, exitCode, err)
	}
}

func TestSyncCommandRunSyncError(t *testing.T) {
	withStubSyncInputs(t, readySyncInputs(t))
	syncErr := errors.New("backup failed, aborting: mkdir /dev/null/backups: not a directory")
	withStubRunSync(t, func(_ *copilot.Host, _ []skill.Skill, _ []agent.Agent, _ *state.Store, _ *backup.Store) (int, error) {
		return 0, syncErr
	})
	var buf bytes.Buffer
	handled, exitCode, err := syncCommand("sync", nil, &buf)
	if !handled || exitCode != 1 {
		t.Fatalf("handled=%v exitCode=%d err=%v, want exitCode=1", handled, exitCode, err)
	}
	// The error is rendered via headless; main.go fires its stderr branch only
	// when err is non-nil. syncCommand returns (true, 1, nil) per install contract.
	if err != nil {
		t.Errorf("syncCommand should return nil err (rendered in output), got: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "backup failed") && !strings.Contains(out, "error") {
		t.Errorf("expected rendered error in output, got:\n%s", out)
	}
}

func TestSyncCommandRejectsUnknownFlag(t *testing.T) {
	withStubSyncInputs(t, readySyncInputs(t))
	var buf bytes.Buffer
	handled, exitCode, err := syncCommand("sync", []string{"--nope"}, &buf)
	if !handled || exitCode != 1 || err == nil {
		t.Error("an unknown flag should error with exitCode=1")
	}
}

func TestSyncCommandAutoRepairNoDrift(t *testing.T) {
	// Genuine no-drift baseline: seed state so EVERY catalog skill and agent is
	// recorded with its current catalog checksum (StaleEngram is false because
	// Engram is unset). Only then do all three drift checks report clean, so
	// --auto-repair must short-circuit WITHOUT calling RunSync.
	in := readySyncInputs(t)
	skillRecs := make([]state.Installed, len(in.catalog))
	for i, sk := range in.catalog {
		skillRecs[i] = state.Installed{Name: sk.Name, Checksum: state.Checksum(sk.CanonicalContent())}
	}
	if err := in.store.Apply("1.0.0", skillRecs, nil); err != nil {
		t.Fatalf("seeding skills: %v", err)
	}
	agentRecs := make([]state.Installed, len(in.agents))
	for i, a := range in.agents {
		agentRecs[i] = state.Installed{Name: a.Name, Checksum: state.Checksum(a.CanonicalContent())}
	}
	if err := in.store.ApplyAgents("1.0.0", agentRecs, nil); err != nil {
		t.Fatalf("seeding agents: %v", err)
	}
	withStubSyncInputs(t, in)

	called := false
	withStubRunSync(t, func(_ *copilot.Host, _ []skill.Skill, _ []agent.Agent, _ *state.Store, _ *backup.Store) (int, error) {
		called = true
		return 0, nil
	})

	var buf bytes.Buffer
	handled, exitCode, err := syncCommand("sync", []string{"--auto-repair"}, &buf)
	if !handled || exitCode != 0 || err != nil {
		t.Fatalf("handled=%v exitCode=%d err=%v", handled, exitCode, err)
	}
	if called {
		t.Error("RunSync must NOT be called when --auto-repair confirms no drift")
	}
	out := buf.String()
	if !strings.Contains(out, "No drift") {
		t.Errorf("expected 'No drift' message, got:\n%s", out)
	}
}

func TestSyncCommandAutoRepairWithDrift(t *testing.T) {
	// Seed the store with a stale checksum so drift.Stale returns items.
	in := readySyncInputs(t)
	// Write a record with a bad checksum so the skill appears stale.
	if err := in.store.Apply("0.0.0", []state.Installed{{Name: "capiko-hello", Checksum: "stale"}}, nil); err != nil {
		t.Fatalf("seeding state: %v", err)
	}
	withStubSyncInputs(t, in)

	called := false
	withStubRunSync(t, func(_ *copilot.Host, _ []skill.Skill, _ []agent.Agent, _ *state.Store, _ *backup.Store) (int, error) {
		called = true
		return 2, nil
	})

	var buf bytes.Buffer
	handled, exitCode, err := syncCommand("sync", []string{"--auto-repair"}, &buf)
	if !handled || exitCode != 0 || err != nil {
		t.Fatalf("handled=%v exitCode=%d err=%v", handled, exitCode, err)
	}
	if !called {
		t.Error("RunSync MUST be called when --auto-repair detects drift")
	}
	// The post-RunSync success render must list the synced items, not be empty.
	out := buf.String()
	if !strings.Contains(out, "capiko-hello") || !strings.Contains(out, "capiko-sdd-explore") {
		t.Errorf("expected synced item names in output after repair, got:\n%s", out)
	}
}

func TestSyncCommandAutoRepairNilStore(t *testing.T) {
	// A nil store means DefaultStore() failed (home dir undetectable) — NOT a
	// clean state. Drift is indeterminable, so --auto-repair must bias toward
	// repairing and call RunSync (consistent with an unconditional sync), rather
	// than silently exiting 0 with no repair.
	in := readySyncInputs(t)
	in.store = nil
	withStubSyncInputs(t, in)

	called := false
	withStubRunSync(t, func(_ *copilot.Host, _ []skill.Skill, _ []agent.Agent, _ *state.Store, _ *backup.Store) (int, error) {
		called = true
		return 0, nil
	})

	var buf bytes.Buffer
	handled, exitCode, err := syncCommand("sync", []string{"--auto-repair"}, &buf)
	if !handled || exitCode != 0 || err != nil {
		t.Fatalf("handled=%v exitCode=%d err=%v", handled, exitCode, err)
	}
	if !called {
		t.Error("RunSync MUST be called with nil store: drift is indeterminable, bias toward repair")
	}
}

// withStubEngramAdvisory swaps the engram advisory seam for the duration of a test.
func withStubEngramAdvisory(t *testing.T, advisory string) {
	t.Helper()
	prev := engramAdvisory
	engramAdvisory = func(_ *state.Store) string { return advisory }
	t.Cleanup(func() { engramAdvisory = prev })
}

func TestSyncCommandSurfacesEngramAdvisory(t *testing.T) {
	withStubSyncInputs(t, readySyncInputs(t))
	withStubEngramAdvisory(t, "engram 1.16.3 is behind the recommended 1.17.0")

	var buf bytes.Buffer
	handled, exitCode, err := syncCommand("sync", nil, &buf)
	if !handled || exitCode != 0 || err != nil {
		t.Fatalf("handled=%v exitCode=%d err=%v", handled, exitCode, err)
	}
	if !strings.Contains(buf.String(), "engram 1.16.3 is behind") {
		t.Errorf("expected engram advisory in sync output, got:\n%s", buf.String())
	}
}

func TestSyncCommandNoEngramAdvisoryWhenEmpty(t *testing.T) {
	withStubSyncInputs(t, readySyncInputs(t))
	withStubEngramAdvisory(t, "") // not managed / up to date

	var buf bytes.Buffer
	if _, _, err := syncCommand("sync", []string{"--json"}, &buf); err != nil {
		t.Fatalf("err=%v", err)
	}
	if strings.Contains(buf.String(), "warnings") {
		t.Errorf("no advisory should mean no warnings key:\n%s", buf.String())
	}
}

// Sanity: gatherSyncInputs default var must exist and be callable.
func TestGatherSyncInputsIsAFunctionVar(t *testing.T) {
	if gatherSyncInputs == nil {
		t.Fatal("gatherSyncInputs must be set")
	}
}
