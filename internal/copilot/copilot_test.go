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

func TestDetect_AgentsDirDerivation(t *testing.T) {
	origLook, origHome := lookPath, userHomeDir
	t.Cleanup(func() { lookPath, userHomeDir = origLook, origHome })

	home := t.TempDir()
	if err := os.MkdirAll(filepath.Join(home, ".copilot"), 0o755); err != nil {
		t.Fatal(err)
	}
	lookPath = func(string) (string, error) { return "/usr/bin/copilot", nil }
	userHomeDir = func() (string, error) { return home, nil }

	h, err := Detect()
	if err != nil || h == nil {
		t.Fatalf("Detect() = (%v, %v), want a host", h, err)
	}
	want := filepath.Join(h.ConfigDir, "agents")
	if h.AgentsDir != want {
		t.Errorf("AgentsDir = %q, want %q", h.AgentsDir, want)
	}
}

func TestHost_InstalledAgents_MixedFiles(t *testing.T) {
	agentsDir := t.TempDir()

	mustWriteAgentFile(t, agentsDir, "sdd-explore")
	mustWriteAgentFile(t, agentsDir, "sdd-apply")
	// A non-.agent.md file must be excluded.
	if err := os.WriteFile(filepath.Join(agentsDir, "README.md"), []byte("readme"), 0o644); err != nil {
		t.Fatal(err)
	}

	h := &Host{AgentsDir: agentsDir}
	got, err := h.InstalledAgents()
	if err != nil {
		t.Fatalf("InstalledAgents error: %v", err)
	}
	if !got["sdd-explore"] {
		t.Error("expected sdd-explore in result")
	}
	if !got["sdd-apply"] {
		t.Error("expected sdd-apply in result")
	}
	if got["README"] {
		t.Error("README must not appear in result")
	}
	if len(got) != 2 {
		t.Errorf("expected 2 entries, got %d: %v", len(got), got)
	}
}

func TestHost_InstalledAgents_MissingDir(t *testing.T) {
	h := &Host{AgentsDir: filepath.Join(t.TempDir(), "does-not-exist")}

	got, err := h.InstalledAgents()
	if err != nil {
		t.Fatalf("missing agentsDir should not error, got: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("expected empty map, got %v", got)
	}
}

func TestHost_UninstallAgent_RemovesFile(t *testing.T) {
	agentsDir := t.TempDir()
	mustWriteAgentFile(t, agentsDir, "sdd-spec")

	h := &Host{AgentsDir: agentsDir}
	if err := h.UninstallAgent("sdd-spec"); err != nil {
		t.Fatalf("UninstallAgent error: %v", err)
	}
	if _, err := os.Stat(filepath.Join(agentsDir, "sdd-spec.agent.md")); !os.IsNotExist(err) {
		t.Errorf("agent file still present after uninstall: %v", err)
	}
}

func TestHost_UninstallAgent_Idempotent(t *testing.T) {
	h := &Host{AgentsDir: t.TempDir()}
	if err := h.UninstallAgent("sdd-spec"); err != nil {
		t.Errorf("uninstalling absent agent should return nil, got %v", err)
	}
}

func TestHost_UninstallAgent_RefusesPathTraversal(t *testing.T) {
	root := t.TempDir()
	agentsDir := filepath.Join(root, "agents")
	if err := os.MkdirAll(agentsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	// A victim file OUTSIDE the agents dir, carrying the .agent.md suffix the
	// resolved target will also carry — so a suffix-only guard cannot catch it.
	victim := filepath.Join(root, "victim.agent.md")
	if err := os.WriteFile(victim, []byte("sensitive"), 0o644); err != nil {
		t.Fatal(err)
	}
	h := &Host{AgentsDir: agentsDir}

	// "../victim" resolves to root/victim.agent.md, outside AgentsDir.
	if err := h.UninstallAgent("../victim"); err == nil {
		t.Error("expected error for path-traversal name, got nil")
	}
	// The file outside AgentsDir must survive.
	if _, err := os.Stat(victim); err != nil {
		t.Errorf("victim file outside agents dir must be untouched: %v", err)
	}
}

func mustWriteAgentFile(t *testing.T, agentsDir, stem string) {
	t.Helper()
	path := filepath.Join(agentsDir, stem+".agent.md")
	if err := os.WriteFile(path, []byte("---\ndescription: test\n---\n"), 0o644); err != nil {
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
