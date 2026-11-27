package daemon

import (
	"context"
	"encoding/base64"
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

// admittedGuest runs a guest through knock, admit and the offer, and returns it
// with the viewer ID the GUI sees.
func admittedGuest(t *testing.T, e *desktopEnv, gui *shareStreamHolder, link string) (*guest, string) {
	t.Helper()
	g := connectGuest(t, link)
	g.waitMsg(t, "admission", "waiting")
	knock := gui.until(t, 5*time.Second, "the admit_request event", gui.event("admit_request"))
	if err := e.client.AdmitDesktopGuest(knock.Viewer.ID); err != nil {
		t.Fatalf("AdmitDesktopGuest: %v", err)
	}
	g.waitMsg(t, "admission", "admitted")
	g.offer(t)
	join := gui.until(t, 10*time.Second, "the join event", gui.event("join"))
	return g, join.Viewer.ID
}

// startedShare holds the stream and starts a share.
func startedShare(t *testing.T, e *desktopEnv) (*shareStreamHolder, DesktopShareStart) {
	t.Helper()
	gui := holdShareStream(t, e.client)
	start, err := e.client.StartDesktopShare()
	if err != nil {
		t.Fatalf("StartDesktopShare: %v", err)
	}
	return gui, start
}

// shareIDOf reads the share ID out of a guest link's token.
func shareIDOf(t *testing.T, link string) string {
	t.Helper()
	u, err := url.Parse(link)
	if err != nil {
		t.Fatal(err)
	}
	payload, err := base64.RawURLEncoding.DecodeString(strings.SplitN(u.Query().Get("cap"), ".", 2)[0])
	if err != nil {
		t.Fatal(err)
	}
	var c struct {
		ShareID string `json:"share_id"`
	}
	if err := json.Unmarshal(payload, &c); err != nil || c.ShareID == "" {
		t.Fatalf("token payload %q: %v", payload, err)
	}
	return c.ShareID
}

func (e *desktopEnv) guestActive() bool { return e.api.currentGuestShare() != nil }

func (e *desktopEnv) hasManager() bool {
	e.api.mu.RLock()
	defer e.api.mu.RUnlock()
	return e.api.desktopMgr != nil
}

// stillOpen reports whether the viewer's socket is still open after d.
func (v *desktopViewer) stillOpen(d time.Duration) bool {
	select {
	case <-v.done:
		return false
	case <-time.After(d):
		return true
	}
}

func TestDesktopShareAPI_StopCutsGuests(t *testing.T) {
	e, _ := guestEnv(t)
	// A terminal share keeps the server up, so the old link can still be asked.
	termID := newShareSession(t, e.client)
	mustShare(t, e.client, termID)
	gui, start := startedShare(t, e)
	g, _ := admittedGuest(t, e, gui, start.URL)

	if err := e.client.StopDesktopShare(); err != nil {
		t.Fatalf("StopDesktopShare: %v", err)
	}
	if code := g.waitClosed(t, 5*time.Second); code != 4433 {
		t.Errorf("guest closed with %d, want 4433", code)
	}
	stop := gui.until(t, 5*time.Second, "the share_stop event", gui.event("share_stop"))
	if stop.Reason != "share_stopped" {
		t.Errorf("share_stop reason = %q, want share_stopped", stop.Reason)
	}
	gui.until(t, 5*time.Second, "an inactive state line", func(l DesktopShareLine) bool { return l.Type == "state" && !l.Active })
	if code, why := guestPageStatus(t, start.URL); code != http.StatusGone || why != "off" {
		t.Errorf("old link after Stop = %d %q; want 410 off", code, why)
	}
	if e.hasManager() {
		t.Error("Stop left the Manager running with always-available off")
	}
	if !shareServerRunning(e.api) {
		t.Error("Stop stopped the server a terminal share still needs")
	}
	if err := e.client.StopDesktopShare(); err != nil {
		t.Errorf("a second Stop = %v; want it harmless", err)
	}

	again, err := e.client.StartDesktopShare()
	if err != nil {
		t.Fatalf("StartDesktopShare after Stop: %v", err)
	}
	if shareIDOf(t, again.URL) == shareIDOf(t, start.URL) {
		t.Error("a new share reused the old share ID")
	}
	if code, why := guestPageStatus(t, start.URL); code != http.StatusUnauthorized || why != "token" {
		t.Errorf("old link beside a new share = %d %q; want 401 token", code, why)
	}
	if code, _ := guestPageStatus(t, again.URL); code != http.StatusOK {
		t.Errorf("new link = %d, want 200", code)
	}
}

func TestDesktopShareAPI_StopStopsIdleServer(t *testing.T) {
	e, _ := guestEnv(t)
	_, start := startedShare(t, e)
	if !shareServerRunning(e.api) || !e.hasManager() {
		t.Fatal("a started share has no server or Manager")
	}
	if err := e.client.StopDesktopShare(); err != nil {
		t.Fatal(err)
	}
	if shareServerRunning(e.api) || e.hasManager() {
		t.Error("Stop left the server or Manager up with nothing else needing them")
	}
	if st, err := e.client.DesktopShare(); err != nil || st.Active || st.URL != "" {
		t.Errorf("GET /desktop/share after Stop = %+v, %v", st, err)
	}
	_ = start
}

func TestDesktopShareAPI_StopKeepsOwnViewer(t *testing.T) {
	e, who := guestEnv(t)
	on := e.mustOn(t)
	who.Set(webservertest.Same)
	own := connectViewer(t, on.URL)
	own.waitIDR(t, 10*time.Second)
	who.Set(webservertest.Colleague)
	gui, start := startedShare(t, e)
	g, _ := admittedGuest(t, e, gui, start.URL)

	if err := e.client.StopDesktopShare(); err != nil {
		t.Fatal(err)
	}
	if code := g.waitClosed(t, 5*time.Second); code != 4433 {
		t.Errorf("guest closed with %d, want 4433", code)
	}
	if !own.stillOpen(700 * time.Millisecond) {
		t.Errorf("Stop closed the own-account viewer (%d)", own.code.Load())
	}
	if !e.hasManager() || !shareServerRunning(e.api) {
		t.Error("Stop took down the always-available Manager or server")
	}
	if st := e.status(t); !st.AlwaysAvailable || st.Viewers != 1 {
		t.Errorf("status after Stop = %+v; want always-available with the own viewer", st)
	}
}

func TestDesktopShareAPI_RegenerateEndsShare(t *testing.T) {
	e, _ := guestEnv(t)
	gui, start := startedShare(t, e)
	g, _ := admittedGuest(t, e, gui, start.URL)

	if err := e.client.RegenerateSigningKey(); err != nil {
		t.Fatalf("RegenerateSigningKey: %v", err)
	}
	if code := g.waitClosed(t, 5*time.Second); code != 4433 {
		t.Errorf("guest closed with %d after regenerate, want 4433 (not 4401)", code)
	}
	if stop := gui.until(t, 5*time.Second, "the share_stop event", gui.event("share_stop")); stop.Reason != "key_regenerated" {
		t.Errorf("share_stop reason = %q, want key_regenerated", stop.Reason)
	}
	if e.guestActive() {
		t.Error("the guest share survived a key regeneration")
	}
}

func TestDesktopShareAPI_AlwaysOffKeepsGuest(t *testing.T) {
	e, who := guestEnv(t)
	on := e.mustOn(t)
	who.Set(webservertest.Same)
	own := connectViewer(t, on.URL)
	own.waitIDR(t, 10*time.Second)
	who.Set(webservertest.Colleague)
	gui, start := startedShare(t, e)
	g, _ := admittedGuest(t, e, gui, start.URL)

	if _, err := e.client.SetDesktopAlwaysAvailable(false); err != nil {
		t.Fatalf("PUT /desktop off: %v", err)
	}
	if code := own.waitClosed(t, 5*time.Second); code != 4410 {
		t.Errorf("own viewer closed with %d, want 4410", code)
	}
	select {
	case <-g.done:
		t.Fatalf("always-available off closed the guest (%d)", g.code.Load())
	case <-time.After(700 * time.Millisecond):
	}
	if !e.hasManager() || !shareServerRunning(e.api) || !e.guestActive() {
		t.Error("always-available off took down the Manager, the server or the share a guest is using")
	}
	if code, _ := guestPageStatus(t, start.URL); code != http.StatusOK {
		t.Errorf("guest link after always-available off = %d, want 200", code)
	}
	if err := e.client.StopDesktopShare(); err != nil {
		t.Fatal(err)
	}
	if shareServerRunning(e.api) || e.hasManager() {
		t.Error("Stop left the server or Manager up although always-available is off")
	}
}

func TestDesktopShareAPI_DaemonExit(t *testing.T) {
	e, _ := guestEnv(t)
	gui, start := startedShare(t, e)
	g, _ := admittedGuest(t, e, gui, start.URL)

	e.api.stopDesktop()
	if code := g.waitClosed(t, 5*time.Second); code != 4433 && code != 1012 {
		t.Errorf("guest closed with %d at daemon exit, want 4433 (or 1012 from the Manager closing)", code)
	}
	if e.guestActive() {
		t.Error("the share outlived the daemon's desktop")
	}
}

func rightsOf(m map[string]any) map[string]any {
	r, _ := m["rights"].(map[string]any)
	return r
}

func TestDesktopShareAPI_Rights(t *testing.T) {
	e, who := guestEnv(t)
	on := e.mustOn(t)
	who.Set(webservertest.Same)
	own := connectViewer(t, on.URL)
	own.waitIDR(t, 10*time.Second)
	who.Set(webservertest.Colleague)
	gui, start := startedShare(t, e)
	g, gid := admittedGuest(t, e, gui, start.URL)

	if err := e.client.SetDesktopViewerRights(gid, true, false); err != nil {
		t.Fatalf("SetDesktopViewerRights(guest): %v", err)
	}
	deadline := time.After(5 * time.Second)
	for got := false; !got; {
		select {
		case m := <-g.msgs:
			if m["type"] == "rights" && rightsOf(m)["control"] == true && rightsOf(m)["clipboard"] == false {
				got = true
			}
		case <-deadline:
			t.Fatal("the guest never got a rights message with control on")
		}
	}
	gui.until(t, 5*time.Second, "a state line with control on", func(l DesktopShareLine) bool {
		return l.Type == "state" && len(l.Viewers) > 0 && l.Viewers[len(l.Viewers)-1].Control
	})

	// The own-account viewer's ID is in the state, with mode own.
	var ownID string
	for _, v := range e.api.shareState().Viewers {
		if v.Mode == "own" {
			ownID = v.ID
		}
	}
	if ownID == "" {
		t.Fatal("the own-account viewer is not in the state")
	}
	if err := e.client.SetDesktopViewerRights(ownID, true, true); !isShareErr(err, "own_viewer") {
		t.Errorf("rights on an own viewer = %v; want own_viewer", err)
	}
	if err := e.client.SetDesktopViewerRights("no-such-viewer", true, true); !isShareErr(err, "no_viewer") {
		t.Errorf("rights on an unknown viewer = %v; want no_viewer", err)
	}
}

func TestDesktopShareAPI_DisconnectViewer(t *testing.T) {
	e, who := guestEnv(t)
	on := e.mustOn(t)
	who.Set(webservertest.Same)
	own := connectViewer(t, on.URL)
	own.waitIDR(t, 10*time.Second)
	who.Set(webservertest.Colleague)
	gui, start := startedShare(t, e)
	g, gid := admittedGuest(t, e, gui, start.URL)

	if err := e.client.DisconnectDesktopViewer(gid); err != nil {
		t.Fatalf("DisconnectDesktopViewer(guest): %v", err)
	}
	if code := g.waitClosed(t, 5*time.Second); code != 4432 {
		t.Errorf("revoked guest closed with %d, want 4432", code)
	}
	// Revoked for the rest of the share: the same person knocking again is refused.
	again := connectGuest(t, start.URL)
	if code := again.waitClosed(t, 5*time.Second); code != 4430 {
		t.Errorf("a revoked person's new knock closed with %d, want 4430", code)
	}

	var ownID string
	for _, v := range e.api.shareState().Viewers {
		if v.Mode == "own" {
			ownID = v.ID
		}
	}
	if err := e.client.DisconnectDesktopViewer(ownID); err != nil {
		t.Fatalf("DisconnectDesktopViewer(own): %v", err)
	}
	if code := own.waitClosed(t, 5*time.Second); code != 1001 {
		t.Errorf("disconnected own viewer closed with %d, want 1001", code)
	}
	if err := e.client.DisconnectDesktopViewer(gid); !isShareErr(err, "no_viewer") {
		t.Errorf("disconnecting a gone viewer = %v; want no_viewer", err)
	}
}

func TestDesktopShareAPI_StartIsIdempotent(t *testing.T) {
	e, _ := guestEnv(t)
	gui, start := startedShare(t, e)
	again, err := e.client.StartDesktopShare()
	if err != nil {
		t.Fatal(err)
	}
	if again.URL != start.URL || again.FirstShare {
		t.Errorf("second start = %+v; want the same link and no firstShare", again)
	}
	st, err := e.client.DesktopShare()
	if err != nil || !st.Active || st.URL != start.URL {
		t.Errorf("GET /desktop/share = %+v, %v; want active with the same link", st, err)
	}
	_ = gui
}

func TestDesktopShareAPI_ClientErrors(t *testing.T) {
	e, _ := guestEnv(t)
	if err := e.client.AdmitDesktopGuest("nope"); !isShareErr(err, "no_request") {
		t.Errorf("admit with no share = %v; want no_request", err)
	}
	gui, _ := startedShare(t, e)
	if err := e.client.AdmitDesktopGuest("nope"); !isShareErr(err, "no_request") {
		t.Errorf("admit of an unknown request = %v; want no_request", err)
	}
	if err := e.client.DenyDesktopGuest("nope"); !isShareErr(err, "no_request") {
		t.Errorf("deny of an unknown request = %v; want no_request", err)
	}
	if err := e.client.DisconnectDesktopViewer("nope"); !isShareErr(err, "no_viewer") {
		t.Errorf("disconnect of an unknown viewer = %v; want no_viewer", err)
	}
	// A transport failure is a plain error, not a ShareError.
	dead := NewDaemonClient(shortSocketPath(t, "none.sock"))
	if err := dead.StopDesktopShare(); err == nil || isShareErr(err, "") {
		t.Errorf("StopDesktopShare against no daemon = %v; want a transport error", err)
	}
	if _, err := dead.StartDesktopShare(); err == nil {
		t.Error("StartDesktopShare against no daemon succeeded")
	}
	// The stream returns nil when its context ends, after at least its state line.
	gui.cancel()
	select {
	case err := <-gui.done:
		if err != nil {
			t.Errorf("StreamDesktopShareEvents after cancel = %v; want nil", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("StreamDesktopShareEvents did not return after its context ended")
	}
}

func TestGuiLiveness_StreamCloseEndsShare(t *testing.T) {
	e, _ := guestEnv(t)
	// A terminal share keeps the server up, so the old link can still be asked.
	mustShare(t, e.client, newShareSession(t, e.client))
	gui, start := startedShare(t, e)
	g, _ := admittedGuest(t, e, gui, start.URL)

	gui.cancel()
	if code := g.waitClosed(t, 2*time.Second); code != 4433 {
		t.Errorf("guest closed with %d after the GUI left, want 4433 within 2 s", code)
	}
	eventually(t, 2*time.Second, "the share to end", func() bool { return !e.guestActive() })
	// With no GUI a new share is refused, and the old link is dead.
	if _, err := e.client.StartDesktopShare(); !isShareErr(err, "gui_required") {
		t.Errorf("start after the GUI left = %v; want gui_required", err)
	}
	if code, _ := guestPageStatus(t, start.URL); code == http.StatusOK {
		t.Error("the old guest link still opens after the GUI left")
	}
}

func TestGuiLiveness_SecondHolderIsBusy(t *testing.T) {
	e, _ := guestEnv(t)
	gui, _ := startedShare(t, e)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	err := e.client.StreamDesktopShareEvents(ctx, func(DesktopShareLine) {})
	if !isShareErr(err, "gui_busy") {
		t.Fatalf("a second stream = %v; want gui_busy", err)
	}
	// The refused holder must not end the share the first one runs.
	time.Sleep(200 * time.Millisecond)
	if !e.guestActive() {
		t.Error("a refused second stream ended the share")
	}
	gui.cancel()
	eventually(t, 2*time.Second, "the first holder's exit to end the share", func() bool { return !e.guestActive() })
}

func TestGuiLiveness_HeartbeatAndCoalescing(t *testing.T) {
	old := shareHeartbeat
	shareHeartbeat = 50 * time.Millisecond
	t.Cleanup(func() { shareHeartbeat = old })
	e, _ := guestEnv(t)
	gui := holdShareStream(t, e.client)
	gui.until(t, 3*time.Second, "a heartbeat line", func(l DesktopShareLine) bool { return l.Type == "heartbeat" })
	// A burst of changes is coalesced to at most 10 state lines a second.
	for range 50 {
		e.api.pokeShare()
	}
	time.Sleep(300 * time.Millisecond)
	n := 0
	for _, line := range gui.wire() {
		if strings.Contains(line, `"type":"state"`) {
			n++
		}
	}
	if n > 2 { // the first line, and at most one more from the unchanged facts
		t.Errorf("%d state lines for an unchanged state; want no repeats", n)
	}
}
