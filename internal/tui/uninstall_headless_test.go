package tui

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/martinhg/capiko-ai/internal/backup"
	"github.com/martinhg/capiko-ai/internal/copilot"
	"github.com/martinhg/capiko-ai/internal/state"
)

// uninstallHost builds a Host rooted at fresh temp dirs for skills and agents.
func uninstallHost(t *testing.T) *copilot.Host {
	t.Helper()
	return &copilot.Host{SkillsDir: t.TempDir(), AgentsDir: t.TempDir()}
}

// seedInstalled writes a skill dir + SKILL.md and an agent file to a host,
// and records them in state, so UninstallAll sees managed items to remove.
func seedInstalled(t *testing.T, host *copilot.Host, store *state.Store) {
	t.Helper()
	cat := testCatalog()
	agents := testAgentCatalog()

	// Write skills to disk.
	for _, sk := range cat {
		dir := filepath.Join(host.SkillsDir, sk.Name)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte(sk.Content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	// Write agents to disk.
	for _, a := range agents {
		if err := os.MkdirAll(host.AgentsDir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(host.AgentsDir, a.Name+".agent.md"), []byte(a.Content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	// Record in state.
	if store != nil {
		recs := make([]state.Installed, len(cat))
		for i, sk := range cat {
			recs[i] = state.Installed{Name: sk.Name, Checksum: state.Checksum(sk.CanonicalContent())}
		}
		if err := store.Apply(Version, recs, nil); err != nil {
			t.Fatal(err)
		}
		agentRecs := make([]state.Installed, len(agents))
		for i, a := range agents {
			agentRecs[i] = state.Installed{Name: a.Name, Checksum: state.Checksum(a.CanonicalContent())}
		}
		if err := store.ApplyAgents(Version, agentRecs, nil); err != nil {
			t.Fatal(err)
		}
	}
}

func TestUninstallAll_FullUninstall(t *testing.T) {
	host := uninstallHost(t)
	store := state.NewStore(t.TempDir())
	bkp := backup.NewStore(t.TempDir())
	seedInstalled(t, host, store)

	res, err := UninstallAll(host, store, bkp)
	if err != nil {
		t.Fatalf("UninstallAll: %v", err)
	}

	cat := testCatalog()
	agents := testAgentCatalog()
	if len(res.RemovedSkills) != len(cat) {
		t.Errorf("RemovedSkills = %v, want %d items", res.RemovedSkills, len(cat))
	}
	if len(res.RemovedAgents) != len(agents) {
		t.Errorf("RemovedAgents = %v, want %d items", res.RemovedAgents, len(agents))
	}
	if len(res.InstalledSkills) != 0 || len(res.InstalledAgents) != 0 {
		t.Errorf("uninstall must not report installed items, got %+v", res)
	}

	// Skills must be gone from disk.
	for _, sk := range cat {
		if _, err := os.Stat(filepath.Join(host.SkillsDir, sk.Name)); !os.IsNotExist(err) {
			t.Errorf("skill %s must be removed from disk", sk.Name)
		}
	}
	// Agents must be gone from disk.
	for _, a := range agents {
		if _, err := os.Stat(filepath.Join(host.AgentsDir, a.Name+".agent.md")); !os.IsNotExist(err) {
			t.Errorf("agent %s must be removed from disk", a.Name)
		}
	}

	// State must have cleared Skills and Agents.
	st, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(st.Skills) != 0 {
		t.Errorf("state.Skills must be empty after full uninstall, got %+v", st.Skills)
	}
	if len(st.Agents) != 0 {
		t.Errorf("state.Agents must be empty after full uninstall, got %+v", st.Agents)
	}
}

func TestUninstallAll_NothingToRemove(t *testing.T) {
	host := uninstallHost(t)
	store := state.NewStore(t.TempDir())
	bkpDir := t.TempDir()
	bkp := backup.NewStore(bkpDir)

	// State is empty (nothing recorded as installed).
	before, err := bkp.List()
	if err != nil {
		t.Fatal(err)
	}

	res, err := UninstallAll(host, store, bkp)
	if err != nil {
		t.Fatalf("UninstallAll on empty state: %v", err)
	}
	if res.TotalChanged() != 0 {
		t.Errorf("TotalChanged() = %d, want 0 when nothing to remove", res.TotalChanged())
	}

	after, err := bkp.List()
	if err != nil {
		t.Fatal(err)
	}
	if len(after) != len(before) {
		t.Errorf("nothing-to-remove must not create a backup: before=%d after=%d", len(before), len(after))
	}
}

func TestUninstallAll_BackupFailureAbortsBeforeMutation(t *testing.T) {
	host := uninstallHost(t)
	store := state.NewStore(t.TempDir())
	seedInstalled(t, host, store)

	// Point backup at a path under a regular file (MkdirAll will fail).
	blocker := filepath.Join(t.TempDir(), "blocker")
	if err := os.WriteFile(blocker, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	bkp := backup.NewStore(filepath.Join(blocker, "backups"))

	res, err := UninstallAll(host, store, bkp)
	if err == nil {
		t.Fatal("expected backup failure to abort UninstallAll")
	}
	if res.TotalChanged() != 0 {
		t.Errorf("aborted uninstall must return empty result, got %+v", res)
	}

	// Skills must still be on disk (no mutation attempted).
	for _, sk := range testCatalog() {
		if _, statErr := os.Stat(filepath.Join(host.SkillsDir, sk.Name, "SKILL.md")); statErr != nil {
			t.Errorf("skill %s must still exist when backup fails: %v", sk.Name, statErr)
		}
	}
}

func TestUninstallAll_SkillErrorFailsFast(t *testing.T) {
	// Make SkillsDir point somewhere that causes removal to fail. We seed a
	// skill record in state but make the SkillsDir point at a non-existent path
	// under a regular file, so UninstallSkill returns an error.
	host := uninstallHost(t)
	store := state.NewStore(t.TempDir())
	bkp := backup.NewStore(t.TempDir())

	// Record a skill in state without writing it to a normal location.
	sk := testCatalog()[0]
	if err := store.Apply(Version, []state.Installed{{Name: sk.Name, Checksum: "x"}}, nil); err != nil {
		t.Fatal(err)
	}

	// Overwrite the skills dir with a regular file so RemoveAll-style paths fail.
	// Actually UninstallSkill returns nil when the target does not exist (idempotent),
	// so we need to write a directory that is NOT a skill (no SKILL.md) to trigger
	// the refusal guard. Create a directory without SKILL.md.
	dir := filepath.Join(host.SkillsDir, sk.Name)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	// No SKILL.md written → UninstallSkill refuses.

	_, err := UninstallAll(host, store, bkp)
	if err == nil {
		t.Fatal("expected an error when UninstallSkill fails")
	}
}

func TestUninstallAll_AgentErrorFailsFast(t *testing.T) {
	host := uninstallHost(t)
	store := state.NewStore(t.TempDir())
	bkp := backup.NewStore(t.TempDir())

	// Record only an agent in state (no skills).
	a := testAgentCatalog()[0]
	if err := store.ApplyAgents(Version, []state.Installed{{Name: a.Name, Checksum: "x"}}, nil); err != nil {
		t.Fatal(err)
	}

	// Write a DIRECTORY instead of the expected agent file so UninstallAgent
	// returns an error ("is not an agent file").
	target := filepath.Join(host.AgentsDir, a.Name+".agent.md")
	if err := os.MkdirAll(target, 0o755); err != nil {
		t.Fatal(err)
	}

	_, err := UninstallAll(host, store, bkp)
	if err == nil {
		t.Fatal("expected an error when UninstallAgent fails")
	}
}

func TestUninstallAll_NilStore_FallsBackToDiskScan(t *testing.T) {
	host := uninstallHost(t)
	bkp := backup.NewStore(t.TempDir())

	// Write skills and agents directly to disk (no state store).
	cat := testCatalog()
	agents := testAgentCatalog()
	for _, sk := range cat {
		dir := filepath.Join(host.SkillsDir, sk.Name)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte(sk.Content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	for _, a := range agents {
		if err := os.MkdirAll(host.AgentsDir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(host.AgentsDir, a.Name+".agent.md"), []byte(a.Content), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	res, err := UninstallAll(host, nil, bkp)
	if err != nil {
		t.Fatalf("UninstallAll with nil store: %v", err)
	}
	if len(res.RemovedSkills) != len(cat) {
		t.Errorf("RemovedSkills = %v, want %d items (disk scan)", res.RemovedSkills, len(cat))
	}
	if len(res.RemovedAgents) != len(agents) {
		t.Errorf("RemovedAgents = %v, want %d items (disk scan)", res.RemovedAgents, len(agents))
	}

	// Everything must be gone from disk.
	for _, sk := range cat {
		if _, err := os.Stat(filepath.Join(host.SkillsDir, sk.Name)); !os.IsNotExist(err) {
			t.Errorf("skill %s must be removed when nil store falls back to disk", sk.Name)
		}
	}
}

func TestUninstallAll_NilBackup_DegradesGracefully(t *testing.T) {
	host := uninstallHost(t)
	store := state.NewStore(t.TempDir())
	seedInstalled(t, host, store)

	res, err := UninstallAll(host, store, nil)
	if err != nil {
		t.Fatalf("UninstallAll with nil backup: %v", err)
	}
	if len(res.RemovedSkills) != len(testCatalog()) {
		t.Errorf("RemovedSkills = %v, want %d items", res.RemovedSkills, len(testCatalog()))
	}
}

func TestUninstallAll_ScopeGuard_PersonaSDDEngram_Untouched(t *testing.T) {
	// After UninstallAll, state fields that represent persona, SDD models,
	// and the engram MCP entry must be identical to what they were before the
	// call. UninstallAll only calls UninstallSkill/UninstallAgent and
	// store.Apply/store.ApplyAgents, so these fields are never written — but
	// this test makes that invariant explicit and detectable.
	host := uninstallHost(t)
	store := state.NewStore(t.TempDir())
	bkp := backup.NewStore(t.TempDir())
	seedInstalled(t, host, store)

	// Seed non-skill state fields to confirm they survive uninstall.
	if err := store.SetPersona("rioplatense"); err != nil {
		t.Fatal(err)
	}
	if err := store.SetSDDModels(map[string]string{"sdd-apply": "sonnet", "sdd-verify": "opus"}); err != nil {
		t.Fatal(err)
	}
	if err := store.SetEngram(&state.EngramRecord{Enabled: true, CloudServer: "https://engram.example.com"}); err != nil {
		t.Fatal(err)
	}

	// Load the before-snapshot.
	before, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}

	_, err = UninstallAll(host, store, bkp)
	if err != nil {
		t.Fatalf("UninstallAll: %v", err)
	}

	after, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}

	if after.Persona != before.Persona {
		t.Errorf("Persona changed: before=%q after=%q", before.Persona, after.Persona)
	}
	if len(after.SDDModels) != len(before.SDDModels) {
		t.Errorf("SDDModels changed: before=%v after=%v", before.SDDModels, after.SDDModels)
	}
	for k, v := range before.SDDModels {
		if after.SDDModels[k] != v {
			t.Errorf("SDDModels[%q] changed: before=%q after=%q", k, v, after.SDDModels[k])
		}
	}
	if before.Engram == nil && after.Engram != nil {
		t.Error("Engram must remain nil if it was nil before uninstall")
	}
	if before.Engram != nil {
		if after.Engram == nil {
			t.Error("Engram was set before uninstall but is nil after")
		} else {
			if after.Engram.Enabled != before.Engram.Enabled {
				t.Errorf("Engram.Enabled changed: before=%v after=%v", before.Engram.Enabled, after.Engram.Enabled)
			}
			if after.Engram.CloudServer != before.Engram.CloudServer {
				t.Errorf("Engram.CloudServer changed: before=%q after=%q", before.Engram.CloudServer, after.Engram.CloudServer)
			}
		}
	}
}
