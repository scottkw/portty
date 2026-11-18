package attach

import (
	"context"
	"errors"
	"io"
	"time"

	"github.com/coder/websocket"
	"github.com/scottkw/portty/internal/statusbar"
)

// Verdict is a re-auth answer before a redial.
type Verdict int

const (
	VerdictOK Verdict = iota
	VerdictRetry
	VerdictShareStopped
)

// Timing holds the reconnect loop's intervals.
type Timing struct {
	PingEvery, DeadAfter, BackoffMin, BackoffMax, HealthyReset time.Duration
}

// RunOptions configures Run.
type RunOptions struct {
	URL         func() string
	Reauth      func(ctx context.Context) Verdict
	OnConnect   func(ctx context.Context, conn *websocket.Conn)
	OnState     func(state string)
	Stdin       io.Reader
	Stdout      io.Writer
	DetachKey   byte
	Bar         *statusbar.Bar
	ClearOnFull bool
	Timing      Timing
}

// Run is not implemented yet (RED scaffold).
func Run(ctx context.Context, first *websocket.Conn, o RunOptions) error {
	return errors.New("attach.Run: not implemented")
}
