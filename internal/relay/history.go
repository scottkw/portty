package relay

import "strings"

// HistoryMaxLines and HistoryMaxBytes bound the hub's rendered-line history
// (SHAR-07, Phase 5 scrollback decision: about 10,000 lines, memory only).
// Measured: 10,000 rendered lines of about 107 columns is about 1.1 MiB. The
// byte cap bounds truecolor-heavy lines, which can run to several KB each.
const (
	HistoryMaxLines = 10_000
	HistoryMaxBytes = 4 << 20
)

// History is a bounded ring of rendered terminal lines (self-contained ANSI per
// line, no line terminator), oldest first. It is never written to disk:
// terminal contents include secrets, so a daemon restart drops it.
//
// It has no mutex of its own: the hub touches it only under Hub.emuMu.
type History struct {
	lines    []string // lines[head:] are the retained lines
	head     int
	bytes    int // total bytes of the retained lines
	maxLines int
	maxBytes int
}

// NewHistory returns an empty History holding at most maxLines lines and
// maxBytes bytes.
func NewHistory(maxLines, maxBytes int) *History {
	return &History{maxLines: maxLines, maxBytes: maxBytes}
}

// Push appends line, then drops oldest lines while either cap is exceeded. A
// single line over the byte cap is dropped too.
func (h *History) Push(line string) {
	h.lines = append(h.lines, line)
	h.bytes += len(line)
	for h.Len() > 0 && (h.Len() > h.maxLines || h.bytes > h.maxBytes) {
		h.bytes -= len(h.lines[h.head])
		h.lines[h.head] = ""
		h.head++
	}
	// Compact once the dropped prefix is half the slice: amortized O(1) per
	// push, and the backing array stays bounded at about twice maxLines.
	if h.head > len(h.lines)/2 {
		n := copy(h.lines, h.lines[h.head:])
		clear(h.lines[n:])
		h.lines = h.lines[:n]
		h.head = 0
	}
}

// Clear drops every line and releases the storage.
func (h *History) Clear() {
	h.lines, h.head, h.bytes = nil, 0, 0
}

// Len returns the number of retained lines.
func (h *History) Len() int { return len(h.lines) - h.head }

// Render joins the retained lines with CRLF, with no trailing CRLF.
func (h *History) Render() string { return strings.Join(h.lines[h.head:], "\r\n") }
