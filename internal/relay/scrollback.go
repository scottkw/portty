package relay

import "sync"

// DefaultScrollbackBytes is the default maximum scrollback buffer size (256 KiB).
const DefaultScrollbackBytes = 256 * 1024

// ViewerReadLimit is the per-message read limit for Go WebSocket readers on the
// viewer side of a session (frames flowing host to viewer): the viewing
// daemon's proxy upstream conn and the CLI attach conn (UAT gap 4).
// coder/websocket defaults to 32768 bytes. The catch-up replay (Hub.Join) is
// now sent in messages of at most ReplayChunk payload, but a live 32 KiB PTY
// read plus its frame header is still one byte over that default, so the
// viewer-side readers keep this larger limit. 4x the ring is far above any
// live frame and still caps what one message can make a reader buffer.
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
	truncated bool   // true once any oldest bytes have been dropped (ring wrapped)
	end       uint64 // total frame bytes ever appended; the resume offset of the ring's last byte
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
	s.end += uint64(len(data))

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
// up to date from the raw byte history alone (see Hub.Join).
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

// End returns the total frame bytes ever appended: the offset just past the
// ring's last byte.
func (s *Scrollback) End() uint64 {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.end
}

// SnapshotFrom returns an independent copy of the bytes from offset off to the
// end, plus the end offset. ok is false (and data nil) when off is outside
// [start, end], i.e. older than the ring still holds or beyond what was written.
func (s *Scrollback) SnapshotFrom(off uint64) (data []byte, end uint64, ok bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	start := s.end - uint64(len(s.buf))
	if off < start || off > s.end {
		return nil, s.end, false
	}
	data = make([]byte, s.end-off)
	copy(data, s.buf[off-start:])
	return data, s.end, true
}
