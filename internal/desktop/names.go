package desktop

import "strings"

// Identity is who a guest (or the sharer's own account) is, as reported by
// the tailnet's WhoIs. Every name field is text another person chose, so
// NewIdentity is the only way to build one and sanitizes them at the source:
// the GUI, notifications, signalling and the audit log all receive clean text.
type Identity struct {
	UserID      int64
	LoginName   string
	DisplayName string
	NodeName    string
	StableID    string
	TailnetIP   string
}

// NewIdentity builds an Identity, routing the three free-text names through
// SanitizeName.
func NewIdentity(userID int64, login, display, node, stableID, ip string) Identity {
	return Identity{
		UserID:      userID,
		LoginName:   SanitizeName(login),
		DisplayName: SanitizeName(display),
		NodeName:    SanitizeName(node),
		StableID:    stableID,
		TailnetIP:   ip,
	}
}

// SanitizeName cleans a name for display and logging.
func SanitizeName(s string) string {
	return strings.TrimSpace(s)
}
