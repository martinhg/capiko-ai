package versions

import (
	"regexp"
	"testing"
)

// TestPinnedVersionsAreConcrete guards against a marker comment losing its
// pinned value (e.g. an empty or templated version), which would break both the
// version output and Renovate's regex manager.
func TestPinnedVersionsAreConcrete(t *testing.T) {
	semverish := regexp.MustCompile(`^\d+\.\d+\.\d+`)

	pins := map[string]string{
		"CopilotCLI": CopilotCLI,
	}
	for name, value := range pins {
		if !semverish.MatchString(value) {
			t.Errorf("%s = %q, want a concrete version", name, value)
		}
	}
}
