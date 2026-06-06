package release

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func makeTarGz(t *testing.T, name string, content []byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	if err := tw.WriteHeader(&tar.Header{Name: name, Mode: 0o755, Size: int64(len(content))}); err != nil {
		t.Fatal(err)
	}
	if _, err := tw.Write(content); err != nil {
		t.Fatal(err)
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func sha256hex(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

func TestMethodString(t *testing.T) {
	tests := []struct {
		method Method
		want   string
	}{
		{MethodBrew, "brew"},
		{MethodGo, "go"},
		{MethodBinary, "binary"},
		{MethodUnknown, "unknown"},
		{Method(99), "unknown"},
	}
	for _, tc := range tests {
		if got := tc.method.String(); got != tc.want {
			t.Errorf("Method(%d).String() = %q, want %q", tc.method, got, tc.want)
		}
	}
}

func TestDetectMethod(t *testing.T) {
	t.Setenv("GOBIN", "/home/dev/go/bin")
	t.Setenv("GOPATH", "")

	tests := []struct {
		name string
		path string
		want Method
	}{
		{"homebrew cellar", "/opt/homebrew/Cellar/capiko-ai/1.0.0/bin/capiko-ai", MethodBrew},
		{"go bin", "/home/dev/go/bin/capiko-ai", MethodGo},
		{"plain binary", "/usr/local/bin/capiko-ai", MethodBinary},
		{"empty", "", MethodUnknown},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := DetectMethod(tc.path); got != tc.want {
				t.Errorf("DetectMethod(%q) = %v, want %v", tc.path, got, tc.want)
			}
		})
	}
}

func TestVerifyChecksum(t *testing.T) {
	data := []byte("binary-archive-bytes")
	name := "capiko-ai_1.2.3_linux_amd64.tar.gz"
	good := fmt.Sprintf("%s  %s\n", sha256hex(data), name)

	if err := verifyChecksum(data, name, []byte(good)); err != nil {
		t.Errorf("matching checksum should pass: %v", err)
	}
	if err := verifyChecksum(data, name, []byte("deadbeef  "+name)); err == nil {
		t.Error("mismatched checksum should fail")
	}
	if err := verifyChecksum(data, name, []byte(sha256hex(data)+"  other.tar.gz")); err == nil {
		t.Error("missing entry should fail closed")
	}
}

func TestExtractBinary(t *testing.T) {
	want := []byte("#!/fake/capiko-ai-binary")
	archive := makeTarGz(t, "capiko-ai", want)

	got, err := extractBinary(archive, "capiko-ai")
	if err != nil {
		t.Fatalf("extractBinary: %v", err)
	}
	if !bytes.Equal(got, want) {
		t.Errorf("extracted = %q, want %q", got, want)
	}

	if _, err := extractBinary(makeTarGz(t, "something-else", want), "capiko-ai"); err == nil {
		t.Error("missing binary in archive should error")
	}
}

func TestReplaceExecutable(t *testing.T) {
	exe := filepath.Join(t.TempDir(), "capiko-ai")
	if err := os.WriteFile(exe, []byte("OLD"), 0o755); err != nil {
		t.Fatal(err)
	}

	if err := replaceExecutable(exe, []byte("NEW-BINARY")); err != nil {
		t.Fatalf("replaceExecutable: %v", err)
	}

	got, err := os.ReadFile(exe)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "NEW-BINARY" {
		t.Errorf("content = %q, want %q", got, "NEW-BINARY")
	}
	info, err := os.Stat(exe)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm()&0o100 == 0 {
		t.Errorf("replaced binary is not executable: %v", info.Mode())
	}
}

func TestBinaryUpgradeEndToEnd(t *testing.T) {
	newBinary := []byte("FRESH-CAPIKO-BINARY")
	archive := makeTarGz(t, "capiko-ai", newBinary)
	archiveName := fmt.Sprintf("capiko-ai_1.2.3_%s_%s.tar.gz", runtime.GOOS, runtime.GOARCH)
	checksums := fmt.Sprintf("%s  %s\n", sha256hex(archive), archiveName)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, archiveName):
			_, _ = w.Write(archive)
		case strings.HasSuffix(r.URL.Path, "checksums.txt"):
			_, _ = w.Write([]byte(checksums))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	restore := releaseBaseURL
	releaseBaseURL = srv.URL
	defer func() { releaseBaseURL = restore }()

	exe := filepath.Join(t.TempDir(), "capiko-ai")
	if err := os.WriteFile(exe, []byte("OLD"), 0o755); err != nil {
		t.Fatal(err)
	}

	if err := binaryUpgrade(context.Background(), exe, "1.2.3"); err != nil {
		t.Fatalf("binaryUpgrade: %v", err)
	}

	got, _ := os.ReadFile(exe)
	if !bytes.Equal(got, newBinary) {
		t.Errorf("binary not replaced: got %q, want %q", got, newBinary)
	}
}

func TestBinaryUpgradeRejectsBadChecksum(t *testing.T) {
	archive := makeTarGz(t, "capiko-ai", []byte("whatever"))
	archiveName := fmt.Sprintf("capiko-ai_1.2.3_%s_%s.tar.gz", runtime.GOOS, runtime.GOARCH)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, archiveName) {
			_, _ = w.Write(archive)
			return
		}
		_, _ = w.Write([]byte("deadbeef  " + archiveName + "\n"))
	}))
	defer srv.Close()

	restore := releaseBaseURL
	releaseBaseURL = srv.URL
	defer func() { releaseBaseURL = restore }()

	exe := filepath.Join(t.TempDir(), "capiko-ai")
	_ = os.WriteFile(exe, []byte("OLD"), 0o755)

	if err := binaryUpgrade(context.Background(), exe, "1.2.3"); err == nil {
		t.Error("expected checksum mismatch to abort the upgrade")
	}
	if got, _ := os.ReadFile(exe); string(got) != "OLD" {
		t.Errorf("binary should be untouched on failure, got %q", got)
	}
}

func TestBinaryUpgradeUnsupportedOnWindows(t *testing.T) {
	restore := goosFn
	goosFn = func() string { return "windows" }
	defer func() { goosFn = restore }()

	if err := binaryUpgrade(context.Background(), "/x/capiko-ai", "1.2.3"); err == nil {
		t.Error("Windows binary upgrade should return a manual-fallback error")
	}
}

func TestDownloadRejectsNon200(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	}))
	defer srv.Close()

	if _, err := download(context.Background(), srv.URL+"/missing.tar.gz"); err == nil {
		t.Error("download should error on a non-200 response")
	}
}

func TestReplaceExecutableFailsOnMissingDir(t *testing.T) {
	exe := filepath.Join(t.TempDir(), "no-such-dir", "capiko-ai")
	if err := replaceExecutable(exe, []byte("data")); err == nil {
		t.Error("replaceExecutable should error when the target dir does not exist")
	}
}

func TestRestartReExecsResolvedPath(t *testing.T) {
	var gotExe string
	var called bool

	restoreExec, restoreLook, restoreOS := reExec, lookExe, goosFn
	reExec = func(argv0 string, argv []string, envv []string) error {
		called, gotExe = true, argv0
		return nil
	}
	lookExe = func(string) (string, error) { return "/opt/homebrew/bin/capiko-ai", nil }
	goosFn = func() string { return "darwin" }
	defer func() { reExec, lookExe, goosFn = restoreExec, restoreLook, restoreOS }()

	if err := Restart(); err != nil {
		t.Fatalf("Restart: %v", err)
	}
	if !called {
		t.Fatal("Restart did not re-exec")
	}
	if gotExe != "/opt/homebrew/bin/capiko-ai" {
		t.Errorf("re-exec path = %q, want the PATH-resolved binary", gotExe)
	}
}

func TestRestartNoOpOnWindows(t *testing.T) {
	restoreExec, restoreOS := reExec, goosFn
	called := false
	reExec = func(string, []string, []string) error { called = true; return nil }
	goosFn = func() string { return "windows" }
	defer func() { reExec, goosFn = restoreExec, restoreOS }()

	if err := Restart(); err != nil {
		t.Fatalf("Restart: %v", err)
	}
	if called {
		t.Error("Restart should be a no-op on Windows")
	}
}
