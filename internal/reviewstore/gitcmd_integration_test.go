package reviewstore

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// --- Real temp git repo integration tests ---
//
// These exercise gitcmd.go's seams against an actual `git` binary and a
// throwaway repository in t.TempDir(), proving the command flags/argument
// order produce the output gitcmd.go's parsers expect — the unit tests in
// gitcmd_test.go cover parsing logic in isolation, but only a real git
// process can prove `git rev-parse --is-bare-repository`,
// `--is-shallow-repository`, etc. actually behave as gitcmd.go assumes.

// initTestRepo creates a real git repository in a t.TempDir(), with local
// user identity configured (so commits don't depend on global git config
// in CI), and returns its workspace path.
func initTestRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	runGitCmd(t, dir, "init", "-q", "-b", "main")
	runGitCmd(t, dir, "config", "user.email", "test@example.com")
	runGitCmd(t, dir, "config", "user.name", "Test")
	return dir
}

// runGitCmd runs git with args against dir, failing the test on error. Used
// only to set up fixtures for integration tests — production code always
// goes through the gitcmd.go seams.
func runGitCmd(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s failed: %v\n%s", strings.Join(args, " "), err, out)
	}
	return strings.TrimSpace(string(out))
}

// writeFile is a small fixture helper writing content to name under dir.
func writeFile(t *testing.T, dir, name, content string) error {
	t.Helper()
	return os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644)
}

func TestGitCommonDir_RealRepo(t *testing.T) {
	dir := initTestRepo(t)

	got, err := GitCommonDir(dir)
	if err != nil {
		t.Fatalf("GitCommonDir() error = %v, want nil", err)
	}
	want, err := filepath.EvalSymlinks(filepath.Join(dir, ".git"))
	if err != nil {
		t.Fatalf("failed to resolve expected .git dir: %v", err)
	}
	gotResolved, err := filepath.EvalSymlinks(got)
	if err != nil {
		t.Fatalf("failed to resolve GitCommonDir() result %q: %v", got, err)
	}
	if gotResolved != want {
		t.Errorf("GitCommonDir() = %q, want %q", gotResolved, want)
	}
}

func TestGitWriteTreeAndRevParseTree_RealRepo(t *testing.T) {
	dir := initTestRepo(t)

	if err := writeFile(t, dir, "a.txt", "hello\n"); err != nil {
		t.Fatalf("writeFile() error = %v", err)
	}
	runGitCmd(t, dir, "add", "a.txt")
	runGitCmd(t, dir, "commit", "-q", "-m", "initial")

	headTree, err := gitRevParseTree(dir, "HEAD")
	if err != nil {
		t.Fatalf("gitRevParseTree() error = %v, want nil", err)
	}
	if headTree == "" {
		t.Error("gitRevParseTree() returned empty tree hash for HEAD")
	}

	writtenTree, err := gitWriteTree(dir)
	if err != nil {
		t.Fatalf("gitWriteTree() error = %v, want nil", err)
	}
	if writtenTree != headTree {
		t.Errorf("gitWriteTree() = %q, want %q (matches unchanged index)", writtenTree, headTree)
	}
}

func TestGitLsTreeAndDiffTree_RealRepo(t *testing.T) {
	dir := initTestRepo(t)

	if err := writeFile(t, dir, "a.txt", "hello\n"); err != nil {
		t.Fatalf("writeFile() error = %v", err)
	}
	runGitCmd(t, dir, "add", "a.txt")
	runGitCmd(t, dir, "commit", "-q", "-m", "initial")
	baseTree := runGitCmd(t, dir, "rev-parse", "HEAD^{tree}")

	entries, err := gitLsTree(dir, baseTree)
	if err != nil {
		t.Fatalf("gitLsTree() error = %v, want nil", err)
	}
	if len(entries) != 1 || entries[0].Path != "a.txt" || entries[0].Mode != "100644" {
		t.Errorf("gitLsTree() = %+v, want [{a.txt 100644}]", entries)
	}

	if err := writeFile(t, dir, "b.txt", "world\n"); err != nil {
		t.Fatalf("writeFile() error = %v", err)
	}
	runGitCmd(t, dir, "add", "b.txt")
	runGitCmd(t, dir, "commit", "-q", "-m", "second")
	candidateTree := runGitCmd(t, dir, "rev-parse", "HEAD^{tree}")

	changed, err := gitDiffTree(dir, baseTree, candidateTree)
	if err != nil {
		t.Fatalf("gitDiffTree() error = %v, want nil", err)
	}
	if len(changed) != 1 || changed[0].Path != "b.txt" || changed[0].Mode != "100644" {
		t.Errorf("gitDiffTree() = %+v, want [{b.txt 100644}]", changed)
	}
}

func TestGitRemoteOriginURL_RealRepo(t *testing.T) {
	dir := initTestRepo(t)
	runGitCmd(t, dir, "remote", "add", "origin", "https://example.com/repo.git")

	got, err := gitRemoteOriginURL(dir)
	if err != nil {
		t.Fatalf("gitRemoteOriginURL() error = %v, want nil", err)
	}
	if got != "https://example.com/repo.git" {
		t.Errorf("gitRemoteOriginURL() = %q, want %q", got, "https://example.com/repo.git")
	}
}

func TestGitRemoteOriginURL_NoOrigin_RealRepo(t *testing.T) {
	dir := initTestRepo(t)

	if _, err := gitRemoteOriginURL(dir); err == nil {
		t.Error("gitRemoteOriginURL() error = nil, want error when no origin remote is configured")
	}
}

func TestGitIsBareRepo_RealRepo(t *testing.T) {
	normal := initTestRepo(t)
	if got, err := gitIsBareRepo(normal); err != nil || got {
		t.Errorf("gitIsBareRepo(normal) = (%v, %v), want (false, nil)", got, err)
	}

	bareDir := t.TempDir()
	runGitCmd(t, bareDir, "init", "-q", "--bare", "-b", "main")
	if got, err := gitIsBareRepo(bareDir); err != nil || !got {
		t.Errorf("gitIsBareRepo(bare) = (%v, %v), want (true, nil)", got, err)
	}
}

func TestGitIsShallowRepo_RealRepo(t *testing.T) {
	full := initTestRepo(t)
	if err := writeFile(t, full, "a.txt", "hello\n"); err != nil {
		t.Fatalf("writeFile() error = %v", err)
	}
	runGitCmd(t, full, "add", "a.txt")
	runGitCmd(t, full, "commit", "-q", "-m", "initial")

	if got, err := gitIsShallowRepo(full); err != nil || got {
		t.Errorf("gitIsShallowRepo(full clone) = (%v, %v), want (false, nil)", got, err)
	}

	shallow := t.TempDir()
	runGitCmd(t, shallow, "clone", "-q", "--depth", "1", "file://"+full, shallow+"/clone")
	clonePath := filepath.Join(shallow, "clone")
	if got, err := gitIsShallowRepo(clonePath); err != nil || !got {
		t.Errorf("gitIsShallowRepo(shallow clone) = (%v, %v), want (true, nil)", got, err)
	}
}
