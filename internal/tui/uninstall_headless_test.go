package tui

import (
	"os"
	"path/filepath"
	"sort"
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

	// Fail-fast must not have recorded a phantom removal: the skill that could
	// not be removed must still be present in state (state matches disk).
	st, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := st.Skills[sk.Name]; !ok {
		t.Errorf("skill %s failed to uninstall but was cleared from state — divergence", sk.Name)
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

	// Fail-fast must not have recorded a phantom removal: the agent that could
	// not be removed must still be present in state.
	st, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := st.Agents[a.Name]; !ok {
		t.Errorf("agent %s failed to uninstall but was cleared from state — divergence", a.Name)
	}
}

func TestUninstallAll_NilStore_Refuses(t *testing.T) {
	// A nil store is a failure condition (DefaultStore() could not resolve a home
	// dir), NOT a request to wipe the disk. Because uninstall is destructive and
	// capiko cannot tell its own items from user-authored ones without state, it
	// must REFUSE — and must not delete anything on disk.
	host := uninstallHost(t)
	bkp := backup.NewStore(t.TempDir())

	// Write skills and agents directly to disk to prove they survive the refusal.
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
	if err == nil {
		t.Fatal("UninstallAll with a nil store must refuse, not delete everything on disk")
	}
	if res.TotalChanged() != 0 {
		t.Errorf("refused uninstall must report no changes, got %+v", res)
	}

	// Nothing on disk may have been touched.
	for _, sk := range cat {
		if _, statErr := os.Stat(filepath.Join(host.SkillsDir, sk.Name, "SKILL.md")); statErr != nil {
			t.Errorf("skill %s must survive a refused (nil-store) uninstall: %v", sk.Name, statErr)
		}
	}
	for _, a := range agents {
		if _, statErr := os.Stat(filepath.Join(host.AgentsDir, a.Name+".agent.md")); statErr != nil {
			t.Errorf("agent %s must survive a refused (nil-store) uninstall: %v", a.Name, statErr)
		}
	}
}

func TestUninstallAll_PartialFailureKeepsStateConsistent(t *testing.T) {
	// When a removal fails mid-loop, the items already removed from disk MUST be
	// flushed to state before the error returns — state can never claim an item
	// is installed after it has been deleted from disk.
	host := uninstallHost(t)
	store := state.NewStore(t.TempDir())
	bkp := backup.NewStore(t.TempDir())
	seedInstalled(t, host, store)

	cat := testCatalog()
	if len(cat) < 2 {
		t.Skip("need >=2 skills to exercise a mid-loop partial failure")
	}

	// UninstallAll processes skills in sorted order. Sabotage the LAST one so the
	// earlier skills are removed (and recorded) before the failure fires.
	names := make([]string, len(cat))
	for i, sk := range cat {
		names[i] = sk.Name
	}
	sort.Strings(names)
	bad := names[len(names)-1]

	badDir := filepath.Join(host.SkillsDir, bad)
	if err := os.RemoveAll(badDir); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(badDir, 0o755); err != nil { // dir without SKILL.md → UninstallSkill refuses
		t.Fatal(err)
	}

	_, err := UninstallAll(host, store, bkp)
	if err == nil {
		t.Fatal("expected a mid-loop failure on the sabotaged skill")
	}

	st, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}

	// Invariant: no skill may be gone from disk yet still present in state.
	removedAtLeastOne := false
	for _, sk := range cat {
		_, statErr := os.Stat(filepath.Join(host.SkillsDir, sk.Name))
		goneFromDisk := os.IsNotExist(statErr)
		_, inState := st.Skills[sk.Name]
		if goneFromDisk && inState {
			t.Errorf("skill %s removed from disk but still recorded in state — divergence", sk.Name)
		}
		if goneFromDisk {
			removedAtLeastOne = true
		}
	}

	// The sabotaged skill must remain in state (it was never removed).
	if _, ok := st.Skills[bad]; !ok {
		t.Errorf("sabotaged skill %s must remain recorded in state", bad)
	}
	// And the test must have actually exercised a partial removal before failing.
	if !removedAtLeastOne {
		t.Error("expected at least one skill removed before the failure (partial removal not exercised)")
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
