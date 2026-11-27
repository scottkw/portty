package daemon

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/pion/webrtc/v4"

	"github.com/scottkw/portty/internal/relay"
	"github.com/scottkw/portty/internal/webserver"
	"github.com/scottkw/portty/internal/webserver/webservertest"
)

// guestEnv is a desktop test daemon whose share server answers WhoIs as who
// (the Colleague by default), so a guest can knock. The fake is shared with the
// test: Set switches who the next request appears to come from.
func guestEnv(t *testing.T) (*desktopEnv, *webservertest.WhoIs) {
	t.Helper()
	e := newDesktopEnv(t)
	who := webservertest.New(webservertest.Colleague)
	e.api.SetShareSeamsForTest(healthyTailnet, func(h webserver.TailscaleHealth, m *relay.HubManager) (*webserver.WebServer, error) {
		ws, err := loopbackShareServer(h, m)
		if err != nil {
			return nil, err
		}
		ws.SetWhoIsClientForTest(who)
		return ws, nil
	}, noWarm)
	return e, who
}

// shareStreamHolder is the GUI: it holds the events stream and keeps every line.
type shareStreamHolder struct {
	cancel context.CancelFunc
	done   chan error
	lines  chan DesktopShareLine

	mu   sync.Mutex
	seen []string // every line as the wire would carry it
}

// holdShareStream starts the stream and waits for its first line.
func holdShareStream(t *testing.T, c *DaemonClient) *shareStreamHolder {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	s := &shareStreamHolder{cancel: cancel, done: make(chan error, 1), lines: make(chan DesktopShareLine, 1024)}
	go func() {
		s.done <- c.StreamDesktopShareEvents(ctx, func(l DesktopShareLine) {
			b, _ := json.Marshal(l)
			s.mu.Lock()
			s.seen = append(s.seen, string(b))
			s.mu.Unlock()
			select {
			case s.lines <- l:
			default:
			}
		})
	}()
	t.Cleanup(cancel)
	if l := s.until(t, 5*time.Second, "the first state line", func(l DesktopShareLine) bool { return l.Type == "state" }); l.Active {
		t.Fatalf("first state line is active before any share: %+v", l)
	}
	return s
}

// until reads lines until pred holds for one.
func (s *shareStreamHolder) until(t *testing.T, within time.Duration, what string, pred func(DesktopShareLine) bool) DesktopShareLine {
	t.Helper()
	deadline := time.After(within)
	for {
		select {
		case l := <-s.lines:
			if pred(l) {
				return l
			}
		case <-deadline:
			t.Fatalf("timed out after %v waiting for %s", within, what)
		}
	}
}

func (s *shareStreamHolder) event(name string) func(DesktopShareLine) bool {
	return func(l DesktopShareLine) bool { return l.Type == "event" && l.Event == name }
}

// wire returns every line seen so far, one string per line.
func (s *shareStreamHolder) wire() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]string(nil), s.seen...)
}

// guest is a pion guest on /desktop/guest/ws: it keeps reading the signalling
// socket so the close code is observed.
type guest struct {
	conn *websocket.Conn
	pc   *webrtc.PeerConnection
	msgs chan map[string]any
	done chan struct{}
	code atomic.Int32
}

func guestWSURL(link string) (wsURL, origin string, err error) {
	u, err := url.Parse(link)
	if err != nil {
		return "", "", err
	}
	return "wss://" + u.Host + "/desktop/guest/ws?cap=" + url.QueryEscape(u.Query().Get("cap")), "https://" + u.Host, nil
}

// connectGuest dials the guest socket and sends hello.
func connectGuest(t *testing.T, link string) *guest {
	t.Helper()
	wsURL, origin, err := guestWSURL(link)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	conn, _, err := websocket.Dial(ctx, wsURL, &websocket.DialOptions{
		HTTPClient: insecureClient(),
		HTTPHeader: http.Header{"Origin": {origin}},
	})
	if err != nil {
		t.Fatalf("dial /desktop/guest/ws: %v", err)
	}
	t.Cleanup(func() { _ = conn.CloseNow() })
	g := &guest{conn: conn, msgs: make(chan map[string]any, 64), done: make(chan struct{})}
	g.code.Store(-1)
	writeWS(ctx, t, conn, map[string]any{"type": "hello", "v": 1, "caps": map[string]int{"video": 1}})
	go func() {
		defer close(g.done)
		for {
			_, b, err := conn.Read(context.Background())
			if err != nil {
				if cs := websocket.CloseStatus(err); cs != -1 {
					g.code.Store(int32(cs))
				}
				return
			}
			var m map[string]any
			if json.Unmarshal(b, &m) == nil {
				select {
				case g.msgs <- m:
				default:
				}
			}
		}
	}()
	return g
}

// waitMsg waits for a message of type typ (and, when state is not empty, with
// that state).
func (g *guest) waitMsg(t *testing.T, typ, state string) map[string]any {
	t.Helper()
	deadline := time.After(15 * time.Second)
	for {
		select {
		case m := <-g.msgs:
			if m["type"] == typ && (state == "" || m["state"] == state) {
				return m
			}
		case <-g.done:
			t.Fatalf("guest socket closed (%d) waiting for %s %s", g.code.Load(), typ, state)
		case <-deadline:
			t.Fatalf("timed out waiting for guest message %s %s", typ, state)
		}
	}
}

// offer finishes the handshake once admitted: offer out, answer in.
func (g *guest) offer(t *testing.T) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	pc, _ := newPionViewer(t)
	g.pc = pc
	off, err := pc.CreateOffer(nil)
	if err != nil {
		t.Fatal(err)
	}
	gathered := webrtc.GatheringCompletePromise(pc)
	if err := pc.SetLocalDescription(off); err != nil {
		t.Fatal(err)
	}
	<-gathered
	writeWS(ctx, t, g.conn, map[string]any{"type": "offer", "sdp": pc.LocalDescription().SDP})
	answer := g.waitMsg(t, "answer", "")
	if err := pc.SetRemoteDescription(webrtc.SessionDescription{Type: webrtc.SDPTypeAnswer, SDP: answer["sdp"].(string)}); err != nil {
		t.Fatal(err)
	}
}

func (g *guest) waitClosed(t *testing.T, within time.Duration) int {
	t.Helper()
	select {
	case <-g.done:
		return int(g.code.Load())
	case <-time.After(within):
		t.Fatalf("guest socket still open after %v", within)
		return 0
	}
}

func isShareErr(err error, code string) bool {
	var se *ShareError
	return errors.As(err, &se) && se.Code == code
}

func TestDesktopShareAPI_Tracer(t *testing.T) {
	e, _ := guestEnv(t)
	gui := holdShareStream(t, e.client)

	start, err := e.client.StartDesktopShare()
	if err != nil {
		t.Fatalf("StartDesktopShare: %v", err)
	}
	if !strings.Contains(start.URL, "/desktop/guest/?cap=") || !start.FirstShare {
		t.Fatalf("start = %+v; want a /desktop/guest/?cap= link and firstShare", start)
	}
	if code, _ := guestPageStatus(t, start.URL); code != http.StatusOK {
		t.Errorf("guest page with the new link = %d, want 200", code)
	}
	gui.until(t, 5*time.Second, "an active state line", func(l DesktopShareLine) bool { return l.Type == "state" && l.Active })

	g := connectGuest(t, start.URL)
	g.waitMsg(t, "admission", "waiting")
	knock := gui.until(t, 5*time.Second, "the admit_request event", gui.event("admit_request"))
	if knock.Viewer == nil || knock.Viewer.DisplayName != "Pat Colleague" || knock.Viewer.Mode != "guest" {
		t.Fatalf("admit_request viewer = %+v; want the Colleague as a guest", knock.Viewer)
	}
	st := gui.until(t, 5*time.Second, "a state line with the pending guest", func(l DesktopShareLine) bool {
		return l.Type == "state" && len(l.Pending) == 1
	})
	if st.Pending[0].ID != knock.Viewer.ID {
		t.Errorf("pending ID %q != the event's request ID %q", st.Pending[0].ID, knock.Viewer.ID)
	}
	if len(st.Viewers) != 0 {
		t.Errorf("a waiting guest is listed as a viewer: %+v", st.Viewers)
	}

	if err := e.client.AdmitDesktopGuest(knock.Viewer.ID); err != nil {
		t.Fatalf("AdmitDesktopGuest: %v", err)
	}
	g.waitMsg(t, "admission", "admitted")
	g.offer(t)
	join := gui.until(t, 10*time.Second, "the join event", gui.event("join"))
	if join.Viewer == nil || join.Viewer.Mode != "guest" || join.Viewer.Control {
		t.Fatalf("join viewer = %+v; want a view-only guest", join.Viewer)
	}
	gui.until(t, 5*time.Second, "a state line with the guest watching", func(l DesktopShareLine) bool {
		return l.Type == "state" && len(l.Viewers) == 1 && l.Viewers[0].Mode == "guest" && !l.Viewers[0].Control && len(l.Pending) == 0
	})

	// The stream never carries a token, a link or an address (T-22-25).
	for _, line := range gui.wire() {
		for _, bad := range []string{"cap=", "token", "100.", "127.0.0.1", "userId", "tailnetIp"} {
			if strings.Contains(line, bad) {
				t.Errorf("stream line %q contains %q", line, bad)
			}
		}
	}
}

func TestDesktopShareAPI_GuiRequired(t *testing.T) {
	e, _ := guestEnv(t)
	_, err := e.client.StartDesktopShare()
	if !isShareErr(err, "gui_required") {
		t.Fatalf("StartDesktopShare with no GUI = %v; want gui_required", err)
	}
	if shareServerRunning(e.api) {
		t.Error("a refused start brought the share server up")
	}
	if st, err := e.client.DesktopShare(); err != nil || st.Active || st.URL != "" {
		t.Errorf("GET /desktop/share after a refused start = %+v, %v; want inactive", st, err)
	}
}

// guestPageStatus fetches the guest page for link as the Colleague.
func guestPageStatus(t *testing.T, link string) (int, string) {
	t.Helper()
	resp, err := insecureClient().Get(link)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	return resp.StatusCode, resp.Header.Get("X-Portty-Desktop")
}
