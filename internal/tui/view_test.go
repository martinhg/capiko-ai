package tui

import (
	"errors"
	"flag"
	"os"
	"path/filepath"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"

	"github.com/martinhg/capiko-ai/internal/copilot"
)

var updateGolden = flag.Bool("update", false, "update golden files")

// TestMain forces a plain ASCII color profile so rendered views are
// deterministic (no ANSI escapes) and golden files stay stable across
// terminals and CI.
func TestMain(m *testing.M) {
	lipgloss.SetColorProfile(termenv.Ascii)
	os.Exit(m.Run())
}

func golden(t *testing.T, name, got string) {
	t.Helper()
	path := filepath.Join("testdata", name+".golden")
	if *updateGolden {
		if err := os.MkdirAll("testdata", 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(got), 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read golden %q: %v (run: go test ./internal/tui -update)", name, err)
	}
	if got != string(want) {
		t.Errorf("%s view mismatch\n--- got ---\n%s\n--- want ---\n%s", name, got, want)
	}
}

func TestViewGolden(t *testing.T) {
	fixedHost := &copilot.Host{SkillsDir: "/home/user/.copilot/skills"}

	svc := services{host: fixedHost}

	installPicking := newInstall(svc, testCatalog(), map[string]bool{"capiko-hello": true})

	installDone := newInstall(svc, testCatalog(), map[string]bool{}).(*selector)
	installDone.state = selDone
	installDone.result = reconcileResult{
		installed: []string{"capiko-conventions"},
		removed:   []string{"capiko-hello"},
	}

	uninstallEmpty := newUninstall(svc, testCatalog(), map[string]bool{})

	cases := []struct {
		name string
		view string
	}{
		{"detecting", App{state: appDetecting}.View()},
		{"notfound", App{state: appNotFound}.View()},
		{"failed", App{state: appFailed, err: errors.New("boom")}.View()},
		{"menu", App{state: appMenu, catalog: testCatalog()}.View()},
		{"menu_update", App{state: appMenu, catalog: testCatalog(), latest: "0.2.0"}.View()},
		{"install_picking", App{state: appScreen, active: installPicking}.View()},
		{"install_done", App{state: appScreen, active: installDone}.View()},
		{"uninstall_empty", App{state: appScreen, active: uninstallEmpty}.View()},
		{"sync_confirm", App{state: appScreen, active: newSync(svc, testCatalog())}.View()},
		{"backups_empty", App{state: appScreen, active: newBackups(svc)}.View()},
		{"soon", App{state: appScreen, active: newSoon("Upgrade tools")}.View()},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			golden(t, tc.name, tc.view)
		})
	}
}
