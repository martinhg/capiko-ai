package tui

import (
	"runtime/debug"
	"strings"
)

// Version is the capiko-ai release. goreleaser injects the real value via
// -ldflags (-X .../internal/tui.Version=X.Y.Z) at build time. The "dev" sentinel
// marks an un-injected local build and is resolved further in init below.
var Version = "dev"

// devVersion is the sentinel for a build that goreleaser did not stamp.
const devVersion = "dev"

func init() {
	Version = resolveVersion(Version, mainModuleVersion())
}

// mainModuleVersion returns the version recorded in the binary's build info, or
// "" when it is unavailable.
func mainModuleVersion() string {
	if info, ok := debug.ReadBuildInfo(); ok {
		return info.Main.Version
	}
	return ""
}

// resolveVersion keeps an ldflags-injected version as-is. Otherwise it recovers
// the version from the module build info, which is a clean tag (vX.Y.Z) for
// `go install module@vX.Y.Z`. A plain `go build`/`go run` or `go install @branch`
// yields "(devel)" or a pseudo-version (vX.Y.Z-0.<timestamp>-<commit>); those are
// not releases, so they stay "dev" rather than leaking an ugly id into the UI.
func resolveVersion(injected, build string) string {
	if injected != devVersion {
		return injected
	}
	v := strings.TrimPrefix(strings.TrimSpace(build), "v")
	if isReleaseVersion(v) {
		return v
	}
	return injected
}

// isReleaseVersion reports whether v is a clean MAJOR.MINOR.PATCH triple. It
// rejects "(devel)", empty strings, pre-releases, and pseudo-versions (which
// carry a "-<timestamp>-<commit>" suffix, so they split into more than three
// dot-separated parts or contain non-numeric segments).
func isReleaseVersion(v string) bool {
	parts := strings.Split(v, ".")
	if len(parts) != 3 {
		return false
	}
	for _, p := range parts {
		if p == "" {
			return false
		}
		for _, r := range p {
			if r < '0' || r > '9' {
				return false
			}
		}
	}
	return true
}
