package relay

import (
	"context"
	"time"

	"github.com/coder/websocket"
)

// PingEvery is how often a server pings a remote peer at the WebSocket level.
// With PingTimeout a dead peer is dropped within 45 s; TCP keepalive alone
// takes about 150 s on macOS. SHAR-07.
const PingEvery = 30 * time.Second

// PingTimeout is how long a peer has to answer one ping before it is dropped.
const PingTimeout = 15 * time.Second

// Heartbeat pings c every interval and closes it with CloseNow when a ping is
// not answered within timeout, returning true. It returns false once ctx is done.
//
// coder/websocket only reads pongs inside Read, so callers must start Heartbeat
// after a goroutine is reading c. Never put a deadline on that read's context:
// an expiring Read context closes the connection.
func Heartbeat(ctx context.Context, c *websocket.Conn, every, timeout time.Duration) bool {
	t := time.NewTicker(every)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return false
		case <-t.C:
			pctx, cancel := context.WithTimeout(ctx, timeout)
			err := c.Ping(pctx)
			cancel()
			if err != nil {
				if ctx.Err() != nil {
					return false // the handler is ending, not a dead peer
				}
				c.CloseNow()
				return true
			}
		}
	}
}
