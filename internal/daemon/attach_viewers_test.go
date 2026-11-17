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

import (
	"bytes"
	"context"
	"crypto/tls"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/scottkw/portty/internal/attach"
	"github.com/scottkw/portty/internal/relay"
)

const viewerWait = 2 * time.Second

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
	deadline := time.Now().Add(viewerWait)
	for !cond() {
		if time.Now().After(deadline) {
			t.Fatalf("timed out after %v waiting for %s", viewerWait, what)
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

// newAttachScene builds host, viewing daemon and the three viewers. Each
// prelude string is typed on the local conn (one line each) before MARKER-1,
// which every viewer must see before this returns.
func newAttachScene(t *testing.T, prelude ...string) *attachScene {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
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
	viewAPI, _, _ := testDaemon(t)
	viewAPI.SetPeerClientForTest(&http.Client{ // no Timeout: proxied WebSockets are long-lived
		Transport: &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: true}}, //nolint:gosec // self-signed loopback share server
	})
	if err := viewAPI.PutRemoteCapForTest(sid, "https://"+link.Host, capToken); err != nil {
		t.Fatalf("PutRemoteCapForTest: %v", err)
	}
	viewRelay := httptest.NewServer(viewAPI.RelayHandler())
	t.Cleanup(viewRelay.Close)

	s := &attachScene{
		ctx:       ctx,
		hub:       hub,
		sid:       sid,
		viewRelay: "ws://" + strings.TrimPrefix(viewRelay.URL, "http://"),
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
	s := newAttachScene(t)

	cli := s.attachCLI(t, s.dialCLI(t, 80, 24))

	s.typeLine(t, "MARKER-2")
	s.waitAll(t, "MARKER-2", "local", "guiTab", "browser")
	waitUntil(t, "cli to see MARKER-2", func() bool { return cli.stdout.has("MARKER-2") })

	cli.detach(t)

	s.typeLine(t, "MARKER-3")
	s.waitAll(t, "MARKER-3", "local", "guiTab", "browser")

	s.checkUndisturbed(t, "guiTab", "browser")
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

	// H-replies: the catch-up replay (relay/hub.go Hub.ReconnectPreamble, raw
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
