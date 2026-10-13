package engram

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestInferEngramProject_EnvVar(t *testing.T) {
	t.Setenv("ENGRAM_PROJECT", "acme/my-service")
	cwd := t.TempDir()
	if got := InferEngramProject(cwd); got != "acme/my-service" {
		t.Errorf("got %q, want acme/my-service", got)
	}
}

func TestInferEngramProject_GitConfigHTTPS(t *testing.T) {
	t.Setenv("ENGRAM_PROJECT", "")
	cwd := t.TempDir()
	gitDir := filepath.Join(cwd, ".git")
	if err := os.MkdirAll(gitDir, 0o755); err != nil {
		t.Fatal(err)
	}
	cfg := "[core]\n\trepositoryformatversion = 0\n[remote \"origin\"]\n\turl = https://github.com/myorg/myrepo.git\n\tfetch = +refs/heads/*:refs/remotes/origin/*\n"
	if err := os.WriteFile(filepath.Join(gitDir, "config"), []byte(cfg), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := InferEngramProject(cwd); got != "myorg/myrepo" {
		t.Errorf("got %q, want myorg/myrepo", got)
	}
}

func TestInferEngramProject_GitConfigSSH(t *testing.T) {
	t.Setenv("ENGRAM_PROJECT", "")
	cwd := t.TempDir()
	gitDir := filepath.Join(cwd, ".git")
	if err := os.MkdirAll(gitDir, 0o755); err != nil {
		t.Fatal(err)
	}
	cfg := "[remote \"origin\"]\n\turl = git@github.com:myorg/myrepo.git\n"
	if err := os.WriteFile(filepath.Join(gitDir, "config"), []byte(cfg), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := InferEngramProject(cwd); got != "myorg/myrepo" {
		t.Errorf("got %q, want myorg/myrepo", got)
	}
}

func TestInferEngramProject_DirBasename(t *testing.T) {
	t.Setenv("ENGRAM_PROJECT", "")
	// No .git/config — falls back to lowercased basename.
	cwd := t.TempDir()
	got := InferEngramProject(cwd)
	want := strings.ToLower(filepath.Base(cwd))
	if got != want {
		t.Errorf("got %q, want %q (lowercased basename)", got, want)
	}
}

func TestProjectFromGitConfig_NoGitDir(t *testing.T) {
	cwd := t.TempDir()
	if got := projectFromGitConfig(cwd); got != "" {
		t.Errorf("got %q, want empty when .git/config is absent", got)
	}
}

func TestProjectFromGitConfig_NoOriginSection(t *testing.T) {
	cwd := t.TempDir()
	gitDir := filepath.Join(cwd, ".git")
	if err := os.MkdirAll(gitDir, 0o755); err != nil {
		t.Fatal(err)
	}
	cfg := "[core]\n\trepositoryformatversion = 0\n"
	if err := os.WriteFile(filepath.Join(gitDir, "config"), []byte(cfg), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := projectFromGitConfig(cwd); got != "" {
		t.Errorf("got %q, want empty when no origin section", got)
	}
}
