package relay

import (
	"encoding/binary"
	"encoding/json"
	"errors"
	"log"
	"slices"
	"strings"
	"sync"
	"time"
	"unicode"
	"unicode/utf8"
)

// File previews (PREV-01, PREV-03) travel on the session's own stream as
// MsgPreview frames, sub-kind in byte 1:
//
//	header: [0x21][0x01] JSON PreviewHeader
//	chunk:  [0x21][0x02][id u32 BE][payload <= PreviewChunk bytes]
//	end:    [0x21][0x03][id u32 BE][status u8]
//
// They are never written to the ring. The frontend parser reads exactly the
// bytes pinned in frontend/src/lib/__tests__/fixtures/preview-frames.json.
const (
	// PreviewMaxBytes caps one preview's bytes, so "20 MB" is exact.
	PreviewMaxBytes = 20_000_000
	// PreviewChunk is the largest payload per chunk frame (ReplayChunk precedent:
	// with its 6 header bytes it fits coder/websocket's 32768 default read limit).
	PreviewChunk = 16 << 10
	// PreviewWindow is how many queued frames a viewer may hold before the
	// preview sender waits: a small share of the 256-slot queue, so terminal
	// output always has room and the slow-viewer kick never fires.
	PreviewWindow = 16
)

// Preview frame sub-kinds (byte 1).
const (
	previewKindHeader byte = 0x01
	previewKindChunk  byte = 0x02
	previewKindEnd    byte = 0x03
)

// End frame statuses.
const (
	PreviewOK       byte = 0
	PreviewAborted  byte = 1
	PreviewTooLarge byte = 2
)

// PreviewMIMEs is the closed set of types a preview may carry.
var PreviewMIMEs = []string{
	"image/png", "image/jpeg", "image/gif", "image/webp", "image/svg+xml",
	"application/pdf", "text/markdown",
}

// previewStallTimeout ends a target that made no progress for this long. A
// var so tests can shorten it.
var previewStallTimeout = 60 * time.Second

var (
	// ErrPreviewBusy: a preview is still streaming on this hub.
	ErrPreviewBusy = errors.New("relay: a preview is already streaming")
	// ErrPreviewInvalid: the preview failed validation; nothing was sent.
	ErrPreviewInvalid = errors.New("relay: invalid preview")
	errPreviewFrame   = errors.New("relay: malformed preview frame")
)

// PreviewHeader is the JSON body of a header frame. Key identifies "the same
// file again" for the viewer; Focus is true only in the focused viewer's copy (the owner, or a path click's clicker).
type PreviewHeader struct {
	ID    uint32 `json:"id"`
	Name  string `json:"name"`
	MIME  string `json:"mime"`
	Size  int    `json:"size"`
	Key   string `json:"key"`
	Focus bool   `json:"focus"`
}

// Preview is one file handed to SendPreview.
type Preview struct {
	Name, MIME, Key string
	Data            []byte
	// FocusSub, when set, is the one viewer whose header has focus: the
	// clicker of a path click, who didn't claim the grid. Nil keeps the owner
	// rule (the last typist).
	FocusSub *Subscriber
}

// PreviewSend reports a started preview: how many viewers it targets, and a
// channel closed once every one of them finished (ok, aborted, left or ended).
// handlePreview doesn't wait on Done; tests do, to know a send has settled (11-REVIEW IN-07: kept).
type PreviewSend struct {
	Viewers int
	Done    <-chan struct{}
}

// MakePreviewHeader builds a header frame.
func MakePreviewHeader(hd PreviewHeader) []byte {
	body, _ := json.Marshal(hd) // plain struct of strings, ints and a bool: cannot fail
	return append([]byte{MsgPreview, previewKindHeader}, body...)
}

// MakePreviewChunk builds a chunk frame; data must be at most PreviewChunk bytes.
func MakePreviewChunk(id uint32, data []byte) []byte {
	f := make([]byte, 6, 6+len(data))
	f[0], f[1] = MsgPreview, previewKindChunk
	binary.BigEndian.PutUint32(f[2:], id)
	return append(f, data...)
}

// MakePreviewEnd builds an end frame.
func MakePreviewEnd(id uint32, status byte) []byte {
	f := []byte{MsgPreview, previewKindEnd, 0, 0, 0, 0, status}
	binary.BigEndian.PutUint32(f[2:], id)
	return f
}

// The Parse* decoders have no production caller (the peer proxy copies frames
// verbatim; the frontend parses them). They stay exported because relay and
// daemon tests both decode frames with them against the shared wire fixture,
// and a _test.go helper can't cross packages (11-REVIEW IN-07: kept).
//
// ParsePreviewHeader decodes a header frame.
func ParsePreviewHeader(frame []byte) (PreviewHeader, error) {
	var hd PreviewHeader
	if len(frame) < 2 || frame[0] != MsgPreview || frame[1] != previewKindHeader {
		return hd, errPreviewFrame
	}
	if err := json.Unmarshal(frame[2:], &hd); err != nil {
		return PreviewHeader{}, errPreviewFrame
	}
	return hd, nil
}

// ParsePreviewChunk decodes a chunk frame; data aliases frame.
func ParsePreviewChunk(frame []byte) (id uint32, data []byte, err error) {
	if len(frame) < 6 || frame[0] != MsgPreview || frame[1] != previewKindChunk || len(frame)-6 > PreviewChunk {
		return 0, nil, errPreviewFrame
	}
	return binary.BigEndian.Uint32(frame[2:]), frame[6:], nil
}

// ParsePreviewEnd decodes an end frame.
func ParsePreviewEnd(frame []byte) (id uint32, status byte, err error) {
	if len(frame) != 7 || frame[0] != MsgPreview || frame[1] != previewKindEnd {
		return 0, 0, errPreviewFrame
	}
	return binary.BigEndian.Uint32(frame[2:]), frame[6], nil
}

// validate is the trust-boundary check on bytes a local process handed us.
func (p Preview) validate() error {
	if len(p.Data) > PreviewMaxBytes || !slices.Contains(PreviewMIMEs, p.MIME) ||
		!isLowerHex32(p.Key) || p.Name == "" || len(p.Name) > 255 || !utf8.ValidString(p.Name) ||
		slices.ContainsFunc([]rune(p.Name), unicode.IsControl) {
		return ErrPreviewInvalid
	}
	return nil
}

func isLowerHex32(s string) bool {
	if len(s) != 32 {
		return false
	}
	for _, c := range []byte(s) {
		if (c < '0' || c > '9') && (c < 'a' || c > 'f') {
			return false
		}
	}
	return true
}

// SendPreview streams p to every viewer subscribed and not replaying right
// now (read-only included, later joiners never) and returns at once. Each
// target gets its own paced sender (sendPreviewTo), never sendLocked or
// broadcast, which kick a full queue. One preview per hub at a time:
// ErrPreviewBusy until Done of the previous one closes. ErrHubEnded once the
// session ended; ErrPreviewInvalid before any frame when p fails validation.
func (h *Hub) SendPreview(p Preview) (PreviewSend, error) {
	if err := p.validate(); err != nil {
		return PreviewSend{}, err
	}
	h.mu.Lock()
	if h.closed {
		h.mu.Unlock()
		return PreviewSend{}, ErrHubEnded
	}
	if h.previewBusy {
		h.mu.Unlock()
		return PreviewSend{}, ErrPreviewBusy
	}
	var targets []*Subscriber
	for sub := range h.subscribers {
		if !sub.replaying && !sub.revoked {
			targets = append(targets, sub)
		}
	}
	done := make(chan struct{})
	if len(targets) == 0 {
		h.mu.Unlock()
		close(done)
		return PreviewSend{Done: done}, nil
	}
	h.previewID++
	h.previewBusy = true
	id, focus := h.previewID, h.owner
	if p.FocusSub != nil {
		focus = p.FocusSub
	}
	h.mu.Unlock()

	log.Printf("viewer-diag: preview start session=%s id=%d viewers=%d bytes=%d mime=%s",
		DiagID(h.sessionID), id, len(targets), len(p.Data), p.MIME)
	var wg sync.WaitGroup
	for _, sub := range targets {
		header := MakePreviewHeader(PreviewHeader{ID: id, Name: p.Name, MIME: p.MIME, Size: len(p.Data), Key: p.Key, Focus: sub == focus})
		wg.Go(func() { h.sendPreviewTo(sub, id, header, p.Data) })
	}
	go func() {
		wg.Wait()
		h.mu.Lock()
		h.previewBusy = false
		h.mu.Unlock()
		close(done)
	}()
	return PreviewSend{Viewers: len(targets), Done: done}, nil
}

// sendPreviewTo paces one target: each frame is queued under mu only while
// the target holds fewer than PreviewWindow frames, so the send never blocks
// and never fills the queue; otherwise it unlocks and sleeps. Every producer
// queues under mu, so the length check cannot race another send. It stops
// when the target leaves (or is being revoked), the hub ends, or nothing
// moved for previewStallTimeout; a stuck target gets a best-effort aborted
// end frame and is never kicked.
func (h *Hub) sendPreviewTo(sub *Subscriber, id uint32, header, data []byte) {
	start := time.Now()
	nChunks := (len(data) + PreviewChunk - 1) / PreviewChunk
	last := nChunks + 1 // frame 0 header, 1..nChunks chunks, last end
	sent, status := 0, ""
	progress := start
	const minWait, maxWait = 2 * time.Millisecond, 50 * time.Millisecond
	wait := minWait
	for next := 0; status == ""; {
		var frame []byte
		switch {
		case next == 0:
			frame = header
		case next == last:
			frame = MakePreviewEnd(id, PreviewOK)
		default:
			off := (next - 1) * PreviewChunk
			frame = MakePreviewChunk(id, data[off:min(off+PreviewChunk, len(data))])
		}
		queued := false
		h.mu.Lock()
		if _, ok := h.subscribers[sub]; !ok || sub.revoked {
			status = "left"
		} else if h.closed {
			status = "ended"
			queueIfRoom(sub, MakePreviewEnd(id, PreviewAborted))
		} else if len(sub.Msgs) < PreviewWindow {
			queued = queueIfRoom(sub, frame)
		}
		if status == "" && !queued && time.Since(progress) >= previewStallTimeout {
			status = "aborted"
			queueIfRoom(sub, MakePreviewEnd(id, PreviewAborted))
		}
		h.mu.Unlock()
		switch {
		case queued:
			if next > 0 && next < last {
				sent += len(frame) - 6
			}
			if next == last {
				status = "ok"
			}
			next++
			progress, wait = time.Now(), minWait
		case status == "":
			time.Sleep(wait)
			wait = min(wait*2, maxWait)
		}
	}
	log.Printf("viewer-diag: preview end session=%s conn=%d id=%d status=%s sent=%d elapsed=%s",
		DiagID(h.sessionID), sub.ID, id, status, sent, time.Since(start).Round(time.Millisecond))
}

// queueIfRoom queues frame without blocking. Caller holds h.mu.
func queueIfRoom(sub *Subscriber, frame []byte) bool {
	select {
	case sub.Msgs <- frame:
		return true
	default:
		return false
	}
}

// CleanPreviewName reduces a name to its base name without control, bidi
// override or isolate characters, trimmed and capped at 255 bytes on a rune
// boundary. Viewers show it as text and clean it again (11-05). The daemon's
// preview route and the path click both use it.
func CleanPreviewName(s string) string {
	s = s[strings.LastIndex(s, "/")+1:]
	s = strings.Map(func(r rune) rune {
		if unicode.IsControl(r) || r == utf8.RuneError || r >= 0x202A && r <= 0x202E || r >= 0x2066 && r <= 0x2069 {
			return -1
		}
		return r
	}, s)
	s = strings.TrimSpace(s)
	for len(s) > 255 {
		_, n := utf8.DecodeLastRuneInString(s)
		s = s[:len(s)-n]
	}
	return strings.TrimSpace(s)
}
