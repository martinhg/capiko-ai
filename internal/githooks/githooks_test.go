package githooks_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/martinhg/capiko-ai/internal/githooks"
)

const (
	testStart = "# >>> capiko:test >>>"
	testEnd   = "# <<< capiko:test <<<"
	testBlock = "engram sync --import"
)

// setup returns a workspace with .git/hooks/ already created and the path to
// the hook file for hookName (the hook file itself is NOT created).
func setup(t *testing.T, hookName string) (workspace, hookPath string) {
	t.Helper()
	workspace = t.TempDir()
	hooksDir := filepath.Join(workspace, ".git", "hooks")
	if err := os.MkdirAll(hooksDir, 0o755); err != nil {
		t.Fatalf("setup: MkdirAll: %v", err)
	}
	hookPath = filepath.Join(hooksDir, hookName)
	return workspace, hookPath
}

// assertExec asserts that path has at least one execute bit set.
func assertExec(t *testing.T, path string) {
	t.Helper()
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat %s: %v", path, err)
	}
	if info.Mode()&0o111 == 0 {
		t.Errorf("want executable bit set on %s, got mode %o", path, info.Mode())
	}
}

func TestWriteBlock_CreateNew(t *testing.T) {
	workspace, hookPath := setup(t, "post-merge")

	if err := githooks.WriteBlock(workspace, "post-merge", testStart, testEnd, testBlock); err != nil {
		t.Fatalf("WriteBlock: %v", err)
	}

	content, err := os.ReadFile(hookPath)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	got := string(content)

	lines := strings.SplitN(got, "\n", 2)
	if lines[0] != "#!/bin/sh" {
		t.Errorf("first line: want %q, got %q", "#!/bin/sh", lines[0])
	}
	if !strings.Contains(got, testStart) {
		t.Errorf("missing start marker in:\n%s", got)
	}
	if !strings.Contains(got, testEnd) {
		t.Errorf("missing end marker in:\n%s", got)
	}
	if !strings.Contains(got, testBlock) {
		t.Errorf("missing block content in:\n%s", got)
	}
	assertExec(t, hookPath)
}

func TestWriteBlock_InjectExisting(t *testing.T) {
	workspace, hookPath := setup(t, "post-merge")

	existing := "#!/bin/sh\n# user content\nsome-other-hook\n"
	if err := os.WriteFile(hookPath, []byte(existing), 0o755); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	if err := githooks.WriteBlock(workspace, "post-merge", testStart, testEnd, testBlock); err != nil {
		t.Fatalf("WriteBlock: %v", err)
	}

	content, err := os.ReadFile(hookPath)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	got := string(content)

	if !strings.Contains(got, "# user content") {
		t.Errorf("pre-existing content lost:\n%s", got)
	}
	if !strings.Contains(got, "some-other-hook") {
		t.Errorf("pre-existing hook lost:\n%s", got)
	}
	if !strings.Contains(got, testStart) {
		t.Errorf("missing start marker:\n%s", got)
	}
	if !strings.Contains(got, testBlock) {
		t.Errorf("missing block content:\n%s", got)
	}
	if strings.Count(got, "#!/bin/sh") != 1 {
		t.Errorf("want exactly 1 shebang, got %d in:\n%s", strings.Count(got, "#!/bin/sh"), got)
	}
}

func TestWriteBlock_Idempotent(t *testing.T) {
	workspace, hookPath := setup(t, "post-merge")

	if err := githooks.WriteBlock(workspace, "post-merge", testStart, testEnd, testBlock); err != nil {
		t.Fatalf("first WriteBlock: %v", err)
	}
	first, err := os.ReadFile(hookPath)
	if err != nil {
		t.Fatalf("ReadFile after first call: %v", err)
	}

	if err := githooks.WriteBlock(workspace, "post-merge", testStart, testEnd, testBlock); err != nil {
		t.Fatalf("second WriteBlock: %v", err)
	}
	second, err := os.ReadFile(hookPath)
	if err != nil {
		t.Fatalf("ReadFile after second call: %v", err)
	}

	if string(first) != string(second) {
		t.Errorf("WriteBlock not idempotent:\nfirst:  %q\nsecond: %q", first, second)
	}
}

func TestRemoveBlock_RemovesBlock(t *testing.T) {
	workspace, hookPath := setup(t, "post-merge")

	initial := "#!/bin/sh\n# user line\n\n" + testStart + "\n" + testBlock + "\n" + testEnd + "\n"
	if err := os.WriteFile(hookPath, []byte(initial), 0o755); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	if err := githooks.RemoveBlock(workspace, "post-merge", testStart, testEnd); err != nil {
		t.Fatalf("RemoveBlock: %v", err)
	}

	content, err := os.ReadFile(hookPath)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	got := string(content)

	if strings.Contains(got, testStart) {
		t.Errorf("start marker still present:\n%s", got)
	}
	if strings.Contains(got, testEnd) {
		t.Errorf("end marker still present:\n%s", got)
	}
	if !strings.Contains(got, "# user line") {
		t.Errorf("user content lost:\n%s", got)
	}
}

func TestRemoveBlock_DeletesWhenOnlyShebang(t *testing.T) {
	workspace, hookPath := setup(t, "post-merge")

	// File has only the shebang + capiko block (no other content).
	initial := "#!/bin/sh\n\n" + testStart + "\n" + testBlock + "\n" + testEnd + "\n"
	if err := os.WriteFile(hookPath, []byte(initial), 0o755); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	if err := githooks.RemoveBlock(workspace, "post-merge", testStart, testEnd); err != nil {
		t.Fatalf("RemoveBlock: %v", err)
	}

	if _, err := os.Stat(hookPath); !os.IsNotExist(err) {
		t.Errorf("want file deleted, but Stat returned: %v", err)
	}
}

func TestRemoveBlock_MissingFile(t *testing.T) {
	workspace, _ := setup(t, "post-merge")

	// Hook file does not exist — must be a no-op.
	if err := githooks.RemoveBlock(workspace, "post-merge", testStart, testEnd); err != nil {
		t.Fatalf("RemoveBlock on missing file: want nil, got %v", err)
	}

	hookPath := filepath.Join(workspace, ".git", "hooks", "post-merge")
	if _, err := os.Stat(hookPath); !os.IsNotExist(err) {
		t.Errorf("no file should be created; Stat returned: %v", err)
	}
}

func TestWriteBlock_CoexistForeignBlock(t *testing.T) {
	workspace, hookPath := setup(t, "post-merge")

	foreignStart := "# >>> other:tool >>>"
	foreignEnd := "# <<< other:tool <<<"
	foreignContent := "other-tool-command"

	// Seed file with a foreign block and our capiko block already present.
	initial := "#!/bin/sh\n\n" +
		foreignStart + "\n" + foreignContent + "\n" + foreignEnd + "\n\n" +
		testStart + "\n" + "old content" + "\n" + testEnd + "\n"
	if err := os.WriteFile(hookPath, []byte(initial), 0o755); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	// WriteBlock should update the capiko block without touching the foreign block.
	if err := githooks.WriteBlock(workspace, "post-merge", testStart, testEnd, testBlock); err != nil {
		t.Fatalf("WriteBlock: %v", err)
	}

	content, err := os.ReadFile(hookPath)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	got := string(content)

	if !strings.Contains(got, foreignStart) {
		t.Errorf("foreign block start lost:\n%s", got)
	}
	if !strings.Contains(got, foreignContent) {
		t.Errorf("foreign block content lost:\n%s", got)
	}
	if !strings.Contains(got, testBlock) {
		t.Errorf("capiko block content missing:\n%s", got)
	}
}
