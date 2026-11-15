// Package relay — sanitize.go provides SanitizeChatContent, the display-safety
// transform applied to chat content before it is persisted, broadcast or exported.
package relay

import "strings"

// SanitizeChatContent strips display-dangerous bytes from chat message content
// before it is persisted to chat.jsonl, broadcast to chat clients, and emitted
// into the Markdown Export(). It preserves the text the user typed while
// removing the renderer-level spoofing vectors that the chat display/export
// surfaces are subject to (WR-01 / SEC-02).
//
// Transformation rules:
//   - C0 control characters (U+0000–U+001F, including ESC, CR, LF, TAB) are
//     stripped. Stripping ESC removes the 2-byte introducer of CSI/OSC/DCS/APC/PM/SOS
//     sequences, but body bytes (which are above U+001F) survive as printable
//     plaintext in the output. DCS body content in chat is cosmetically confusing
//     but is neutralized by react-markdown + rehype-sanitize before rendering.
//   - DEL (U+007F) is stripped.
//   - C1 controls (U+0080–U+009F) are stripped.
//   - Unicode bidi-override characters (see isBidiOverride) are stripped to
//     mitigate Trojan-Source class attacks (CVE-2021-42574).
//
// Stored content is therefore "display-safe text", not "raw keystrokes": the
// dangerous bytes never reach chat.jsonl, BroadcastChat, or Export().
func SanitizeChatContent(input string) string {
	var b strings.Builder
	b.Grow(len(input))
	for _, r := range input {
		switch {
		case r >= 0x00 && r <= 0x1F:
			// C0 control (includes ESC, CR, LF, TAB): strip
		case r == 0x7F:
			// DEL: strip
		case r >= 0x80 && r <= 0x9F:
			// C1 control (Unicode U+0080–U+009F): strip
		case isBidiOverride(r):
			// Bidi override (Trojan-Source class): strip
		default:
			b.WriteRune(r)
		}
	}
	return b.String()
}

// isBidiOverride returns true for Unicode bidi-override and directional
// formatting characters that can be exploited to spoof terminal output
// (Trojan-Source class, CVE-2021-42574).
func isBidiOverride(r rune) bool {
	switch r {
	case 0x061C, // ARABIC LETTER MARK (ALM)
		0x200E, 0x200F, // LRM, RLM
		0x202A, 0x202B, 0x202C, 0x202D, 0x202E, // LRE, RLE, PDF, LRO, RLO
		0x2066, 0x2067, 0x2068, 0x2069: // LRI, RLI, FSI, PDI
		return true
	}
	return false
}
