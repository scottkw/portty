package main

import (
	"fmt"
	"strings"
	"testing"
)

// TestTrayTooltip verifies the tooltip string formatting with em dash.
func TestTrayTooltip(t *testing.T) {
	tests := []struct {
		n    int
		want string
	}{
		{0, "Portty \u2014 no sessions"},
		{1, "Portty \u2014 1 session"},
		{3, "Portty \u2014 3 sessions"},
	}
	for _, tt := range tests {
		got := trayTooltip(tt.n)
		if got != tt.want {
			t.Errorf("trayTooltip(%d) = %q, want %q", tt.n, got, tt.want)
		}
	}
}

// TestBuildMenuItemsEmpty verifies menu structure with no sessions:
// Open Portty, separator, separator, Quit Portty (4 items total).
func TestBuildMenuItemsEmpty(t *testing.T) {
	items := BuildMenuItems(nil)
	if len(items) != 4 {
		t.Fatalf("expected 4 items with no sessions, got %d", len(items))
	}
	if items[0].Kind != MenuItemAction || items[0].Label != "Open Portty" {
		t.Errorf("items[0] = %+v, want {Kind: MenuItemAction, Label: Open Portty}", items[0])
	}
	if items[1].Kind != MenuItemSeparator {
		t.Errorf("items[1] = %+v, want MenuItemSeparator", items[1])
	}
	if items[2].Kind != MenuItemSeparator {
		t.Errorf("items[2] = %+v, want MenuItemSeparator", items[2])
	}
	if items[3].Kind != MenuItemAction || items[3].Label != "Quit Portty" {
		t.Errorf("items[3] = %+v, want {Kind: MenuItemAction, Label: Quit Portty}", items[3])
	}
}

// TestBuildMenuItemsWithSessions verifies menu structure with 2 sessions:
// Open Portty, separator, sess1, sess2, separator, Quit Portty (6 items total).
// Also verifies SessionID and Index fields are correctly set.
func TestBuildMenuItemsWithSessions(t *testing.T) {
	sessions := []SessionInfo{
		{ID: "id-1", Name: "Session 1"},
		{ID: "id-2", Name: "Session 2"},
	}
	items := BuildMenuItems(sessions)
	if len(items) != 6 {
		t.Fatalf("expected 6 items with 2 sessions, got %d", len(items))
	}
	// Open Portty
	if items[0].Kind != MenuItemAction || items[0].Label != "Open Portty" {
		t.Errorf("items[0] = %+v, want Open Portty action", items[0])
	}
	// Separator before sessions
	if items[1].Kind != MenuItemSeparator {
		t.Errorf("items[1] = %+v, want separator", items[1])
	}
	// Session 1
	if items[2].Kind != MenuItemAction || items[2].Label != "Session 1" || items[2].SessionID != "id-1" || items[2].Index != 0 {
		t.Errorf("items[2] = %+v, want Session 1 / id-1 / index 0", items[2])
	}
	// Session 2
	if items[3].Kind != MenuItemAction || items[3].Label != "Session 2" || items[3].SessionID != "id-2" || items[3].Index != 1 {
		t.Errorf("items[3] = %+v, want Session 2 / id-2 / index 1", items[3])
	}
	// Separator after sessions
	if items[4].Kind != MenuItemSeparator {
		t.Errorf("items[4] = %+v, want separator", items[4])
	}
	// Quit
	if items[5].Kind != MenuItemAction || items[5].Label != "Quit Portty" {
		t.Errorf("items[5] = %+v, want Quit Portty action", items[5])
	}
}

// TestBuildMenuItemsLabels verifies that "Open Portty" is first and "Quit Portty" is last
// regardless of session count.
func TestBuildMenuItemsLabels(t *testing.T) {
	for _, n := range []int{0, 1, 5} {
		sessions := make([]SessionInfo, n)
		for i := range sessions {
			sessions[i] = SessionInfo{ID: fmt.Sprintf("id-%d", i), Name: fmt.Sprintf("Session %d", i+1)}
		}
		items := BuildMenuItems(sessions)
		if items[0].Label != "Open Portty" {
			t.Errorf("n=%d: items[0].Label = %q, want Open Portty", n, items[0].Label)
		}
		if items[len(items)-1].Label != "Quit Portty" {
			t.Errorf("n=%d: last item.Label = %q, want Quit Portty", n, items[len(items)-1].Label)
		}
	}
}

// TestQuitToMenuBarBody locks the 0/1/n quit-to-menu-bar copy and the
// notification identifier and title (NOTF-01).
func TestQuitToMenuBarBody(t *testing.T) {
	tests := []struct {
		n    int
		want string
	}{
		{0, "Portty is still running in the menu bar."},
		{1, "Portty is still running in the menu bar with 1 session open."},
		{3, "Portty is still running in the menu bar with 3 sessions open."},
	}
	for _, tt := range tests {
		got := quitToMenuBarBody(tt.n)
		if got != tt.want {
			t.Errorf("quitToMenuBarBody(%d) = %q, want %q", tt.n, got, tt.want)
		}
		if strings.Contains(got, "1 sessions") || strings.Contains(got, "0 sessions") {
			t.Errorf("quitToMenuBarBody(%d) = %q has a bad plural", tt.n, got)
		}
	}
	if quitNotificationID != "portty.quit-gui-only" || quitNotificationTitle != "Portty" {
		t.Errorf("notification id/title = %q/%q, want portty.quit-gui-only/Portty", quitNotificationID, quitNotificationTitle)
	}
}
