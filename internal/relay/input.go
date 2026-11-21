package relay

import (
	"errors"
	"log"
	"time"
)

// Hub input FIFO (INPT-01). The read pumps hand each filtered MsgInput frame to
// InputConn.Write, which queues it and returns at once, so a pump never blocks
// on a slow PTY and keeps reading (WebSocket pings and app pongs are only
// processed inside Read). One writer goroutine per hub pops frames in arrival
// order and writes them to the PTY with no lock held.
//
// Order is arrival order across every viewer of the session, so a dropped
// viewer's backlog can't interleave with its reconnect's input; across an
// upgrade handoff the takeover holds its writer until the old daemon, which
// drains its own backlog, has exited (HoldInput, WR-02). The consequence: a Ctrl-C typed after a large backlog waits behind it, as it did
// behind the paste in the TCP stream before.
//
// The writer outlives the socket: a pump that exits (InputConn.Close) leaves
// its queued frames to be delivered. The queue is dropped only when the PTY
// write fails or the hub shuts down; blocked pumps then get ErrInputClosed.
// A revoked viewer's frames are dropped too (RevokeWebViewers, CR-01).
//
// shortcut: past InputMaxFrames / InputMaxBytes queued for one connection, or
// InputHubMaxBytes for the whole hub, a pump blocks again, so a paste over
// the quota into a slow reader can still get that viewer dropped by the
// heartbeat (the client's partial-paste notice then shows); raise the quota or
// spill to disk if that is ever hit for real.

const (
	InputMaxFrames = 1024     // per connection, queued plus in flight
	InputMaxBytes  = 32 << 20 // per connection; frames are <= MaxInputPayload, so frames bind first
	// InputHubMaxBytes bounds the whole hub (WR-01): every connection and every
	// reconnect has its own quota, and closed ones keep theirs until drained.
	// A connection with nothing queued may still add one frame, so the bound
	// is this plus one frame per connection.
	InputHubMaxBytes = 64 << 20
)

// ErrInputClosed: the hub's input writer stopped (PTY write failed or the hub
// shut down). The pump keeps reading and drops the input: the write pump ends
// the connection with "session ended" on hub.Done (WR-04).
var ErrInputClosed = errors.New("relay: input writer stopped")

type inputReq struct {
	c    *InputConn
	data []byte
}

// InputConn is one read pump's handle on the hub's input FIFO. All fields are
// under hub.inMu.
type InputConn struct {
	h             *Hub
	sub           *Subscriber // nil in tests; its inputRevoked drops this conn's input
	id            uint64      // Subscriber.ID, for diag lines only
	frames, bytes int         // queued plus in flight
	closed        bool        // the pump is gone; queued frames are still delivered
	backlogged    bool        // closed with frames queued (the drained line follows)
}

// OpenInput returns a handle for one read pump. sub only tags diag lines with
// its conn id and may be nil.
func (h *Hub) OpenInput(sub *Subscriber) *InputConn {
	c := &InputConn{h: h, sub: sub}
	if sub != nil {
		c.id = sub.ID
	}
	return c
}

// Write queues b for the PTY. It blocks only while this connection or the hub
// is at its quota, and returns only once b is queued or the input is dead
// (ErrInputClosed). It deliberately takes no context: a frame already read
// must not be dropped because its socket died. The caller must not reuse b
// (InputAbsorber.Filter allocates per call).
func (c *InputConn) Write(b []byte) error {
	h := c.h
	h.inMu.Lock()
	defer h.inMu.Unlock()
	h.startInputLocked()
	for !h.inDead && !c.revoked() && (c.frames >= InputMaxFrames ||
		(c.frames > 0 && (c.bytes+len(b) > InputMaxBytes || h.inBytes+len(b) > InputHubMaxBytes))) {
		h.inCond.Wait() // woken after each written frame, on death and on revoke
	}
	if h.inDead {
		return ErrInputClosed
	}
	if c.revoked() {
		return nil // dropped; the kick closes the socket and ends the pump
	}
	h.inq = append(h.inq, inputReq{c, b})
	c.frames++
	c.bytes += len(b)
	h.inBytes += len(b)
	h.inCond.Broadcast()
	return nil
}

// Close marks the pump gone. Its queued frames are still written.
func (c *InputConn) Close() {
	h := c.h
	h.inMu.Lock()
	defer h.inMu.Unlock()
	if c.closed {
		return
	}
	c.closed = true
	if c.frames > 0 && !h.inDead {
		c.backlogged = true
		log.Printf("viewer-diag: input backlog session=%s conn=%d frames=%d bytes=%d", DiagID(h.sessionID), c.id, c.frames, c.bytes)
	}
}

func (c *InputConn) revoked() bool { return c.sub != nil && c.sub.inputRevoked }

// revokeInput marks subs' input revoked and drops their queued frames
// (CR-01). It returns what was dropped.
func (h *Hub) revokeInput(subs []*Subscriber) (frames, bytes int) {
	h.inMu.Lock()
	defer h.inMu.Unlock()
	for _, s := range subs {
		s.inputRevoked = true
	}
	q := h.inq[h.inHead:]
	kept := q[:0]
	for _, r := range q {
		if r.c.revoked() {
			r.c.frames--
			r.c.bytes -= len(r.data)
			h.inBytes -= len(r.data)
			frames++
			bytes += len(r.data)
			continue
		}
		kept = append(kept, r)
	}
	clear(q[len(kept):])
	h.inq = h.inq[:h.inHead+len(kept)]
	h.inCond.Broadcast()
	return frames, bytes
}

// HoldInput stops the writer taking frames until ReleaseInput; Write still
// queues within the quotas. A handoff takeover holds its imported hubs until
// the old daemon, which still writes its own viewers' backlog to the same
// PTYs, has exited (WR-02).
func (h *Hub) HoldInput() {
	h.inMu.Lock()
	h.inHeld = true
	h.inMu.Unlock()
}

// ReleaseInput lets the writer go on in queue order.
func (h *Hub) ReleaseInput() {
	h.inMu.Lock()
	h.inHeld = false
	h.inCond.Broadcast()
	h.inMu.Unlock()
}

// WaitInputIdle waits up to timeout for every queued input byte to reach the
// PTY. It returns whether the input is idle (or dead, so nothing more will be
// written) and the bytes still queued or in flight.
func (h *Hub) WaitInputIdle(timeout time.Duration) (idle bool, queued int) {
	expired := false
	t := time.AfterFunc(timeout, func() {
		h.inMu.Lock()
		expired = true
		h.inCond.Broadcast()
		h.inMu.Unlock()
	})
	defer t.Stop()
	h.inMu.Lock()
	defer h.inMu.Unlock()
	for h.inBytes > 0 && !h.inDead && !expired {
		h.inCond.Wait()
	}
	return h.inBytes == 0 || h.inDead, h.inBytes
}

// startInputLocked starts the writer and the done watcher once.
func (h *Hub) startInputLocked() {
	if h.inStarted {
		return
	}
	h.inStarted = true
	go h.inputWriter()
	go func() {
		<-h.done
		h.inMu.Lock()
		h.killInputLocked(nil)
		h.inMu.Unlock()
	}()
}

// killInputLocked drops the queue and wakes every waiter. Bytes in flight
// stay counted until their write returns.
func (h *Hub) killInputLocked(err error) {
	if h.inDead {
		return
	}
	h.inDead = true
	dropped := 0
	for _, r := range h.inq[h.inHead:] {
		dropped += len(r.data)
	}
	h.inBytes -= dropped
	h.inq, h.inHead = nil, 0
	h.inCond.Broadcast()
	log.Printf("viewer-diag: input writer stopped session=%s queued=%d err=%v", DiagID(h.sessionID), dropped, err)
}

func (h *Hub) inputWriter() {
	h.inMu.Lock()
	defer h.inMu.Unlock()
	for {
		for (h.inHead == len(h.inq) || h.inHeld) && !h.inDead {
			h.inCond.Wait()
		}
		if h.inDead {
			return
		}
		r := h.inq[h.inHead]
		h.inq[h.inHead] = inputReq{}
		h.inHead++
		// O(1) pop: reset when empty, compact once the dead head is half the slice.
		if h.inHead == len(h.inq) {
			h.inq, h.inHead = h.inq[:0], 0
		} else if h.inHead >= 64 && h.inHead*2 >= len(h.inq) {
			n := copy(h.inq, h.inq[h.inHead:])
			clear(h.inq[n:])
			h.inq, h.inHead = h.inq[:n], 0
		}
		h.inMu.Unlock()

		_, err := h.writer.Write(r.data) // may block for minutes on a slow reader

		h.inMu.Lock()
		r.c.frames--
		r.c.bytes -= len(r.data)
		h.inBytes -= len(r.data)
		if err != nil {
			h.killInputLocked(err)
		} else if r.c.backlogged && r.c.frames == 0 {
			log.Printf("viewer-diag: input backlog drained session=%s conn=%d", DiagID(h.sessionID), r.c.id)
		}
		h.inCond.Broadcast()
	}
}
