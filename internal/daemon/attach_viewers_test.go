package daemon

// UAT gap 1 (04-UAT.md, major, intermittent): the first
// `portty attach <office>:"shell 1"` from the home Mac blanked the home GUI tab
// and a browser on the same shared session (a lone `-` top-left), the CLI bar
// read `4 viewers` / `[reconnecting]`, the office window kept working, and
// detaching the CLI (Ctrl-\) restored the GUI tab and browser within about
// 10 s. A second attach and a 3x attach/detach loop did not reproduce.
//
// This harness runs the real attach path in-process with fake local clients and
// no tailnet: the host's share server (cap and Origin gated), the host's relay
// hub, the viewing Mac's daemon proxy (/api/relay/remote/{id}/ws) and the real
// CLI pump (attach.AttachSession), against one shared /bin/cat session.
//
// Verdicts (04-07, ranked hypotheses; results on unmodified code):
//
//   - H-grid (host-local tiny grid push, the best fit for "guests blank, office
//     fine"): no in-process trigger. Only Hub.ResizeClient broadcasts a 0x02 and
//     only for an Origin "local" subscriber; the remote CLI is Origin "web".
//     Not reproducible in-process. Live evidence: 04-08 viewer-diag grid lines,
//     TESTING.md M-83.
//   - H-net (first-contact tailnet path stall, fits `[reconnecting]` and the
//     ~10 s recovery): no in-process trigger. Not reproducible in-process. Live
//     evidence: 04-08 proxy dial/close lines, M-83 `tailscale ping`.
//   - H-replies (replayed query replies typed into the shared shell):
//     replayed_query_replies FAILED (local, guiTab and browser each got 54
//     echoed reply bytes). Reproduced and fixed: Hub.Join strips
//     reply-eliciting queries. It hits every viewer including the office's own
//     panel, so it does not match the UAT signature.
//   - H-resize (the CLI reports a tiny size): tiny_resize_0x0, tiny_resize_1x1
//     and tiny_resize_2x1 PASSED. Refuted.
//   - H-stuck (a CLI that stops reading blocks fan-out): stuck_cli_never_reads
//     PASSED. Refuted.
//
// Root cause: not pinned — no in-process variant reproduces the UAT signature (guests lose output or get a non-host grid while the office panel is fine), so the live repro relies on the 04-08 diagnostic log and TESTING.md M-83.
//
// Live evidence (04-08): the daemon writes `viewer-diag:` lines to daemon.log
// (grid, join, leave, slow, revoke, first input, web resize ignored on the host;
// proxy open, dial failed, end on the viewing Mac). TestCLIAttach_OtherViewersKeepOutput
// checks a whole CLI attach logs that timeline without the cap.
//
// Long history (04-10, UAT gap 4): TestCLIAttach_LongHistoryJoiners has a GUI
// tab and the real CLI join through the proxy after about 120 KiB of history.
// Both failed after the grid frame on the old code (coder/websocket's 32 KiB
// per-message default on the proxy upstream and the CLI conn). Go readers of
// host-to-viewer frames now use relay.ViewerReadLimit.
//
// Reconnect (05-06, SHAR-07): the CLI here is the real remote loop
// (attach.Run) where a test needs reconnects. TestCLIAttach_ReconnectsWithDelta
// kicks it with a non-final close and diffs every typed line against the host's
// own viewer; TestCLIAttach_ViewingDaemonRestart reconnects it through a fresh
// viewing daemon after re-auth; TestCLIAttach_ShareOffWhileConnected and
// TestCLIAttach_ReauthShareStoppedEnds pin the two ways a share-off ends it
// without a redial. The UAT gap 1 [reconnecting]-on-idle false positive (the
// old five-second no-output watcher) is fixed by loop-driven state and pinned by
// TestCLIAttach_IdleNoReconnecting.

import (
	"bytes"
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"net/url"
	"regexp"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	xvt "github.com/charmbracelet/x/vt"
	"github.com/coder/websocket"
	"github.com/scottkw/portty/internal/attach"
	"github.com/scottkw/portty/internal/relay"
)

const viewerWait = 2 * time.Second

// reconnectWait bounds a CLI redial. A healthy one takes about 100 ms, but under a
// loaded race run macOS sometimes leaves a fresh loopback connect pending until
// the dialer gives up. The CLI then abandons that dial after attach's 10 s
// dialTimeout and redials at once, which is the product working as designed
// (seen at 10.1 s, full replay intact), so the wait allows one stalled dial.
const reconnectWait = 20 * time.Second

// viewerTap records everything one viewer connection receives.
type viewerTap struct {
	mu    sync.Mutex
	out   []byte   // MsgOutput payloads in arrival order
	grids [][2]int // every MsgResize (0x02) grid frame as {cols, rows}
	err   error    // the read error that ended the loop; nil while open
}

func newViewerTap(ctx context.Context, conn *websocket.Conn) *viewerTap {
	v := &viewerTap{}
	go func() {
		for {
			_, m, err := conn.Read(ctx)
			v.mu.Lock()
			if err != nil {
				v.err = err
				v.mu.Unlock()
				return
			}
			typ, p, _ := relay.ParseFrame(m)
			switch {
			case typ == relay.MsgOutput:
				v.out = append(v.out, p...)
			case typ == relay.MsgResize && len(p) >= 4:
				v.grids = append(v.grids, [2]int{int(p[0])<<8 | int(p[1]), int(p[2])<<8 | int(p[3])})
			}
			v.mu.Unlock()
		}
	}()
	return v
}

func (v *viewerTap) state() (out []byte, grids [][2]int, err error) {
	v.mu.Lock()
	defer v.mu.Unlock()
	return append([]byte(nil), v.out...), append([][2]int(nil), v.grids...), v.err
}

func (v *viewerTap) outLen() int {
	v.mu.Lock()
	defer v.mu.Unlock()
	return len(v.out)
}

func (v *viewerTap) has(s string) bool {
	v.mu.Lock()
	defer v.mu.Unlock()
	return bytes.Contains(v.out, []byte(s))
}

// waitUntil polls cond for up to viewerWait.
func waitUntil(t *testing.T, what string, cond func() bool) {
	t.Helper()
	waitFor(t, what, viewerWait, cond)
}

// waitFor polls cond for up to d.
func waitFor(t *testing.T, what string, d time.Duration, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(d)
	for !cond() {
		if time.Now().After(deadline) {
			t.Fatalf("timed out after %v waiting for %s", d, what)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

// lockedBuffer is the CLI's stdout: AttachSession writes from its pump goroutine.
type lockedBuffer struct {
	mu sync.Mutex
	b  bytes.Buffer
}

func (l *lockedBuffer) Write(p []byte) (int, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.b.Write(p)
}

func (l *lockedBuffer) bytes() []byte {
	l.mu.Lock()
	defer l.mu.Unlock()
	return bytes.Clone(l.b.Bytes())
}

func (l *lockedBuffer) has(s string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	return bytes.Contains(l.b.Bytes(), []byte(s))
}

// attachScene is one shared session with three viewers attached: the office
// GUI's local relay panel at 120x40, the home GUI tab through the viewing
// daemon's proxy, and a browser on the share link.
type attachScene struct {
	ctx       context.Context
	hub       *relay.Hub
	sid       string
	capToken  string
	shareBase string // https://<host share server>, the cap's base URL
	viewRelay string // ws://<viewing daemon relay>
	local     *websocket.Conn
	viewers   map[string]*viewerTap // "local", "guiTab", "browser"
}

func dialViewer(ctx context.Context, t *testing.T, name, wsURL, origin string, client *http.Client) *websocket.Conn {
	t.Helper()
	hdr := http.Header{}
	if origin != "" {
		hdr.Set("Origin", origin)
	}
	conn, _, err := websocket.Dial(ctx, wsURL, &websocket.DialOptions{HTTPHeader: hdr, HTTPClient: client})
	if err != nil {
		t.Fatalf("dial %s: %v", name, err) // never print wsURL: the browser's carries the cap
	}
	t.Cleanup(func() { conn.CloseNow() })
	return conn
}

// newViewingDaemon builds a viewing Mac's daemon with an empty cap store and
// serves its relay; it returns the API and the relay's ws:// base.
func newViewingDaemon(t *testing.T) (*API, string) {
	t.Helper()
	viewAPI, _, _ := testDaemon(t)
	viewAPI.SetPeerClientForTest(&http.Client{ // no Timeout: proxied WebSockets are long-lived
		Transport: &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: true}}, //nolint:gosec // self-signed loopback share server
	})
	viewRelay := httptest.NewServer(viewAPI.RelayHandler())
	t.Cleanup(viewRelay.Close)
	return viewAPI, "ws://" + strings.TrimPrefix(viewRelay.URL, "http://")
}

// newAttachScene builds host, viewing daemon and the three viewers. Each
// prelude string is typed on the local conn (one line each) before MARKER-1,
// which every viewer must see before this returns.
func newAttachScene(t *testing.T, prelude ...string) *attachScene {
	t.Helper()
	return newAttachSceneTimeout(t, 15*time.Second, prelude...)
}

// newAttachSceneTimeout is newAttachScene with a longer life for tests that
// type a lot under load.
func newAttachSceneTimeout(t *testing.T, timeout time.Duration, prelude ...string) *attachScene {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	t.Cleanup(cancel)

	// Host Mac: a shared /bin/cat session and the office GUI's local relay.
	hostAPI, hostClient := shareTestDaemon(t)
	sid := newShareSession(t, hostClient)
	link, capToken := linkParts(t, mustShare(t, hostClient, sid).URL)
	hub, ok := hostAPI.engine.Manager().Get(sid)
	if !ok {
		t.Fatal("host hub not found for the shared session")
	}
	hostRelay := httptest.NewServer(hostAPI.RelayHandler())
	t.Cleanup(hostRelay.Close)

	// Viewing Mac: its own daemon holding the cap, as /peers/connect leaves it.
	viewAPI, viewRelay := newViewingDaemon(t)
	if err := viewAPI.PutRemoteCapForTest(sid, "https://"+link.Host, capToken); err != nil {
		t.Fatalf("PutRemoteCapForTest: %v", err)
	}

	s := &attachScene{
		ctx:       ctx,
		hub:       hub,
		sid:       sid,
		capToken:  capToken,
		shareBase: "https://" + link.Host,
		viewRelay: viewRelay,
		viewers:   map[string]*viewerTap{},
	}

	// (a) local: the office GUI panel sizes the grid, as its onOpen does.
	s.local = dialViewer(ctx, t, "local", "ws://"+strings.TrimPrefix(hostRelay.URL, "http://")+"/sessions/"+sid+"/ws", "", nil)
	s.viewers["local"] = newViewerTap(ctx, s.local)
	if err := s.local.Write(ctx, websocket.MessageBinary, attach.MakeClientResizeFrame(120, 40)); err != nil {
		t.Fatalf("local resize: %v", err)
	}
	waitUntil(t, "host hub grid 120x40", func() bool { return hub.Cols() == 120 && hub.Rows() == 40 })

	// (b) guiTab: the home GUI tab's route through the viewing daemon.
	gui := dialViewer(ctx, t, "guiTab", s.viewRelay+"/api/relay/remote/"+sid+"/ws", "wails://wails", nil)
	s.viewers["guiTab"] = newViewerTap(ctx, gui)

	// (c) browser: direct to the share server with the link's cap and origin.
	browserURL := "wss://" + link.Host + "/sessions/" + sid + "/ws?cap=" + url.QueryEscape(capToken)
	browser := dialViewer(ctx, t, "browser", browserURL, "https://"+link.Host, insecureNoRedirectClient())
	s.viewers["browser"] = newViewerTap(ctx, browser)

	for _, name := range []string{"guiTab", "browser"} {
		v := s.viewers[name]
		waitUntil(t, name+" first grid frame", func() bool { _, g, _ := v.state(); return len(g) > 0 })
		if _, g, _ := v.state(); g[0] != [2]int{120, 40} {
			t.Fatalf("%s first grid frame = %v; want [120 40]", name, g[0])
		}
	}

	for _, line := range prelude {
		s.typeLine(t, line)
	}
	s.typeLine(t, "MARKER-1")
	s.waitAll(t, "MARKER-1", "local", "guiTab", "browser")
	return s
}

// typeLine writes line plus a newline as MsgInput on the local conn.
func (s *attachScene) typeLine(t *testing.T, line string) {
	t.Helper()
	if err := s.local.Write(s.ctx, websocket.MessageBinary, relay.MakeInputFrame([]byte(line+"\n"))); err != nil {
		t.Fatalf("local input: %v", err)
	}
}

func (s *attachScene) waitAll(t *testing.T, marker string, names ...string) {
	t.Helper()
	for _, name := range names {
		v := s.viewers[name]
		waitUntil(t, name+" to see "+marker, func() bool { return v.has(marker) })
	}
}

// dialCLI dials the viewing daemon's proxy with no Origin (remoteRelayURL's
// route) and writes the initial resize, in runAttach's order.
func (s *attachScene) dialCLI(t *testing.T, cols, rows uint16) *websocket.Conn {
	t.Helper()
	conn := dialViewer(s.ctx, t, "cli", s.viewRelay+"/api/relay/remote/"+s.sid+"/ws", "", nil)
	if err := conn.Write(s.ctx, websocket.MessageBinary, attach.MakeClientResizeFrame(cols, rows)); err != nil {
		t.Fatalf("cli initial resize: %v", err)
	}
	return conn
}

// cliSession is a running attach.AttachSession with piped stdin and a captured stdout.
type cliSession struct {
	stdin  *io.PipeWriter
	stdout *lockedBuffer
	done   chan error
}

// attachCLI runs the real CLI pump on conn and waits for the catch-up replay.
func (s *attachScene) attachCLI(t *testing.T, conn *websocket.Conn) *cliSession {
	t.Helper()
	pr, pw := io.Pipe()
	t.Cleanup(func() { pw.Close() })
	c := &cliSession{stdin: pw, stdout: &lockedBuffer{}, done: make(chan error, 1)}
	go func() { c.done <- attach.AttachSession(s.ctx, conn, pr, c.stdout, 0x1C, nil, nil) }()
	waitUntil(t, "cli catch-up replay (MARKER-1)", func() bool { return c.stdout.has("MARKER-1") })
	return c
}

// detach types Ctrl-\ into the CLI and waits for AttachSession to return nil.
func (c *cliSession) detach(t *testing.T) {
	t.Helper()
	if _, err := c.stdin.Write([]byte{0x1C}); err != nil {
		t.Fatalf("write detach key: %v", err)
	}
	select {
	case err := <-c.done:
		if err != nil {
			t.Fatalf("AttachSession after detach = %v; want nil", err)
		}
	case <-time.After(viewerWait):
		t.Fatalf("AttachSession did not return within %v of the detach key", viewerWait)
	}
}

// loopCLI is a running attach.Run (the remote reconnect loop) with piped stdin,
// a captured stdout and the OnState calls it made.
type loopCLI struct {
	cliSession
	mu     sync.Mutex
	states []string
}

// cliURL is the viewing daemon's proxy route the CLI dials (remoteRelayURL).
func (s *attachScene) cliURL() string { return s.viewRelay + "/api/relay/remote/" + s.sid + "/ws" }

// fastTiming keeps the default liveness intervals and shortens the backoff so
// a redial lands well inside reconnectWait.
var fastTiming = attach.Timing{PingEvery: 15 * time.Second, DeadAfter: 40 * time.Second, BackoffMin: 10 * time.Millisecond, BackoffMax: 50 * time.Millisecond, HealthyReset: 30 * time.Second}

// runCLI runs attach.Run on conn (Stdin, Stdout and OnState are filled in) and
// waits for the catch-up replay.
func (s *attachScene) runCLI(t *testing.T, conn *websocket.Conn, o attach.RunOptions) *loopCLI {
	t.Helper()
	pr, pw := io.Pipe()
	t.Cleanup(func() { pw.Close() })
	c := &loopCLI{cliSession: cliSession{stdin: pw, stdout: &lockedBuffer{}, done: make(chan error, 1)}}
	o.Stdin, o.Stdout, o.DetachKey = pr, c.stdout, 0x1C
	o.OnState = func(state string) {
		c.mu.Lock()
		c.states = append(c.states, state)
		c.mu.Unlock()
	}
	if o.URL == nil {
		o.URL = s.cliURL
	}
	go func() { c.done <- attach.Run(s.ctx, conn, o) }()
	waitUntil(t, "cli catch-up replay (MARKER-1)", func() bool { return c.stdout.has("MARKER-1") })
	return c
}

// stateSeen reports whether OnState was called with state.
func (c *loopCLI) stateSeen(state string) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	for _, st := range c.states {
		if st == state {
			return true
		}
	}
	return false
}

// lastState is the most recent OnState value; ok is false before any call.
func (c *loopCLI) lastState() (state string, ok bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if len(c.states) == 0 {
		return "", false
	}
	return c.states[len(c.states)-1], true
}

// waitReconnected waits until the loop went to reconnecting and back to "".
func (c *loopCLI) waitReconnected(t *testing.T) {
	t.Helper()
	waitFor(t, "cli to reconnect", reconnectWait, func() bool {
		st, ok := c.lastState()
		return ok && st == "" && c.stateSeen("reconnecting")
	})
}

// checkSameLines asserts every tag occurs in the CLI's output (SOH frame
// headers from a replay removed) exactly as often as in the local viewer's.
func checkSameLines(t *testing.T, cli []byte, local []byte, tags []string) {
	t.Helper()
	cli = bytes.ReplaceAll(cli, []byte{relay.MsgOutput}, nil)
	local = bytes.ReplaceAll(local, []byte{relay.MsgOutput}, nil)
	for _, tag := range tags {
		if c, l := bytes.Count(cli, []byte(tag)), bytes.Count(local, []byte(tag)); c != l || l == 0 {
			t.Errorf("%s: cli has %d copies, local viewer %d (want equal, non-zero)", tag, c, l)
		}
	}
}

// captureDiag routes the standard logger (viewer-diag lines) into a buffer.
func captureDiag(t *testing.T) *lockedBuffer {
	t.Helper()
	diag := &lockedBuffer{}
	prevOut, prevFlags := log.Writer(), log.Flags()
	log.SetOutput(diag)
	t.Cleanup(func() {
		log.SetOutput(prevOut)
		log.SetFlags(prevFlags)
	})
	return diag
}

func lineTags(from, to int) []string {
	var tags []string
	for i := from; i <= to; i++ {
		tags = append(tags, fmt.Sprintf("LINE-%04d", i))
	}
	return tags
}

// SHAR-07: a remote CLI kicked with a non-final close reconnects through the
// real proxy with ?resume= and gets only what it missed. Lines typed while it
// was down reach it once each, exactly like the host's own viewer.
func TestCLIAttach_ReconnectsWithDelta(t *testing.T) {
	diag := captureDiag(t)
	s := newAttachScene(t)

	release := make(chan struct{})
	reauth := func(ctx context.Context) attach.Verdict {
		select {
		case <-release:
			return attach.VerdictOK
		case <-ctx.Done():
			return attach.VerdictRetry
		}
	}
	cli := s.runCLI(t, s.dialCLI(t, 80, 24), attach.RunOptions{Reauth: reauth, Timing: fastTiming})

	for _, tag := range lineTags(1, 50) {
		s.typeLine(t, tag)
	}
	waitUntil(t, "cli to see LINE-0050", func() bool { return cli.stdout.has("LINE-0050") })

	kickAt := len(diag.bytes())
	s.hub.RevokeWebViewers("test drop")
	waitUntil(t, "cli to go reconnecting", func() bool { return cli.stateSeen("reconnecting") })
	for _, tag := range lineTags(51, 100) {
		s.typeLine(t, tag)
	}
	s.waitAll(t, "LINE-0100", "local")
	close(release)
	cli.waitReconnected(t)

	s.typeLine(t, "MARKER-END")
	s.waitAll(t, "MARKER-END", "local")
	waitUntil(t, "cli to see MARKER-END", func() bool { return cli.stdout.has("MARKER-END") })

	local, _, _ := s.viewers["local"].state()
	checkSameLines(t, cli.stdout.bytes(), local, append(lineTags(1, 100), "MARKER-END"))
	delta := regexp.MustCompile(`viewer-diag: replay session=` + regexp.QuoteMeta(relay.DiagID(s.sid)) + ` conn=\d+ mode=delta`)
	if !delta.Match(diag.bytes()[kickAt:]) {
		t.Errorf("no delta replay logged after the kick:\n%s", diag.bytes()[kickAt:])
	}
	cli.detach(t)
}

// UAT gap 1: an idle remote attach showed [reconnecting] after five seconds
// without output. Pongs through the proxy now count as frames: an idle CLI
// with a watchdog far shorter than the idle time never leaves "connected".
func TestCLIAttach_IdleNoReconnecting(t *testing.T) {
	s := newAttachScene(t)
	tm := fastTiming
	tm.PingEvery, tm.DeadAfter = 50*time.Millisecond, 200*time.Millisecond
	cli := s.runCLI(t, s.dialCLI(t, 80, 24), attach.RunOptions{Timing: tm})

	time.Sleep(time.Second)
	if cli.stateSeen("reconnecting") {
		t.Error("idle CLI with pongs flowing went reconnecting")
	}
	s.typeLine(t, "MARKER-2")
	waitUntil(t, "cli to see MARKER-2", func() bool { return cli.stdout.has("MARKER-2") })
	cli.detach(t)
}

// waitEnded waits for Run to return and checks it ended with the host's
// share-stopped close.
func (c *loopCLI) waitEnded(t *testing.T) {
	t.Helper()
	select {
	case err := <-c.done:
		var ce websocket.CloseError
		if !errors.As(err, &ce) || ce.Code != websocket.StatusNormalClosure || ce.Reason != relay.CloseReasonShareStopped {
			t.Fatalf("Run = %v; want close 1000 %q", err, relay.CloseReasonShareStopped)
		}
	case <-time.After(viewerWait):
		t.Fatalf("Run did not end within %v", viewerWait)
	}
}

// countingURL is a RunOptions.URL that counts redials.
func countingURL(s *attachScene) (func() string, *atomic.Int32) {
	var n atomic.Int32
	return func() string { n.Add(1); return s.cliURL() }, &n
}

// Share turned off on the host while the CLI is connected: the proxy forwards
// the host's 1000 "share stopped" and the CLI stops instead of redialing.
func TestCLIAttach_ShareOffWhileConnected(t *testing.T) {
	s := newAttachScene(t)
	u, dials := countingURL(s)
	cli := s.runCLI(t, s.dialCLI(t, 80, 24), attach.RunOptions{URL: u, Timing: fastTiming})
	s.hub.RevokeWebViewers(relay.CloseReasonShareStopped)
	cli.waitEnded(t)
	if n := dials.Load(); n != 0 || cli.stateSeen("reconnecting") {
		t.Errorf("after share stopped: %d redials, reconnecting=%v; want none", n, cli.stateSeen("reconnecting"))
	}
}

// A drop, then a re-auth that says the share is gone: the CLI never dials
// again with the revoked cap (T-05-22).
func TestCLIAttach_ReauthShareStoppedEnds(t *testing.T) {
	s := newAttachScene(t)
	u, dials := countingURL(s)
	reauth := func(context.Context) attach.Verdict { return attach.VerdictShareStopped }
	cli := s.runCLI(t, s.dialCLI(t, 80, 24), attach.RunOptions{URL: u, Reauth: reauth, Timing: fastTiming})
	s.hub.RevokeWebViewers("test drop")
	cli.waitEnded(t)
	if n := dials.Load(); n != 0 {
		t.Errorf("redialed %d times after a share-stopped re-auth; want 0", n)
	}
}

// The home Mac's daemon restarted: a new relay port and an empty cap store.
// Re-auth (what /peers/connect does) refills the new daemon's cap and the CLI
// reconnects through it with a delta replay and no lost or doubled line.
func TestCLIAttach_ViewingDaemonRestart(t *testing.T) {
	diag := captureDiag(t)
	s := newAttachScene(t)
	api2, relay2 := newViewingDaemon(t)
	route2 := relay2 + "/api/relay/remote/" + s.sid + "/ws"
	if _, resp, err := websocket.Dial(s.ctx, route2, nil); err == nil || resp == nil || resp.StatusCode != http.StatusNotFound {
		t.Fatalf("dial to the restarted daemon before re-auth: err=%v resp=%v; want the no-cap 404", err, resp)
	}

	var mu sync.Mutex
	cur := s.cliURL()
	url := func() string { mu.Lock(); defer mu.Unlock(); return cur }
	release := make(chan struct{})
	reauth := func(ctx context.Context) attach.Verdict {
		select {
		case <-release:
		case <-ctx.Done():
			return attach.VerdictRetry
		}
		if err := api2.PutRemoteCapForTest(s.sid, s.shareBase, s.capToken); err != nil {
			t.Errorf("PutRemoteCapForTest: %v", err)
			return attach.VerdictRetry
		}
		mu.Lock()
		cur = route2
		mu.Unlock()
		return attach.VerdictOK
	}
	cli := s.runCLI(t, s.dialCLI(t, 80, 24), attach.RunOptions{URL: url, Reauth: reauth, Timing: fastTiming})

	for _, tag := range lineTags(1, 20) {
		s.typeLine(t, tag)
	}
	waitUntil(t, "cli to see LINE-0020", func() bool { return cli.stdout.has("LINE-0020") })
	kickAt := len(diag.bytes())
	s.hub.RevokeWebViewers("test drop")
	waitUntil(t, "cli to go reconnecting", func() bool { return cli.stateSeen("reconnecting") })
	for _, tag := range lineTags(21, 40) {
		s.typeLine(t, tag)
	}
	s.waitAll(t, "LINE-0040", "local")
	close(release)
	cli.waitReconnected(t)

	s.typeLine(t, "MARKER-END")
	s.waitAll(t, "MARKER-END", "local")
	waitUntil(t, "cli to see MARKER-END", func() bool { return cli.stdout.has("MARKER-END") })
	local, _, _ := s.viewers["local"].state()
	checkSameLines(t, cli.stdout.bytes(), local, append(lineTags(1, 40), "MARKER-END"))
	delta := regexp.MustCompile(`viewer-diag: replay session=` + regexp.QuoteMeta(relay.DiagID(s.sid)) + ` conn=\d+ mode=delta`)
	if !delta.Match(diag.bytes()[kickAt:]) {
		t.Errorf("no delta replay logged after the restart:\n%s", diag.bytes()[kickAt:])
	}
	cli.detach(t)
}

// checkUndisturbed asserts the named viewers are still open, every grid frame
// they got is the host's 120x40, and the host hub is still 120x40.
func (s *attachScene) checkUndisturbed(t *testing.T, names ...string) {
	t.Helper()
	for _, name := range names {
		_, grids, err := s.viewers[name].state()
		if err != nil {
			t.Errorf("%s was closed: %v", name, err)
		}
		for _, g := range grids {
			if g != [2]int{120, 40} {
				t.Errorf("%s got grid frame %v; want only [120 40] (the host's)", name, g)
			}
		}
	}
	if c, r := s.hub.Cols(), s.hub.Rows(); c != 120 || r != 40 {
		t.Errorf("host hub grid = %dx%d; want 120x40", c, r)
	}
}

func TestCLIAttach_OtherViewersKeepOutput(t *testing.T) {
	diag := &lockedBuffer{}
	prevOut, prevFlags := log.Writer(), log.Flags()
	log.SetOutput(diag)
	t.Cleanup(func() {
		log.SetOutput(prevOut)
		log.SetFlags(prevFlags)
	})

	s := newAttachScene(t)

	cli := s.attachCLI(t, s.dialCLI(t, 80, 24))

	s.typeLine(t, "MARKER-2")
	s.waitAll(t, "MARKER-2", "local", "guiTab", "browser")
	waitUntil(t, "cli to see MARKER-2", func() bool { return cli.stdout.has("MARKER-2") })

	cli.detach(t)

	s.typeLine(t, "MARKER-3")
	s.waitAll(t, "MARKER-3", "local", "guiTab", "browser")

	s.checkUndisturbed(t, "guiTab", "browser")

	// 04-08 viewer-diag timeline: four joins, the CLI's ignored 80x24 resize
	// and its leave under the same conn number, the proxy's open and end lines.
	id8 := relay.DiagID(s.sid)
	resize := regexp.MustCompile(`viewer-diag: web resize ignored session=` + regexp.QuoteMeta(id8) + ` conn=(\d+) cols=80 rows=24`)
	var cliConn string
	waitUntil(t, "web resize ignored line for the CLI", func() bool {
		if m := resize.FindSubmatch(diag.bytes()); m != nil {
			cliConn = string(m[1])
			return true
		}
		return false
	})
	waitUntil(t, "leave line for the CLI conn", func() bool {
		return diag.has("viewer-diag: leave session=" + id8 + " conn=" + cliConn + " origin=web")
	})
	waitUntil(t, "proxy end line", func() bool { return diag.has("viewer-diag: proxy end session=" + id8) })
	if n := bytes.Count(diag.bytes(), []byte("viewer-diag: join session="+id8)); n < 4 {
		t.Errorf("join lines = %d; want 4 (local, guiTab, browser, cli):\n%s", n, diag.bytes())
	}
	if !diag.has("viewer-diag: proxy open session=" + id8) {
		t.Errorf("no proxy open line:\n%s", diag.bytes())
	}
	if diag.has(s.capToken) || diag.has(url.QueryEscape(s.capToken)) || diag.has("cap=") {
		t.Errorf("diagnostic log leaks the share link's cap")
	}
}

// Terminal queries the replayed_query_replies variant puts in the session
// history, each with the reply a real terminal (Warp, say) would type back.
var queryReplies = []struct{ query, reply string }{
	{"\x1b[6n", "\x1b[24;1R"},                           // DSR -> CPR
	{"\x1b[>c", "\x1b[>1;10;0c"},                        // DA2
	{"\x1b[?2026$p", "\x1b[?2026;2$y"},                  // DECRQM -> DECRPM
	{"\x1b[>0q", "\x1bP>|fake-term\x1b\\"},              // XTVERSION
	{"\x1b[?u", "\x1b[?0u"},                             // kitty keyboard query
	{"\x1b]11;?\x07", "\x1b]11;rgb:0000/0000/0000\x07"}, // OSC 11 background query
}

func TestCLIAttach_Hypotheses(t *testing.T) {
	// H-resize: the CLI reports a tiny size. Checks webserver/server.go
	// handleWSSRelay (MsgResize2 from a web viewer is dropped) and
	// relay/hub.go Hub.ResizeClient (non-local origin never drives the grid).
	for _, sz := range []struct {
		name       string
		cols, rows uint16
	}{
		{"tiny_resize_0x0", 0, 0},
		{"tiny_resize_1x1", 1, 1},
		{"tiny_resize_2x1", 2, 1},
	} {
		t.Run(sz.name, func(t *testing.T) {
			s := newAttachScene(t)
			cli := s.attachCLI(t, s.dialCLI(t, sz.cols, sz.rows))
			s.typeLine(t, "MARKER-2")
			s.waitAll(t, "MARKER-2", "guiTab", "browser")
			s.checkUndisturbed(t, "guiTab", "browser")
			cli.detach(t)
		})
	}

	// H-stuck: a CLI connection that never reads. Checks relay/hub.go
	// Hub.broadcast / BroadcastMeta / broadcastResize (non-blocking send plus
	// CloseSlow) end to end through the viewing daemon's copyWS proxy.
	t.Run("stuck_cli_never_reads", func(t *testing.T) {
		s := newAttachScene(t)
		s.dialCLI(t, 80, 24) // never read, no AttachSession; the host may close it
		s.typeLine(t, "MARKER-2")
		s.waitAll(t, "MARKER-2", "guiTab", "browser")
		s.checkUndisturbed(t, "guiTab", "browser")
	})

	// H-replies: the catch-up replay (relay/hub.go Hub.Join, raw
	// scrollback) carries historic queries; the CLI writes them to the user's
	// terminal, which answers on stdin; attach/attach.go StdinPump forwards the
	// answers as MsgInput, and relay/oscabsorb.go InputAbsorber.Filter drops
	// only OSC 10/11 and DA1 replies, so the rest reach the shared PTY.
	t.Run("replayed_query_replies", func(t *testing.T) {
		var prelude []string
		for _, q := range queryReplies {
			prelude = append(prelude, q.query)
		}
		s := newAttachScene(t, prelude...)

		before := map[string]int{}
		for name, v := range s.viewers {
			before[name] = v.outLen()
		}
		cli := s.attachCLI(t, s.dialCLI(t, 80, 24))

		// Fake terminal: answer each query the replay showed it. (The OSC 11
		// query typed on the local conn is absorbed by the host's InputAbsorber
		// before it reaches the PTY, so it is never in the history.)
		for _, q := range queryReplies {
			if cli.stdout.has(q.query) {
				if _, err := cli.stdin.Write([]byte(q.reply)); err != nil {
					t.Fatalf("fake terminal reply: %v", err)
				}
			}
		}
		time.Sleep(time.Second)

		// The cat session is idle, so any output now is shell input the attach
		// injected (the tty echoes it).
		for _, name := range []string{"local", "guiTab", "browser"} {
			out, _, _ := s.viewers[name].state()
			if extra := out[before[name]:]; len(extra) > 0 {
				t.Errorf("%s received %d output bytes after the CLI attached, with no one typing: %q", name, len(extra), extra)
			}
		}

		s.typeLine(t, "MARKER-2")
		s.waitAll(t, "MARKER-2", "local", "guiTab", "browser")
		cli.detach(t)
	})
}

// UAT gap 4: a viewer joining through the viewing daemon's proxy after more
// than 32 KiB of history. The catch-up preamble used to arrive as one message,
// so any Go reader left at coder/websocket's 32 KiB default dropped the joiner
// after the grid frame. Since SHAR-07 the replay is chunked (relay.ReplayChunk),
// so the joiners wait for HIST-END, the last replayed line, not MARKER-1.
func TestCLIAttach_LongHistoryJoiners(t *testing.T) {
	const histLines = 200
	tag := func(i int) string { return fmt.Sprintf("HIST-%04d-", i) }

	// The viewers join first (newAttachScene), then the history is typed one
	// line at a time: typed in one burst, the tty input queue overflows while
	// cat catches up and most lines reach the ring only once. Waiting for cat's
	// copy of each line keeps the history at two copies per line (~125 KB).
	newLongScene := func(t *testing.T) *attachScene {
		s := newAttachScene(t)
		for i := 0; i < histLines; i++ {
			s.typeLine(t, tag(i)+strings.Repeat("x", 300))
			waitUntil(t, "cat to echo "+tag(i), func() bool {
				return bytes.Count(s.hub.ScrollbackSnapshot(), []byte(tag(i))) >= 2
			})
		}
		s.typeLine(t, "HIST-END")
		s.waitAll(t, "HIST-END", "local", "guiTab", "browser")
		if n := len(s.hub.ScrollbackSnapshot()); n < 3*32768 || n >= relay.DefaultScrollbackBytes {
			t.Fatalf("history = %d bytes; it must sit between 96 KiB and the %d-byte ring, so the raw replay is one message well over the 32 KiB default and the ring has not wrapped", n, relay.DefaultScrollbackBytes)
		}
		return s
	}

	checkHistory := func(t *testing.T, who string, out []byte) {
		t.Helper()
		missing, first := 0, -1
		for i := 0; i < histLines; i++ {
			if !bytes.Contains(out, []byte(tag(i))) {
				if first < 0 {
					first = i
				}
				missing++
			}
		}
		if missing > 0 {
			t.Errorf("%s catch-up replay is missing %d of %d history lines (first missing HIST-%04d)", who, missing, histLines, first)
		}
	}

	t.Run("gui_tab_through_proxy", func(t *testing.T) {
		s := newLongScene(t)
		conn := dialViewer(s.ctx, t, "lateGuiTab", s.viewRelay+"/api/relay/remote/"+s.sid+"/ws", "wails://wails", nil)
		// This conn stands in for the webview's browser WebSocket, which has no
		// per-message limit, so only the proxy's upstream read is under test.
		conn.SetReadLimit(-1)
		late := newViewerTap(s.ctx, conn)
		s.viewers["lateGuiTab"] = late
		waitUntil(t, "late GUI tab catch-up replay (HIST-END)", func() bool { return late.has("HIST-END") })
		out, _, _ := late.state()
		checkHistory(t, "late GUI tab", out)

		s.typeLine(t, "MARKER-2")
		s.waitAll(t, "MARKER-2", "local", "guiTab", "browser", "lateGuiTab")
		s.checkUndisturbed(t, "guiTab", "browser", "lateGuiTab")
	})

	t.Run("cli_through_proxy", func(t *testing.T) {
		s := newLongScene(t)
		cli := s.attachCLI(t, s.dialCLI(t, 80, 24))
		waitUntil(t, "cli catch-up replay (HIST-END)", func() bool { return cli.stdout.has("HIST-END") })
		checkHistory(t, "cli", cli.stdout.bytes())

		s.typeLine(t, "MARKER-2")
		s.waitAll(t, "MARKER-2", "local", "guiTab", "browser")
		waitUntil(t, "cli to see MARKER-2", func() bool { return cli.stdout.has("MARKER-2") })
		s.checkUndisturbed(t, "guiTab", "browser")
		cli.detach(t)
	})
}

// screenOf feeds out (SOH frame headers removed) into an emulator at the host
// grid and returns its screen text and cursor.
func screenOf(out []byte) (string, string) {
	e := xvt.NewEmulator(120, 40)
	_, _ = e.Write(bytes.ReplaceAll(out, []byte{relay.MsgOutput}, nil))
	return e.String(), fmt.Sprint(e.CursorPosition())
}

// SHAR-07 / VERF-01 matrix row "long output during the gap": the host writes
// more than the 256 KiB ring while the CLI is down, so its resume offset falls
// off the ring. The CLI must get a full replay, clear first, and end with the
// host's screen and every line exactly as often as the host's own viewer.
func TestCLIAttach_LongOutputDuringGap(t *testing.T) {
	const gapLines = 200 // each about 1.8 KB in the ring (cat echoes it twice): ~360 KB
	tag := func(i int) string { return fmt.Sprintf("GAP-%04d-", i) }
	diag := captureDiag(t)
	s := newAttachSceneTimeout(t, 90*time.Second)

	release := make(chan struct{})
	reauth := func(ctx context.Context) attach.Verdict {
		select {
		case <-release:
			return attach.VerdictOK
		case <-ctx.Done():
			return attach.VerdictRetry
		}
	}
	cli := s.runCLI(t, s.dialCLI(t, 120, 40), attach.RunOptions{Reauth: reauth, Timing: fastTiming, ClearOnFull: true})
	for _, tg := range lineTags(1, 20) {
		s.typeLine(t, tg)
	}
	waitUntil(t, "cli to see LINE-0020", func() bool { return cli.stdout.has("LINE-0020") })

	kickAt, outAt := len(diag.bytes()), s.viewers["local"].outLen()
	s.hub.RevokeWebViewers("test drop")
	waitUntil(t, "cli to go reconnecting", func() bool { return cli.stateSeen("reconnecting") })
	// One line at a time, each under the 1024-byte canonical limit: a burst
	// overflows the tty input queue while cat catches up. The next line waits
	// for both whole copies (SOH frame headers removed): typed while cat's
	// write is still blocked, its echo lands mid-row and a row wrap can split
	// its tag in the rendered history.
	for i := 0; i < gapLines; i++ {
		line := tag(i) + strings.Repeat("y", 900)
		s.typeLine(t, line)
		waitUntil(t, "cat to echo "+tag(i), func() bool {
			ring := bytes.ReplaceAll(s.hub.ScrollbackSnapshot(), []byte{relay.MsgOutput}, nil)
			return bytes.Count(ring, []byte(line+"\r\n")) >= 2
		})
	}
	if n := s.viewers["local"].outLen() - outAt; n <= relay.DefaultScrollbackBytes {
		t.Fatalf("gap output = %d bytes; want more than the %d-byte ring", n, relay.DefaultScrollbackBytes)
	}
	close(release)
	cli.waitReconnected(t)

	s.typeLine(t, "MARKER-END")
	end := []byte("MARKER-END\r\n")
	waitUntil(t, "local and cli to show MARKER-END twice", func() bool {
		local, _, _ := s.viewers["local"].state()
		return bytes.Count(local, end) >= 2 && bytes.Count(cli.stdout.bytes(), end) >= 2
	})

	full := regexp.MustCompile(`viewer-diag: replay session=` + regexp.QuoteMeta(relay.DiagID(s.sid)) + ` conn=\d+ mode=full`)
	if !full.Match(diag.bytes()[kickAt:]) {
		t.Errorf("no full replay logged for the CLI's reconnect:\n%s", diag.bytes()[kickAt:])
	}
	out := cli.stdout.bytes()
	i := bytes.LastIndex(out, []byte("\x1b[2J\x1b[H"))
	if i < 0 {
		t.Fatal("the CLI never cleared before the full replay")
	}
	post := out[i:]
	local, _, _ := s.viewers["local"].state()
	gotText, gotCur := screenOf(post)
	wantText, wantCur := screenOf(local)
	if gotText != wantText || gotCur != wantCur {
		t.Errorf("CLI screen after the full replay differs from the host's\n--- cli (cursor %s)\n%s\n--- host (cursor %s)\n%s", gotCur, gotText, wantCur, wantText)
	}
	tags := append(lineTags(1, 20), "MARKER-END")
	for i := 0; i < gapLines; i++ {
		tags = append(tags, tag(i))
	}
	checkSameLines(t, post, local, tags) // nothing lost or duplicated by the replay
	cli.detach(t)
}
