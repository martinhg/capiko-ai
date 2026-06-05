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
// the version from the module build info, which is set for `go install
// module@vX.Y.Z` but is "(devel)" (or empty) for a plain `go build`/`go run` —
// those stay "dev" so local builds are not mistaken for a release.
func resolveVersion(injected, build string) string {
	if injected != devVersion {
		return injected
	}
	if v := strings.TrimPrefix(strings.TrimSpace(build), "v"); v != "" && v != "(devel)" {
		return v
	}
	return injected
}
