package desktop_test

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/coder/websocket"

	"github.com/scottkw/portty/internal/desktop"
)

// guestSession is an admitted guest with an open input channel. It reuses
// surfViewer's message log and helpers and also notes how the socket closed.
type guestSession struct {
	*surfViewer
	ident desktop.Identity

	cmu    sync.Mutex
	code   websocket.StatusCode
	reason string
	closed chan struct{}
}

// withGuestInput gives a guest env a fake input bridge, a display for the
// mapping.
func withGuestInput(fb *fakeInputBridge) func(*desktop.Deps) {
	return func(d *desktop.Deps) {
		d.Input = fb
		d.DisplayBounds = testDisplay
	}
}

// joinGuest knocks as user uid on node, is admitted (view-only), opens the
// input channel and returns once it is open.
func (e *guestEnv) joinGuest(ctx context.Context, t *testing.T, uid int64, node string) *guestSession {
	t.Helper()
	return e.joinGuestAs(ctx, t, uid, node, true)
}

// joinGuestAs is joinGuest; with read false the guest never reads its socket
// after the join, so it never answers a close handshake.
func (e *guestEnv) joinGuestAs(ctx context.Context, t *testing.T, uid int64, node string, read bool) *guestSession {
	t.Helper()
	c := e.dial(ctx, t, uid, node)
	writeJSON(ctx, t, c, map[string]any{"type": "hello", "v": 1, "caps": map[string]int{"video": 1, "input": 1}})
	hello := readType(ctx, t, c, "hello")
	if m := readType(ctx, t, c, "admission"); m["state"] != "waiting" && m["state"] != "admitted" {
		t.Fatalf("admission state %v", m["state"])
	}
	ident := guestIdent(uid, node)
	eventually(t, 3*time.Second, "the request to be queued", func() bool {
		for _, r := range e.gs.Pending() {
			if r.Ident.UserID == uid && r.Ident.StableID == ident.StableID {
				return e.gs.Admit(r.ID) == nil
			}
		}
		return false
	})
	readType(ctx, t, c, "rights")
	lc := newLoopbackClient(t)
	dc, err := lc.pc.CreateDataChannel("input", nil)
	if err != nil {
		t.Fatal(err)
	}
	open := make(chan struct{})
	dc.OnOpen(func() { close(open) })
	writeJSON(ctx, t, c, map[string]any{"type": "offer", "sdp": lc.offer(t)})
	lc.accept(t, readType(ctx, t, c, "answer")["sdp"].(string))
	g := &guestSession{
		surfViewer: &surfViewer{id: hello["viewer"].(string), conn: c, dc: dc},
		ident:      ident, closed: make(chan struct{}),
	}
	if read {
		go g.run(ctx)
	}
	select {
	case <-open:
	case <-time.After(10 * time.Second):
		t.Fatal("the input channel did not open")
	}
	return g
}

// run keeps every signalling message and records the close.
func (g *guestSession) run(ctx context.Context) {
	defer close(g.closed)
	for {
		_, b, err := g.conn.Read(ctx)
		if err != nil {
			g.cmu.Lock()
			g.code = websocket.CloseStatus(err)
			var ce websocket.CloseError
			if errors.As(err, &ce) {
				g.reason = ce.Reason
			}
			g.cmu.Unlock()
			return
		}
		var m map[string]any
		if json.Unmarshal(b, &m) == nil {
			g.mu.Lock()
			g.msgs = append(g.msgs, m)
			g.mu.Unlock()
		}
	}
}

// waitClosed waits for the socket to end and returns its close code and reason.
func (g *guestSession) waitClosed(t *testing.T, within time.Duration) (websocket.StatusCode, string) {
	t.Helper()
	select {
	case <-g.closed:
	case <-time.After(within):
		t.Fatalf("the guest socket did not close within %v", within)
	}
	g.cmu.Lock()
	defer g.cmu.Unlock()
	return g.code, g.reason
}

func (g *guestSession) viewerID() desktop.ViewerID { return desktop.ViewerID(g.id) }

// press is the left button down at one point of the surface.
func (g *guestSession) press(t *testing.T, ts float64) {
	t.Helper()
	g.input(t, map[string]any{"t": "btn", "b": 0, "d": true, "x": 0.5, "y": 0.25, "e": 1, "ts": ts})
}

// rightsIs matches a rights message with these control and clipboard values.
func rightsIs(control, clipboard bool) func(map[string]any) bool {
	return func(m map[string]any) bool {
		r, _ := m["rights"].(map[string]any)
		return r["control"] == control && r["clipboard"] == clipboard
	}
}
