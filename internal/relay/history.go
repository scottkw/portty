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
// line, no line terminator), oldest first. A line pushed as wrapped soft-wrapped
// onto the next one on the host (DEBT-02), so Render joins the two without a
// CRLF and the viewer re-wraps them. It is never written to disk: terminal
// contents include secrets, so a daemon restart drops it. An upgrade handoff
// moves it memory to memory over the 0600 handoff socket (HubState), never
// through a file.
//
// It has no mutex of its own: the hub touches it only under Hub.emuMu.
type History struct {
	lines    []string // lines[head:] are the retained lines
	wrapped  []bool   // wrapped[i]: lines[i] continues onto lines[i+1]
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
// single line over the byte cap is dropped too. wrapped means line continues
// onto the next line pushed.
func (h *History) Push(line string, wrapped bool) {
	h.lines = append(h.lines, line)
	h.wrapped = append(h.wrapped, wrapped)
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
		copy(h.wrapped, h.wrapped[h.head:])
		clear(h.lines[n:])
		h.lines, h.wrapped = h.lines[:n], h.wrapped[:n]
		h.head = 0
	}
}

// Clear drops every line and releases the storage.
func (h *History) Clear() {
	h.lines, h.wrapped, h.head, h.bytes = nil, nil, 0, 0
}

// Len returns the number of retained lines.
func (h *History) Len() int { return len(h.lines) - h.head }

// EndsWrapped reports whether the newest line continues onto what follows the
// history (the screen's first row).
func (h *History) EndsWrapped() bool { return h.Len() > 0 && h.wrapped[len(h.wrapped)-1] }

// ClearEndWrap marks the newest line as not continuing: the screen row it
// wrapped onto was erased (IN-02).
func (h *History) ClearEndWrap() {
	if h.Len() > 0 {
		h.wrapped[len(h.wrapped)-1] = false
	}
}

// Render joins the retained lines with CRLF, with no trailing CRLF. A wrapped
// line is joined to a non-empty next line with nothing: before an empty one
// the CRLF stays, or the viewer's pending wrap would swallow that row.
func (h *History) Render() string {
	var b strings.Builder
	b.Grow(h.bytes + 2*h.Len())
	for i := h.head; i < len(h.lines); i++ {
		b.WriteString(h.lines[i])
		if i+1 < len(h.lines) && !(h.wrapped[i] && h.lines[i+1] != "") {
			b.WriteString("\r\n")
		}
	}
	return b.String()
}
