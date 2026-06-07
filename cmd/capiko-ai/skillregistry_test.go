package main

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestSkillRegistryCommandUnknown(t *testing.T) {
	var out bytes.Buffer
	handled, err := skillRegistryCommand("not-a-command", nil, &out)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if handled {
		t.Errorf("expected handled=false for unknown command")
	}
}

func TestSkillRegistryCommandRendersMarkdown(t *testing.T) {
	home := t.TempDir()
	cwd := t.TempDir()
	t.Setenv("HOME", home)

	// One user skill so the table has a row.
	skillDir := filepath.Join(home, ".copilot", "skills", "demo")
	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	content := "---\nname: demo\ndescription: \"Demo skill. Trigger: demo\"\n---\n\nbody\n"
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(content), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	var out bytes.Buffer
	handled, err := skillRegistryCommand("skill-registry", []string{"--cwd", cwd}, &out)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !handled {
		t.Fatalf("expected handled=true")
	}
	for _, want := range []string{"# Skill Registry", "`demo`", "Demo skill. Trigger: demo"} {
		if !bytes.Contains(out.Bytes(), []byte(want)) {
			t.Errorf("output missing %q\n---\n%s", want, out.String())
		}
	}
}
