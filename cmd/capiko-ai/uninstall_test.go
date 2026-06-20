package main

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"github.com/martinhg/capiko-ai/internal/backup"
	"github.com/martinhg/capiko-ai/internal/copilot"
	"github.com/martinhg/capiko-ai/internal/state"
	"github.com/martinhg/capiko-ai/internal/tui"
)

// withStubUninstallInputs swaps the gather seam for the duration of a test.
func withStubUninstallInputs(t *testing.T, in uninstallInputs) {
	t.Helper()
	prev := gatherUninstallInputs
	gatherUninstallInputs = func() (uninstallInputs, error) { return in, nil }
	t.Cleanup(func() { gatherUninstallInputs = prev })
}

func withStubUninstallInputsErr(t *testing.T, err error) {
	t.Helper()
	prev := gatherUninstallInputs
	gatherUninstallInputs = func() (uninstallInputs, error) { return uninstallInputs{}, err }
	t.Cleanup(func() { gatherUninstallInputs = prev })
}

// readyUninstallInputs returns a seeded uninstall environment with one skill
// and one agent already installed, so uninstallCommand has something to remove.
func readyUninstallInputs(t *testing.T) uninstallInputs {
	t.Helper()
	host := &copilot.Host{SkillsDir: t.TempDir(), AgentsDir: t.TempDir()}
	store := state.NewStore(t.TempDir())
	bkp := backup.NewStore(t.TempDir())

	// Seed state directly so UninstallAll finds managed items to remove.
	if err := store.Apply(tui.Version, []state.Installed{{Name: "capiko-hello", Checksum: "x"}}, nil); err != nil {
		t.Fatal(err)
	}
	if err := store.ApplyAgents(tui.Version, []state.Installed{{Name: "capiko-sdd-explore", Checksum: "x"}}, nil); err != nil {
		t.Fatal(err)
	}
	return uninstallInputs{
		host:  host,
		store: store,
		bkp:   bkp,
	}
}

func TestUninstallCommandNotHandledForOtherName(t *testing.T) {
	var buf bytes.Buffer
	handled, exitCode, err := uninstallCommand("sync", nil, &buf)
	if handled || exitCode != 0 || err != nil {
		t.Fatalf("uninstall should not handle %q (handled=%v exitCode=%d err=%v)", "sync", handled, exitCode, err)
	}
}

func TestUninstallCommandTextOutput(t *testing.T) {
	withStubUninstallInputs(t, readyUninstallInputs(t))
	var buf bytes.Buffer
	handled, exitCode, err := uninstallCommand("uninstall", nil, &buf)
	if !handled || exitCode != 0 || err != nil {
		t.Fatalf("handled=%v exitCode=%d err=%v", handled, exitCode, err)
	}
	out := buf.String()
	// Both facts are required: the removal action AND the item name. Using && here
	// would let the test pass if one silently regressed.
	if !strings.Contains(out, "removed") || !strings.Contains(out, "capiko-hello") {
		t.Errorf("text output missing uninstall summary (need both 'removed' and item name):\n%s", out)
	}
}

func TestUninstallCommandJSONOutput(t *testing.T) {
	withStubUninstallInputs(t, readyUninstallInputs(t))
	var buf bytes.Buffer
	handled, exitCode, err := uninstallCommand("uninstall", []string{"--json"}, &buf)
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
}

func TestUninstallCommandAllFlagParity(t *testing.T) {
	withStubUninstallInputs(t, readyUninstallInputs(t))
	var buf bytes.Buffer
	handled, exitCode, err := uninstallCommand("uninstall", []string{"--all"}, &buf)
	if !handled || exitCode != 0 || err != nil {
		t.Fatalf("handled=%v exitCode=%d err=%v", handled, exitCode, err)
	}
	// --all behaves identically to bare uninstall.
	out := buf.String()
	if !strings.Contains(out, "capiko-ai uninstall") {
		t.Errorf("--all should produce same output as bare uninstall:\n%s", out)
	}
}

func TestUninstallCommandCopilotNotFound(t *testing.T) {
	withStubUninstallInputs(t, uninstallInputs{hostExitCode: 2})
	var buf bytes.Buffer
	handled, exitCode, err := uninstallCommand("uninstall", nil, &buf)
	if !handled || exitCode != 2 {
		t.Fatalf("handled=%v exitCode=%d err=%v, want exitCode=2", handled, exitCode, err)
	}
	if !strings.Contains(buf.String(), "not found") {
		t.Errorf("expected 'not found' message, got:\n%s", buf.String())
	}
}

func TestUninstallCommandDetectError(t *testing.T) {
	detectErr := errors.New("no home dir")
	withStubUninstallInputs(t, uninstallInputs{hostExitCode: 1, hostErr: detectErr})
	var buf bytes.Buffer
	handled, exitCode, err := uninstallCommand("uninstall", nil, &buf)
	if !handled || exitCode != 1 || err != detectErr {
		t.Fatalf("handled=%v exitCode=%d err=%v, want exitCode=1 err=%v", handled, exitCode, err, detectErr)
	}
}

func TestUninstallCommandGatherError(t *testing.T) {
	withStubUninstallInputsErr(t, errors.New("state failed: boom"))
	var buf bytes.Buffer
	handled, exitCode, err := uninstallCommand("uninstall", nil, &buf)
	if !handled || exitCode != 1 || err == nil {
		t.Fatalf("handled=%v exitCode=%d err=%v, want exitCode=1 with error", handled, exitCode, err)
	}
}

func TestUninstallCommandRejectsUnknownFlag(t *testing.T) {
	withStubUninstallInputs(t, readyUninstallInputs(t))
	var buf bytes.Buffer
	handled, exitCode, err := uninstallCommand("uninstall", []string{"--nope"}, &buf)
	if !handled || exitCode != 1 || err == nil {
		t.Error("an unknown flag should error with exitCode=1")
	}
}

func TestUninstallCommandNothingInstalledNoOp(t *testing.T) {
	// Empty state: no skills, no agents recorded.
	host := &copilot.Host{SkillsDir: t.TempDir(), AgentsDir: t.TempDir()}
	store := state.NewStore(t.TempDir())
	bkp := backup.NewStore(t.TempDir())
	withStubUninstallInputs(t, uninstallInputs{
		host:  host,
		store: store,
		bkp:   bkp,
	})

	before, err := bkp.List()
	if err != nil {
		t.Fatal(err)
	}

	var buf bytes.Buffer
	handled, exitCode, err := uninstallCommand("uninstall", nil, &buf)
	if !handled || exitCode != 0 || err != nil {
		t.Fatalf("handled=%v exitCode=%d err=%v, want exit 0 on empty state", handled, exitCode, err)
	}
	out := buf.String()
	if !strings.Contains(out, "Nothing to do") {
		t.Errorf("expected 'Nothing to do' message for empty state, got:\n%s", out)
	}

	after, err := bkp.List()
	if err != nil {
		t.Fatal(err)
	}
	if len(after) != len(before) {
		t.Errorf("no-op uninstall must not create a backup: before=%d after=%d", len(before), len(after))
	}
}

// Sanity: gatherUninstallInputs default var must exist and be callable.
func TestGatherUninstallInputsIsAFunctionVar(t *testing.T) {
	if gatherUninstallInputs == nil {
		t.Fatal("gatherUninstallInputs must be set")
	}
}

var _ = tui.ReconcileResult{} // keep tui import alive if test bodies are trimmed later
