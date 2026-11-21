package relay

import (
	"context"

	"github.com/coder/websocket"
)

// RED stub (10-03 Task 2): no owed pongs and today's write loop. The GREEN
// commit replaces it.

type PongOwed struct{ kick chan struct{} }

func NewPongOwed() *PongOwed              { return &PongOwed{kick: make(chan struct{}, 1)} }
func (p *PongOwed) Owe()                  {}
func (p *PongOwed) Kick() <-chan struct{} { return p.kick }
func (p *PongOwed) Take() int             { return 0 }

func WritePump(ctx context.Context, conn *websocket.Conn, hub *Hub, sub *Subscriber, pongs *PongOwed, readDone <-chan struct{}) {
	for {
		select {
		case frame := <-sub.Msgs:
			if err := conn.Write(ctx, websocket.MessageBinary, frame); err != nil {
				return
			}
		case <-ctx.Done():
			return
		case <-hub.Done():
			conn.Close(websocket.StatusNormalClosure, CloseReasonSessionEnded)
			return
		case <-readDone:
			return
		}
	}
}
