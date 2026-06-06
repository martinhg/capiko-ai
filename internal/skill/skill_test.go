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

func TestInstallWritesBundleWithExtraFiles(t *testing.T) {
	skillsDir := t.TempDir()
	s := Skill{
		Name:    "sdd-apply",
		Content: "---\nname: sdd-apply\n---\nbody\n",
		Extra: []File{
			{Path: "references/strict-tdd.md", Content: "tdd notes"},
			{Path: "_shared.md", Content: "shared"},
		},
	}

	if _, err := s.Install(skillsDir); err != nil {
		t.Fatalf("Install error: %v", err)
	}

	// SKILL.md and every Extra file land under the skill dir, subdirs created.
	wantFiles := map[string]string{
		filepath.Join(skillsDir, "sdd-apply", "SKILL.md"):                    s.Content,
		filepath.Join(skillsDir, "sdd-apply", "references", "strict-tdd.md"): "tdd notes",
		filepath.Join(skillsDir, "sdd-apply", "_shared.md"):                  "shared",
	}
	for p, want := range wantFiles {
		got, err := os.ReadFile(p)
		if err != nil {
			t.Errorf("missing %s: %v", p, err)
			continue
		}
		if string(got) != want {
			t.Errorf("%s = %q, want %q", p, got, want)
		}
	}
}

func TestCanonicalContentSingleFileEqualsContent(t *testing.T) {
	// A single-file skill's canonical content MUST equal its raw Content so the
	// recorded checksum stays stable across the multi-file upgrade (no drift).
	s := Skill{Name: "x", Content: "the skill body"}
	if s.CanonicalContent() != s.Content {
		t.Errorf("CanonicalContent() = %q, want it to equal Content %q", s.CanonicalContent(), s.Content)
	}
}

func TestCanonicalContentReflectsExtraFiles(t *testing.T) {
	base := Skill{Name: "x", Content: "body", Extra: []File{{Path: "a.md", Content: "A"}}}
	changed := Skill{Name: "x", Content: "body", Extra: []File{{Path: "a.md", Content: "DIFFERENT"}}}
	if base.CanonicalContent() == changed.CanonicalContent() {
		t.Error("changing an extra file's content must change CanonicalContent")
	}
	// Order of Extra must not affect the canonical form (deterministic).
	reordered := Skill{Name: "x", Content: "body", Extra: []File{
		{Path: "b.md", Content: "B"}, {Path: "a.md", Content: "A"},
	}}
	ordered := Skill{Name: "x", Content: "body", Extra: []File{
		{Path: "a.md", Content: "A"}, {Path: "b.md", Content: "B"},
	}}
	if reordered.CanonicalContent() != ordered.CanonicalContent() {
		t.Error("CanonicalContent must be independent of Extra ordering")
	}
}

func TestLoadCatalogBundlesExtraFiles(t *testing.T) {
	fsys := fstest.MapFS{
		"sdd-apply/SKILL.md":            &fstest.MapFile{Data: []byte("---\ndescription: \"apply\"\n---\nbody")},
		"sdd-apply/references/notes.md": &fstest.MapFile{Data: []byte("notes")},
		"sdd-apply/contract.md":         &fstest.MapFile{Data: []byte("contract")},
		"plain/SKILL.md":                &fstest.MapFile{Data: []byte("---\ndescription: \"plain\"\n---\nx")},
	}

	got, err := LoadCatalog(fsys)
	if err != nil {
		t.Fatalf("LoadCatalog: %v", err)
	}
	bundles := map[string]Skill{}
	for _, s := range got {
		bundles[s.Name] = s
	}
	apply := bundles["sdd-apply"]
	if len(apply.Extra) != 2 {
		t.Fatalf("sdd-apply Extra = %d files, want 2: %+v", len(apply.Extra), apply.Extra)
	}
	// Extra is sorted by path and excludes SKILL.md.
	if apply.Extra[0].Path != "contract.md" || apply.Extra[1].Path != "references/notes.md" {
		t.Errorf("Extra paths = %v, want [contract.md references/notes.md]", []string{apply.Extra[0].Path, apply.Extra[1].Path})
	}
	if len(bundles["plain"].Extra) != 0 {
		t.Errorf("single-file skill should have no Extra, got %+v", bundles["plain"].Extra)
	}
}
