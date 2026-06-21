package skill

import (
	"os"
	"path/filepath"
	"strings"
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
			Data: []byte("---\ndescription: \"First alphabetically. Trigger: always.\"\n---\nbody"),
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

func TestLoadCatalogRejectsMissingDependency(t *testing.T) {
	fsys := fstest.MapFS{
		"sdd-apply/SKILL.md": &fstest.MapFile{
			Data: []byte("---\ndescription: apply\ndepends_on: [sdd-shared]\n---\nbody"),
		},
		// sdd-shared intentionally absent.
	}
	if _, err := LoadCatalog(fsys); err == nil {
		t.Error("expected LoadCatalog to reject a catalog with a missing dependency")
	}
}

func TestLoadCatalogRejectsDependencyCycle(t *testing.T) {
	fsys := fstest.MapFS{
		"a/SKILL.md": &fstest.MapFile{Data: []byte("---\ndescription: a\ndepends_on: [b]\n---\nx")},
		"b/SKILL.md": &fstest.MapFile{Data: []byte("---\ndescription: b\ndepends_on: [a]\n---\nx")},
	}
	if _, err := LoadCatalog(fsys); err == nil {
		t.Error("expected LoadCatalog to reject a dependency cycle")
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
		"sdd-apply/SKILL.md":            &fstest.MapFile{Data: []byte("---\ndescription: \"apply. Trigger: applying.\"\n---\nbody")},
		"sdd-apply/references/notes.md": &fstest.MapFile{Data: []byte("notes")},
		"sdd-apply/contract.md":         &fstest.MapFile{Data: []byte("contract")},
		"plain/SKILL.md":                &fstest.MapFile{Data: []byte("---\ndescription: \"plain. Trigger: plain.\"\n---\nx")},
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

func TestParseDependsOn(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    []string
	}{
		{
			name:    "inline list",
			content: "---\ndescription: x\ndepends_on: [sdd-shared, capiko-conventions]\n---\nbody",
			want:    []string{"sdd-shared", "capiko-conventions"},
		},
		{
			name:    "block list",
			content: "---\ndescription: x\ndepends_on:\n  - sdd-shared\n  - capiko-conventions\n---\nbody",
			want:    []string{"sdd-shared", "capiko-conventions"},
		},
		{
			name:    "absent means no dependencies",
			content: "---\ndescription: x\n---\nbody",
			want:    nil,
		},
		{
			name:    "CRLF line endings",
			content: "---\r\ndescription: x\r\ndepends_on: [sdd-shared]\r\n---\r\nbody",
			want:    []string{"sdd-shared"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s, err := Parse("dependent", tt.content)
			if err != nil {
				t.Fatalf("Parse error: %v", err)
			}
			if len(s.DependsOn) != len(tt.want) {
				t.Fatalf("DependsOn = %v, want %v", s.DependsOn, tt.want)
			}
			for i := range tt.want {
				if s.DependsOn[i] != tt.want[i] {
					t.Errorf("DependsOn[%d] = %q, want %q", i, s.DependsOn[i], tt.want[i])
				}
			}
		})
	}
}

func TestValidateDependenciesAcceptsValidGraph(t *testing.T) {
	catalog := []Skill{
		{Name: "sdd-apply", DependsOn: []string{"sdd-shared"}},
		{Name: "sdd-verify", DependsOn: []string{"sdd-shared"}},
		{Name: "sdd-shared"},
	}
	if err := ValidateDependencies(catalog); err != nil {
		t.Errorf("valid graph rejected: %v", err)
	}
}

func TestValidateDependenciesRejectsMissingDep(t *testing.T) {
	catalog := []Skill{
		{Name: "sdd-apply", DependsOn: []string{"sdd-shared"}},
		// sdd-shared is absent
	}
	err := ValidateDependencies(catalog)
	if err == nil {
		t.Fatal("expected error for missing dependency")
	}
	if !strings.Contains(err.Error(), "sdd-shared") || !strings.Contains(err.Error(), "sdd-apply") {
		t.Errorf("error should name both the dependent and the missing dep, got: %v", err)
	}
}

func TestValidateDependenciesRejectsCycle(t *testing.T) {
	catalog := []Skill{
		{Name: "a", DependsOn: []string{"b"}},
		{Name: "b", DependsOn: []string{"a"}},
	}
	if err := ValidateDependencies(catalog); err == nil {
		t.Error("expected error for dependency cycle a→b→a")
	}
}

func TestValidateDependenciesRejectsSelfDependency(t *testing.T) {
	catalog := []Skill{{Name: "a", DependsOn: []string{"a"}}}
	if err := ValidateDependencies(catalog); err == nil {
		t.Error("expected error for self-dependency a→a")
	}
}

func TestResolveDependenciesIncludesTransitiveClosure(t *testing.T) {
	catalog := []Skill{
		{Name: "a", DependsOn: []string{"b"}},
		{Name: "b", DependsOn: []string{"c"}},
		{Name: "c"},
		{Name: "unrelated"},
	}
	got, err := ResolveDependencies(catalog, []string{"a"})
	if err != nil {
		t.Fatalf("ResolveDependencies error: %v", err)
	}
	want := map[string]bool{"a": true, "b": true, "c": true}
	if len(got) != len(want) {
		t.Fatalf("resolved = %v, want keys %v", got, want)
	}
	for _, n := range got {
		if !want[n] {
			t.Errorf("unexpected skill %q in closure", n)
		}
	}
}

func TestResolveDependenciesIsDeterministicAndDeduped(t *testing.T) {
	catalog := []Skill{
		{Name: "a", DependsOn: []string{"shared"}},
		{Name: "b", DependsOn: []string{"shared"}},
		{Name: "shared"},
	}
	got, err := ResolveDependencies(catalog, []string{"b", "a"})
	if err != nil {
		t.Fatalf("ResolveDependencies error: %v", err)
	}
	// Sorted, no duplicate "shared".
	want := []string{"a", "b", "shared"}
	if len(got) != len(want) {
		t.Fatalf("resolved = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("resolved[%d] = %q, want %q (must be sorted, deduped)", i, got[i], want[i])
		}
	}
}

func TestResolveDependenciesRejectsUnknownSelection(t *testing.T) {
	catalog := []Skill{{Name: "a"}}
	if _, err := ResolveDependencies(catalog, []string{"ghost"}); err == nil {
		t.Error("expected error when a selected skill is not in the catalog")
	}
}

func TestParseHandlesCRLFLineEndings(t *testing.T) {
	// Git for Windows with core.autocrlf=true checks SKILL.md out with \r\n.
	// The parser must read the same name/description as it would with \n, and
	// must not let a trailing \r leak into the description value.
	content := "---\r\nname: ignored\r\ndescription: A CRLF skill\r\n---\r\n\r\n# Body\r\n"
	sk, err := Parse("crlf-skill", content)
	if err != nil {
		t.Fatalf("Parse with CRLF returned error: %v", err)
	}
	if sk.Description != "A CRLF skill" {
		t.Errorf("description = %q, want %q (a stray \\r likely leaked)", sk.Description, "A CRLF skill")
	}
}
