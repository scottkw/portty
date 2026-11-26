package main

import (
	"fmt"

	"github.com/scottkw/portty/internal/daemon"
)

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
	MenuItemInfo                          // dimmed, unclickable text (the desktop line)
)

// MenuItem represents a single entry in the tray context menu.
type MenuItem struct {
	Kind      MenuItemKind
	Label     string
	SessionID string // non-empty for session items
	Index     int    // session index for callback dispatch
}

// BuildMenuItems constructs the canonical tray menu structure:
// "Open Portty" -> separator -> [sessions] -> separator -> [desktop line,
// separator] -> "Quit Portty"
// The first separator is always present. When sessions is empty and desktopLine
// is "" the result is: Open Portty, separator, separator, Quit Portty (4 items).
// A non-empty desktopLine adds one inert info item and one separator before Quit.
// This is the single source of truth for menu layout across all platforms.
func BuildMenuItems(sessions []SessionInfo, desktopLine string) []MenuItem {
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
	items = append(items, MenuItem{Kind: MenuItemSeparator})
	if desktopLine != "" {
		items = append(items,
			MenuItem{Kind: MenuItemInfo, Label: desktopLine},
			MenuItem{Kind: MenuItemSeparator},
		)
	}
	items = append(items, MenuItem{Kind: MenuItemAction, Label: "Quit Portty"})
	return items
}

// desktopTrayLine is the Tray column of the 19-UI-SPEC state table. It is ""
// while always-available is off or the state is not one the table names. Waiting
// for Tailscale wins over every daemon state because nothing else can be reached.
func desktopTrayLine(st daemon.DesktopStatusResponse) string {
	if !st.AlwaysAvailable {
		return ""
	}
	switch st.Tailscale {
	case "off", "userspace", "https_off":
		return "Desktop: Waiting for Tailscale"
	}
	switch st.State {
	case "needs_permission":
		return "Desktop: Needs permission"
	case "awaiting_approval":
		return "Desktop: Waiting for approval at the Mac"
	case "display_asleep":
		return "Desktop: Display asleep"
	case "capture_paused":
		return "Desktop: Paused after repeated errors"
	case "locked":
		if st.Viewers > 0 {
			return "Desktop: Locked, streaming to " + viewerCount(st.Viewers)
		}
		return "Desktop: Locked"
	case "streaming":
		return "Desktop: Streaming to " + viewerCount(st.Viewers)
	case "idle":
		return "Desktop: Ready"
	}
	return ""
}

func viewerCount(n int) string {
	if n == 1 {
		return "1 viewer"
	}
	return fmt.Sprintf("%d viewers", n)
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
