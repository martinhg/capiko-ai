package tui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/martinhg/capiko-ai/internal/copilot"
	"github.com/martinhg/capiko-ai/internal/copilothooks"
	"github.com/martinhg/capiko-ai/internal/state"
)

// TestCopilotHooksPostureCycles asserts the posture dropdown row cycles
// off -> warn -> strict -> off forward (space/right) and backward (left).
func TestCopilotHooksPostureCycles(t *testing.T) {
	s := newCopilotHooks(services{}).(*copilotHooksScreen)
	if s.posture != copilothooks.PostureOff {
		t.Fatalf("default posture = %q, want off", s.posture)
	}

	// Cursor starts on the posture row; space cycles forward.
	forward := []copilothooks.Posture{copilothooks.PostureWarn, copilothooks.PostureStrict, copilothooks.PostureOff}
	for _, want := range forward {
		s.Update(key(" "))
		if s.posture != want {
			t.Errorf("after space posture = %q, want %q", s.posture, want)
		}
	}

	// Left cycles backward: off -> strict.
	s.Update(key("left"))
	if s.posture != copilothooks.PostureStrict {
		t.Errorf("after left posture = %q, want strict", s.posture)
	}
}

// TestCopilotHooksApplyFromStrictWritesFileAndDone drives Apply from the strict
// posture: enter on Apply emits a cmd whose message is copilotHooksAppliedMsg,
// the hook file lands on disk, and feeding the message back moves to Done.
func TestCopilotHooksApplyFromStrictWritesFile(t *testing.T) {
	hooksDir := filepath.Join(t.TempDir(), "hooks")
	svc := services{host: &copilot.Host{HooksDir: hooksDir}, state: state.NewStore(t.TempDir())}
	s := newCopilotHooks(svc).(*copilotHooksScreen)

	s.Update(key(" ")) // off -> warn
	s.Update(key(" ")) // warn -> strict
	if s.posture != copilothooks.PostureStrict {
		t.Fatalf("posture = %q, want strict", s.posture)
	}

	s.Update(key("down")) // move to Apply
	_, cmd := s.Update(key("enter"))
	if cmd == nil {
		t.Fatal("enter on Apply should return a command")
	}
	msg, ok := cmd().(copilotHooksAppliedMsg)
	if !ok {
		t.Fatalf("Apply should emit copilotHooksAppliedMsg, got %T", cmd())
	}
	if msg.err != nil {
		t.Fatalf("apply failed: %v", msg.err)
	}

	if _, err := os.Stat(filepath.Join(hooksDir, copilothooks.GuardrailsFile)); err != nil {
		t.Errorf("Apply should have written the guardrails file: %v", err)
	}

	next, _ := s.Update(msg)
	if next.(*copilotHooksScreen).state != copilotHooksDone {
		t.Errorf("state = %d, want Done", next.(*copilotHooksScreen).state)
	}
}

// TestCopilotHooksApplyFromOffDisables asserts Apply from the off posture routes
// to disable: no guardrails file is left on disk and state records disabled.
func TestCopilotHooksApplyFromOffDisables(t *testing.T) {
	hooksDir := filepath.Join(t.TempDir(), "hooks")
	store := state.NewStore(t.TempDir())
	svc := services{host: &copilot.Host{HooksDir: hooksDir}, state: store}
	s := newCopilotHooks(svc).(*copilotHooksScreen)

	if s.posture != copilothooks.PostureOff {
		t.Fatalf("posture = %q, want off", s.posture)
	}
	s.Update(key("down")) // move to Apply
	_, cmd := s.Update(key("enter"))
	if cmd == nil {
		t.Fatal("enter on Apply should return a command")
	}
	if msg := cmd().(copilotHooksAppliedMsg); msg.err != nil {
		t.Fatalf("apply(off) failed: %v", msg.err)
	}

	if _, err := os.Stat(filepath.Join(hooksDir, copilothooks.GuardrailsFile)); !os.IsNotExist(err) {
		t.Errorf("off posture should leave no guardrails file, stat err = %v", err)
	}
	st, _ := store.Load()
	if st.CopilotHooks == nil || st.CopilotHooks.Enabled {
		t.Errorf("off posture should record disabled, got %+v", st.CopilotHooks)
	}
}

// TestCopilotHooksAppliedTransitions covers the terminal FSM edges: a nil-error
// message goes to Done, an error to Failed, and any key from a terminal state
// returns to the menu.
func TestCopilotHooksAppliedTransitions(t *testing.T) {
	tests := []struct {
		name string
		msg  copilotHooksAppliedMsg
		want copilotHooksState
	}{
		{"success goes to done", copilotHooksAppliedMsg{}, copilotHooksDone},
		{"error goes to failed", copilotHooksAppliedMsg{err: errTest}, copilotHooksFailed},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := newCopilotHooks(services{}).(*copilotHooksScreen)
			s.Update(tt.msg)
			if s.state != tt.want {
				t.Fatalf("state = %d, want %d", s.state, tt.want)
			}
			_, cmd := s.Update(key("enter")) // any key from a terminal state
			if cmd == nil {
				t.Fatal("a terminal state should go back on any key")
			}
			if _, ok := cmd().(backMsg); !ok {
				t.Error("terminal state should emit backMsg on any key")
			}
		})
	}
}

// TestCopilotHooksHydratesFromState asserts newCopilotHooks seeds the posture
// from a recorded CopilotHooksRecord, defaulting to off when unmanaged.
func TestCopilotHooksHydratesFromState(t *testing.T) {
	store := state.NewStore(t.TempDir())
	if err := store.SetCopilotHooks(&state.CopilotHooksRecord{Enabled: true, Posture: string(copilothooks.PostureWarn)}); err != nil {
		t.Fatal(err)
	}
	s := newCopilotHooks(services{state: store}).(*copilotHooksScreen)
	if s.posture != copilothooks.PostureWarn {
		t.Errorf("hydrated posture = %q, want warn", s.posture)
	}

	// A store with no record defaults to off.
	empty := newCopilotHooks(services{state: state.NewStore(t.TempDir())}).(*copilotHooksScreen)
	if empty.posture != copilothooks.PostureOff {
		t.Errorf("unmanaged posture = %q, want off", empty.posture)
	}
}

// TestCopilotHooksBackRowGoesBack asserts enter on the Back row returns to the
// menu.
func TestCopilotHooksBackRowGoesBack(t *testing.T) {
	s := newCopilotHooks(services{}).(*copilotHooksScreen)
	s.cursor = copilotHooksRows - 1 // Back row

	_, cmd := s.Update(key("enter"))
	if cmd == nil {
		t.Fatal("enter on Back should emit a command")
	}
	if _, ok := cmd().(backMsg); !ok {
		t.Error("enter on the Back row should go back to the menu")
	}
}

// TestCopilotHooksViewShowsFutureNotes asserts the editing view surfaces the
// repo-level future note and the admin-policy override note (REQ-9.3).
func TestCopilotHooksViewShowsFutureNotes(t *testing.T) {
	view := newCopilotHooks(services{}).View()
	if !strings.Contains(view, "user-level") {
		t.Errorf("view should note the user-level scope:\n%s", view)
	}
	if !strings.Contains(strings.ToLower(view), "policy") {
		t.Errorf("view should note the org-policy override:\n%s", view)
	}
}
