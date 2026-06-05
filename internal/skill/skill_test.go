package skill

import (
	"os"
	"path/filepath"
	"testing"
	"testing/fstest"
)

func TestInstallWritesContentVerbatim(t *testing.T) {
	skillsDir := t.TempDir()
	s := Skill{
		Name:    "capiko-hello",
		Content: "---\nname: capiko-hello\n---\n\nbody\n",
	}

	path, err := s.Install(skillsDir)
	if err != nil {
		t.Fatalf("Install error: %v", err)
	}

	want := filepath.Join(skillsDir, "capiko-hello", "SKILL.md")
	if path != want {
		t.Errorf("path = %q, want %q", path, want)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading written file: %v", err)
	}
	if string(data) != s.Content {
		t.Errorf("written content = %q, want verbatim %q", data, s.Content)
	}
}

func TestLoadCatalog(t *testing.T) {
	fsys := fstest.MapFS{
		"capiko-hello/SKILL.md": &fstest.MapFile{
			Data: []byte("---\nname: capiko-hello\ndescription: \"Hi. Trigger: verify.\"\n---\n\nbody"),
		},
		"capiko-aaa/SKILL.md": &fstest.MapFile{
			Data: []byte("---\ndescription: \"First alphabetically.\"\n---\nbody"),
		},
		// A directory without SKILL.md must be skipped, not fail.
		"not-a-skill/README.md": &fstest.MapFile{Data: []byte("nope")},
	}

	got, err := LoadCatalog(fsys)
	if err != nil {
		t.Fatalf("LoadCatalog error: %v", err)
	}

	if len(got) != 2 {
		t.Fatalf("got %d skills, want 2: %+v", len(got), got)
	}
	// Sorted by name: capiko-aaa before capiko-hello.
	if got[0].Name != "capiko-aaa" || got[1].Name != "capiko-hello" {
		t.Errorf("names = [%s %s], want [capiko-aaa capiko-hello]", got[0].Name, got[1].Name)
	}
	if got[1].Description != "Hi. Trigger: verify." {
		t.Errorf("description = %q, want parsed from frontmatter", got[1].Description)
	}
	if got[1].Content == "" {
		t.Error("content should be the full SKILL.md, got empty")
	}
}

func TestLoadCatalogRejectsBadFrontmatter(t *testing.T) {
	fsys := fstest.MapFS{
		"broken/SKILL.md": &fstest.MapFile{Data: []byte("no frontmatter here")},
	}

	if _, err := LoadCatalog(fsys); err == nil {
		t.Error("expected error for SKILL.md without frontmatter")
	}
}
