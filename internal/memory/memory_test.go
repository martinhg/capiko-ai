package memory

import (
	"strings"
	"testing"

	"github.com/martinhg/capiko-ai/internal/efficiency"
	"github.com/martinhg/capiko-ai/internal/persona"
	"github.com/martinhg/capiko-ai/internal/sdd"
)

func TestMarkersAreDistinctAndNamespaced(t *testing.T) {
	if MarkerStart == MarkerEnd {
		t.Fatal("start and end markers must differ")
	}
	for _, m := range []string{MarkerStart, MarkerEnd} {
		if !strings.Contains(m, "capiko:memory") {
			t.Errorf("marker %q is not namespaced to capiko:memory", m)
		}
	}
	// Must not overlap with other managed block markers.
	for _, other := range []string{
		efficiency.MarkerStart, efficiency.MarkerEnd,
		sdd.MarkerStart, sdd.MarkerEnd,
		persona.MarkerStart, persona.MarkerEnd,
	} {
		if MarkerStart == other || MarkerEnd == other {
			t.Errorf("memory marker collides with another managed marker: %q", other)
		}
	}
}

func TestRenderReturnsProtocol(t *testing.T) {
	block := strings.ToLower(Render())
	if block == "" {
		t.Fatal("Render returned an empty block")
	}
	for _, want := range []string{"search", "mem_save", "proactiv"} {
		if !strings.Contains(block, want) {
			t.Errorf("memory block missing %q", want)
		}
	}
}

func TestRenderCoversTriggers(t *testing.T) {
	block := strings.ToLower(Render())
	for _, want := range []string{"decision", "root cause", "mem_session_summary"} {
		if !strings.Contains(block, want) {
			t.Errorf("memory block missing trigger keyword %q", want)
		}
	}
}

func TestRenderLifecycleAware(t *testing.T) {
	block := strings.ToLower(Render())
	for _, want := range []string{"active", "needs_review"} {
		if !strings.Contains(block, want) {
			t.Errorf("memory block missing lifecycle keyword %q", want)
		}
	}
}

func TestRenderMilestoneAdvisory(t *testing.T) {
	block := strings.ToLower(Render())
	// Must carry the no-timer or milestone advisory.
	if !strings.Contains(block, "no timer") && !strings.Contains(block, "milestone") {
		t.Error("memory block must carry milestone/no-timer advisory")
	}
	// Must NOT promise a literal timer or scheduled reminder.
	for _, forbidden := range []string{"every 15", "every fifteen", "reminder"} {
		if strings.Contains(block, forbidden) {
			t.Errorf("memory block must not contain runtime-promise phrase %q", forbidden)
		}
	}
	// The word "timer" is only allowed in the context of "no timer".
	if strings.Contains(block, "timer") && !strings.Contains(block, "no timer") {
		t.Error("memory block contains 'timer' outside of 'no timer' advisory")
	}
}
