package desktop

import (
	"strings"
	"unicode"
)

// maxNameRunes caps a sanitized name. The login name is never truncated
// further downstream, so this is the one place a long name is cut.
const maxNameRunes = 128

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

// SanitizeName cleans a name for display and logging. It replaces invalid
// UTF-8, drops control (Cc) and format (Cf) characters (bidi overrides and
// isolates, zero-width characters, NUL), turns every whitespace run into one
// space, trims, and caps at maxNameRunes.
//
// Why Cc and Cf and not a character allow-list: bidi and zero-width
// characters are format characters, not "unusual ASCII". A raw U+202E would
// make the name render reversed in the admit prompt or when the audit file is
// printed. A newline or tab is whitespace (Cc), so it collapses to a space
// rather than vanishing and gluing two words together.
func SanitizeName(s string) string {
	s = strings.ToValidUTF8(s, "�")
	var b strings.Builder
	pendingSpace := false
	for _, r := range s {
		switch {
		case unicode.IsSpace(r):
			pendingSpace = true
		case unicode.Is(unicode.Cc, r), unicode.Is(unicode.Cf, r):
			// dropped
		default:
			if pendingSpace && b.Len() > 0 {
				b.WriteByte(' ')
			}
			pendingSpace = false
			b.WriteRune(r)
		}
	}
	out := b.String()
	if r := []rune(out); len(r) > maxNameRunes {
		out = strings.TrimSpace(string(r[:maxNameRunes]))
	}
	return out
}
