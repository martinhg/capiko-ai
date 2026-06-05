package tui

import "testing"

func reviewFor(t *testing.T, s *selector) *reviewScreen {
	t.Helper()
	rv, ok := newReview(s).(*reviewScreen)
	if !ok {
		t.Fatal("newReview did not return *reviewScreen")
	}
	return rv
}

func TestReviewShowsPlan(t *testing.T) {
	s := installScreen(t, t.TempDir())
	s.Update(key("space")) // mark capiko-hello (cursor 0)

	rv := reviewFor(t, s)
	if len(rv.install) != 1 || rv.install[0] != "capiko-hello" {
		t.Errorf("install plan = %v, want [capiko-hello]", rv.install)
	}
	if len(rv.remove) != 0 {
		t.Errorf("remove plan = %v, want none", rv.remove)
	}
}

func TestReviewApplyTriggersReconcile(t *testing.T) {
	s := installScreen(t, t.TempDir())
	s.Update(key("space"))

	rv := reviewFor(t, s)
	next, cmd := rv.Update(key("enter")) // cursor 0 = Apply
	if next != screen(s) {
		t.Errorf("Apply should hand back to the selector, got %T", next)
	}
	if s.state != selApplying {
		t.Errorf("selector state = %d, want selApplying", s.state)
	}
	if _, ok := cmd().(reconciledMsg); !ok {
		t.Error("Apply should run the reconcile")
	}
}

func TestReviewBackReturnsToSelector(t *testing.T) {
	s := installScreen(t, t.TempDir())
	s.Update(key("space"))

	rv := reviewFor(t, s)
	rv.cursor = 1 // Back
	next, _ := rv.Update(key("enter"))
	if next != screen(s) {
		t.Errorf("Back should return the selector, got %T", next)
	}

	rv2 := reviewFor(t, s)
	next, _ = rv2.Update(key("esc"))
	if next != screen(s) {
		t.Errorf("esc should return the selector, got %T", next)
	}
}
