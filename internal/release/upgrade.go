package release

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
)

// Method is how capiko-ai was installed, which decides how it upgrades itself.
type Method int

const (
	MethodUnknown Method = iota
	MethodBrew
	MethodGo
	MethodBinary
)

func (m Method) String() string {
	switch m {
	case MethodBrew:
		return "brew"
	case MethodGo:
		return "go"
	case MethodBinary:
		return "binary"
	default:
		return "unknown"
	}
}

// Test seams: redirected in tests so no real network, exec, or process replace
// happens.
var (
	releaseBaseURL = fmt.Sprintf("https://github.com/%s/%s/releases/download", owner, repo)

	goosFn = func() string { return runtime.GOOS }
	archFn = func() string { return runtime.GOARCH }

	reExec  = syscall.Exec
	lookExe = exec.LookPath
)

// DetectMethod infers the install method from the running executable's resolved
// path. Homebrew binaries live under a Cellar; go-installed binaries live under
// GOBIN / GOPATH/bin; anything else is treated as a downloaded binary.
func DetectMethod(exePath string) Method {
	if exePath == "" {
		return MethodUnknown
	}
	p := filepath.ToSlash(exePath)
	switch {
	case strings.Contains(p, "/Cellar/") || strings.Contains(p, "/homebrew/"):
		return MethodBrew
	case underGoBin(p):
		return MethodGo
	default:
		return MethodBinary
	}
}

func underGoBin(p string) bool {
	var dirs []string
	if gobin := os.Getenv("GOBIN"); gobin != "" {
		dirs = append(dirs, gobin)
	}
	if gopath := os.Getenv("GOPATH"); gopath != "" {
		dirs = append(dirs, filepath.Join(gopath, "bin"))
	}
	if home, err := os.UserHomeDir(); err == nil {
		dirs = append(dirs, filepath.Join(home, "go", "bin"))
	}
	for _, d := range dirs {
		if d != "" && strings.HasPrefix(p, filepath.ToSlash(d)+"/") {
			return true
		}
	}
	return false
}

// Upgrade upgrades capiko-ai to the given release using the method inferred from
// the running binary. On success the caller should Restart so the new binary
// takes over.
func Upgrade(ctx context.Context, latest string) error {
	exe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("locate executable: %w", err)
	}
	switch DetectMethod(exe) {
	case MethodBrew:
		return brewUpgrade(ctx)
	case MethodGo:
		return goUpgrade(ctx, latest)
	case MethodBinary:
		return binaryUpgrade(ctx, exe, latest)
	default:
		return fmt.Errorf("could not determine how capiko-ai was installed; please update manually")
	}
}

func brewUpgrade(ctx context.Context) error {
	// Re-tap and refresh first; both are non-fatal so a stale cache or missing
	// tap never blocks the actual upgrade.
	_ = exec.CommandContext(ctx, "brew", "tap", owner+"/homebrew-tap").Run()
	_ = exec.CommandContext(ctx, "brew", "update").Run()

	cmd := exec.CommandContext(ctx, "brew", "upgrade", repo)
	cmd.Stdin = nil
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("brew upgrade %s: %w (%s)", repo, err, strings.TrimSpace(string(out)))
	}
	return nil
}

func goUpgrade(ctx context.Context, latest string) error {
	target := fmt.Sprintf("github.com/%s/%s@v%s", owner, repo, strings.TrimPrefix(latest, "v"))
	cmd := exec.CommandContext(ctx, "go", "install", target)
	cmd.Stdin = nil
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("go install %s: %w (%s)", target, err, strings.TrimSpace(string(out)))
	}
	return nil
}

// binaryUpgrade downloads the release archive for this OS/arch, verifies its
// checksum, and atomically replaces the running binary. Windows is intentionally
// unsupported: replacing a running .exe in place is fragile, so the user is told
// to download manually (matching the install scripts' fallback).
func binaryUpgrade(ctx context.Context, exe, latest string) error {
	if goosFn() == "windows" {
		return fmt.Errorf("automatic binary upgrade is not supported on Windows; download the latest release from https://github.com/%s/%s/releases", owner, repo)
	}

	version := strings.TrimPrefix(latest, "v")
	archive := fmt.Sprintf("%s_%s_%s_%s.tar.gz", repo, version, goosFn(), archFn())
	base := fmt.Sprintf("%s/v%s", releaseBaseURL, version)

	archiveBytes, err := download(ctx, base+"/"+archive)
	if err != nil {
		return fmt.Errorf("download %s: %w", archive, err)
	}

	sums, err := download(ctx, base+"/checksums.txt")
	if err != nil {
		return fmt.Errorf("download checksums: %w", err)
	}
	if err := verifyChecksum(archiveBytes, archive, sums); err != nil {
		return err
	}

	binData, err := extractBinary(archiveBytes, repo)
	if err != nil {
		return err
	}
	return replaceExecutable(exe, binData)
}

func download(ctx context.Context, url string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "capiko-ai")

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	return io.ReadAll(io.LimitReader(resp.Body, 64<<20))
}

// verifyChecksum fails closed: a missing entry or a mismatch both abort.
func verifyChecksum(data []byte, name string, checksums []byte) error {
	want := ""
	for _, line := range strings.Split(string(checksums), "\n") {
		fields := strings.Fields(line)
		if len(fields) == 2 && fields[1] == name {
			want = fields[0]
			break
		}
	}
	if want == "" {
		return fmt.Errorf("checksum for %s not found; refusing to install unverified binary", name)
	}
	sum := sha256.Sum256(data)
	if got := hex.EncodeToString(sum[:]); got != want {
		return fmt.Errorf("checksum mismatch for %s: want %s, got %s", name, want, got)
	}
	return nil
}

// extractBinary pulls the named file out of a gzipped tarball.
func extractBinary(archive []byte, name string) ([]byte, error) {
	gz, err := gzip.NewReader(bytes.NewReader(archive))
	if err != nil {
		return nil, fmt.Errorf("open gzip: %w", err)
	}
	defer gz.Close()

	tr := tar.NewReader(gz)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("read tar: %w", err)
		}
		if filepath.Base(hdr.Name) == name {
			return io.ReadAll(io.LimitReader(tr, 256<<20))
		}
	}
	return nil, fmt.Errorf("binary %q not found in archive", name)
}

// replaceExecutable writes the new binary next to the current one and renames it
// over the top. The temp file shares the target's directory so the rename stays
// on one filesystem and is therefore atomic; on Unix a running binary can be
// replaced this way.
func replaceExecutable(exe string, data []byte) error {
	dir := filepath.Dir(exe)
	tmp, err := os.CreateTemp(dir, ".capiko-ai-*")
	if err != nil {
		return fmt.Errorf("create temp binary: %w", err)
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName) // no-op once the rename succeeds

	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return fmt.Errorf("write temp binary: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close temp binary: %w", err)
	}
	if err := os.Chmod(tmpName, 0o755); err != nil {
		return fmt.Errorf("chmod temp binary: %w", err)
	}
	if err := os.Rename(tmpName, exe); err != nil {
		return fmt.Errorf("replace binary: %w", err)
	}
	return nil
}

// Restart re-executes capiko-ai so the freshly installed binary takes over. On
// Windows it is a no-op (the caller prints a "please restart" message) because
// the binary path may still be held by the old process. The PATH lookup is
// preferred over os.Executable() because Homebrew's symlink points at the new
// version while os.Executable() may resolve to the old Cellar path.
func Restart() error {
	if goosFn() == "windows" {
		return nil
	}
	exe, err := lookExe(repo)
	if err != nil {
		exe, err = os.Executable()
		if err != nil {
			return err
		}
	}
	return reExec(exe, os.Args, os.Environ())
}
