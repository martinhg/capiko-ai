package engram

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// gitOriginURLRe finds the `url` field inside a [remote "origin"] section in
// a git config file. The `[^\[]*?` part matches across lines up to (but not
// including) the next section header.
var gitOriginURLRe = regexp.MustCompile(`(?m)^\[remote "origin"\][^\[]*?url\s*=\s*(\S+)`)

// gitOwnerRepoRe extracts the owner/repo slug from a git remote URL, stripping
// a trailing .git suffix. Works for both HTTPS and SSH URLs.
var gitOwnerRepoRe = regexp.MustCompile(`[:/]([^/:]+/[^/]+?)(?:\.git)?$`)

// InferEngramProject returns the project identifier for the Engram matching
// step, using the first match from this chain:
//  1. ENGRAM_PROJECT environment variable (if non-empty)
//  2. owner/repo from [remote "origin"] in <cwd>/.git/config (lowercased)
//  3. Lowercased basename of cwd
func InferEngramProject(cwd string) string {
	if p := strings.TrimSpace(os.Getenv("ENGRAM_PROJECT")); p != "" {
		return p
	}
	if p := projectFromGitConfig(cwd); p != "" {
		return p
	}
	return strings.ToLower(filepath.Base(cwd))
}

// projectFromGitConfig reads <cwd>/.git/config and extracts the lowercased
// owner/repo from the [remote "origin"] url field. Returns "" when the file
// is absent, the section is missing, or the URL does not match the expected
// shape — the caller falls back to the directory basename.
func projectFromGitConfig(cwd string) string {
	raw, err := os.ReadFile(filepath.Join(cwd, ".git", "config"))
	if err != nil {
		return ""
	}
	m := gitOriginURLRe.FindSubmatch(raw)
	if m == nil {
		return ""
	}
	m2 := gitOwnerRepoRe.FindSubmatch(m[1])
	if m2 == nil {
		return ""
	}
	return strings.ToLower(string(m2[1]))
}
