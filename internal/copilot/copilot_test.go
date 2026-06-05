package copilot

import (
	"os"
	"path/filepath"
	"testing"
)

func TestInstalledSkills(t *testing.T) {
	skillsDir := t.TempDir()

	// A valid skill: dir with SKILL.md.
	mustWriteSkill(t, skillsDir, "capiko-hello")
	// A directory without SKILL.md must be ignored.
	if err := os.MkdirAll(filepath.Join(skillsDir, "not-a-skill"), 0o755); err != nil {
		t.Fatal(err)
	}
	// A loose file must be ignored.
	if err := os.WriteFile(filepath.Join(skillsDir, "loose.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	h := &Host{SkillsDir: skillsDir}
	got, err := h.InstalledSkills()
	if err != nil {
		t.Fatalf("InstalledSkills error: %v", err)
	}

	if len(got) != 1 || !got["capiko-hello"] {
		t.Errorf("got %v, want only capiko-hello", got)
	}
}

func TestInstalledSkillsMissingDir(t *testing.T) {
	h := &Host{SkillsDir: filepath.Join(t.TempDir(), "does-not-exist")}

	got, err := h.InstalledSkills()
	if err != nil {
		t.Fatalf("missing dir should not error, got: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("got %v, want empty", got)
	}
}

func TestUninstallSkill(t *testing.T) {
	dir := t.TempDir()
	mustWriteSkill(t, dir, "capiko-hello")
	h := &Host{SkillsDir: dir}

	if err := h.UninstallSkill("capiko-hello"); err != nil {
		t.Fatalf("UninstallSkill error: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "capiko-hello")); !os.IsNotExist(err) {
		t.Errorf("skill dir still present after uninstall: %v", err)
	}
}

func TestUninstallMissingIsIdempotent(t *testing.T) {
	h := &Host{SkillsDir: t.TempDir()}
	if err := h.UninstallSkill("does-not-exist"); err != nil {
		t.Errorf("uninstalling an absent skill should be nil, got %v", err)
	}
}

func TestUninstallRefusesNonSkill(t *testing.T) {
	dir := t.TempDir()
	// A directory with no SKILL.md must never be removed.
	if err := os.MkdirAll(filepath.Join(dir, "random"), 0o755); err != nil {
		t.Fatal(err)
	}
	h := &Host{SkillsDir: dir}

	if err := h.UninstallSkill("random"); err == nil {
		t.Error("expected refusal removing a non-skill directory")
	}
	if _, err := os.Stat(filepath.Join(dir, "random")); err != nil {
		t.Errorf("non-skill dir must be left untouched: %v", err)
	}
}

func mustWriteSkill(t *testing.T, skillsDir, name string) {
	t.Helper()
	dir := filepath.Join(skillsDir, name)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte("---\nname: "+name+"\n---\n"), 0o644); err != nil {
		t.Fatal(err)
	}
}
