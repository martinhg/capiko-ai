package engram

import (
	"errors"
	"strings"
	"testing"
)

// stubRunOut swaps the engram --version probe for the duration of a test.
func stubRunOut(t *testing.T, out string, err error) {
	t.Helper()
	prev := runOut
	runOut = func(_ ...string) (string, error) { return out, err }
	t.Cleanup(func() { runOut = prev })
}

func TestInstalledVersionParses(t *testing.T) {
	// engram --version may print an update nag; the installed version comes first.
	stubRunOut(t, "Update available: 1.16.3 -> 1.17.0", nil)
	if v := InstalledVersion(); v != "1.16.3" {
		t.Errorf("want 1.16.3, got %q", v)
	}
}

func TestInstalledVersionNotOnPath(t *testing.T) {
	stubRunOut(t, "", errors.New("exec: engram not found"))
	if v := InstalledVersion(); v != "" {
		t.Errorf("want empty when engram absent, got %q", v)
	}
}

func TestOutdatedAdvisoryWarnsWhenBehind(t *testing.T) {
	stubRunOut(t, "engram 1.16.3", nil)
	got := OutdatedAdvisory(true, "1.17.0")
	if !strings.Contains(got, "1.16.3") || !strings.Contains(got, "1.17.0") {
		t.Errorf("want advisory naming both versions, got %q", got)
	}
}

func TestOutdatedAdvisoryCurrentIsEmpty(t *testing.T) {
	stubRunOut(t, "engram 1.17.0", nil)
	if got := OutdatedAdvisory(true, "1.17.0"); got != "" {
		t.Errorf("want empty when current, got %q", got)
	}
}

func TestOutdatedAdvisoryUnmanagedIsEmpty(t *testing.T) {
	stubRunOut(t, "engram 1.16.3", nil) // outdated, but capiko does not manage it
	if got := OutdatedAdvisory(false, "1.17.0"); got != "" {
		t.Errorf("want empty when unmanaged, got %q", got)
	}
}

func TestOutdatedAdvisoryNotInstalledIsEmpty(t *testing.T) {
	stubRunOut(t, "", errors.New("not found"))
	if got := OutdatedAdvisory(true, "1.17.0"); got != "" {
		t.Errorf("want empty when not installed, got %q", got)
	}
}
