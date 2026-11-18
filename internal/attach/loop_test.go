package attach

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/scottkw/portty/internal/relay"
	"github.com/scottkw/portty/internal/wakewatch"
)

const loopWait = 2 * time.Second

// fakeRelay is a scripted relay: each accepted connection runs serve(i, conn)
// and is closed with CloseNow when serve returns. Reject makes the i-th dial
// fail before the upgrade.
type fakeRelay struct {
	t        *testing.T
	url      string
	done     chan struct{} // closed at cleanup: unblocks parked serve funcs
	serve    func(f *fakeRelay, i int, c *websocket.Conn)
	reject   func(i int) bool
	mu       sync.Mutex
	urls     []string       // request URL per connection, including rejected dials
	inputs   []bytes.Buffer // MsgInput payloads per accepted connection
	accepted []time.Time
	closed   []time.Time
	pings    int // MsgPing frames pump saw, all connections
}

func newFakeRelay(t *testing.T, serve func(f *fakeRelay, i int, c *websocket.Conn), reject func(i int) bool) *fakeRelay {
	t.Helper()
	f := &fakeRelay{t: t, done: make(chan struct{}), serve: serve, reject: reject}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		f.urls = append(f.urls, r.URL.String())
		i := len(f.inputs)
		rejected := f.reject != nil && f.reject(len(f.urls)-1)
		if !rejected {
			f.inputs = append(f.inputs, bytes.Buffer{})
			f.accepted = append(f.accepted, time.Now())
			f.closed = append(f.closed, time.Time{})
		}
		f.mu.Unlock()
		if rejected {
			http.Error(w, "no", http.StatusServiceUnavailable)
			return
		}
		c, err := websocket.Accept(w, r, nil)
		if err != nil {
			return
		}
		f.serve(f, i, c)
		f.mu.Lock()
		f.closed[i] = time.Now()
		f.mu.Unlock()
		c.CloseNow()
	}))
	t.Cleanup(srv.Close)
	t.Cleanup(func() { close(f.done) }) // runs before srv.Close
	f.url = "ws://" + strings.TrimPrefix(srv.URL, "http://") + "/sessions/s1/ws"
	return f
}

// pump reads conn until it fails, recording input and answering pings
// (pong decides whether MsgPing gets a MsgPong). It returns the read error.
func (f *fakeRelay) pump(i int, c *websocket.Conn, pong func() bool) error {
	for {
		_, m, err := c.Read(context.Background())
		if err != nil {
			return err
		}
		typ, p, _ := relay.ParseFrame(m)
		switch typ {
		case relay.MsgInput:
			f.mu.Lock()
			f.inputs[i].Write(p)
			f.mu.Unlock()
		case relay.MsgPing:
			f.mu.Lock()
			f.pings++
			f.mu.Unlock()
			if pong == nil || pong() {
				_ = c.Write(context.Background(), websocket.MessageBinary, relay.PongFrame)
			}
		}
	}
}

func (f *fakeRelay) accepts() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.inputs)
}

func (f *fakeRelay) pingCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.pings
}

func (f *fakeRelay) input(i int) string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.inputs[i].String()
}

func (f *fakeRelay) dial(t *testing.T) *websocket.Conn {
	t.Helper()
	c, _, err := websocket.Dial(context.Background(), f.url, nil)
	if err != nil {
		t.Fatalf("dial fake relay: %v", err)
	}
	t.Cleanup(func() { c.CloseNow() })
	return c
}

func writeFrames(c *websocket.Conn, frames ...[]byte) {
	for _, fr := range frames {
		_ = c.Write(context.Background(), websocket.MessageBinary, fr)
	}
}

func output(s string) []byte { return append([]byte{relay.MsgOutput}, s...) }

func replayMeta(mode string) []byte { return relay.MakeMeta(relay.MetaPayload{Replay: &mode}) }

func offsetMeta(n uint64) []byte { return relay.MakeMeta(relay.MetaPayload{Offset: &n}) }

// runLoop is a running Run with piped stdin, captured stdout and states.
type runLoop struct {
	stdin  *io.PipeWriter
	mu     sync.Mutex
	out    bytes.Buffer
	states []string
	done   chan error
}

func (r *runLoop) Write(p []byte) (int, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.out.Write(p)
}

func (r *runLoop) stdout() string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.out.String()
}

func (r *runLoop) stateList() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]string(nil), r.states...)
}

func startRun(t *testing.T, f *fakeRelay, o RunOptions) *runLoop {
	t.Helper()
	pr, pw := io.Pipe()
	t.Cleanup(func() { pw.Close() })
	r := &runLoop{stdin: pw, done: make(chan error, 1)}
	o.Stdin, o.Stdout, o.DetachKey = pr, r, 0x1C
	o.URL = func() string { return f.url }
	o.OnState = func(s string) {
		r.mu.Lock()
		r.states = append(r.states, s)
		r.mu.Unlock()
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	t.Cleanup(cancel)
	first := f.dial(t)
	go func() { r.done <- Run(ctx, first, o) }()
	return r
}

func (r *runLoop) typeIn(t *testing.T, s string) {
	t.Helper()
	if _, err := r.stdin.Write([]byte(s)); err != nil {
		t.Fatalf("stdin write: %v", err)
	}
}

func (r *runLoop) detach(t *testing.T) {
	t.Helper()
	r.typeIn(t, "\x1c")
	r.wantDone(t, nil)
}

func (r *runLoop) wantDone(t *testing.T, want error) {
	t.Helper()
	select {
	case err := <-r.done:
		if err != want {
			t.Fatalf("Run = %v; want %v", err, want)
		}
	case <-time.After(loopWait):
		t.Fatalf("Run did not return within %v", loopWait)
	}
}

func waitFor(t *testing.T, what string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(loopWait)
	for !cond() {
		if time.Now().After(deadline) {
			t.Fatalf("timed out after %v waiting for %s", loopWait, what)
		}
		time.Sleep(5 * time.Millisecond)
	}
}

// fast backs off in milliseconds and never pings within a test.
var fast = Timing{PingEvery: time.Hour, DeadAfter: time.Hour, BackoffMin: 10 * time.Millisecond, BackoffMax: 50 * time.Millisecond, HealthyReset: time.Hour}

func TestBackoffDelay(t *testing.T) {
	want := []time.Duration{1, 2, 4, 8, 16, 30, 30}
	for a, w := range want {
		if got := backoffDelay(DefaultTiming, a, 0.5); got != w*time.Second {
			t.Errorf("attempt %d at r=0.5 = %v; want %v", a, got, w*time.Second)
		}
		base := min(30*time.Second, time.Second<<a)
		for _, r := range []float64{0, 0.999999} {
			got := backoffDelay(DefaultTiming, a, r)
			if got < base*8/10 || got > base*12/10 || got > 30*time.Second {
				t.Errorf("attempt %d at r=%v = %v; want within ±20%% of %v and <= 30s", a, r, got, base)
			}
		}
	}
	if got := backoffDelay(DefaultTiming, 100, 1); got != 30*time.Second {
		t.Errorf("attempt 100 = %v; want the 30s cap", got)
	}
}

func TestRun_AbruptCloseRetries(t *testing.T) {
	f := newFakeRelay(t, func(f *fakeRelay, i int, c *websocket.Conn) {
		if i == 0 {
			return // CloseNow: no close frame
		}
		writeFrames(c, replayMeta(relay.ReplayDelta), offsetMeta(0), output("BACK"))
		f.pump(i, c, nil) //nolint:errcheck
	}, nil)
	r := startRun(t, f, RunOptions{Timing: fast})
	waitFor(t, "output from the redialed connection", func() bool { return strings.Contains(r.stdout(), "BACK") })
	if got := r.stateList(); len(got) != 2 || got[0] != "reconnecting" || got[1] != "" {
		t.Errorf("states = %q; want [reconnecting \"\"]", got)
	}
	r.detach(t)
}

func TestRun_WatchdogDropsSilentConn(t *testing.T) {
	f := newFakeRelay(t, func(f *fakeRelay, i int, c *websocket.Conn) {
		if i == 0 { // answer the first ping, then go silent with TCP open
			for {
				_, m, err := c.Read(context.Background())
				if err != nil {
					return
				}
				if typ, _, _ := relay.ParseFrame(m); typ == relay.MsgPing {
					writeFrames(c, relay.PongFrame)
					<-f.done
					return
				}
			}
		}
		f.pump(i, c, nil) //nolint:errcheck
	}, nil)
	tm := fast
	tm.PingEvery, tm.DeadAfter = 20*time.Millisecond, 100*time.Millisecond
	start := time.Now()
	r := startRun(t, f, RunOptions{Timing: tm})
	waitFor(t, "a redial after the silent connection", func() bool { return f.accepts() >= 2 })
	if d := time.Since(start); d > time.Second {
		t.Errorf("silent connection replaced after %v; want within 1s", d)
	}
	r.detach(t)
}

func TestRun_NoPongNeverDropped(t *testing.T) {
	f := newFakeRelay(t, func(f *fakeRelay, i int, c *websocket.Conn) {
		f.pump(i, c, func() bool { return false }) //nolint:errcheck
	}, nil)
	tm := fast
	tm.PingEvery, tm.DeadAfter = 20*time.Millisecond, 60*time.Millisecond
	r := startRun(t, f, RunOptions{Timing: tm})
	time.Sleep(300 * time.Millisecond)
	if n := f.accepts(); n != 1 {
		t.Errorf("an older host that never pongs was redialed (%d connections); the watchdog must arm only after a pong", n)
	}
	r.detach(t)
}

func TestRun_DetachWhileReconnecting(t *testing.T) {
	f := newFakeRelay(t, func(*fakeRelay, int, *websocket.Conn) {}, func(i int) bool { return i > 0 })
	r := startRun(t, f, RunOptions{Timing: fast})
	waitFor(t, "reconnecting", func() bool { return len(r.stateList()) > 0 })
	waitFor(t, "a refused redial", func() bool { f.mu.Lock(); defer f.mu.Unlock(); return len(f.urls) >= 3 })
	r.detach(t)
}

func TestRun_InputWhileDisconnectedNeverSent(t *testing.T) {
	kick := make(chan struct{})
	f := newFakeRelay(t, func(f *fakeRelay, i int, c *websocket.Conn) {
		if i == 0 {
			writeFrames(c, output("READY")) // the host answered: connected
			go f.pump(i, c, nil)            //nolint:errcheck
			<-kick
			c.Close(websocket.StatusGoingAway, "") //nolint:errcheck
			return
		}
		writeFrames(c, replayMeta(relay.ReplayDelta), offsetMeta(0))
		f.pump(i, c, nil) //nolint:errcheck
	}, nil)
	release := make(chan struct{})
	reauth := func(ctx context.Context) Verdict {
		select {
		case <-release:
		case <-ctx.Done():
		}
		return VerdictOK
	}
	r := startRun(t, f, RunOptions{Timing: fast, Reauth: reauth})
	r.ready(t)
	r.typeIn(t, "LIVE")
	waitFor(t, "LIVE on the first connection", func() bool { return f.input(0) == "LIVE" })
	close(kick)
	waitFor(t, "reconnecting", func() bool { return len(r.stateList()) == 1 })
	r.typeIn(t, "STALE")
	close(release)
	waitFor(t, "the redial", func() bool { s := r.stateList(); return len(s) == 2 && s[1] == "" })
	r.typeIn(t, "FRESH")
	waitFor(t, "FRESH on the second connection", func() bool { return strings.Contains(f.input(1), "FRESH") })
	if got0, got1 := f.input(0), f.input(1); strings.Contains(got0+got1, "STALE") {
		t.Errorf("gap keystrokes reached the shell: conn0=%q conn1=%q", got0, got1)
	}
	r.detach(t)
}

func TestRun_ResumeAndClearOnFull(t *testing.T) {
	f := newFakeRelay(t, func(f *fakeRelay, i int, c *websocket.Conn) {
		if i == 0 {
			writeFrames(c, replayMeta(relay.ReplayFull), output("AAA"), offsetMeta(100), output("BBBB"), output("CC"))
			c.Close(websocket.StatusGoingAway, "") //nolint:errcheck
			return
		}
		writeFrames(c, replayMeta(relay.ReplayFull), output("XYZ"), offsetMeta(200))
		f.pump(i, c, nil) //nolint:errcheck
	}, nil)
	r := startRun(t, f, RunOptions{Timing: fast, ClearOnFull: true})
	waitFor(t, "the second connection's replay", func() bool { return strings.Contains(r.stdout(), "XYZ") })
	if got, want := r.stdout(), "AAABBBBCC\x1b[2J\x1b[HXYZ"; got != want {
		t.Errorf("stdout = %q; want %q", got, want)
	}
	f.mu.Lock()
	redial := f.urls[1]
	f.mu.Unlock()
	u, _ := url.Parse(redial)
	if got := u.Query().Get("resume"); got != "108" { // 100 + (4+1) + (2+1)
		t.Errorf("redial resume = %q; want 108", got)
	}
	r.detach(t)
}

// A socket that dies before its offset meta leaves no trusted offset: the
// next redial asks for a full replay.
func TestRun_NoOffsetMetaMeansFullReplay(t *testing.T) {
	f := newFakeRelay(t, func(f *fakeRelay, i int, c *websocket.Conn) {
		switch i {
		case 0:
			writeFrames(c, offsetMeta(7))
		case 1:
			writeFrames(c, replayMeta(relay.ReplayDelta), output("PART"))
		default:
			f.pump(i, c, nil) //nolint:errcheck
		}
	}, nil)
	r := startRun(t, f, RunOptions{Timing: fast})
	waitFor(t, "a third connection", func() bool { return f.accepts() >= 3 })
	f.mu.Lock()
	urls := append([]string(nil), f.urls...)
	f.mu.Unlock()
	if !strings.Contains(urls[1], "resume=7") || strings.Contains(urls[2], "resume=") {
		t.Errorf("redial URLs = %q; want resume=7 then none", urls[1:])
	}
	r.detach(t)
}

// WR-02: a socket the daemon proxy accepts and then refuses (1013, before any
// frame), or one that dies after only the grid frame, rendered nothing: the
// old offset is still exact and the next redial keeps asking for the delta.
func TestRun_RefusedSocketKeepsResume(t *testing.T) {
	f := newFakeRelay(t, func(f *fakeRelay, i int, c *websocket.Conn) {
		switch i {
		case 0:
			writeFrames(c, replayMeta(relay.ReplayFull), output("AAA"), offsetMeta(100))
			c.Close(websocket.StatusGoingAway, "") //nolint:errcheck
		case 1:
			c.Close(websocket.StatusTryAgainLater, "remote unreachable") //nolint:errcheck
		case 2:
			writeFrames(c, relay.MakeResizeFrame(80, 24))
			c.Close(websocket.StatusGoingAway, "") //nolint:errcheck
		default:
			f.pump(i, c, nil) //nolint:errcheck
		}
	}, nil)
	r := startRun(t, f, RunOptions{Timing: fast})
	waitFor(t, "a fourth connection", func() bool { return f.accepts() >= 4 })
	f.mu.Lock()
	urls := append([]string(nil), f.urls...)
	f.mu.Unlock()
	for _, u := range urls[1:4] {
		if !strings.Contains(u, "resume=100") {
			t.Errorf("redial URLs = %q; want resume=100 on each", urls[1:4])
			break
		}
	}
	r.detach(t)
}

// WR-03: a socket is connected only once the host's first frame arrives. While
// the daemon proxy is still dialing the peer, [reconnecting] stays up and
// keystrokes are discarded, never queued for the shell.
func TestRun_NotConnectedUntilFirstFrame(t *testing.T) {
	sendGrid := make(chan struct{})
	f := newFakeRelay(t, func(f *fakeRelay, i int, c *websocket.Conn) {
		if i == 0 {
			c.Close(websocket.StatusGoingAway, "") //nolint:errcheck
			return
		}
		go f.pump(i, c, nil) //nolint:errcheck
		// The proxy upgraded the socket and is still dialing the peer.
		select {
		case <-sendGrid:
		case <-f.done:
			return
		}
		writeFrames(c, relay.MakeResizeFrame(80, 24))
		<-f.done
	}, nil)
	r := startRun(t, f, RunOptions{Timing: fast})
	waitFor(t, "the redial accepted", func() bool { return f.accepts() >= 2 })
	r.typeIn(t, "EARLY")
	time.Sleep(50 * time.Millisecond)
	if s := r.stateList(); len(s) != 1 || s[0] != "reconnecting" {
		t.Errorf("states before the host's first frame = %q; want only reconnecting", s)
	}
	close(sendGrid)
	waitFor(t, "connected", func() bool { s := r.stateList(); return len(s) == 2 && s[1] == "" })
	r.typeIn(t, "FRESH")
	waitFor(t, "FRESH on the second connection", func() bool { return strings.Contains(f.input(1), "FRESH") })
	if got := f.input(1); strings.Contains(got, "EARLY") {
		t.Errorf("keystrokes typed before the host answered reached the shell: %q", got)
	}
	r.detach(t)
}

func TestRun_HealthyResetRestartsBackoff(t *testing.T) {
	const minB = 200 * time.Millisecond
	f := newFakeRelay(t, func(f *fakeRelay, i int, c *websocket.Conn) {
		switch i {
		case 0, 1: // short-lived: the schedule continues
		case 2:
			time.Sleep(3 * minB) // open longer than HealthyReset
		default:
			f.pump(i, c, nil) //nolint:errcheck
		}
	}, nil)
	tm := Timing{PingEvery: time.Hour, DeadAfter: time.Hour, BackoffMin: minB, BackoffMax: 10 * time.Second, HealthyReset: 5 * minB / 2}
	r := startRun(t, f, RunOptions{Timing: tm})
	deadline := time.Now().Add(5 * time.Second)
	for f.accepts() < 4 {
		if time.Now().After(deadline) {
			t.Fatal("no fourth connection within 5s")
		}
		time.Sleep(10 * time.Millisecond)
	}
	f.mu.Lock()
	gap := func(i int) time.Duration { return f.accepted[i+1].Sub(f.closed[i]) }
	afterShort, afterHealthy := gap(1), gap(2)
	f.mu.Unlock()
	if afterShort < 2*minB*8/10 { // attempt 1: 2*min ±20 %
		t.Errorf("gap after a short-lived connection = %v; want the schedule to continue (>= %v)", afterShort, 2*minB*8/10)
	}
	if afterHealthy >= 4*minB*8/10 { // attempt 2 would be 4*min ±20 %
		t.Errorf("gap after a healthy connection = %v; want a reset to about %v", afterHealthy, minB)
	}
	r.detach(t)
}

func TestRun_FinalCloseStops(t *testing.T) {
	f := newFakeRelay(t, func(f *fakeRelay, i int, c *websocket.Conn) {
		c.Close(websocket.StatusNormalClosure, relay.CloseReasonSessionEnded) //nolint:errcheck
	}, nil)
	r := startRun(t, f, RunOptions{Timing: fast})
	select {
	case err := <-r.done:
		if !relay.IsTerminalClose(err) {
			t.Fatalf("Run = %v; want the session-ended close", err)
		}
	case <-time.After(loopWait):
		t.Fatal("Run did not return after a session-ended close")
	}
	if n, states := f.accepts(), r.stateList(); n != 1 || len(states) != 0 {
		t.Errorf("after a final close: %d connections, states %q; want 1 and none", n, states)
	}
}

// slowWake waits whole seconds between redials, so a redial within 200 ms of
// a wake can only come from the wake.
var slowWake = Timing{PingEvery: time.Hour, DeadAfter: time.Hour, BackoffMin: time.Second, BackoffMax: 10 * time.Second, HealthyReset: time.Hour, ProbeGrace: time.Hour, WakeMinGap: 5 * time.Second}

// sendWake delivers one wake signal or fails the test if nobody takes it.
func sendWake(t *testing.T, wake chan wakewatch.Reason, r wakewatch.Reason) {
	t.Helper()
	select {
	case wake <- r:
	case <-time.After(loopWait):
		t.Fatalf("wake signal %q not taken within %v", r, loopWait)
	}
}

func TestRun_WakeSkipsBackoff(t *testing.T) {
	f := newFakeRelay(t, func(f *fakeRelay, i int, c *websocket.Conn) {
		if i < 2 {
			return // connections 0 and 1 drop at once
		}
		f.pump(i, c, nil) //nolint:errcheck
	}, nil)
	wake := make(chan wakewatch.Reason, 1)
	r := startRun(t, f, RunOptions{Timing: slowWake, Wake: wake})
	waitFor(t, "reconnecting", func() bool { return len(r.stateList()) > 0 })
	woke := time.Now()
	sendWake(t, wake, wakewatch.Woke)
	waitFor(t, "the wake redial", func() bool { return f.accepts() >= 2 })
	if d := time.Since(woke); d > 200*time.Millisecond {
		t.Errorf("redial %v after the wake; want within 200ms (no backoff wait)", d)
	}
	waitFor(t, "a third connection", func() bool { return f.accepts() >= 3 })
	f.mu.Lock()
	gap := f.accepted[2].Sub(f.closed[1])
	f.mu.Unlock()
	if gap >= 1500*time.Millisecond { // first step is 1s ±20 %, the second 2s ±20 %
		t.Errorf("backoff after the wake redial = %v; want the first step (about 1s): the attempt counter must restart", gap)
	}
	r.detach(t)
}

func TestRun_WakeMinGap(t *testing.T) {
	f := newFakeRelay(t, func(*fakeRelay, int, *websocket.Conn) {}, nil)
	var mu sync.Mutex
	calls := 0
	reauth := func(context.Context) Verdict { mu.Lock(); calls++; mu.Unlock(); return VerdictRetry }
	n := func() int { mu.Lock(); defer mu.Unlock(); return calls }
	wake := make(chan wakewatch.Reason, 1)
	r := startRun(t, f, RunOptions{Timing: slowWake, Wake: wake, Reauth: reauth})
	waitFor(t, "reconnecting", func() bool { return len(r.stateList()) > 0 })
	woke := time.Now()
	sendWake(t, wake, wakewatch.Woke)
	waitFor(t, "the wake redial", func() bool { return n() >= 1 })
	if d := time.Since(woke); d > 200*time.Millisecond {
		t.Errorf("redial %v after the wake; want within 200ms", d)
	}
	time.Sleep(100 * time.Millisecond)
	sendWake(t, wake, wakewatch.NetworkChanged)
	time.Sleep(300 * time.Millisecond)
	if got := n(); got != 1 {
		t.Errorf("two wake signals 100ms apart caused %d redials; want 1", got)
	}
	r.detach(t)
}

// ready waits for the READY output a fake writes after its opening frames, so
// the loop has processed them (a leading pong arms the watchdog).
func (r *runLoop) ready(t *testing.T) {
	t.Helper()
	waitFor(t, "READY", func() bool { return strings.Contains(r.stdout(), "READY") })
}

// A wake or network change on an open, ponged connection probes at once; an
// unanswered probe drops the socket after ProbeGrace and redials with no
// backoff wait (BackoffMin is longer than waitFor allows).
func TestRun_WakeProbesOpenSocket(t *testing.T) {
	for _, reason := range []wakewatch.Reason{wakewatch.Woke, wakewatch.NetworkChanged} {
		t.Run(string(reason), func(t *testing.T) {
			probed := make(chan time.Time, 1)
			f := newFakeRelay(t, func(f *fakeRelay, i int, c *websocket.Conn) {
				if i > 0 {
					f.pump(i, c, nil) //nolint:errcheck
					return
				}
				writeFrames(c, relay.PongFrame, output("READY"))
				for { // then silent: read, never answer
					_, m, err := c.Read(context.Background())
					if err != nil {
						return
					}
					if typ, _, _ := relay.ParseFrame(m); typ == relay.MsgPing {
						select {
						case probed <- time.Now():
						default:
						}
					}
				}
			}, nil)
			tm := slowWake
			tm.BackoffMin, tm.ProbeGrace = 5*time.Second, 200*time.Millisecond
			wake := make(chan wakewatch.Reason, 1)
			r := startRun(t, f, RunOptions{Timing: tm, Wake: wake})
			r.ready(t)
			woke := time.Now()
			sendWake(t, wake, reason)
			select {
			case p := <-probed:
				if d := p.Sub(woke); d > 200*time.Millisecond {
					t.Errorf("probe ping %v after the wake; want at once", d)
				}
			case <-time.After(loopWait):
				t.Fatal("no probe ping after the wake")
			}
			waitFor(t, "the redial after the unanswered probe", func() bool { return f.accepts() >= 2 })
			if d := time.Since(woke); d < tm.ProbeGrace || d > tm.ProbeGrace+time.Second {
				t.Errorf("redial %v after the wake; want just after the %v probe grace", d, tm.ProbeGrace)
			}
			r.detach(t)
		})
	}
}

func TestRun_WakeHealthySocketKept(t *testing.T) {
	f := newFakeRelay(t, func(f *fakeRelay, i int, c *websocket.Conn) {
		writeFrames(c, relay.PongFrame, output("READY"))
		f.pump(i, c, nil) //nolint:errcheck
	}, nil)
	tm := slowWake
	tm.ProbeGrace = 100 * time.Millisecond
	wake := make(chan wakewatch.Reason, 1)
	r := startRun(t, f, RunOptions{Timing: tm, Wake: wake})
	r.ready(t)
	sendWake(t, wake, wakewatch.Woke)
	waitFor(t, "the probe ping", func() bool { return f.pingCount() >= 1 })
	time.Sleep(3 * tm.ProbeGrace)
	if n, s := f.accepts(), r.stateList(); n != 1 || len(s) != 0 {
		t.Errorf("a healthy socket after a wake: %d connections, states %q; want 1 and none", n, s)
	}
	r.detach(t)
}

func TestRun_WakeNoPongHostKept(t *testing.T) {
	f := newFakeRelay(t, func(f *fakeRelay, i int, c *websocket.Conn) {
		writeFrames(c, output("READY"))
		f.pump(i, c, func() bool { return false }) //nolint:errcheck
	}, nil)
	tm := slowWake
	tm.ProbeGrace = 100 * time.Millisecond
	wake := make(chan wakewatch.Reason, 1)
	r := startRun(t, f, RunOptions{Timing: tm, Wake: wake})
	r.ready(t)
	sendWake(t, wake, wakewatch.Woke)
	waitFor(t, "the probe ping", func() bool { return f.pingCount() >= 1 })
	time.Sleep(3 * tm.ProbeGrace)
	if n := f.accepts(); n != 1 {
		t.Errorf("an older host that never pongs was redialed after a wake (%d connections)", n)
	}
	r.detach(t)
}

func TestRun_WakeStaleInputNeverSent(t *testing.T) {
	f := newFakeRelay(t, func(f *fakeRelay, i int, c *websocket.Conn) {
		if i == 0 {
			writeFrames(c, relay.PongFrame, output("READY"))
			f.pump(i, c, func() bool { return false }) //nolint:errcheck // records input, ignores the probe
			return
		}
		writeFrames(c, replayMeta(relay.ReplayDelta), offsetMeta(0))
		f.pump(i, c, nil) //nolint:errcheck
	}, nil)
	release := make(chan struct{})
	reauth := func(ctx context.Context) Verdict {
		select {
		case <-release:
		case <-ctx.Done():
		}
		return VerdictOK
	}
	tm := slowWake
	tm.BackoffMin, tm.ProbeGrace = 5*time.Second, 100*time.Millisecond
	wake := make(chan wakewatch.Reason, 1)
	r := startRun(t, f, RunOptions{Timing: tm, Wake: wake, Reauth: reauth})
	r.ready(t)
	r.typeIn(t, "LIVE")
	waitFor(t, "LIVE on the first connection", func() bool { return f.input(0) == "LIVE" })
	sendWake(t, wake, wakewatch.Woke)
	waitFor(t, "reconnecting after the unanswered probe", func() bool { return len(r.stateList()) == 1 })
	r.typeIn(t, "STALE")
	close(release)
	waitFor(t, "the wake redial", func() bool { s := r.stateList(); return len(s) == 2 && s[1] == "" })
	r.typeIn(t, "FRESH")
	waitFor(t, "FRESH on the second connection", func() bool { return strings.Contains(f.input(1), "FRESH") })
	if got0, got1 := f.input(0), f.input(1); strings.Contains(got0+got1, "STALE") {
		t.Errorf("gap keystrokes reached the shell: conn0=%q conn1=%q", got0, got1)
	}
	r.detach(t)
}
