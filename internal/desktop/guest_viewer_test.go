package desktop_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/coder/websocket"

	"github.com/scottkw/portty/internal/desktop"
	"github.com/scottkw/portty/internal/screencap"
)

// guestEnv is a Manager and a GuestShare behind a bare loopback WebSocket
// route: it stands in for the guest route (gate and token check are 22-01's),
// so these tests cover the signalling life cycle only. The caller's identity
// comes from the X-User and X-Node request headers.
type guestEnv struct {
	mgr *desktop.Manager
	gs  *desktop.GuestShare
	srv *httptest.Server

	admitOK atomic.Bool // the token re-check result
	joins   atomic.Int32
	leaves  atomic.Int32

	mu     sync.Mutex
	events []desktop.GuestEvent
	peer   func(addr string) (int64, string, error) // nil answers with the caller's own identity
	idents map[string]desktop.Identity              // node name to the identity the route resolved
}

func (e *guestEnv) record(ev desktop.GuestEvent) {
	e.mu.Lock()
	e.events = append(e.events, ev)
	e.mu.Unlock()
}

func (e *guestEnv) eventsOf(kind string) []desktop.GuestEvent {
	e.mu.Lock()
	defer e.mu.Unlock()
	var out []desktop.GuestEvent
	for _, ev := range e.events {
		if ev.Kind == kind {
			out = append(out, ev)
		}
	}
	return out
}

func guestIdent(uid int64, node string) desktop.Identity {
	return desktop.NewIdentity(uid, "ada"+strconv.FormatInt(uid, 10)+"@example.com", "Ada", node, "stable-"+node, "100.64.0.2")
}

// newGuestEnv builds the env; mod may adjust the Deps before the Manager is made.
func newGuestEnv(t *testing.T, mod func(*desktop.Deps)) *guestEnv {
	t.Helper()
	desktop.AllowLoopbackForTest()
	e := &guestEnv{idents: map[string]desktop.Identity{}}
	e.admitOK.Store(true)
	gs, err := desktop.NewGuestShare(nil, e.record)
	if err != nil {
		t.Fatal(err)
	}
	e.gs = gs
	deps := desktop.Deps{
		Bind:         netip.MustParseAddr("127.0.0.1"),
		MediaPort:    freeUDPPort(t),
		NewSource:    func() screencap.Source { return screencap.NewSyntheticSource(1280, 800) },
		OnViewerJoin: func(desktop.Viewer) { e.joins.Add(1) },
		OnViewerLeave: func(desktop.Viewer) {
			e.leaves.Add(1)
		},
		PeerIdentity: func(_ context.Context, addr string) (int64, string, error) {
			e.mu.Lock()
			f := e.peer
			e.mu.Unlock()
			if f != nil {
				return f(addr)
			}
			// Every test viewer here is user 1001 on node "nodeA" unless it says otherwise.
			return 1001, "stable-nodeA", nil
		},
	}
	if mod != nil {
		mod(&deps)
	}
	mgr, err := desktop.NewManager(deps)
	if err != nil {
		t.Fatal(err)
	}
	if err := mgr.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = mgr.Close() })
	e.mgr = mgr
	e.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, err := websocket.Accept(w, r, nil)
		if err != nil {
			return
		}
		uid, _ := strconv.ParseInt(r.Header.Get("X-User"), 10, 64)
		mgr.ServeGuestViewer(r.Context(), c, func() bool { return e.admitOK.Load() }, e.gs, guestIdent(uid, r.Header.Get("X-Node")))
	}))
	t.Cleanup(e.srv.Close)
	return e
}

// dial opens a guest socket as user uid on node.
func (e *guestEnv) dial(ctx context.Context, t *testing.T, uid int64, node string) *websocket.Conn {
	t.Helper()
	c, _, err := websocket.Dial(ctx, "ws"+strings.TrimPrefix(e.srv.URL, "http"), &websocket.DialOptions{
		HTTPHeader: http.Header{"X-User": {strconv.FormatInt(uid, 10)}, "X-Node": {node}},
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = c.CloseNow() })
	return c
}

// knock dials, says hello and waits for the waiting message.
func (e *guestEnv) knock(ctx context.Context, t *testing.T, uid int64, node string) *websocket.Conn {
	t.Helper()
	c := e.dial(ctx, t, uid, node)
	writeJSON(ctx, t, c, map[string]any{"type": "hello", "v": 1, "caps": map[string]int{"video": 1, "input": 1}})
	readType(ctx, t, c, "hello")
	if m := readType(ctx, t, c, "admission"); m["state"] != "waiting" {
		t.Fatalf("admission state %v, want waiting", m["state"])
	}
	return c
}

// closeOf reads until the socket ends and returns the close code and reason.
func closeOf(ctx context.Context, t *testing.T, c *websocket.Conn) (websocket.StatusCode, string) {
	t.Helper()
	for {
		_, _, err := c.Read(ctx)
		if err == nil {
			continue
		}
		var ce websocket.CloseError
		if !errors.As(err, &ce) {
			t.Fatalf("socket ended without a close frame: %v", err)
		}
		return ce.Code, ce.Reason
	}
}

// pendingN waits for n requests to be queued and returns them.
func (e *guestEnv) pendingN(t *testing.T, n int) []desktop.Request {
	t.Helper()
	eventually(t, 3*time.Second, "pending requests", func() bool { return len(e.gs.Pending()) == n })
	return e.gs.Pending()
}

// TestGuestViewer_TracerAdmit is the plan tracer: a guest knocks, waits with no
// PeerConnection, is admitted, and watches as a view-only viewer that carries
// its tailnet identity.
func TestGuestViewer_TracerAdmit(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	t.Run("admit", func(t *testing.T) {
		e := newGuestEnv(t, nil)
		c := e.dial(ctx, t, 1001, "nodeA")
		writeJSON(ctx, t, c, map[string]any{"type": "hello", "v": 1, "caps": map[string]int{"video": 1, "input": 1}})
		hello := readType(ctx, t, c, "hello")
		rights, _ := hello["rights"].(map[string]any)
		for _, k := range []string{"control", "clipboard", "surface"} {
			if v, ok := rights[k]; !ok || v != false {
				t.Errorf("hello rights[%s] = %v, want false", k, v)
			}
		}
		if m := readType(ctx, t, c, "admission"); m["state"] != "waiting" {
			t.Fatalf("admission state %v, want waiting", m["state"])
		}

		// Waiting means no PeerConnection and no viewer; an offer is refused.
		reqs := e.pendingN(t, 1)
		if n := e.mgr.Viewers(); n != 0 {
			t.Fatalf("Viewers() = %d while waiting, want 0", n)
		}
		if reqs[0].Ident.UserID != 1001 || reqs[0].Ident.StableID != "stable-nodeA" {
			t.Errorf("pending identity %+v", reqs[0].Ident)
		}

		// Ping is answered while waiting.
		writeJSON(ctx, t, c, map[string]any{"type": "ping", "n": 7})
		if pong := readType(ctx, t, c, "pong"); pong["n"] != float64(7) {
			t.Errorf("pong %v, want n 7", pong)
		}

		if err := e.gs.Admit(reqs[0].ID); err != nil {
			t.Fatal(err)
		}
		if m := readType(ctx, t, c, "admission"); m["state"] != "admitted" {
			t.Fatalf("admission state %v, want admitted", m["state"])
		}
		rm := readType(ctx, t, c, "rights")
		for k, v := range rm["rights"].(map[string]any) {
			if v != false {
				t.Errorf("rights[%s] = %v after admit, want false", k, v)
			}
		}

		lc := newLoopbackClient(t)
		writeJSON(ctx, t, c, map[string]any{"type": "offer", "sdp": lc.offer(t)})
		lc.accept(t, readType(ctx, t, c, "answer")["sdp"].(string))

		list := e.mgr.ViewerList()
		if len(list) != 1 {
			t.Fatalf("ViewerList has %d viewers, want 1", len(list))
		}
		v := list[0]
		if v.Mode != desktop.ModeGuest || v.Rights != desktop.RightsViewOnly || v.Identity != guestIdent(1001, "nodeA") {
			t.Errorf("viewer %+v, want a view-only guest carrying its identity", v)
		}
		if got := e.joins.Load(); got != 1 {
			t.Errorf("OnViewerJoin fired %d times, want 1", got)
		}
		if got := len(e.eventsOf("admitted")); got != 1 {
			t.Errorf("admitted events %d, want 1", got)
		}
	})

	t.Run("deny", func(t *testing.T) {
		e := newGuestEnv(t, nil)
		c := e.knock(ctx, t, 1001, "nodeA")
		if err := e.gs.Deny(e.pendingN(t, 1)[0].ID); err != nil {
			t.Fatal(err)
		}
		if code, reason := closeOf(ctx, t, c); code != desktop.CloseGuestDenied || reason != "denied" {
			t.Errorf("close %d %q, want 4430 denied", code, reason)
		}
		if n := e.mgr.Viewers(); n != 0 {
			t.Errorf("Viewers() = %d, want 0", n)
		}
	})
}
