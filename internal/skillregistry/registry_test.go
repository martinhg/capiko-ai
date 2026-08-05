package skillregistry

import (
	"os"
	"path/filepath"
	"testing"
)

// writeSkill creates <dir>/<name>/SKILL.md with minimal valid frontmatter.
func writeSkill(t *testing.T, dir, name, description string) {
	t.Helper()
	skillDir := filepath.Join(dir, name)
	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", skillDir, err)
	}
	content := "---\nname: " + name + "\ndescription: \"" + description + "\"\n---\n\nbody\n"
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(content), 0o644); err != nil {
		t.Fatalf("write SKILL.md: %v", err)
	}
}

func TestResolveScansUserAndProjectSkills(t *testing.T) {
	home := t.TempDir()
	cwd := t.TempDir()

	userSkills := filepath.Join(home, ".copilot", "skills")
	projSkills := filepath.Join(cwd, ".copilot", "skills")
	writeSkill(t, userSkills, "beta", "Beta skill. Trigger: b")
	writeSkill(t, userSkills, "alpha", "Alpha skill. Trigger: a")
	writeSkill(t, projSkills, "gamma", "Gamma skill. Trigger: g")

	// A directory without a SKILL.md must be skipped, not error.
	if err := os.MkdirAll(filepath.Join(userSkills, "notaskill"), 0o755); err != nil {
		t.Fatalf("mkdir notaskill: %v", err)
	}

	reg, err := Resolve(ResolveOptions{Cwd: cwd, Home: home})
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}

	if len(reg.Entries) != 3 {
		t.Fatalf("expected 3 entries, got %d: %+v", len(reg.Entries), reg.Entries)
	}

	// Sorted by name.
	wantNames := []string{"alpha", "beta", "gamma"}
	for i, w := range wantNames {
		if reg.Entries[i].Name != w {
			t.Errorf("entry %d: name = %q, want %q", i, reg.Entries[i].Name, w)
		}
	}

	// Scope is derived from the source.
	byName := map[string]Entry{}
	for _, e := range reg.Entries {
		byName[e.Name] = e
	}
	if byName["alpha"].Scope != "user" {
		t.Errorf("alpha scope = %q, want user", byName["alpha"].Scope)
	}
	if byName["gamma"].Scope != "project" {
		t.Errorf("gamma scope = %q, want project", byName["gamma"].Scope)
	}

	// Path points at the real SKILL.md.
	wantPath := filepath.Join(projSkills, "gamma", "SKILL.md")
	if byName["gamma"].Path != wantPath {
		t.Errorf("gamma path = %q, want %q", byName["gamma"].Path, wantPath)
	}

	// Description carries the frontmatter (trigger embedded).
	if byName["alpha"].Description != "Alpha skill. Trigger: a" {
		t.Errorf("alpha description = %q", byName["alpha"].Description)
	}
}

func TestResolveSkipsMalformedSkill(t *testing.T) {
	home := t.TempDir()
	cwd := t.TempDir()
	userSkills := filepath.Join(home, ".copilot", "skills")
	writeSkill(t, userSkills, "good", "Good skill. Trigger: g")

	// A skill whose SKILL.md has no frontmatter must not break the whole scan.
	badDir := filepath.Join(userSkills, "bad")
	if err := os.MkdirAll(badDir, 0o755); err != nil {
		t.Fatalf("mkdir bad: %v", err)
	}
	if err := os.WriteFile(filepath.Join(badDir, "SKILL.md"), []byte("no frontmatter here\n"), 0o644); err != nil {
		t.Fatalf("write bad SKILL.md: %v", err)
	}

	reg, err := Resolve(ResolveOptions{Cwd: cwd, Home: home})
	if err != nil {
		t.Fatalf("Resolve must tolerate a malformed skill, got: %v", err)
	}
	if len(reg.Entries) != 1 || reg.Entries[0].Name != "good" {
		t.Fatalf("expected only the good skill, got %+v", reg.Entries)
	}
}

func TestResolveMissingDirsIsNotAnError(t *testing.T) {
	home := t.TempDir()
	cwd := t.TempDir()
	// Neither has a .copilot/skills directory.
	reg, err := Resolve(ResolveOptions{Cwd: cwd, Home: home})
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if len(reg.Entries) != 0 {
		t.Errorf("expected 0 entries, got %d", len(reg.Entries))
	}
}

func TestResolveFingerprintChangesOnSameSizeEdit(t *testing.T) {
	home := t.TempDir()
	cwd := t.TempDir()
	userSkills := filepath.Join(home, ".copilot", "skills")

	writeSkill(t, userSkills, "demo", "AAAA skill. Trigger: x")

	reg1, err := Resolve(ResolveOptions{Cwd: cwd, Home: home})
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if reg1.Fingerprint == "" {
		t.Fatal("expected non-empty fingerprint")
	}

	// Edit the SKILL.md to same-size content (swap AAAA → BBBB, same byte count).
	skillFile := filepath.Join(userSkills, "demo", "SKILL.md")
	data, err := os.ReadFile(skillFile)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	edited := make([]byte, len(data))
	copy(edited, data)
	for i := range edited {
		if edited[i] == 'A' {
			edited[i] = 'B'
		}
	}
	if len(edited) != len(data) {
		t.Fatal("edited content must be same size")
	}
	if err := os.WriteFile(skillFile, edited, 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	reg2, err := Resolve(ResolveOptions{Cwd: cwd, Home: home})
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if reg1.Fingerprint == reg2.Fingerprint {
		t.Error("fingerprint must change on same-size content edit")
	}
}

func TestResolveFingerprintStableOnNoChange(t *testing.T) {
	home := t.TempDir()
	cwd := t.TempDir()
	writeSkill(t, filepath.Join(home, ".copilot", "skills"), "stable", "Stable skill")

	reg1, err := Resolve(ResolveOptions{Cwd: cwd, Home: home})
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	reg2, err := Resolve(ResolveOptions{Cwd: cwd, Home: home})
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if reg1.Fingerprint != reg2.Fingerprint {
		t.Errorf("fingerprint must be stable: %q != %q", reg1.Fingerprint, reg2.Fingerprint)
	}
}

func TestResolveFingerprintEmptyWhenNoSkills(t *testing.T) {
	home := t.TempDir()
	cwd := t.TempDir()
	reg, err := Resolve(ResolveOptions{Cwd: cwd, Home: home})
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if reg.Fingerprint != "" {
		t.Errorf("expected empty fingerprint with no skills, got %q", reg.Fingerprint)
	}
}

func TestResolveListsSourcesScanned(t *testing.T) {
	home := t.TempDir()
	cwd := t.TempDir()
	reg, err := Resolve(ResolveOptions{Cwd: cwd, Home: home})
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	// Both candidate sources are documented even when empty.
	wantUser := filepath.Join(home, ".copilot", "skills")
	wantProj := filepath.Join(cwd, ".copilot", "skills")
	found := map[string]bool{}
	for _, s := range reg.Sources {
		found[s] = true
	}
	if !found[wantUser] {
		t.Errorf("sources missing user dir %q: %v", wantUser, reg.Sources)
	}
	if !found[wantProj] {
		t.Errorf("sources missing project dir %q: %v", wantProj, reg.Sources)
	}
}
