package sddstatus

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// artifactStoreRe matches an artifact_store or artifactStore YAML key and captures
// the value. Case-insensitive multiline to handle either key variant.
var artifactStoreRe = regexp.MustCompile(`(?mi)^\s*artifact[_]?[Ss]tore\s*:\s*["']?([A-Za-z]+)`)

// configArtifactStoreIsEngram reports whether the openspec config file declares
// artifact_store (or artifactStore) as "engram" or "hybrid". It reads
// openspec/config.yaml first, then openspec/config.yml. No YAML dependency — a
// narrow regex over the raw text is sufficient.
func configArtifactStoreIsEngram(cwd string) bool {
	for _, name := range []string{"config.yaml", "config.yml"} {
		p := filepath.Join(cwd, "openspec", name)
		raw, err := os.ReadFile(p)
		if err != nil {
			continue
		}
		m := artifactStoreRe.FindSubmatch(raw)
		if m == nil {
			continue
		}
		val := strings.ToLower(string(m[1]))
		if val == "engram" || val == "hybrid" {
			return true
		}
	}
	return false
}

// shouldTryEngram reports whether the Engram fallback is enabled for the given
// workspace. Any one of the three triggers is independently sufficient:
//
//   - CAPIKO_SDD_STATUS_ENGRAM environment variable is set (any non-empty value)
//   - A .engram/ directory exists at <cwd>/.engram
//   - openspec/config.yaml or openspec/config.yml declares artifact_store: engram|hybrid
func shouldTryEngram(cwd string) bool {
	if os.Getenv("CAPIKO_SDD_STATUS_ENGRAM") != "" {
		return true
	}
	if info, err := os.Stat(filepath.Join(cwd, ".engram")); err == nil && info.IsDir() {
		return true
	}
	return configArtifactStoreIsEngram(cwd)
}
