// Package relay provides the binary framing protocol, scrollback buffer,
// and fan-out hub for PTY session relay over WebSocket.
package relay

import (
	"encoding/json"
	"errors"
	"strconv"
)

// Message type bytes — single-byte prefix for every framed message.
const (
	MsgOutput  byte = 0x01 // PTY stdout/stderr → client
	MsgResize  byte = 0x02 // Terminal resize (cols, rows as big-endian uint16)
	MsgTitle   byte = 0x03 // Window title update
	MsgPong    byte = 0x04 // Answer to MsgPing: lets JS and proxied CLIs see end-to-end liveness (SHAR-07)
	MsgInput   byte = 0x10 // Client keyboard input → PTY stdin
	MsgResize2 byte = 0x11 // Alternative resize format (reserved)
	MsgPing    byte = 0x12 // Keep-alive ping
	// MsgPathClick is a viewer asking the host to preview a path the session
	// printed (PATH-02, pathclick.go): the payload is the clicked text, UTF-8,
	// at most PathClickMaxBytes. Old hosts drop it (no case, no default arm).
	MsgPathClick byte = 0x13

	// MsgMeta is the server-to-client metadata push channel (JSON payload).
	// Reserved range 0x20-0x2F for future server-push frame types.
	MsgMeta byte = 0x20
	// MsgPreview carries a file preview (preview.go): server push only, never
	// written to the scrollback ring, so offsets and resume ignore it.
	MsgPreview byte = 0x21
)

// PongFrame is the whole MsgPong frame. Shared by every connection: never mutate it.
var PongFrame = []byte{MsgPong}

// MakeOutputFrame prepends the MsgOutput type byte to data.
func MakeOutputFrame(data []byte) []byte {
	frame := make([]byte, 1+len(data))
	frame[0] = MsgOutput
	copy(frame[1:], data)
	return frame
}

// MakeInputFrame prepends the MsgInput type byte to data.
func MakeInputFrame(data []byte) []byte {
	frame := make([]byte, 1+len(data))
	frame[0] = MsgInput
	copy(frame[1:], data)
	return frame
}

// PathClickMaxBytes is the largest clicked-text payload a MsgPathClick may
// carry; a longer one is dropped.
const PathClickMaxBytes = 1024

// MakePathClickFrame prepends the MsgPathClick type byte to text.
func MakePathClickFrame(text string) []byte {
	return append([]byte{MsgPathClick}, text...)
}

// MakeResizeFrame encodes cols and rows as a 5-byte big-endian frame.
//
//	[MsgResize, cols_hi, cols_lo, rows_hi, rows_lo]
func MakeResizeFrame(cols, rows uint16) []byte {
	return []byte{
		MsgResize,
		byte(cols >> 8), byte(cols),
		byte(rows >> 8), byte(rows),
	}
}

// ResizeClaim is bit 0 of MsgResize2 payload[4] (INPT-06): set, the size
// change is the user's (window resize, font change, SIGWINCH) and the viewer
// takes the grid; clear or absent, the frame only reports the viewer's size.
// Older viewers send 4 bytes and older hosts read only 4, so mixed versions
// degrade to plain reports. Other bits are reserved and ignored.
const ResizeClaim byte = 0x01

// ParseResize decodes a client MsgResize2 payload:
// [cols_hi, cols_lo, rows_hi, rows_lo, flags?]. ok is false under 4 bytes.
func ParseResize(payload []byte) (cols, rows int, claim, ok bool) {
	if len(payload) < 4 {
		return 0, 0, false, false
	}
	cols = int(payload[0])<<8 | int(payload[1])
	rows = int(payload[2])<<8 | int(payload[3])
	return cols, rows, len(payload) >= 5 && payload[4]&ResizeClaim != 0, true
}

// ParseFrame splits a raw frame into its type byte and payload.
// Returns an error if the frame is empty.
func ParseFrame(frame []byte) (msgType byte, payload []byte, err error) {
	if len(frame) == 0 {
		return 0, nil, errors.New("relay: empty frame")
	}
	return frame[0], frame[1:], nil
}

// MetaPayload is the extensible JSON payload for MsgMeta frames.
// All fields are pointers so omitempty works correctly for partial updates.
type MetaPayload struct {
	ViewerCount *int `json:"viewerCount,omitempty"`
	// Replay is sent once before a join's catch-up output: ReplayDelta (only
	// what the client missed since its resume offset) or ReplayFull (the client
	// resets its buffer first). SHAR-07.
	Replay *string `json:"replay,omitempty"`
	// Offset is sent once after a join's catch-up output: the ring end in frame
	// bytes. The client adds the length of every later MsgOutput frame to it and
	// sends the total back as ?resume= when it reconnects.
	Offset *uint64 `json:"offset,omitempty"`
	// PathClick rides the replay meta of a subscriber that may write: the host
	// understands MsgPathClick and will honour this viewer's clicks (PATH-02).
	// Hosts older than Phase 12 never send it, so viewers offer no path links.
	PathClick *bool `json:"pathClick,omitempty"`
	// Notice is a one-viewer message with a fixed code (NoticePath*), sent only
	// to the viewer that clicked, never broadcast.
	Notice *string `json:"notice,omitempty"`
}

// Notice codes carried in MetaPayload.Notice.
const (
	NoticePathUnsupported = "path-unsupported" // the clicked file's type can't be previewed
	NoticePathTooLarge    = "path-too-large"   // the clicked file is over PreviewMaxBytes
)

// Replay modes carried in MetaPayload.Replay.
const (
	ReplayFull  = "full"
	ReplayDelta = "delta"
)

// ReplayChunk is the largest replay payload per MsgOutput message. Any value up
// to 32767 fits coder/websocket's 32768-byte default read limit with the type
// byte. A var so tests can shrink it.
var ReplayChunk = 16 << 10

// MaxInputPayload is the largest payload one MsgInput frame may carry: 32768
// (coder/websocket's default read limit, kept by every input reader as the DoS
// guard) minus the type byte. A larger frame closes the socket, so senders
// split longer input into frames of at most this many payload bytes (HARD-02).
const MaxInputPayload = 32767

// ReplayPendingMax caps the bytes of live frames held for one viewer while its
// replay is written (Hub.TakePending). A viewer that keeps up with the output
// holds less than the replay itself (at most the 4 MiB history plus a screen),
// so passing this means output outran the link: it is kicked as slow (WR-01).
// A var so tests can shrink it.
var ReplayPendingMax = HistoryMaxBytes

// ParseResume parses a ?resume= query value. Only a decimal uint64 is ok; an
// empty, negative, non-decimal or overflowing value is not ok and takes the
// full-replay path.
func ParseResume(s string) (uint64, bool) {
	n, err := strconv.ParseUint(s, 10, 64)
	return n, err == nil
}

// MakeMeta encodes a MetaPayload as a MsgMeta frame.
func MakeMeta(p MetaPayload) []byte {
	b, _ := json.Marshal(p) // MetaPayload is always serialisable
	frame := make([]byte, 1+len(b))
	frame[0] = MsgMeta
	copy(frame[1:], b)
	return frame
}

// 0x30-0x37 are RETIRED (chat, chat-send, presence, typing, alias, inject,
// inject NAK, self): removed in Portty Phase 1 (0x35/0x36) and Phase 2 (the
// rest, CHAT-01). Never reuse them. The read pumps have no case and no default
// arm for them, so an old client's frames are dropped without a PTY write.
