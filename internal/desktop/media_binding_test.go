package desktop_test

import (
	"context"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/coder/websocket"

	"github.com/scottkw/portty/internal/desktop"
)

// setPeer replaces what the fake PeerIdentity answers.
func (e *guestEnv) setPeer(f func(addr string) (int64, string, error)) {
	e.mu.Lock()
	e.peer = f
	e.mu.Unlock()
}

// connectedGuest brings a guest for user 1001 on nodeA all the way up over
// loopback with a matching identity, and returns its socket and viewer ID.
func connectedGuest(ctx context.Context, t *testing.T, e *guestEnv) (*websocket.Conn, desktop.ViewerID) {
	t.Helper()
	c := e.knock(ctx, t, 1001, "nodeA")
	e.connect(ctx, t, c)
	list := e.mgr.ViewerList()
	if len(list) != 1 {
		t.Fatalf("ViewerList has %d viewers, want 1", len(list))
	}
	return c, list[0].ID
}

func wantCut(ctx context.Context, t *testing.T, e *guestEnv, c *websocket.Conn) {
	t.Helper()
	if code, reason := closeOf(ctx, t, c); code != desktop.CloseGuestRevoked || reason != "revoked" {
		t.Errorf("close %d %q, want 4432 revoked", code, reason)
	}
	got := e.eventsOf("revoked")
	if len(got) != 1 || got[0].Reason != desktop.ReasonMediaIdentityMismatch || got[0].Ident.UserID != 1001 {
		t.Errorf("revoked events %+v, want one media_identity_mismatch for user 1001", got)
	}
}

func TestMediaBinding_MatchingIdentityStaysAndTheConnectedCheckRuns(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 40*time.Second)
	defer cancel()
	e := newGuestEnv(t, nil)
	var calls atomic.Int32
	var addr atomic.Value
	e.setPeer(func(a string) (int64, string, error) {
		calls.Add(1)
		addr.Store(a)
		return 1001, "stable-nodeA", nil
	})
	connectedGuest(ctx, t, e)
	// The Connected state runs the check on the real selected pair.
	eventually(t, 10*time.Second, "the Connected check", func() bool { return calls.Load() >= 1 })
	if host, _, err := net.SplitHostPort(addr.Load().(string)); err != nil || host != "127.0.0.1" {
		t.Errorf("lookup address %q, want the loopback peer as host:port", addr.Load())
	}
	time.Sleep(300 * time.Millisecond)
	if n := e.mgr.Viewers(); n != 1 {
		t.Errorf("Viewers() = %d after a matching check, want 1", n)
	}
	if got := e.eventsOf("revoked"); len(got) != 0 {
		t.Errorf("revoked events %+v on a matching identity", got)
	}
}

func TestMediaBinding_PairChangeToAnotherNodeCutsTheGuest(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 40*time.Second)
	defer cancel()
	e := newGuestEnv(t, nil)
	c, id := connectedGuest(ctx, t, e)
	eventually(t, 10*time.Second, "the first check to settle", func() bool { return e.mgr.Viewers() == 1 })
	e.setPeer(func(string) (int64, string, error) { return 1001, "stable-someone-elses", nil })
	if !e.mgr.CheckMediaIdentityForTest(id, true, "100.64.0.9", 4000) {
		t.Fatal("viewer not found")
	}
	wantCut(ctx, t, e, c)
}

func TestMediaBinding_AnotherUserIsCut(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 40*time.Second)
	defer cancel()
	e := newGuestEnv(t, nil)
	c, id := connectedGuest(ctx, t, e)
	e.setPeer(func(string) (int64, string, error) { return 2002, "stable-nodeA", nil })
	e.mgr.CheckMediaIdentityForTest(id, true, "100.64.0.9", 4000)
	wantCut(ctx, t, e, c)
}

func TestMediaBinding_LookupErrorFailsClosed(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 40*time.Second)
	defer cancel()
	e := newGuestEnv(t, nil)
	c, id := connectedGuest(ctx, t, e)
	e.setPeer(func(string) (int64, string, error) { return 0, "", errors.New("tailscaled unreachable") })
	e.mgr.CheckMediaIdentityForTest(id, true, "100.64.0.9", 4000)
	wantCut(ctx, t, e, c)
}

func TestMediaBinding_NoSelectedPairFailsClosed(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 40*time.Second)
	defer cancel()
	e := newGuestEnv(t, nil)
	c, id := connectedGuest(ctx, t, e)
	e.mgr.CheckMediaIdentityForTest(id, false, "", 0)
	wantCut(ctx, t, e, c)
}

func TestMediaBinding_TwoMismatchesCutOnceAndReportOnce(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 40*time.Second)
	defer cancel()
	e := newGuestEnv(t, nil)
	c, id := connectedGuest(ctx, t, e)
	e.setPeer(func(string) (int64, string, error) { return 2002, "x", nil })
	e.mgr.CheckMediaIdentityForTest(id, true, "100.64.0.9", 4000)
	e.mgr.CheckMediaIdentityForTest(id, true, "100.64.0.9", 4001) // a second pair change
	wantCut(ctx, t, e, c)
}

func TestMediaBinding_NilSeamClosesAGuestButNeverAnOwnAccountViewer(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	e := newGuestEnv(t, func(d *desktop.Deps) { d.PeerIdentity = nil })

	// The guest's media connects and the check finds no way to vouch for it.
	c := e.knock(ctx, t, 1001, "nodeA")
	e.connect(ctx, t, c)
	wantCut(ctx, t, e, c)
	eventually(t, 5*time.Second, "the cut guest to leave", func() bool { return e.mgr.Viewers() == 0 })

	// An own-account viewer on the same Manager is never checked.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		oc, err := websocket.Accept(w, r, nil)
		if err != nil {
			return
		}
		e.mgr.ServeDesktopViewerAs(r.Context(), oc, func() bool { return true }, guestIdent(7, "mine"))
	}))
	t.Cleanup(srv.Close)
	oc, _, err := websocket.Dial(ctx, "ws"+strings.TrimPrefix(srv.URL, "http"), nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = oc.CloseNow() })
	writeJSON(ctx, t, oc, map[string]any{"type": "hello", "v": 1})
	readType(ctx, t, oc, "hello")
	lc := newLoopbackClient(t)
	writeJSON(ctx, t, oc, map[string]any{"type": "offer", "sdp": lc.offer(t)})
	lc.accept(t, readType(ctx, t, oc, "answer")["sdp"].(string))
	eventually(t, 10*time.Second, "the own-account media to come up", func() bool {
		select {
		case <-lc.aus:
			return true
		default:
			return false
		}
	})
	if n := e.mgr.Viewers(); n != 1 {
		t.Errorf("Viewers() = %d, want the own-account viewer still joined", n)
	}
}
