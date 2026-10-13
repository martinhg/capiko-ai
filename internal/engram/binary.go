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

const engramRepo = "Gentleman-Programming/engram"

var (
	latestURL  = fmt.Sprintf("https://api.github.com/repos/%s/releases/latest", engramRepo)
	httpClient = &http.Client{Timeout: 5 * time.Second}
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
