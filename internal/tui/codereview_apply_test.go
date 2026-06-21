package tui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/martinhg/capiko-ai/internal/backup"
	"github.com/martinhg/capiko-ai/internal/codereview"
	"github.com/martinhg/capiko-ai/internal/state"
)

// stubGGAHooks swaps the gga install/uninstall seams so apply tests never shell
// out to a real gga binary or touch a real git repo. It returns pointers the test
// can read to assert which hook ran.
func stubGGAHooks(t *testing.T) (installed, uninstalled *bool) {
	t.Helper()
	var didInstall, didUninstall bool
	prevInstall, prevUninstall := ggaInstallHook, ggaUninstallHook
	ggaInstallHook = func(string) error { didInstall = true; return nil }
	ggaUninstallHook = func(string) error { didUninstall = true; return nil }
	t.Cleanup(func() { ggaInstallHook, ggaUninstallHook = prevInstall, prevUninstall })
	return &didInstall, &didUninstall
}

func TestApplyCodeReviewWritesConfigRulesAndHook(t *testing.T) {
	installed, _ := stubGGAHooks(t)
	ws := t.TempDir()
	store := state.NewStore(t.TempDir())
	rec := &state.CodeReviewRecord{Enabled: true, Provider: "claude", StrictMode: true}

	if err := applyCodeReview(ws, store, nil, rec); err != nil {
		t.Fatalf("applyCodeReview: %v", err)
	}

	gga, err := os.ReadFile(filepath.Join(ws, ".gga"))
	if err != nil {
		t.Fatalf(".gga not written: %v", err)
	}
	if !strings.Contains(string(gga), `PROVIDER="claude"`) {
		t.Errorf(".gga missing provider:\n%s", gga)
	}

	agents, err := os.ReadFile(filepath.Join(ws, "AGENTS.md"))
	if err != nil {
		t.Fatalf("AGENTS.md not written: %v", err)
	}
	if !strings.Contains(string(agents), codereview.MarkerStart) || !strings.Contains(string(agents), "REJECT") {
		t.Errorf("AGENTS.md missing the managed rules block:\n%s", agents)
	}

	if !*installed {
		t.Error("apply should install the gga git hook")
	}

	st, _ := store.Load()
	if st.CodeReview == nil || !st.CodeReview.Enabled {
		t.Error("apply should record the enabled code-review state")
	}
}

func TestApplyCodeReviewIncludesActivePersona(t *testing.T) {
	stubGGAHooks(t)
	ws := t.TempDir()
	store := state.NewStore(t.TempDir())
	if err := store.SetPersona("capiko"); err != nil {
		t.Fatal(err)
	}

	if err := applyCodeReview(ws, store, nil, &state.CodeReviewRecord{Enabled: true}); err != nil {
		t.Fatalf("applyCodeReview: %v", err)
	}

	agents, _ := os.ReadFile(filepath.Join(ws, "AGENTS.md"))
	if !strings.Contains(string(agents), "capiko") {
		t.Errorf("AGENTS.md should reference the active persona:\n%s", agents)
	}
}

func TestApplyCodeReviewPreservesUserRules(t *testing.T) {
	stubGGAHooks(t)
	ws := t.TempDir()
	rulesPath := filepath.Join(ws, "AGENTS.md")
	if err := os.WriteFile(rulesPath, []byte("# My own rules\n\n- REQUIRE: keep this line\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := applyCodeReview(ws, state.NewStore(t.TempDir()), nil, &state.CodeReviewRecord{Enabled: true}); err != nil {
		t.Fatalf("applyCodeReview: %v", err)
	}

	agents, _ := os.ReadFile(rulesPath)
	if !strings.Contains(string(agents), "keep this line") {
		t.Errorf("apply must not clobber user-authored rules:\n%s", agents)
	}
	if !strings.Contains(string(agents), codereview.MarkerStart) {
		t.Errorf("apply should inject the managed block alongside user rules:\n%s", agents)
	}
}

func TestApplyCodeReviewDisableRemovesBlockAndHook(t *testing.T) {
	_, uninstalled := stubGGAHooks(t)
	ws := t.TempDir()
	store := state.NewStore(t.TempDir())

	// Enable first so there is a managed block to remove.
	if err := applyCodeReview(ws, store, nil, &state.CodeReviewRecord{Enabled: true}); err != nil {
		t.Fatalf("enable: %v", err)
	}
	if err := applyCodeReview(ws, store, nil, &state.CodeReviewRecord{Enabled: false}); err != nil {
		t.Fatalf("disable: %v", err)
	}

	agents, _ := os.ReadFile(filepath.Join(ws, "AGENTS.md"))
	if strings.Contains(string(agents), codereview.MarkerStart) {
		t.Errorf("disable should remove capiko's managed block:\n%s", agents)
	}
	if !*uninstalled {
		t.Error("disable should uninstall the gga git hook")
	}
	st, _ := store.Load()
	if st.CodeReview == nil || st.CodeReview.Enabled {
		t.Error("disable should record code review as off")
	}
}

func TestApplyCodeReviewBacksUpExistingFiles(t *testing.T) {
	stubGGAHooks(t)
	ws := t.TempDir()
	if err := os.WriteFile(filepath.Join(ws, ".gga"), []byte("PROVIDER=\"old\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	bkp := backup.NewStore(t.TempDir())

	if err := applyCodeReview(ws, state.NewStore(t.TempDir()), bkp, &state.CodeReviewRecord{Enabled: true}); err != nil {
		t.Fatalf("applyCodeReview: %v", err)
	}

	manifests, err := bkp.List()
	if err != nil {
		t.Fatal(err)
	}
	if len(manifests) == 0 {
		t.Error("apply should snapshot existing files before overwriting")
	}
}
