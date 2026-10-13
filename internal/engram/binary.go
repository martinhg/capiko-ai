package engram

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os/exec"
	"strings"
	"time"
)

const (
	engramRepo   = "Gentleman-Programming/engram"
	engramModule = "github.com/Gentleman-Programming/engram/v3/cmd/engram"
)

var (
	latestURL  = fmt.Sprintf("https://api.github.com/repos/%s/releases/latest", engramRepo)
	httpClient = &http.Client{Timeout: 5 * time.Second}

	// runGoInstall is a test seam so Upgrade never shells out during tests.
	runGoInstall = func(ctx context.Context, pkg string) (string, error) {
		cmd := exec.CommandContext(ctx, "go", "install", pkg)
		out, err := cmd.CombinedOutput()
		return string(out), err
	}
)

// Detected reports whether the engram binary is findable on PATH.
func Detected() bool {
	_, err := exec.LookPath("engram")
	return err == nil
}

// LatestVersion fetches the most recent published engram release version from
// GitHub, without a leading "v". Returns an error when the network call fails
// or the repo has no releases.
func LatestVersion(ctx context.Context) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, latestURL, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "capiko-ai")

	resp, err := httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("engram releases: HTTP %d", resp.StatusCode)
	}

	var payload struct {
		TagName string `json:"tag_name"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&payload); err != nil {
		return "", err
	}

	tag := strings.TrimSpace(payload.TagName)
	if tag == "" {
		return "", fmt.Errorf("engram releases: empty tag_name")
	}
	return strings.TrimPrefix(tag, "v"), nil
}

// GoAvailable reports whether the go toolchain is on PATH.
func GoAvailable() bool {
	_, err := exec.LookPath("go")
	return err == nil
}

// Upgrade installs or upgrades engram to the given version via go install.
// version should be a semver string without the "v" prefix (e.g. "3.4.0").
// An empty version installs @latest.
func Upgrade(ctx context.Context, version string) error {
	if !GoAvailable() {
		return fmt.Errorf("go toolchain not found on PATH; install Go first")
	}
	pkg := engramModule + "@latest"
	if version != "" {
		pkg = engramModule + "@v" + strings.TrimPrefix(version, "v")
	}
	out, err := runGoInstall(ctx, pkg)
	if err != nil {
		return fmt.Errorf("go install engram: %s: %w", strings.TrimSpace(out), err)
	}
	return nil
}
