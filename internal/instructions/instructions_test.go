package instructions

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const (
	start = "<!-- test:start -->"
	end   = "<!-- test:end -->"
)

func TestInjectInsertReplaceRemove(t *testing.T) {
	// insert into empty
	out := Inject("", start, end, "BODY")
	if !strings.Contains(out, start) || !strings.Contains(out, "BODY") || !strings.Contains(out, end) {
		t.Fatalf("insert failed: %q", out)
	}

	// replace, preserving surrounding content
	existing := "Top\n\n" + start + "\nOLD\n" + end + "\n\nBottom\n"
	out = Inject(existing, start, end, "NEW")
	if strings.Contains(out, "OLD") || !strings.Contains(out, "NEW") {
		t.Errorf("replace failed: %q", out)
	}
	if !strings.Contains(out, "Top") || !strings.Contains(out, "Bottom") {
		t.Errorf("surrounding content lost: %q", out)
	}
	if strings.Count(out, start) != 1 {
		t.Errorf("want one block, got %d", strings.Count(out, start))
	}

	// remove on empty block, keeping surroundings
	out = Inject(existing, start, end, "")
	if strings.Contains(out, start) || strings.Contains(out, "OLD") {
		t.Errorf("block not removed: %q", out)
	}
	if !strings.Contains(out, "Top") || !strings.Contains(out, "Bottom") {
		t.Errorf("surrounding content lost on remove: %q", out)
	}
}

func TestRenderAndWrite(t *testing.T) {
	path := filepath.Join(t.TempDir(), "instructions.md")
	if err := os.WriteFile(path, []byte("keep me\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	content, changed, err := Render(path, start, end, "BODY")
	if err != nil || !changed {
		t.Fatalf("Render: changed=%v err=%v", changed, err)
	}
	if err := Write(path, content); err != nil {
		t.Fatalf("Write: %v", err)
	}

	data, _ := os.ReadFile(path)
	if !strings.Contains(string(data), "keep me") || !strings.Contains(string(data), "BODY") {
		t.Errorf("file = %q", data)
	}

	// re-rendering the same block is a no-op
	if _, changed, _ := Render(path, start, end, "BODY"); changed {
		t.Error("re-render should report no change")
	}
}
