package main

import "fmt"

// trayTooltip returns the tooltip string for the given session count.
// Uses an em dash (U+2014) as specified in the UI-SPEC copywriting contract.
func trayTooltip(n int) string {
	switch n {
	case 0:
		return "Portty \u2014 no sessions"
	case 1:
		return "Portty \u2014 1 session"
	default:
		return fmt.Sprintf("Portty \u2014 %d sessions", n)
	}
}

// MenuItemKind classifies tray menu entries.
type MenuItemKind int

const (
	MenuItemAction    MenuItemKind = iota // clickable item
	MenuItemSeparator                     // visual divider
)

// MenuItem represents a single entry in the tray context menu.
type MenuItem struct {
	Kind      MenuItemKind
	Label     string
	SessionID string // non-empty for session items
	Index     int    // session index for callback dispatch
}

// BuildMenuItems constructs the canonical tray menu structure:
// "Open Portty" -> separator -> [sessions] -> separator -> "Quit Portty"
// The first separator is always present. When sessions is empty the result is:
// Open Portty, separator, separator, Quit Portty (4 items).
// This is the single source of truth for menu layout across all platforms.
func BuildMenuItems(sessions []SessionInfo) []MenuItem {
	items := []MenuItem{
		{Kind: MenuItemAction, Label: "Open Portty"},
		{Kind: MenuItemSeparator},
	}
	for i, s := range sessions {
		items = append(items, MenuItem{
			Kind:      MenuItemAction,
			Label:     s.Name,
			SessionID: s.ID,
			Index:     i,
		})
	}
	items = append(items,
		MenuItem{Kind: MenuItemSeparator},
		MenuItem{Kind: MenuItemAction, Label: "Quit Portty"},
	)
	return items
}

// Identifier and title of the quit-to-menu-bar notification (NOTF-01).
const (
	quitNotificationID    = "portty.quit-gui-only"
	quitNotificationTitle = "Portty"
)

// quitToMenuBarBody is the quit-to-menu-bar notification body for n open sessions.
func quitToMenuBarBody(n int) string {
	switch n {
	case 0:
		return "Portty is still running in the menu bar."
	case 1:
		return "Portty is still running in the menu bar with 1 session open."
	default:
		return fmt.Sprintf("Portty is still running in the menu bar with %d sessions open.", n)
	}
}
