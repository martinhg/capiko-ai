package copilot

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestDetect(t *testing.T) {
	origLook, origHome := lookPath, userHomeDir
	t.Cleanup(func() { lookPath, userHomeDir = origLook, origHome })

	found := func(string) (string, error) { return "/usr/bin/copilot", nil }

	t.Run("not installed", func(t *testing.T) {
		lookPath = func(string) (string, error) { return "", exec.ErrNotFound }
		h, err := Detect()
		if h != nil || err != nil {
			t.Errorf("got (%v, %v), want (nil, nil)", h, err)
		}
	})

	t.Run("home dir error", func(t *testing.T) {
		lookPath = found
		userHomeDir = func() (string, error) { return "", errors.New("no home") }
		h, err := Detect()
		if h != nil || err == nil {
			t.Errorf("got (%v, %v), want (nil, error)", h, err)
		}
	})

	t.Run("installed but never logged in", func(t *testing.T) {
		lookPath = found
		userHomeDir = func() (string, error) { return t.TempDir(), nil } // no .copilot inside
		h, err := Detect()
		if h != nil || err != nil {
			t.Errorf("got (%v, %v), want (nil, nil)", h, err)
		}
	})

	t.Run("detected", func(t *testing.T) {
		home := t.TempDir()
		if err := os.MkdirAll(filepath.Join(home, ".copilot"), 0o755); err != nil {
			t.Fatal(err)
		}
		lookPath = found
		userHomeDir = func() (string, error) { return home, nil }

		h, err := Detect()
		if err != nil || h == nil {
			t.Fatalf("got (%v, %v), want a host", h, err)
		}
		if want := filepath.Join(home, ".copilot", "skills"); h.SkillsDir != want {
			t.Errorf("SkillsDir = %q, want %q", h.SkillsDir, want)
		}
	})
}

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

func TestUninstallRefusesNonDirectory(t *testing.T) {
	dir := t.TempDir()
	// A loose file (not a directory) must never be removed by name.
	if err := os.WriteFile(filepath.Join(dir, "loose"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	h := &Host{SkillsDir: dir}

	if err := h.UninstallSkill("loose"); err == nil {
		t.Error("expected refusal removing a non-directory")
	}
	if _, err := os.Stat(filepath.Join(dir, "loose")); err != nil {
		t.Errorf("the loose file must be left untouched: %v", err)
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

func TestCustomInstructionDirs(t *testing.T) {
	tests := []struct {
		name string
		env  string
		want []string
	}{
		{"unset", "", nil},
		{"whitespace only", "  ,  , ", nil},
		{"single", "/a/dir", []string{"/a/dir"}},
		{"multiple trimmed", "/a, /b ,,/c ", []string{"/a", "/b", "/c"}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("COPILOT_CUSTOM_INSTRUCTIONS_DIRS", tc.env)
			got := CustomInstructionDirs()
			if len(got) != len(tc.want) {
				t.Fatalf("CustomInstructionDirs() = %v, want %v", got, tc.want)
			}
			for i := range got {
				if got[i] != tc.want[i] {
					t.Errorf("dir[%d] = %q, want %q", i, got[i], tc.want[i])
				}
			}
		})
	}
}
