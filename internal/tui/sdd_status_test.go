package tui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/martinhg/capiko-ai/internal/sddstatus"
)

// statusFixture builds a resolved-looking Status with the given dependency states
// and task progress, so view/navigation tests need no filesystem.
func sddChangeFixture(name string, next string, deps sddstatus.Dependencies, done, total int) sddChange {
	return sddChange{
		name: name,
		status: sddstatus.Status{
			ChangeName:      &name,
			NextRecommended: next,
			Dependencies:    deps,
			TaskProgress:    sddstatus.TaskProgress{Completed: done, Total: total},
		},
	}
}

func TestSDDStatusListShowsChanges(t *testing.T) {
	s := &sddStatusScreen{entries: []sddChange{
		sddChangeFixture("add-auth", "apply", sddstatus.Dependencies{Apply: sddstatus.DependencyReady}, 2, 5),
		sddChangeFixture("rework-tui", "spec", sddstatus.Dependencies{Specs: sddstatus.DependencyReady}, 0, 0),
	}}
	v := s.View()
	if !strings.Contains(v, "add-auth") || !strings.Contains(v, "rework-tui") {
		t.Errorf("list view should show every active change:\n%s", v)
	}
	if !strings.Contains(v, "apply") || !strings.Contains(v, "spec") {
		t.Errorf("list view should show each change's next phase:\n%s", v)
	}
}

func TestSDDStatusEmptyState(t *testing.T) {
	s := &sddStatusScreen{}
	if !strings.Contains(s.View(), "No active SDD changes") {
		t.Errorf("empty state should tell the user there are no changes:\n%s", s.View())
	}
	_, cmd := s.Update(key("esc"))
	if cmd == nil {
		t.Fatal("esc on empty state should return a command")
	}
	if _, ok := cmd().(backMsg); !ok {
		t.Error("esc on the list should go back to the menu")
	}
}

func TestSDDStatusEnterOpensDetail(t *testing.T) {
	s := &sddStatusScreen{entries: []sddChange{
		sddChangeFixture("add-auth", "apply", sddstatus.Dependencies{
			Proposal: sddstatus.DependencyAllDone,
			Specs:    sddstatus.DependencyAllDone,
			Design:   sddstatus.DependencyAllDone,
			Tasks:    sddstatus.DependencyAllDone,
			Apply:    sddstatus.DependencyReady,
		}, 2, 5),
	}}
	next, _ := s.Update(key("enter"))
	ds := next.(*sddStatusScreen)
	if !ds.detail {
		t.Fatal("enter should open the detail view")
	}
	v := ds.View()
	for _, phase := range []string{"proposal", "specs", "design", "tasks", "apply", "verify", "archive"} {
		if !strings.Contains(v, phase) {
			t.Errorf("detail view should render the %q phase row:\n%s", phase, v)
		}
	}
	if !strings.Contains(v, "2/5") {
		t.Errorf("detail view should show task progress:\n%s", v)
	}
}

func TestSDDStatusDetailEscReturnsToList(t *testing.T) {
	s := &sddStatusScreen{
		detail:  true,
		entries: []sddChange{sddChangeFixture("add-auth", "apply", sddstatus.Dependencies{}, 0, 0)},
	}
	next, cmd := s.Update(key("esc"))
	ds := next.(*sddStatusScreen)
	if ds.detail {
		t.Error("esc in detail should return to the list, not stay in detail")
	}
	if cmd != nil {
		t.Error("esc in detail should not emit a command (stays on screen)")
	}
}

func TestSDDStatusDetailShowsBlockedReasons(t *testing.T) {
	name := "add-auth"
	s := &sddStatusScreen{
		detail: true,
		entries: []sddChange{{
			name: name,
			status: sddstatus.Status{
				ChangeName:     &name,
				BlockedReasons: []string{"proposal.md is missing or partial."},
			},
		}},
	}
	if !strings.Contains(s.View(), "proposal.md is missing or partial.") {
		t.Errorf("detail view should surface blocked reasons:\n%s", s.View())
	}
}

func TestSDDStatusCursorClamps(t *testing.T) {
	s := &sddStatusScreen{entries: []sddChange{
		sddChangeFixture("a", "spec", sddstatus.Dependencies{}, 0, 0),
		sddChangeFixture("b", "spec", sddstatus.Dependencies{}, 0, 0),
	}}
	s.Update(key("up")) // already at top
	if s.cursor != 0 {
		t.Errorf("cursor = %d, want 0", s.cursor)
	}
	s.Update(key("down"))
	s.Update(key("down")) // clamp at last
	if s.cursor != 1 {
		t.Errorf("cursor = %d, want 1", s.cursor)
	}
}

// TestSDDStatusReloadResolvesActiveChanges drives newSDDStatus against a real
// OpenSpec tree (via the getwd seam) so it exercises the actual status engine, not
// a stub: a change directory under openspec/changes must surface as an entry.
func TestSDDStatusReloadResolvesActiveChanges(t *testing.T) {
	root := t.TempDir()
	changeDir := filepath.Join(root, "openspec", "changes", "add-auth")
	if err := os.MkdirAll(changeDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(changeDir, "proposal.md"), []byte("# Proposal\n\nDo the thing."), 0o644); err != nil {
		t.Fatal(err)
	}

	prev := sddStatusGetwd
	sddStatusGetwd = func() (string, error) { return root, nil }
	t.Cleanup(func() { sddStatusGetwd = prev })

	s := newSDDStatus(services{}).(*sddStatusScreen)
	if s.err != nil {
		t.Fatalf("reload errored: %v", s.err)
	}
	if len(s.entries) != 1 || s.entries[0].name != "add-auth" {
		t.Fatalf("entries = %+v, want one resolved change add-auth", s.entries)
	}
	if s.entries[0].status.ChangeName == nil || *s.entries[0].status.ChangeName != "add-auth" {
		t.Error("entry should carry the resolved status from the engine")
	}
}
