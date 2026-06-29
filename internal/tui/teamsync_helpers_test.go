package tui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// shSingleQuote
// ---------------------------------------------------------------------------

func TestShSingleQuote_NoSpecialChars(t *testing.T) {
	if got := shSingleQuote("my-project"); got != "'my-project'" {
		t.Errorf("shSingleQuote = %q, want %q", got, "'my-project'")
	}
}

func TestShSingleQuote_EmptyString(t *testing.T) {
	if got := shSingleQuote(""); got != "''" {
		t.Errorf("shSingleQuote(\"\") = %q, want %q", got, "''")
	}
}

func TestShSingleQuote_ContainsSingleQuote(t *testing.T) {
	// it's → 'it'\''s'
	if got := shSingleQuote("it's"); got != `'it'\''s'` {
		t.Errorf("shSingleQuote(\"it's\") = %q, want %q", got, `'it'\''s'`)
	}
}

func TestShSingleQuote_InjectionAttempt(t *testing.T) {
	// A name designed to escape quoting: name'; rm -rf /
	// POSIX single-quote escaping: each ' in s becomes '\'' (end-quote, backslash-escaped
	// single-quote, open-quote). The ; ends up inside a new single-quoted section, so it
	// is NOT a command separator — the escaping is safe.
	//
	// Expected: 'name'\''; rm -rf /'
	//   - 'name' → literal "name"
	//   - \' → literal "'"  (outside single-quote, \' = escaped ')
	//   - '; rm -rf /' → literal "; rm -rf /" (inside single-quote)
	name := "name'; rm -rf /"
	got := shSingleQuote(name)
	want := `'name'\''; rm -rf /'`
	if got != want {
		t.Errorf("shSingleQuote injection = %q, want %q", got, want)
	}
}

// ---------------------------------------------------------------------------
// detectHookConflict
// ---------------------------------------------------------------------------

// setupWorkspace creates a fake workspace with a .git directory.
func setupWorkspace(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	return root
}

func writeGitConfig(t *testing.T, root, content string) {
	t.Helper()
	p := filepath.Join(root, ".git", "config")
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestDetectHookConflict_NoConflict(t *testing.T) {
	root := setupWorkspace(t)
	// .git/config with no hooksPath, no framework files.
	writeGitConfig(t, root, "[core]\n\trepositoryformatversion = 0\n\tfilemode = true\n")
	if got := detectHookConflict(root); got != "" {
		t.Errorf("expected no conflict, got %q", got)
	}
}

func TestDetectHookConflict_HooksPathNonDefault(t *testing.T) {
	root := setupWorkspace(t)
	writeGitConfig(t, root, "[core]\n\thooksPath = .my-hooks\n")
	got := detectHookConflict(root)
	if got == "" {
		t.Error("expected conflict for non-default hooksPath, got empty string")
	}
	if !strings.Contains(got, "hooksPath") {
		t.Errorf("conflict reason should mention hooksPath, got %q", got)
	}
}

func TestDetectHookConflict_HooksPathIsDefault(t *testing.T) {
	// If hooksPath is explicitly set to .git/hooks, it's not a conflict.
	root := setupWorkspace(t)
	writeGitConfig(t, root, "[core]\n\thooksPath = .git/hooks\n")
	if got := detectHookConflict(root); got != "" {
		t.Errorf("hooksPath=.git/hooks should not be a conflict, got %q", got)
	}
}

func TestDetectHookConflict_HuskyDir(t *testing.T) {
	root := setupWorkspace(t)
	writeGitConfig(t, root, "[core]\n\trepositoryformatversion = 0\n")
	if err := os.MkdirAll(filepath.Join(root, ".husky"), 0o755); err != nil {
		t.Fatal(err)
	}
	got := detectHookConflict(root)
	if got == "" {
		t.Error("expected conflict for .husky/, got empty string")
	}
	if !strings.Contains(got, "husky") {
		t.Errorf("conflict reason should mention husky, got %q", got)
	}
}

func TestDetectHookConflict_LefhookYml(t *testing.T) {
	root := setupWorkspace(t)
	writeGitConfig(t, root, "[core]\n\trepositoryformatversion = 0\n")
	if err := os.WriteFile(filepath.Join(root, "lefthook.yml"), []byte("pre-commit:\n  commands:\n    test:\n      run: go test ./...\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	got := detectHookConflict(root)
	if got == "" {
		t.Error("expected conflict for lefthook.yml, got empty string")
	}
	if !strings.Contains(got, "lefthook") {
		t.Errorf("conflict reason should mention lefthook, got %q", got)
	}
}

func TestDetectHookConflict_DotLefhookYml(t *testing.T) {
	root := setupWorkspace(t)
	writeGitConfig(t, root, "[core]\n\trepositoryformatversion = 0\n")
	if err := os.WriteFile(filepath.Join(root, ".lefthook.yml"), []byte("pre-commit: {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	got := detectHookConflict(root)
	if got == "" {
		t.Error("expected conflict for .lefthook.yml, got empty string")
	}
	if !strings.Contains(got, "lefthook") {
		t.Errorf("conflict reason should mention lefthook, got %q", got)
	}
}

func TestDetectHookConflict_PreCommitConfig(t *testing.T) {
	root := setupWorkspace(t)
	writeGitConfig(t, root, "[core]\n\trepositoryformatversion = 0\n")
	if err := os.WriteFile(filepath.Join(root, ".pre-commit-config.yaml"), []byte("repos: []\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	got := detectHookConflict(root)
	if got == "" {
		t.Error("expected conflict for .pre-commit-config.yaml, got empty string")
	}
	if !strings.Contains(strings.ToLower(got), "pre-commit") {
		t.Errorf("conflict reason should mention pre-commit, got %q", got)
	}
}

func TestDetectHookConflict_NoGitConfig(t *testing.T) {
	root := setupWorkspace(t)
	// No .git/config at all — should not error, just return no conflict from git config check.
	if got := detectHookConflict(root); got != "" {
		t.Errorf("missing .git/config should be no conflict, got %q", got)
	}
}

// ---------------------------------------------------------------------------
// resolveProject
// ---------------------------------------------------------------------------

func TestResolveProject_ConfigPresent(t *testing.T) {
	root := t.TempDir()
	confDir := filepath.Join(root, ".engram")
	if err := os.MkdirAll(confDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(confDir, "config.json"), []byte(`{"project_name":"team-alpha"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := resolveProject(root); got != "team-alpha" {
		t.Errorf("resolveProject = %q, want %q", got, "team-alpha")
	}
}

func TestResolveProject_ConfigAbsent(t *testing.T) {
	root := t.TempDir()
	// No .engram/config.json — should fall back to basename.
	want := filepath.Base(root)
	if got := resolveProject(root); got != want {
		t.Errorf("resolveProject (absent) = %q, want basename %q", got, want)
	}
}

// ---------------------------------------------------------------------------
// renderPostMerge / renderPrePush
// ---------------------------------------------------------------------------

func TestRenderPostMerge(t *testing.T) {
	got := renderPostMerge()
	if !strings.Contains(got, "engram sync --import") {
		t.Errorf("renderPostMerge = %q, must contain 'engram sync --import'", got)
	}
	if !strings.Contains(got, "|| true") {
		t.Errorf("renderPostMerge = %q, must contain '|| true'", got)
	}
}

func TestRenderPrePush_SimpleName(t *testing.T) {
	got := renderPrePush("my-project")
	if !strings.Contains(got, "engram sync --project") {
		t.Errorf("renderPrePush = %q, must contain 'engram sync --project'", got)
	}
	if !strings.Contains(got, "'my-project'") {
		t.Errorf("renderPrePush = %q, must contain single-quoted project name", got)
	}
	if !strings.Contains(got, "|| true") {
		t.Errorf("renderPrePush = %q, must contain '|| true'", got)
	}
	if !strings.Contains(got, "echo") {
		t.Errorf("renderPrePush = %q, must contain echo reminder", got)
	}
}

func TestRenderPrePush_NameWithQuote(t *testing.T) {
	// A name with a single quote must be safely escaped.
	got := renderPrePush("it's")
	if strings.Contains(got, `"it's"`) {
		t.Error("name with single quote must not be double-quoted")
	}
	// The raw single quote must not appear unescaped inside the shell string.
	// Safe form: 'it'\''s'
	if !strings.Contains(got, `'it'\''s'`) {
		t.Errorf("renderPrePush single-quote name = %q, must use POSIX escaping", got)
	}
}
