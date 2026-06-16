package release

import (
	"context"
	"encoding/json"
	"io"
	"strings"
)

const advisoryTag = "advisory"

// advisoryURL is a package-level var so tests can redirect the request.
var advisoryURL = "https://api.github.com/repos/" + owner + "/" + repo + "/releases/tags/" + advisoryTag

// Advisory fetches the advisory message from the dedicated GitHub release tag.
// A missing tag, empty body, or any error returns an empty string — the feature
// is strictly fail-open and must never block startup.
func Advisory(ctx context.Context) string {
	req, err := newGitHubRequest(ctx, advisoryURL)
	if err != nil {
		return ""
	}
	resp, err := httpClient.Do(req)
	if err != nil {
		return ""
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return ""
	}

	var payload struct {
		Body string `json:"body"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&payload); err != nil {
		return ""
	}
	return SanitizeAdvisory(payload.Body)
}

// SanitizeAdvisory strips ANSI escape sequences and control characters, takes
// only the first line, and caps the result at 200 characters. Exported so tests
// can verify sanitization independently.
func SanitizeAdvisory(s string) string {
	var b strings.Builder
	inEscape := false
	for _, r := range s {
		if r == '\x1b' {
			inEscape = true
			continue
		}
		if inEscape {
			if (r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z') {
				inEscape = false
			}
			continue
		}
		if r == '\n' || r == '\r' {
			break
		}
		if r < 0x20 && r != '\t' {
			continue
		}
		b.WriteRune(r)
	}
	result := strings.TrimSpace(b.String())
	if len(result) > 200 {
		result = result[:200] + "…"
	}
	return result
}
