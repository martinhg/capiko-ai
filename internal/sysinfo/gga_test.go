package sysinfo

import (
	"strings"
	"testing"
)

func TestDependencySpecsIncludesGGAOptional(t *testing.T) {
	found := false
	for _, s := range dependencySpecs("darwin") {
		if s.name == "gga" {
			found = true
			if s.required {
				t.Error("gga should be optional (capiko configures it; it is not required to run)")
			}
		}
	}
	if !found {
		t.Error("dependencySpecs should list gga so detection surfaces it")
	}
}

func TestInstallInfoGGAUsesBrewTapAndIsNotAuto(t *testing.T) {
	cmd, auto := installInfo("gga", "brew")
	if !strings.Contains(cmd, "gentleman-programming/tap/gga") {
		t.Errorf("gga install hint should point at the brew tap, got %q", cmd)
	}
	if auto {
		t.Error("gga must not be one-click auto-installed: capiko configures gga, never installs the binary")
	}
}
