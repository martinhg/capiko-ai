package tui

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/martinhg/capiko-ai/internal/backup"
	"github.com/martinhg/capiko-ai/internal/copilot"
	"github.com/martinhg/capiko-ai/internal/state"
)

// installHost builds a Host rooted at fresh temp dirs for skills and agents.
func installHost(t *testing.T) *copilot.Host {
	t.Helper()
	return &copilot.Host{SkillsDir: t.TempDir(), AgentsDir: t.TempDir()}
}

func TestInstallAll_FreshInstall(t *testing.T) {
	host := installHost(t)
	store := state.NewStore(t.TempDir())
	bkp := backup.NewStore(t.TempDir())
	agents := testAgentCatalog()

	res, err := InstallAll(host, testCatalog(), agents, store, bkp)
	if err != nil {
		t.Fatalf("InstallAll: %v", err)
	}

	if len(res.InstalledSkills) != len(testCatalog()) {
		t.Errorf("InstalledSkills = %v, want %d items", res.InstalledSkills, len(testCatalog()))
	}
	if len(res.InstalledAgents) != len(agents) {
		t.Errorf("InstalledAgents = %v, want %d items", res.InstalledAgents, len(agents))
	}
	if len(res.RemovedSkills) != 0 || len(res.RemovedAgents) != 0 {
		t.Errorf("fresh install must not remove anything, got %+v", res)
	}

	for _, sk := range testCatalog() {
		if _, err := os.Stat(filepath.Join(host.SkillsDir, sk.Name, "SKILL.md")); err != nil {
			t.Errorf("%s not written: %v", sk.Name, err)
		}
	}
	for _, a := range agents {
		if _, err := os.Stat(filepath.Join(host.AgentsDir, a.Name+".agent.md")); err != nil {
			t.Errorf("agent %s not written: %v", a.Name, err)
		}
	}

	st, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	for _, sk := range testCatalog() {
		if _, ok := st.Skills[sk.Name]; !ok {
			t.Errorf("%s not recorded in state", sk.Name)
		}
	}
	for _, a := range agents {
		if _, ok := st.Agents[a.Name]; !ok {
			t.Errorf("agent %s not recorded in state", a.Name)
		}
	}
}

func TestInstallAll_PartialInstall_OnlyMissingInstalled(t *testing.T) {
	host := installHost(t)
	store := state.NewStore(t.TempDir())
	bkp := backup.NewStore(t.TempDir())
	cat := testCatalog()
	agents := testAgentCatalog()

	// Pre-install the first skill and first agent directly on disk + state, so
	// InstallAll must treat them as already present.
	already := cat[0]
	if _, err := already.Install(host.SkillsDir); err != nil {
		t.Fatal(err)
	}
	if err := store.Apply(Version, []state.Installed{{Name: already.Name, Checksum: state.Checksum(already.Content)}}, nil); err != nil {
		t.Fatal(err)
	}
	alreadyAgent := agents[0]
	if _, err := alreadyAgent.Install(host.AgentsDir); err != nil {
		t.Fatal(err)
	}
	if err := store.ApplyAgents(Version, []state.Installed{{Name: alreadyAgent.Name, Checksum: state.Checksum(alreadyAgent.CanonicalContent())}}, nil); err != nil {
		t.Fatal(err)
	}

	res, err := InstallAll(host, cat, agents, store, bkp)
	if err != nil {
		t.Fatalf("InstallAll: %v", err)
	}

	if len(res.InstalledSkills) != len(cat)-1 {
		t.Errorf("InstalledSkills = %v, want %d items (excluding already-installed)", res.InstalledSkills, len(cat)-1)
	}
	for _, name := range res.InstalledSkills {
		if name == already.Name {
			t.Errorf("already-installed skill %q must not be re-listed as installed", already.Name)
		}
	}
	if len(res.InstalledAgents) != len(agents)-1 {
		t.Errorf("InstalledAgents = %v, want %d items (excluding already-installed)", res.InstalledAgents, len(agents)-1)
	}
	for _, name := range res.InstalledAgents {
		if name == alreadyAgent.Name {
			t.Errorf("already-installed agent %q must not be re-listed as installed", alreadyAgent.Name)
		}
	}

	// Every catalog item must end up installed on disk regardless.
	for _, sk := range cat {
		if _, err := os.Stat(filepath.Join(host.SkillsDir, sk.Name, "SKILL.md")); err != nil {
			t.Errorf("%s not present on disk: %v", sk.Name, err)
		}
	}
}

func TestInstallAll_EverythingInstalled_NoOp(t *testing.T) {
	host := installHost(t)
	store := state.NewStore(t.TempDir())
	bkpDir := t.TempDir()
	bkp := backup.NewStore(bkpDir)
	cat := testCatalog()
	agents := testAgentCatalog()

	// Pre-install everything.
	if _, err := InstallAll(host, cat, agents, store, bkp); err != nil {
		t.Fatalf("seed InstallAll: %v", err)
	}
	before, err := bkp.List()
	if err != nil {
		t.Fatal(err)
	}

	res, err := InstallAll(host, cat, agents, store, bkp)
	if err != nil {
		t.Fatalf("InstallAll (idempotent run): %v", err)
	}

	if res.TotalChanged() != 0 {
		t.Errorf("TotalChanged() = %d, want 0 when everything already installed", res.TotalChanged())
	}

	after, err := bkp.List()
	if err != nil {
		t.Fatal(err)
	}
	if len(after) != len(before) {
		t.Errorf("no-op install must not create a new backup: before=%d after=%d", len(before), len(after))
	}
}

func TestInstallAll_BackupFailureAborts(t *testing.T) {
	host := installHost(t)
	store := state.NewStore(t.TempDir())

	// Point the backup store's parent at a file (not a dir), so Store.Create's
	// MkdirAll fails before any skill/agent is touched.
	blocker := filepath.Join(t.TempDir(), "blocker")
	if err := os.WriteFile(blocker, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	bkp := backup.NewStore(filepath.Join(blocker, "backups"))

	cat := testCatalog()
	agents := testAgentCatalog()

	res, err := InstallAll(host, cat, agents, store, bkp)
	if err == nil {
		t.Fatal("expected backup failure to abort InstallAll")
	}
	if res.TotalChanged() != 0 {
		t.Errorf("aborted install must return an empty result, got %+v", res)
	}

	for _, sk := range cat {
		if _, statErr := os.Stat(filepath.Join(host.SkillsDir, sk.Name, "SKILL.md")); statErr == nil {
			t.Errorf("%s must not be written when backup fails", sk.Name)
		}
	}
	st, loadErr := store.Load()
	if loadErr != nil {
		t.Fatal(loadErr)
	}
	if len(st.Skills) != 0 || len(st.Agents) != 0 {
		t.Errorf("state must not record anything when backup fails, got %+v", st)
	}
}

func TestInstallAll_SkillInstallErrorFailsFast(t *testing.T) {
	// A SkillsDir nested under a path component that is a regular file makes
	// every MkdirAll inside Skill.Install fail, exercising the fail-fast path.
	blocker := filepath.Join(t.TempDir(), "blocker")
	if err := os.WriteFile(blocker, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	host := &copilot.Host{SkillsDir: filepath.Join(blocker, "skills"), AgentsDir: t.TempDir()}
	store := state.NewStore(t.TempDir())
	bkp := backup.NewStore(t.TempDir())

	cat := testCatalog()
	agents := testAgentCatalog()

	_, err := InstallAll(host, cat, agents, store, bkp)
	if err == nil {
		t.Fatal("expected an error when a skill install fails")
	}
}

func TestInstallAll_NilStoreDegradesGracefully(t *testing.T) {
	host := installHost(t)
	bkp := backup.NewStore(t.TempDir())
	cat := testCatalog()
	agents := testAgentCatalog()

	res, err := InstallAll(host, cat, agents, nil, bkp)
	if err != nil {
		t.Fatalf("InstallAll with nil store: %v", err)
	}
	if len(res.InstalledSkills) != len(cat) {
		t.Errorf("InstalledSkills = %v, want %d items", res.InstalledSkills, len(cat))
	}
	for _, sk := range cat {
		if _, err := os.Stat(filepath.Join(host.SkillsDir, sk.Name, "SKILL.md")); err != nil {
			t.Errorf("%s not written despite nil store: %v", sk.Name, err)
		}
	}
}

func TestInstallAll_NilBackupDegradesGracefully(t *testing.T) {
	host := installHost(t)
	store := state.NewStore(t.TempDir())
	cat := testCatalog()
	agents := testAgentCatalog()

	res, err := InstallAll(host, cat, agents, store, nil)
	if err != nil {
		t.Fatalf("InstallAll with nil backup: %v", err)
	}
	if len(res.InstalledSkills) != len(cat) {
		t.Errorf("InstalledSkills = %v, want %d items", res.InstalledSkills, len(cat))
	}
}

func TestInstallAll_EmptyCatalog(t *testing.T) {
	host := installHost(t)
	store := state.NewStore(t.TempDir())
	bkp := backup.NewStore(t.TempDir())

	res, err := InstallAll(host, nil, nil, store, bkp)
	if err != nil {
		t.Fatalf("InstallAll with empty catalog: %v", err)
	}
	if res.TotalChanged() != 0 {
		t.Errorf("TotalChanged() = %d, want 0 for empty catalog", res.TotalChanged())
	}
}
