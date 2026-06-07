package tui

import (
	"strings"
	"testing"

	"github.com/martinhg/capiko-ai/internal/copilot"
	"github.com/martinhg/capiko-ai/internal/state"
)

func TestStaleBannerVisibility(t *testing.T) {
	tests := []struct {
		name  string
		stale []string
		shown bool
		want  string
	}{
		{"hidden when none", nil, false, ""},
		{"singular", []string{"a"}, true, "1 skill out of date"},
		{"plural", []string{"a", "b"}, true, "2 skills out of date"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			b := App{stale: tc.stale}.staleBanner()
			if shown := strings.TrimSpace(b) != ""; shown != tc.shown {
				t.Fatalf("shown = %v, want %v (banner=%q)", shown, tc.shown, b)
			}
			if tc.want != "" && !strings.Contains(b, tc.want) {
				t.Errorf("banner %q missing %q", b, tc.want)
			}
		})
	}
}

func TestDetectComputesStale(t *testing.T) {
	store := state.NewStore(t.TempDir())
	// Record capiko-hello with a checksum that does not match the catalog content.
	if err := store.Apply("1.0.0", []state.Installed{{Name: "capiko-hello", Checksum: "stale"}}, nil); err != nil {
		t.Fatal(err)
	}

	next, _ := NewApp(testCatalog(), nil, store, nil).Update(detectedMsg{
		host:      &copilot.Host{SkillsDir: t.TempDir()},
		installed: map[string]bool{"capiko-hello": true},
	})
	got := next.(App).stale
	if len(got) != 1 || got[0] != "capiko-hello" {
		t.Errorf("stale = %v, want [capiko-hello]", got)
	}
}

func TestDetectNoStaleWhenChecksumsMatch(t *testing.T) {
	cat := testCatalog()
	store := state.NewStore(t.TempDir())
	if err := store.Apply("1.0.0", []state.Installed{{Name: cat[0].Name, Checksum: state.Checksum(cat[0].Content)}}, nil); err != nil {
		t.Fatal(err)
	}

	next, _ := NewApp(cat, nil, store, nil).Update(detectedMsg{
		host:      &copilot.Host{SkillsDir: t.TempDir()},
		installed: map[string]bool{cat[0].Name: true},
	})
	if got := next.(App).stale; len(got) != 0 {
		t.Errorf("stale = %v, want none", got)
	}
}
