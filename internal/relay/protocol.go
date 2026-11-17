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
	MsgInput   byte = 0x10 // Client keyboard input → PTY stdin
	MsgResize2 byte = 0x11 // Alternative resize format (reserved)
	MsgPing    byte = 0x12 // Keep-alive ping

	// MsgMeta is the server-to-client metadata push channel (JSON payload).
	// Reserved range 0x20-0x2F for future server-push frame types.
	MsgMeta byte = 0x20
)

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
}

// Replay modes carried in MetaPayload.Replay.
const (
	ReplayFull  = "full"
	ReplayDelta = "delta"
)

// ReplayChunk is the largest replay payload per MsgOutput message. Any value up
// to 32767 fits coder/websocket's 32768-byte default read limit with the type
// byte. A var so tests can shrink it.
var ReplayChunk = 16 << 10

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
