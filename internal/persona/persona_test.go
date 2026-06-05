package persona

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAvailableHasContent(t *testing.T) {
	got := Available()
	if len(got) != 3 {
		t.Fatalf("personas = %d, want 3", len(got))
	}
	for _, p := range got {
		if p.ID == None {
			if p.Content != "" {
				t.Error("None must carry no content")
			}
			continue
		}
		if !strings.Contains(p.Content, "## Rules") {
			t.Errorf("%s content looks empty/malformed", p.ID)
		}
	}
}

func TestInjectSectionInsertsAndReplaces(t *testing.T) {
	// insert into empty
	out := injectSection("", "BODY")
	if !strings.Contains(out, MarkerStart) || !strings.Contains(out, "BODY") || !strings.Contains(out, MarkerEnd) {
		t.Fatalf("insert failed: %q", out)
	}

	// replace existing block, preserving surrounding content
	existing := "Top matter\n\n" + MarkerStart + "\nOLD\n" + MarkerEnd + "\n\nBottom matter\n"
	out = injectSection(existing, "NEW")
	if strings.Contains(out, "OLD") {
		t.Error("old block content should be replaced")
	}
	if !strings.Contains(out, "NEW") {
		t.Error("new block content missing")
	}
	if !strings.Contains(out, "Top matter") || !strings.Contains(out, "Bottom matter") {
		t.Errorf("surrounding content not preserved: %q", out)
	}
	if strings.Count(out, MarkerStart) != 1 {
		t.Errorf("expected exactly one block, got %d", strings.Count(out, MarkerStart))
	}
}

func TestInjectSectionRemovesOnEmpty(t *testing.T) {
	existing := "Keep me\n\n" + MarkerStart + "\nBODY\n" + MarkerEnd + "\n"
	out := injectSection(existing, "")
	if strings.Contains(out, MarkerStart) || strings.Contains(out, "BODY") {
		t.Errorf("block should be removed: %q", out)
	}
	if !strings.Contains(out, "Keep me") {
		t.Errorf("surrounding content lost: %q", out)
	}
}

func TestByID(t *testing.T) {
	if p, ok := ByID(Capiko); !ok || p.Name != "Capiko" {
		t.Errorf("ByID(Capiko) = %+v, %v", p, ok)
	}
	if _, ok := ByID("nope"); ok {
		t.Error("unknown id should not resolve")
	}
}

func TestRenderWritePreservesUserContent(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "copilot-instructions.md")
	if err := os.WriteFile(path, []byte("user's own notes\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	content, changed, err := Render(path, Available()[0]) // Capiko
	if err != nil || !changed {
		t.Fatalf("Render: changed=%v err=%v", changed, err)
	}
	if err := Write(path, content); err != nil {
		t.Fatalf("Write: %v", err)
	}

	data, _ := os.ReadFile(path)
	if !strings.Contains(string(data), MarkerStart) || !strings.Contains(string(data), "user's own notes") {
		t.Errorf("file should contain the block and preserve user notes: %q", data)
	}

	// Re-rendering the same persona is a no-op.
	if _, changed, _ := Render(path, Available()[0]); changed {
		t.Error("re-rendering the same persona should report no change")
	}
}

func TestRenderNoneRemovesBlock(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "copilot-instructions.md")

	content, _, _ := Render(path, Available()[0]) // Capiko
	if err := Write(path, content); err != nil {
		t.Fatal(err)
	}
	content, changed, _ := Render(path, Available()[2]) // None
	if !changed {
		t.Fatal("None over an existing block should change the file")
	}
	if err := Write(path, content); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(path)
	if strings.Contains(string(data), MarkerStart) {
		t.Errorf("None should have removed the block: %q", data)
	}
}
