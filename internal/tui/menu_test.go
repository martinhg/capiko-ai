package tui

import (
	"strings"
	"testing"
)

func TestMenuStarBadgeOnUpgrade(t *testing.T) {
	withUpdate := App{state: appMenu, catalog: testCatalog(), latest: "9.9.9"}.viewMenu()
	if !strings.Contains(withUpdate, "Upgrade tools ★") {
		t.Error("expected a ★ badge on Upgrade tools when an update is available")
	}

	upToDate := App{state: appMenu, catalog: testCatalog()}.viewMenu()
	if strings.Contains(upToDate, "★") {
		t.Error("no star should show when there is no update")
	}
}

func TestMenuCursorOnFocusedItem(t *testing.T) {
	view := App{state: appMenu, catalog: testCatalog(), cursor: 0}.viewMenu()
	if !strings.Contains(view, menuCursor+"Start installation") {
		t.Errorf("focused item should be prefixed with %q", menuCursor)
	}
}
