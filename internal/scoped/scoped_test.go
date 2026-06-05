package scoped

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoad(t *testing.T) {
	got, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(got) == 0 {
		t.Fatal("no embedded instructions")
	}
	byName := map[string]bool{}
	for _, in := range got {
		if !strings.Contains(in.Content, "applyTo") {
			t.Errorf("%s missing applyTo frontmatter", in.Name)
		}
		byName[in.Name] = true
	}
	if !byName["go"] {
		t.Errorf("expected a go instruction, got %v", byName)
	}
}

func TestInstallWritesScopedFile(t *testing.T) {
	dir := Dir(t.TempDir())
	ins := Instruction{Name: "go", Content: "---\napplyTo: \"**/*.go\"\n---\nx"}

	p, err := Install(dir, ins)
	if err != nil {
		t.Fatalf("Install: %v", err)
	}
	if filepath.Base(p) != "go.instructions.md" {
		t.Errorf("path = %q, want go.instructions.md", p)
	}
	data, err := os.ReadFile(p)
	if err != nil || string(data) != ins.Content {
		t.Errorf("content = %q, err = %v", data, err)
	}
}
