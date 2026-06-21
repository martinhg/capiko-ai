package engram

import (
	"fmt"
	"os/exec"
	"regexp"

	"github.com/martinhg/capiko-ai/internal/release"
)

// repoURL is engram's canonical source, surfaced in the upgrade advisory. capiko
// configures engram, it never installs or upgrades the binary.
const repoURL = "https://github.com/Gentleman-Programming/engram"

// engramVersionRe matches a MAJOR.MINOR.PATCH triple — the shape release.IsNewer
// needs. The first match in `engram --version` output is the installed version
// (an update nag like "1.16.3 -> 1.17.0" still yields the installed one first).
var engramVersionRe = regexp.MustCompile(`\d+\.\d+\.\d+`)

// runOut runs an engram subcommand and returns its combined output. It is a test
// seam so version probing never shells out during tests.
var runOut = func(args ...string) (string, error) {
	out, err := exec.Command("engram", args...).CombinedOutput()
	return string(out), err
}

// InstalledVersion returns engram's reported MAJOR.MINOR.PATCH, or "" when engram
// is not on PATH or its version cannot be parsed.
func InstalledVersion() string {
	out, err := runOut("--version")
	if err != nil {
		return ""
	}
	return engramVersionRe.FindString(out)
}

// OutdatedAdvisory returns a one-line upgrade advisory when engram is managed by
// capiko, installed, and behind recommended; "" otherwise. It is the single
// decision point shared by the doctor, headless sync, and TUI sync surfaces.
// capiko never upgrades engram itself — this only informs. managed gates on
// whether capiko configured engram, so users who do not manage engram through
// capiko (or never installed it) see nothing.
func OutdatedAdvisory(managed bool, recommended string) string {
	if !managed || recommended == "" {
		return ""
	}
	v := InstalledVersion()
	if v == "" || !release.IsNewer(v, recommended) {
		return ""
	}
	return fmt.Sprintf("engram %s is behind the recommended %s — upgrade from %s", v, recommended, repoURL)
}
