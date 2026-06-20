package efficiency

import (
	"strings"
	"testing"
)

func TestRenderTrimsCeremony(t *testing.T) {
	block := strings.ToLower(Render())
	if block == "" {
		t.Fatal("Render returned an empty block")
	}
	for _, want := range []string{"preamble", "unchanged", "terse"} {
		if !strings.Contains(block, want) {
			t.Errorf("efficiency block missing %q:\n%s", want, block)
		}
	}
}

func TestRenderPreservesRigor(t *testing.T) {
	// The block must keep full rigor on new questions / errors, not just cut tokens.
	block := strings.ToLower(Render())
	if !strings.Contains(block, "error") || !strings.Contains(block, "rigor") {
		t.Errorf("block must preserve rigor on errors / new questions:\n%s", Render())
	}
}

func TestMarkersAreDistinctAndNamespaced(t *testing.T) {
	if MarkerStart == MarkerEnd {
		t.Fatal("start and end markers must differ")
	}
	for _, m := range []string{MarkerStart, MarkerEnd} {
		if !strings.Contains(m, "capiko:efficiency") {
			t.Errorf("marker %q is not namespaced to capiko:efficiency", m)
		}
	}
}
