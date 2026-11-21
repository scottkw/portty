package relay

import (
	"errors"
	"time"
)

// RED stub (10-03 Task 1): today's behavior, a direct blocking write. The
// hub input FIFO replaces it in the GREEN commit.

const (
	InputMaxFrames = 1024
	InputMaxBytes  = 32 << 20
)

var ErrInputClosed = errors.New("relay: input writer stopped")

type InputConn struct{ h *Hub }

func (h *Hub) OpenInput(sub *Subscriber) *InputConn { return &InputConn{h: h} }

func (c *InputConn) Write(b []byte) error { return c.h.WriteInput(b) }

func (c *InputConn) Close() {}

func (h *Hub) WaitInputIdle(timeout time.Duration) (idle bool, queued int) { return true, 0 }
