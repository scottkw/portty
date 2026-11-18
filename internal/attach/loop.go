package attach

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"math/rand/v2"
	"net/url"
	"strconv"
	"sync/atomic"
	"time"

	"github.com/coder/websocket"
	"github.com/scottkw/portty/internal/relay"
	"github.com/scottkw/portty/internal/statusbar"
	"github.com/scottkw/portty/internal/wakewatch"
)

// Verdict is a re-auth answer before a redial.
type Verdict int

const (
	// VerdictOK: access confirmed; dial.
	VerdictOK Verdict = iota
	// VerdictRetry: could not tell (daemon or host unreachable); wait the next backoff.
	VerdictRetry
	// VerdictShareStopped: the host no longer shares the session with us; stop.
	VerdictShareStopped
)

// Timing holds the reconnect loop's intervals. A zero RunOptions.Timing means
// DefaultTiming.
type Timing struct {
	PingEvery    time.Duration // MsgPing interval
	DeadAfter    time.Duration // silence after the first MsgPong that abandons a connection
	BackoffMin   time.Duration // first redial delay
	BackoffMax   time.Duration // redial delay cap
	HealthyReset time.Duration // a connection open this long restarts the backoff
	ProbeGrace   time.Duration // after a wake probe, silence this long abandons a ponged connection
	WakeMinGap   time.Duration // wake signals closer than this cause one wake action
}

// DefaultTiming is the one place for the CLI's reconnect numbers (they mirror
// the GUI RelayClient's RECONNECT constants).
var DefaultTiming = Timing{
	PingEvery:    15 * time.Second,
	DeadAfter:    40 * time.Second,
	BackoffMin:   time.Second,
	BackoffMax:   30 * time.Second,
	HealthyReset: 30 * time.Second,
	ProbeGrace:   5 * time.Second,
	WakeMinGap:   5 * time.Second,
}

// dialTimeout bounds one redial of the local daemon's proxy (the proxy's own
// upstream dial is bounded at 10 s too).
const dialTimeout = 10 * time.Second

// backoffDelay is min(max, min*2^attempt) with ±20 % jitter (r in [0,1)),
// never above max.
func backoffDelay(t Timing, attempt int, r float64) time.Duration {
	d := t.BackoffMax
	if attempt < 32 {
		if b := t.BackoffMin << attempt; b > 0 && b < d {
			d = b
		}
	}
	return min(t.BackoffMax, time.Duration(float64(d)*(0.8+0.4*r)))
}

// RunOptions configures Run. A nil Reauth means VerdictOK; nil OnConnect and
// OnState are skipped.
type RunOptions struct {
	URL         func() string                                   // proxy URL to redial (no resume query)
	Reauth      func(ctx context.Context) Verdict               // runs before every redial
	OnConnect   func(ctx context.Context, conn *websocket.Conn) // per connection; ctx ends with it
	OnState     func(state string)                              // "reconnecting" or "" (connected)
	Stdin       io.Reader
	Stdout      io.Writer
	DetachKey   byte
	Bar         *statusbar.Bar
	ClearOnFull bool // clear the screen before a full replay once output was rendered
	Timing      Timing
	Wake        <-chan wakewatch.Reason // wake / network-change signals; nil means none
}

// chunk is one stdin read. stale marks bytes read while no connection was
// open: they are never sent (only the detach key in them counts).
type chunk struct {
	data  []byte
	stale bool
}

// outputError is a Stdout write failure: the terminal is gone, so Run ends.
type outputError struct{ error }

// errSilent ends a connection the watchdog declared dead.
var errSilent = errors.New("attach: no frame from the relay within the dead-after interval")

type loop struct {
	o            RunOptions
	t            Timing
	input        chan chunk
	stdinErr     chan error
	connected    atomic.Bool // the host's replay on the current socket is written (it reads input)
	reconnecting bool        // last state shown was "reconnecting"; Run's goroutine only

	// Stream state, owned by the current connection's frame reader and read by
	// the gap only after that reader exited.
	offset      uint64
	offsetValid bool
	rendered    bool

	lastWake   time.Time // last wake signal acted on; used by serve and the gap in turn
	wakeRedial bool      // serve dropped a socket after a wake probe: redial without waiting
}

// Run is the remote attach loop (SHAR-07). It serves first, and whenever a
// connection ends with anything but a detach or a final close (relay.
// IsTerminalClose) it shows "reconnecting", backs off, re-authenticates and
// redials URL with ?resume=<offset>, forever. Keystrokes typed while no
// connection is open are discarded, never queued into the shell; the detach
// key works in every state. It returns nil on detach or ctx done.
func Run(ctx context.Context, first *websocket.Conn, o RunOptions) error {
	l := &loop{o: o, t: o.Timing, input: make(chan chunk), stdinErr: make(chan error, 1)}
	if l.t == (Timing{}) {
		l.t = DefaultTiming
	}
	done := make(chan struct{})
	defer close(done)
	// One reader for the whole run: a blocking stdin Read cannot be cancelled,
	// so a reader per connection would leak and steal keystrokes.
	go l.readStdin(done)

	conn, attempt := first, 0
	for {
		opened := time.Now()
		err, retry := l.serve(ctx, conn)
		if !retry {
			return err
		}
		if time.Since(opened) >= l.t.HealthyReset {
			attempt = 0
		}
		conn, err = l.reconnect(ctx, &attempt)
		if conn == nil {
			return err
		}
	}
}

func (l *loop) readStdin(done <-chan struct{}) {
	for {
		buf := make([]byte, relay.MaxInputPayload) // one read becomes one frame under the relay's read limit
		n, err := l.o.Stdin.Read(buf)
		if n > 0 {
			select {
			case l.input <- chunk{data: buf[:n], stale: !l.connected.Load()}:
			case <-done:
				return
			}
		}
		if err != nil {
			l.stdinErr <- err
			return
		}
	}
}

func (l *loop) state(s string) {
	l.reconnecting = s != ""
	if l.o.OnState != nil {
		l.o.OnState(s)
	}
}

// serve runs one connection. retry is true when the loop should reconnect.
func (l *loop) serve(ctx context.Context, conn *websocket.Conn) (err error, retry bool) {
	conn.SetReadLimit(relay.ViewerReadLimit)
	cctx, cancel := context.WithCancel(ctx)
	defer cancel()
	defer l.connected.Store(false)

	var lastFrame atomic.Int64 // wall clock (UnixNano): a Mac's sleep counts as silence
	var ponged atomic.Bool
	lastFrame.Store(time.Now().UnixNano())
	readDone := make(chan error, 1)
	// Connected means the host's replay is written (WR-03, WR-01 it3): the daemon
	// proxy upgrades before it dials the peer, and the host reads no input until
	// its replay is out. Until then keystrokes stay stale (T-05-14).
	first := make(chan struct{})
	go func() { readDone <- l.readFrames(cctx, conn, &lastFrame, &ponged, first) }()
	if l.o.OnConnect != nil {
		l.o.OnConnect(cctx, conn)
	}
	ping := time.NewTicker(l.t.PingEvery)
	defer ping.Stop()
	var probe <-chan time.Time // armed by a wake signal
	var probeSent int64

	// broken ends the connection after a failed write or the watchdog: the
	// reader's error says whether the host closed it for good.
	broken := func(cause error) (error, bool) {
		conn.CloseNow()
		if rerr := <-readDone; rerr != nil {
			if e, retry := l.classify(rerr); !retry {
				return e, false
			}
		}
		return cause, true
	}

	for {
		select {
		case <-first:
			first = nil
			if l.reconnecting {
				l.state("")
			}
		case rerr := <-readDone:
			conn.CloseNow()
			return l.classify(rerr)
		case c := <-l.input:
			data, detach := c.data, false
			if i := bytes.IndexByte(data, l.o.DetachKey); i >= 0 {
				data, detach = data[:i], true
			}
			if len(data) > 0 && !c.stale {
				if werr := conn.Write(cctx, websocket.MessageBinary, relay.MakeInputFrame(data)); werr != nil && !detach {
					return broken(werr)
				}
			}
			if detach {
				conn.Close(websocket.StatusNormalClosure, "detach") //nolint:errcheck
				return nil, false
			}
		case serr := <-l.stdinErr:
			conn.Close(websocket.StatusNormalClosure, "detach") //nolint:errcheck
			return serr, false
		case <-ping.C:
			silent := time.Duration(time.Now().UnixNano() - lastFrame.Load())
			if ponged.Load() && silent >= l.t.DeadAfter {
				return broken(errSilent)
			}
			_ = conn.Write(cctx, websocket.MessageBinary, []byte{relay.MsgPing})
		case <-l.o.Wake: // a nil channel never fires
			if l.wakeAllowed() {
				probeSent, probe = time.Now().UnixNano(), time.After(l.t.ProbeGrace)
				_ = conn.Write(cctx, websocket.MessageBinary, []byte{relay.MsgPing})
			}
		case <-probe:
			probe = nil
			// Only a host that pongs is held to the probe (an older one never answers).
			if ponged.Load() && lastFrame.Load() < probeSent {
				l.wakeRedial = true
				return broken(errSilent)
			}
		case <-ctx.Done():
			conn.Close(websocket.StatusNormalClosure, "detach") //nolint:errcheck
			return nil, false
		}
	}
}

// classify maps how a connection ended to Run's next step.
func (l *loop) classify(err error) (error, bool) {
	var oe outputError
	switch {
	case errors.As(err, &oe):
		return oe.error, false
	case relay.IsTerminalClose(err):
		return err, false
	default:
		return err, true
	}
}

// readFrames renders one connection's frames until its read fails. first is
// closed, and the loop marked connected, at the replay's offset meta, or at
// the first output of an older host that sends no replay meta.
func (l *loop) readFrames(ctx context.Context, conn *websocket.Conn, lastFrame *atomic.Int64, ponged *atomic.Bool, first chan struct{}) error {
	// The old offset stays valid until this socket starts its replay (WR-02): a
	// socket the proxy accepts and then refuses rendered nothing.
	sawReplay, full := false, false
	markConnected := func() {
		if first != nil {
			l.connected.Store(true)
			close(first)
			first = nil
		}
	}
	for {
		_, msg, err := conn.Read(ctx)
		if err != nil {
			return err
		}
		lastFrame.Store(time.Now().UnixNano())
		typ, payload, ferr := relay.ParseFrame(msg)
		if ferr != nil {
			continue
		}
		switch typ {
		case relay.MsgOutput:
			// A full replay (or output before any replay meta: an older host)
			// repaints the screen; clear what an earlier connection rendered.
			if l.o.ClearOnFull && l.rendered && (full || !sawReplay) {
				if _, werr := l.o.Stdout.Write([]byte("\x1b[2J\x1b[H")); werr != nil {
					return outputError{werr}
				}
			}
			if !sawReplay {
				l.offsetValid = false // an older host's unannounced replay
				markConnected()       // before it is shown, so typing at it is live
			}
			sawReplay, full = true, false
			if _, werr := l.o.Stdout.Write(payload); werr != nil {
				return outputError{werr}
			}
			l.rendered = true
			if l.offsetValid {
				l.offset += uint64(len(payload)) + 1
			}
		case relay.MsgMeta:
			var meta relay.MetaPayload
			if json.Unmarshal(payload, &meta) != nil {
				l.offsetValid = false // explicit full replay next time, never a silent partial
				continue
			}
			if meta.ViewerCount != nil && l.o.Bar != nil {
				l.o.Bar.SetViewerCount(*meta.ViewerCount)
			}
			if meta.Replay != nil {
				l.offsetValid = false // trusted again only after this socket's offset meta
				sawReplay, full = true, *meta.Replay == relay.ReplayFull
			}
			if meta.Offset != nil {
				l.offset, l.offsetValid = *meta.Offset, true
				markConnected() // the replay is written: the host reads input from here
			}
		case relay.MsgPong:
			ponged.Store(true)
		}
	}
}

// reconnect waits out the gap and returns the next connection. A nil conn
// ends Run with err (nil on detach or ctx done).
func (l *loop) reconnect(ctx context.Context, attempt *int) (*websocket.Conn, error) {
	l.state("reconnecting")
	gctx, cancel := context.WithCancel(ctx)
	type result struct {
		conn *websocket.Conn
		err  error
	}
	res := make(chan result, 1)
	go func() { c, err := l.redial(gctx, attempt); res <- result{c, err} }()
	stop := func() {
		cancel()
		go func() { // a dial already past its ctx check may still hand back a conn
			if r := <-res; r.conn != nil {
				r.conn.CloseNow()
			}
		}()
	}
	for {
		select {
		case c := <-l.input: // discarded: never queued for the next connection
			if bytes.IndexByte(c.data, l.o.DetachKey) >= 0 {
				stop()
				return nil, nil
			}
		case serr := <-l.stdinErr:
			stop()
			return nil, serr
		case <-ctx.Done():
			stop()
			return nil, nil
		case r := <-res:
			cancel()
			return r.conn, r.err // serve clears "reconnecting" once the host's replay is written
		}
	}
}

// redial backs off, re-authenticates and dials until it has a connection. A
// wake cuts the backoff short and restarts the schedule from its first step.
func (l *loop) redial(ctx context.Context, attempt *int) (*websocket.Conn, error) {
	woke := l.wakeRedial
	l.wakeRedial = false
	for {
		if !woke {
			var ok bool
			if woke, ok = l.backoff(ctx, *attempt); !ok {
				return nil, nil
			}
		}
		*attempt++
		if woke {
			*attempt, woke = 0, false
		}
		v := VerdictOK
		if l.o.Reauth != nil {
			v = l.o.Reauth(ctx)
		}
		if ctx.Err() != nil {
			return nil, nil
		}
		switch v {
		case VerdictShareStopped:
			return nil, websocket.CloseError{Code: websocket.StatusNormalClosure, Reason: relay.CloseReasonShareStopped}
		case VerdictRetry:
			continue
		}
		dctx, dcancel := context.WithTimeout(ctx, dialTimeout)
		conn, _, err := websocket.Dial(dctx, l.resumeURL(), nil)
		dcancel()
		if err == nil {
			return conn, nil
		}
	}
}

// backoff waits out one redial delay. woke reports a wake signal that cut it
// short; ok is false when ctx ended.
func (l *loop) backoff(ctx context.Context, attempt int) (woke, ok bool) {
	wait := time.NewTimer(backoffDelay(l.t, attempt, rand.Float64()))
	defer wait.Stop()
	for {
		select {
		case <-wait.C:
			return false, true
		case <-l.o.Wake: // a nil channel never fires
			if l.wakeAllowed() {
				return true, true
			}
		case <-ctx.Done():
			return false, false
		}
	}
}

// wakeAllowed rate-limits wake actions to one per WakeMinGap, so a flapping
// network cannot drive a redial storm (T-05-32).
func (l *loop) wakeAllowed() bool {
	if time.Since(l.lastWake) < l.t.WakeMinGap {
		return false
	}
	l.lastWake = time.Now()
	return true
}

// resumeURL is URL() plus resume=<offset> when the offset is trusted.
func (l *loop) resumeURL() string {
	raw := l.o.URL()
	if !l.offsetValid {
		return raw
	}
	u, err := url.Parse(raw)
	if err != nil {
		return raw
	}
	q := u.Query()
	q.Set("resume", strconv.FormatUint(l.offset, 10))
	u.RawQuery = q.Encode()
	return u.String()
}
