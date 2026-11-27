package desktop_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"

	"github.com/scottkw/portty/internal/desktop"
)

// connect admits the waiting guest at c and runs the offer path over loopback.
func (e *guestEnv) connect(ctx context.Context, t *testing.T, c *websocket.Conn) {
	t.Helper()
	reqs := e.pendingN(t, 1)
	if err := e.gs.Admit(reqs[0].ID); err != nil {
		t.Fatal(err)
	}
	readType(ctx, t, c, "admission")
	readType(ctx, t, c, "rights")
	lc := newLoopbackClient(t)
	writeJSON(ctx, t, c, map[string]any{"type": "offer", "sdp": lc.offer(t)})
	lc.accept(t, readType(ctx, t, c, "answer")["sdp"].(string))
}

func TestGuestViewer_NewKnockFromTheSameNodeClosesTheWaitingOne(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	e := newGuestEnv(t, nil)
	first := e.knock(ctx, t, 1001, "nodeA")
	e.pendingN(t, 1)
	e.knock(ctx, t, 1001, "nodeA")
	if code, reason := closeOf(ctx, t, first); code != websocket.StatusNormalClosure || reason != "replaced" {
		t.Errorf("replaced socket closed %d %q, want 1000 replaced", code, reason)
	}
	if n := len(e.gs.Pending()); n != 1 {
		t.Errorf("%d pending, want 1", n)
	}
}

func TestGuestViewer_NinthWaitingGuestIsClosedFull(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	e := newGuestEnv(t, nil)
	for i := int64(0); i < 8; i++ {
		e.knock(ctx, t, 4000+i, "n")
	}
	c := e.dial(ctx, t, 4999, "n")
	writeJSON(ctx, t, c, map[string]any{"type": "hello", "v": 1})
	if code, reason := closeOf(ctx, t, c); code != desktop.CloseFull || reason != "full" {
		t.Errorf("ninth guest closed %d %q, want 4429 full", code, reason)
	}
	if n := len(e.gs.Pending()); n != 8 {
		t.Errorf("%d pending, want 8", n)
	}
}

func TestGuestViewer_NoAnswerClosesAdmitTimeout(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	e := newGuestEnv(t, nil)
	fire := make(chan time.Time)
	e.gs.SetTimerForTest(func(time.Duration) (<-chan time.Time, func()) { return fire, func() {} })
	c := e.knock(ctx, t, 1001, "nodeA")
	fire <- time.Now()
	if code, reason := closeOf(ctx, t, c); code != desktop.CloseGuestTimeout || reason != "admit_timeout" {
		t.Errorf("close %d %q, want 4431 admit_timeout", code, reason)
	}
	if got := e.eventsOf("denied"); len(got) != 1 || got[0].Reason != desktop.ReasonTimeout {
		t.Errorf("denied events %+v, want one with reason timeout", got)
	}
}

func TestGuestViewer_DeniedUserIsRefusedAtOnceFromAnotherNode(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	e := newGuestEnv(t, nil)
	first := e.knock(ctx, t, 1001, "nodeA")
	if err := e.gs.Deny(e.pendingN(t, 1)[0].ID); err != nil {
		t.Fatal(err)
	}
	closeOf(ctx, t, first)
	requests := len(e.eventsOf("admit_request"))

	c := e.dial(ctx, t, 1001, "nodeB")
	writeJSON(ctx, t, c, map[string]any{"type": "hello", "v": 1})
	if code, reason := closeOf(ctx, t, c); code != desktop.CloseGuestDenied || reason != "denied" {
		t.Errorf("close %d %q, want 4430 denied", code, reason)
	}
	if got := len(e.eventsOf("admit_request")); got != requests {
		t.Errorf("a denied user's knock fired admit_request")
	}
}

func TestGuestViewer_ReconnectIsAdmittedAtOnceAndTheOlderSocketCloses(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 40*time.Second)
	defer cancel()
	e := newGuestEnv(t, nil)
	old := e.knock(ctx, t, 1001, "nodeA")
	e.connect(ctx, t, old)
	key := desktop.AdmissionKey{UserID: 1001, StableID: "stable-nodeA"}
	if !e.gs.SetGrants(key, desktop.RightsControl|desktop.RightsClipboard) {
		t.Fatal("SetGrants: not admitted")
	}
	requests := len(e.eventsOf("admit_request"))

	c := e.dial(ctx, t, 1001, "nodeA")
	writeJSON(ctx, t, c, map[string]any{"type": "hello", "v": 1, "caps": map[string]int{"input": 1}})
	readType(ctx, t, c, "hello")
	// No waiting card for a re-admission: the next admission message is admitted.
	if m := readType(ctx, t, c, "admission"); m["state"] != "admitted" {
		t.Fatalf("admission state %v, want admitted at once", m["state"])
	}
	rights := readType(ctx, t, c, "rights")["rights"].(map[string]any)
	if rights["control"] != true || rights["clipboard"] != true || rights["surface"] != false {
		t.Errorf("rights %v, want the stored control and clipboard and no surface", rights)
	}
	if got := len(e.eventsOf("admit_request")); got != requests {
		t.Errorf("a reconnect fired admit_request")
	}
	if code, reason := closeOf(ctx, t, old); code != websocket.StatusNormalClosure || reason != "replaced" {
		t.Errorf("older socket closed %d %q, want 1000 replaced", code, reason)
	}
}

func TestGuestViewer_SecondNodeOfAnAdmittedUserWaits(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 40*time.Second)
	defer cancel()
	e := newGuestEnv(t, nil)
	a := e.knock(ctx, t, 1001, "nodeA")
	e.connect(ctx, t, a)
	e.knock(ctx, t, 1001, "nodeB") // fails the test unless the waiting message arrives
	if p := e.pendingN(t, 1); p[0].Ident.StableID != "stable-nodeB" {
		t.Errorf("pending identity %+v, want nodeB", p[0].Ident)
	}
}

func TestGuestViewer_ShareEndClosesWaitersAndLaterGuests(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	e := newGuestEnv(t, nil)
	c := e.knock(ctx, t, 1001, "nodeA")
	e.pendingN(t, 1)
	e.gs.End("test")
	if code, reason := closeOf(ctx, t, c); code != desktop.CloseShareEnded || reason != "share_ended" {
		t.Errorf("waiter closed %d %q, want 4433 share_ended", code, reason)
	}
	late := e.dial(ctx, t, 1002, "n")
	writeJSON(ctx, t, late, map[string]any{"type": "hello", "v": 1})
	if code, reason := closeOf(ctx, t, late); code != desktop.CloseShareEnded || reason != "share_ended" {
		t.Errorf("later guest closed %d %q, want 4433 share_ended", code, reason)
	}
}

func TestGuestViewer_AnythingButPingWhileWaitingIsBadMessage(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	e := newGuestEnv(t, nil)
	for _, msg := range []map[string]any{
		{"type": "offer", "sdp": "v=0"},
		{"type": "select", "kind": "display", "id": 0},
		{"type": "clip_get", "xid": "x"},
	} {
		c := e.knock(ctx, t, 1001, "nodeA")
		e.pendingN(t, 1)
		writeJSON(ctx, t, c, msg)
		if code, reason := closeOf(ctx, t, c); code != websocket.StatusProtocolError || reason != "bad_message" {
			t.Errorf("%v while waiting: closed %d %q, want protocol error bad_message", msg["type"], code, reason)
		}
		eventually(t, 3*time.Second, "the withdrawn request to leave the queue", func() bool { return len(e.gs.Pending()) == 0 })
		if n := e.mgr.Viewers(); n != 0 {
			t.Errorf("Viewers() = %d, want 0", n)
		}
	}
}

func TestGuestViewer_TokenThatNoLongerVerifiesIsRefused(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	t.Run("at the knock", func(t *testing.T) {
		e := newGuestEnv(t, nil)
		e.admitOK.Store(false)
		c := e.dial(ctx, t, 1001, "nodeA")
		if code, reason := closeOf(ctx, t, c); code != desktop.CloseTokenRejected || reason != "token_invalid" {
			t.Errorf("close %d %q, want 4401 token_invalid", code, reason)
		}
		if n := len(e.gs.Pending()); n != 0 {
			t.Errorf("a refused token queued %d requests", n)
		}
	})
	t.Run("at the join", func(t *testing.T) {
		e := newGuestEnv(t, nil)
		c := e.knock(ctx, t, 1001, "nodeA")
		if err := e.gs.Admit(e.pendingN(t, 1)[0].ID); err != nil {
			t.Fatal(err)
		}
		readType(ctx, t, c, "rights")
		e.admitOK.Store(false) // the key was regenerated while the guest was admitted
		lc := newLoopbackClient(t)
		writeJSON(ctx, t, c, map[string]any{"type": "offer", "sdp": lc.offer(t)})
		if code, reason := closeOf(ctx, t, c); code != desktop.CloseTokenRejected || reason != "token_invalid" {
			t.Errorf("close %d %q, want 4401 token_invalid", code, reason)
		}
		if n := e.mgr.Viewers(); n != 0 {
			t.Errorf("Viewers() = %d, want 0", n)
		}
	})
}

func TestGuestViewer_SelectRefused(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 40*time.Second)
	defer cancel()
	e := newGuestEnv(t, nil)
	c := e.knock(ctx, t, 1001, "nodeA")
	e.connect(ctx, t, c)
	writeJSON(ctx, t, c, map[string]any{"type": "select", "kind": "display", "id": 0})
	if m := readType(ctx, t, c, "select_failed"); m["code"] != "forbidden" {
		t.Errorf("select_failed %v, want code forbidden", m)
	}
}

func TestGuestViewer_OwnAccountViewerKeepsModeOwnAndFiresTheHooks(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 40*time.Second)
	defer cancel()
	e := newGuestEnv(t, nil)
	own := guestIdent(7, "mine")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, err := websocket.Accept(w, r, nil)
		if err != nil {
			return
		}
		e.mgr.ServeDesktopViewerAs(r.Context(), c, func() bool { return true }, own)
	}))
	t.Cleanup(srv.Close)
	c, _, err := websocket.Dial(ctx, "ws"+strings.TrimPrefix(srv.URL, "http"), nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = c.CloseNow() })
	writeJSON(ctx, t, c, map[string]any{"type": "hello", "v": 1, "caps": map[string]int{"input": 1}})
	hello := readType(ctx, t, c, "hello")
	if rights := hello["rights"].(map[string]any); rights["control"] != true || rights["clipboard"] != true || rights["surface"] != true {
		t.Errorf("own-account rights %v, want all true", rights)
	}
	lc := newLoopbackClient(t)
	writeJSON(ctx, t, c, map[string]any{"type": "offer", "sdp": lc.offer(t)})
	lc.accept(t, readType(ctx, t, c, "answer")["sdp"].(string))
	list := e.mgr.ViewerList()
	if len(list) != 1 || list[0].Mode != desktop.ModeOwn || list[0].Identity != own {
		t.Fatalf("ViewerList %+v, want one own-account viewer carrying its identity", list)
	}
	if e.joins.Load() != 1 {
		t.Errorf("OnViewerJoin fired %d times, want 1", e.joins.Load())
	}
	_ = c.Close(websocket.StatusNormalClosure, "")
	eventually(t, 3*time.Second, "OnViewerLeave", func() bool { return e.leaves.Load() == 1 })
}
