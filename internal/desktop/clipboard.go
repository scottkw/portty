package desktop

// clipboard.go: Copy from Mac and Paste to Mac (DCTL-06). The daemon touches
// the Mac's pasteboard only when a viewer asks: it reads on clip_get and writes
// when a clip_put completes. Nothing polls and nothing syncs, and Cmd+C or
// Cmd+V typed in a viewer is an ordinary key that never comes here. Text moves
// in chunks that fit the signalling socket's read limit in both directions.
// Log lines carry the outcome and a byte count, never the text.

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"strings"
	"sync"
	"sync/atomic"
	"time"
	"unicode/utf8"

	"github.com/scottkw/portty/internal/screencap"
)

// The clipboard wire types.
const (
	typeClipGet    = "clip_get"
	typeClipPut    = "clip_put"
	typeClipData   = "clip_data"
	typeClipResult = "clip_result"
)

// Limits. A chunk carries at most clipChunkBytes of text and its encoded frame
// at most clipFrameMax bytes, so it fits the 32768-byte read limit both ways.
const (
	clipChunkBytes = 16384
	clipFrameMax   = 30000
	clipMaxBytes   = 1 << 20
	clipMaxXid     = 64 // the viewer's transfer ID, echoed on every answer
	// clipReadTimeout bounds a pasteboard read: past it the macOS privacy
	// prompt is assumed to be waiting for a click nobody can give.
	clipReadTimeout = 2 * time.Second
)

// The clip_result outcomes.
const (
	clipOK              = "ok"
	clipConcealed       = "concealed"
	clipEmpty           = "empty"
	clipTooLarge        = "too_large"
	clipWaitingApproval = "waiting_approval"
	clipFailed          = "failed"
	clipForbidden       = "forbidden"
	clipBusy            = "busy"
)

type msgClipData struct {
	Type  string `json:"type"`
	Xid   string `json:"xid"`
	Seq   int    `json:"seq"`
	Total int    `json:"total"`
	Len   int    `json:"len"`
	Text  string `json:"text"`
}

type msgClipResult struct {
	Type    string `json:"type"`
	Xid     string `json:"xid"`
	Outcome string `json:"outcome"`
}

// PasteboardBridge is the Mac's pasteboard. Real calls refuse under tests, so
// tests inject a fake; there is deliberately no way to type into an app.
type PasteboardBridge interface {
	ReadText(maxBytes int) (string, screencap.ReadOutcome, error)
	WriteText(s string) error
}

// clipState is the clipboard's runtime state: one pasteboard read in flight
// per process, and each viewer's paste in progress.
type clipState struct {
	readPending atomic.Bool
	mu          sync.Mutex // a leaf lock
	puts        map[ViewerID]*clipPut
}

// clipPut is one viewer's paste being reassembled.
type clipPut struct {
	xid   string
	total int
	size  int // the declared byte length of the whole paste
	next  int // the next seq expected
	buf   []byte
}

func (c *clipState) hasPut(id ViewerID) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.puts[id] != nil
}

// drop discards a viewer's paste in progress.
func (c *clipState) drop(id ViewerID) {
	c.mu.Lock()
	delete(c.puts, id)
	c.mu.Unlock()
}

func (m *Manager) clipResult(ctx context.Context, v *viewer, xid, outcome string, n int) {
	slog.Info("desktop: clipboard", "viewer", string(v.ID)[:8], "outcome", outcome, "bytes", n)
	_ = send(ctx, v.ws, msgClipResult{Type: typeClipResult, Xid: xid, Outcome: outcome})
}

// clipGate is the checks both directions share: the viewer's right, then the
// lock. It returns the outcome to answer, or "" to go on.
func (m *Manager) clipGate(v *viewer) string {
	switch {
	case m.rightsOf(v)&RightsClipboard == 0:
		return clipForbidden
	case m.Locked():
		return clipFailed
	}
	return ""
}

type clipRead struct {
	text    string
	outcome screencap.ReadOutcome
	err     error
}

// handleClipGet answers a viewer's Copy from Mac: the Mac's text in chunks and
// an ok, or one outcome saying why not. The read runs on its own goroutine and
// waits clipReadTimeout, so a privacy prompt nobody can answer never hangs the
// daemon; while one read is unfinished every other copy answers
// waiting_approval at once.
func (m *Manager) handleClipGet(ctx context.Context, v *viewer, xid string) {
	if !validXid(xid) {
		xid = ""
		m.clipResult(ctx, v, xid, clipFailed, 0)
		return
	}
	if out := m.clipGate(v); out != "" {
		m.clipResult(ctx, v, xid, out, 0)
		return
	}
	if m.clip.hasPut(v.ID) {
		m.clipResult(ctx, v, xid, clipBusy, 0)
		return
	}
	if !m.clip.readPending.CompareAndSwap(false, true) {
		m.clipResult(ctx, v, xid, clipWaitingApproval, 0)
		return
	}
	done := make(chan clipRead, 1)
	go func() {
		r := clipRead{err: errors.New("desktop: clipboard read did not finish")}
		defer func() {
			m.clip.readPending.Store(false)
			done <- r
		}()
		defer recoverLog()
		r.text, r.outcome, r.err = m.d.Pasteboard.ReadText(clipMaxBytes)
	}()
	timer := time.NewTimer(clipReadTimeout)
	defer timer.Stop()
	var r clipRead
	select {
	case r = <-done:
	case <-timer.C: // the read's result is discarded when it returns
		m.clipResult(ctx, v, xid, clipWaitingApproval, 0)
		return
	case <-ctx.Done():
		return
	}
	switch {
	case r.err != nil:
		m.clipResult(ctx, v, xid, clipFailed, 0)
	case r.outcome == screencap.ReadConcealed:
		m.clipResult(ctx, v, xid, clipConcealed, 0)
	case r.outcome == screencap.ReadDenied:
		m.clipResult(ctx, v, xid, clipWaitingApproval, 0)
	case r.outcome == screencap.ReadTooLarge || len(r.text) > clipMaxBytes:
		m.clipResult(ctx, v, xid, clipTooLarge, 0)
	case r.outcome != screencap.ReadOK || r.text == "":
		m.clipResult(ctx, v, xid, clipEmpty, 0)
	default:
		m.sendClipText(ctx, v, xid, r.text)
	}
}

// sendClipText streams text as clip_data chunks, then clip_result ok.
func (m *Manager) sendClipText(ctx context.Context, v *viewer, xid, text string) {
	chunks := chunkText(text, clipChunkBytes, clipFrameMax)
	for i, c := range chunks {
		if send(ctx, v.ws, msgClipData{Type: typeClipData, Xid: xid, Seq: i, Total: len(chunks), Len: len(text), Text: c}) != nil {
			return // the socket is gone
		}
	}
	m.clipResult(ctx, v, xid, clipOK, len(text))
}

// handleClipPut takes one chunk of a viewer's Paste to Mac. Only the last
// chunk of a valid transfer is answered ok, after the Mac's clipboard has been
// set to one plain string; a chunk that breaks the rules discards the transfer
// and answers at once. Nothing is typed into any app.
func (m *Manager) handleClipPut(ctx context.Context, v *viewer, in inbound) {
	xid := in.Xid
	if !validXid(xid) {
		m.clip.drop(v.ID)
		m.clipResult(ctx, v, "", clipFailed, 0)
		return
	}
	if out := m.clipGate(v); out != "" {
		m.clip.drop(v.ID)
		m.clipResult(ctx, v, xid, out, 0)
		return
	}
	text, out := m.clip.accept(v.ID, in)
	switch out {
	case "": // more chunks to come
	case clipOK:
		if err := m.d.Pasteboard.WriteText(text); err != nil {
			m.clipResult(ctx, v, xid, clipFailed, 0)
			return
		}
		m.clipResult(ctx, v, xid, clipOK, len(text))
	default:
		m.clipResult(ctx, v, xid, out, 0)
	}
}

// accept takes one chunk into the viewer's transfer. It returns "" while more
// chunks are due, clipOK with the whole text when the last one has arrived and
// everything checks out, or the outcome to answer; a failure discards the
// transfer. The first chunk fixes xid, total and len and every later one must
// match them, arrive in order and stay within len.
func (c *clipState) accept(id ViewerID, in inbound) (string, string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	t := c.puts[id]
	fail := func() (string, string) {
		delete(c.puts, id)
		return "", clipFailed
	}
	if t == nil {
		switch {
		case in.Seq != 0 || in.Total < 1 || in.Len < 0:
			return fail()
		case in.Len == 0:
			return "", clipEmpty
		case in.Len > clipMaxBytes:
			return "", clipTooLarge
		}
		t = &clipPut{xid: in.Xid, total: in.Total, size: in.Len, buf: make([]byte, 0, in.Len)}
		if c.puts == nil {
			c.puts = make(map[ViewerID]*clipPut)
		}
		c.puts[id] = t
	}
	if in.Xid != t.xid || in.Total != t.total || in.Len != t.size || in.Seq != t.next ||
		len(in.Text) > clipChunkBytes || len(t.buf)+len(in.Text) > t.size {
		return fail()
	}
	t.buf = append(t.buf, in.Text...)
	t.next++
	if t.next < t.total {
		return "", ""
	}
	delete(c.puts, id)
	if len(t.buf) != t.size || !utf8.Valid(t.buf) {
		return "", clipFailed
	}
	return string(t.buf), clipOK
}

func validXid(xid string) bool { return xid != "" && len(xid) <= clipMaxXid }

// clipFrameOverhead is the encoded size of a clip_data frame with no text, for
// the worst-case transfer ID (every byte escaped) and the largest numbers.
var clipFrameOverhead = func() int {
	b, _ := json.Marshal(msgClipData{Type: typeClipData, Xid: strings.Repeat("\x00", clipMaxXid), Seq: clipMaxBytes, Total: clipMaxBytes, Len: clipMaxBytes})
	return len(b)
}()

// escapedLen is the most bytes r takes inside a JSON string, never less than
// the real encoding: quote and backslash take 2, other control characters up
// to 6, the characters Go escapes for HTML and the line separators 6.
func escapedLen(r rune, width int) int {
	switch {
	case r == '"' || r == '\\' || r == '\n' || r == '\r' || r == '\t':
		return 2
	case r < 0x20 || r == '<' || r == '>' || r == '&' || r == ' ' || r == ' ' || r == utf8.RuneError:
		return 6
	}
	return width
}

// chunkText cuts s at code-point boundaries into pieces of at most maxBytes
// bytes whose clip_data frame, JSON escaping included, is at most maxFrame.
func chunkText(s string, maxBytes, maxFrame int) []string {
	budget := maxFrame - clipFrameOverhead
	var out []string
	for start := 0; start < len(s); {
		end, esc := start, 0
		for end < len(s) {
			r, w := utf8.DecodeRuneInString(s[end:])
			c := escapedLen(r, w)
			if end > start && (end-start+w > maxBytes || esc+c > budget) {
				break
			}
			end += w
			esc += c
		}
		out = append(out, s[start:end])
		start = end
	}
	return out
}
