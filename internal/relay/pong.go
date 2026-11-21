package relay

import (
	"context"
	"sync/atomic"

	"github.com/coder/websocket"
)

// PongOwed carries the app-level pongs a read pump owes to its write pump
// (INPT-02). The read pump only counts (Owe never blocks, so it never stalls a
// read), and the write pump writes the pongs ahead of queued output, never
// through sub.Msgs, so a full outgoing queue cannot drop one. A count, not a
// flag: relayClient matches pongs by count, so two pings need two pongs.
type PongOwed struct {
	n    atomic.Int32
	kick chan struct{} // cap 1: "pongs are owed"
}

func NewPongOwed() *PongOwed { return &PongOwed{kick: make(chan struct{}, 1)} }

// Owe records one pong owed and wakes the write pump.
func (p *PongOwed) Owe() {
	p.n.Add(1)
	select {
	case p.kick <- struct{}{}:
	default:
	}
}

// Kick is ready when pongs may be owed.
func (p *PongOwed) Kick() <-chan struct{} { return p.kick }

// Take returns the pongs owed and resets the count.
func (p *PongOwed) Take() int { return int(p.n.Swap(0)) }

// WritePump forwards the hub's frames in sub.Msgs to conn, writing owed pongs
// first, until the socket, the hub or the read pump ends. Both the local relay
// and the share server run it.
func WritePump(ctx context.Context, conn *websocket.Conn, hub *Hub, sub *Subscriber, pongs *PongOwed, readDone <-chan struct{}) {
	writePongs := func() bool {
		for i := pongs.Take(); i > 0; i-- {
			if err := conn.Write(ctx, websocket.MessageBinary, PongFrame); err != nil {
				return false
			}
		}
		return true
	}
	for {
		// select picks randomly among ready cases; check owed pongs first so one
		// goes out ahead of a full sub.Msgs.
		select {
		case <-pongs.Kick():
			if !writePongs() {
				return
			}
		default:
		}
		select {
		case frame := <-sub.Msgs:
			if err := conn.Write(ctx, websocket.MessageBinary, frame); err != nil {
				return
			}
		case <-pongs.Kick():
			if !writePongs() {
				return
			}
		case <-ctx.Done():
			return
		case <-hub.Done():
			// BUG-02 (#125): the owner ended/stopped the session — tell every
			// connected viewer instead of a bare return (which the deferred
			// conn.CloseNow() would then turn into a silent abnormal close
			// with no code/reason). Fixed generic reason only — never a raw
			// Go error string or a user-set session name (IN-01 convention,
			// T-175-06-01).
			conn.Close(websocket.StatusNormalClosure, CloseReasonSessionEnded)
			return
		case <-readDone:
			return
		}
	}
}
