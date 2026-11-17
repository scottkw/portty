package relay

import "sync"

// DefaultScrollbackBytes is the default maximum scrollback buffer size (256 KiB).
const DefaultScrollbackBytes = 256 * 1024

// ViewerReadLimit is the per-message read limit for Go WebSocket readers on the
// viewer side of a session (frames flowing host to viewer): the viewing
// daemon's proxy upstream conn and the CLI attach conn (UAT gap 4).
// coder/websocket defaults to 32768 bytes, but the host sends the catch-up
// preamble (Hub.ReconnectPreamble) as one message, and a full 32 KiB PTY read
// plus its frame header is already one byte over that default.
//
// Bound: the raw-ring preamble is hard-capped by the ring (Scrollback.Append
// keeps at most DefaultScrollbackBytes). The wrapped-ring preamble is one
// RenderSnapshot of the live emulator's current screen; its size is estimated,
// not enforced (about 20 bytes per styled cell, so well under 1 MiB for any
// real screen). 4x the ring fits both and still caps what one message can make
// a reader buffer.
//
// Readers of viewer input (the share server, the local relay, the proxy's
// client side) keep the 32 KiB default: input frames are small, and a guest
// must not be able to make the host buffer large messages (DoS).
const ViewerReadLimit = 4 * DefaultScrollbackBytes

// Scrollback is a bounded, mutex-protected byte buffer.
// When the buffer exceeds its maximum capacity, the oldest bytes are discarded.
type Scrollback struct {
	mu        sync.Mutex
	buf       []byte
	max       int
	truncated bool // true once any oldest bytes have been dropped (ring wrapped)
}

// NewScrollback creates a new Scrollback with the given maximum byte capacity.
func NewScrollback(maxBytes int) *Scrollback {
	return &Scrollback{
		buf: make([]byte, 0, maxBytes),
		max: maxBytes,
	}
}

// Append adds data to the scrollback buffer.
// If the resulting length exceeds max, the oldest bytes are dropped from the front.
func (s *Scrollback) Append(data []byte) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.buf = append(s.buf, data...)

	if len(s.buf) > s.max {
		// Discard oldest bytes so the buffer fits within max.
		excess := len(s.buf) - s.max
		// Shift contents left — avoids a separate allocation.
		copy(s.buf, s.buf[excess:])
		s.buf = s.buf[:s.max]
		s.truncated = true // history lost — raw replay is no longer complete
	}
}

// Truncated reports whether the ring has ever dropped oldest bytes (wrapped).
// Once true it stays true: a reconnecting client can no longer be brought fully
// up to date from the raw byte history alone (see Hub.ReconnectPreamble).
func (s *Scrollback) Truncated() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.truncated
}

// Snapshot returns an independent copy of the current buffer contents.
// Mutating the returned slice does not affect the internal buffer.
func (s *Scrollback) Snapshot() []byte {
	s.mu.Lock()
	defer s.mu.Unlock()

	out := make([]byte, len(s.buf))
	copy(out, s.buf)
	return out
}
