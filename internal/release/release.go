// Package release checks GitHub Releases for a newer capiko-ai version. It is
// the read-only half of self-update: it answers "is there a newer release?" so
// the TUI can show the update banner. Actually applying an upgrade lives
// elsewhere.
package release

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

const (
	owner = "martinhg"
	repo  = "capiko-ai"
)

// latestURL and httpClient are package-level vars so tests can redirect the
// request to a local server without making real network calls.
var (
	latestURL = fmt.Sprintf("https://api.github.com/repos/%s/%s/releases/latest", owner, repo)

	httpClient = &http.Client{Timeout: 5 * time.Second}
)

// newGitHubRequest builds a GET request with the headers GitHub expects.
func newGitHubRequest(ctx context.Context, url string) (*http.Request, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "capiko-ai")
	return req, nil
}

// Latest returns the most recent published release version, without a leading
// "v". A repository with no releases yet (HTTP 404) returns an error, which
// callers treat as "no update".
func Latest(ctx context.Context) (string, error) {
	req, err := newGitHubRequest(ctx, latestURL)
	if err != nil {
		return "", err
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("github releases: HTTP %d", resp.StatusCode)
	}

	var payload struct {
		TagName string `json:"tag_name"`
	}
	// Cap the body so a misbehaving endpoint cannot exhaust memory.
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&payload); err != nil {
		return "", err
	}

	tag := strings.TrimSpace(payload.TagName)
	if tag == "" {
		return "", fmt.Errorf("github releases: empty tag_name")
	}
	return strings.TrimPrefix(tag, "v"), nil
}

// ReleaseURL returns the GitHub Releases URL for a given version.
func ReleaseURL(version string) string {
	v := strings.TrimPrefix(strings.TrimSpace(version), "v")
	return fmt.Sprintf("https://github.com/%s/%s/releases/tag/v%s", owner, repo, v)
}

// IsNewer reports whether latest is a strictly higher MAJOR.MINOR.PATCH than
// current. Either value may carry a leading "v". A current version that carries
// pre-release or build metadata (a local or snapshot build, e.g.
// "0.0.0-SNAPSHOT-abc") is never reported as outdated, so dev builds are not
// nagged. Non-numeric inputs return false.
func IsNewer(current, latest string) bool {
	c := strings.TrimPrefix(strings.TrimSpace(current), "v")
	if strings.ContainsAny(c, "-+") {
		return false
	}

	cur, ok1 := parseSemver(c)
	lat, ok2 := parseSemver(latest)
	if !ok1 || !ok2 {
		return false
	}

	for i := 0; i < 3; i++ {
		if lat[i] != cur[i] {
			return lat[i] > cur[i]
		}
	}
	return false
}

// parseSemver extracts the leading MAJOR.MINOR.PATCH triple, ignoring any
// pre-release or build metadata. It reports false when the core is not three
// non-negative integers.
func parseSemver(v string) ([3]int, bool) {
	var out [3]int
	v = strings.TrimPrefix(strings.TrimSpace(v), "v")
	if i := strings.IndexAny(v, "-+"); i >= 0 {
		v = v[:i]
	}

	parts := strings.Split(v, ".")
	if len(parts) != 3 {
		return out, false
	}
	for i, p := range parts {
		n, err := strconv.Atoi(p)
		if err != nil || n < 0 {
			return out, false
		}
		out[i] = n
	}
	return out, true
}
