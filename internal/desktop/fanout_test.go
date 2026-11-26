package desktop_test

import (
	"context"
	"encoding/json"
	"sync"
	"testing"
	"time"

	"github.com/coder/websocket"

	"github.com/scottkw/portty/internal/desktop"
)

// fakeClock drives Deps.Now so the keyframe coalescer is tested without
// sleeping through its window.
type fakeClock struct {
	mu sync.Mutex
	t  time.Time
}

func newFakeClock() *fakeClock { return &fakeClock{t: time.Unix(1_700_000_000, 0)} }

func (c *fakeClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.t
}

func (c *fakeClock) Advance(d time.Duration) {
	c.mu.Lock()
	c.t = c.t.Add(d)
	c.mu.Unlock()
}

func withClock(c *fakeClock) tracerOpt {
	return func(_ *tracerEnv, d *desktop.Deps) { d.Now = c.Now }
}

// fanViewer is one full viewer: signalling socket plus pion client. A reader
// goroutine keeps the latest state and size messages and notes how the socket
// closed.
type fanViewer struct {
	conn *websocket.Conn
	lc   *loopbackClient

	mu        sync.Mutex
	viewers   int
	w, h      int
	sizeCount int
	closeCode websocket.StatusCode
	closed    chan struct{}
}

// join runs hello, offer and answer for a new viewer and starts its reader.
// extra runs on the client's PeerConnection before the offer is made (to add a
// data channel).
func (e *tracerEnv) join(ctx context.Context, t *testing.T, tok string, extra ...func(*loopbackClient)) *fanViewer {
	t.Helper()
	conn, _, err := e.dial(ctx, tok)
	if err != nil {
		t.Fatalf("dial /desktop/ws: %v", err)
	}
	t.Cleanup(func() { _ = conn.CloseNow() })
	writeJSON(ctx, t, conn, map[string]any{"type": "hello", "v": 1, "caps": map[string]int{"video": 1}})
	readType(ctx, t, conn, "hello")
	lc := newLoopbackClient(t)
	for _, f := range extra {
		f(lc)
	}
	writeJSON(ctx, t, conn, map[string]any{"type": "offer", "sdp": lc.offer(t)})
	lc.accept(t, readType(ctx, t, conn, "answer")["sdp"].(string))
	v := &fanViewer{conn: conn, lc: lc, closed: make(chan struct{})}
	go v.read(ctx)
	return v
}

func (v *fanViewer) read(ctx context.Context) {
	defer close(v.closed)
	for {
		_, b, err := v.conn.Read(ctx)
		if err != nil {
			v.mu.Lock()
			v.closeCode = websocket.CloseStatus(err)
			v.mu.Unlock()
			return
		}
		var m map[string]any
		if json.Unmarshal(b, &m) != nil {
			continue
		}
		v.mu.Lock()
		switch m["type"] {
		case "state":
			v.viewers = int(m["viewers"].(float64))
		case "size":
			v.w, v.h = int(m["w"].(float64)), int(m["h"].(float64))
			v.sizeCount++
		}
		v.mu.Unlock()
	}
}

func (v *fanViewer) stateViewers() int {
	v.mu.Lock()
	defer v.mu.Unlock()
	return v.viewers
}

func (v *fanViewer) size() (int, int) {
	v.mu.Lock()
	defer v.mu.Unlock()
	return v.w, v.h
}

// firstAU waits for the next access unit the client depacketized.
func (v *fanViewer) firstAU(t *testing.T, within time.Duration) []byte {
	t.Helper()
	select {
	case au := <-v.lc.aus:
		return au
	case <-time.After(within):
		t.Fatalf("no access unit within %v", within)
		return nil
	}
}

// requireKeyframe fails unless au holds SPS, PPS and an IDR slice.
func requireKeyframe(t *testing.T, who string, au []byte) {
	t.Helper()
	if types := nalTypes(au); !hasAll(types, 7, 8, 5) {
		t.Fatalf("%s: access unit has NAL types %v, want SPS+PPS+IDR (7, 8, 5)", who, types)
	}
}

// TestFanout_ThreeViewersEachGetKeyframe: three viewers join one after
// another on one source. Each one's first access unit is a keyframe (a viewer
// gets no delta before its first keyframe) within 2 s of its own join, all see
// the same picture size, and each sees the viewer count of the others.
func TestFanout_ThreeViewersEachGetKeyframe(t *testing.T) {
	clk := newFakeClock()
	env := newTracerServer(t, withClock(clk))
	tok := env.token(t, "g1")
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	var vs []*fanViewer
	for i, name := range []string{"A", "B", "C"} {
		if i > 0 {
			clk.Advance(2 * desktop.KeyframeCoalesce) // past the coalescing window
		}
		start := time.Now()
		v := env.join(ctx, t, tok)
		requireKeyframe(t, name, v.firstAU(t, 2*time.Second))
		if d := time.Since(start); d > 2*time.Second {
			t.Errorf("viewer %s took %v to its first keyframe, want under 2s", name, d)
		}
		vs = append(vs, v)
	}

	for _, v := range vs {
		eventually(t, 2*time.Second, "viewer count 3", func() bool { return v.stateViewers() == 3 })
		eventually(t, 2*time.Second, "size message", func() bool { w, h := v.size(); return w == 1280 && h == 800 })
	}
	if n := env.sourceCount(); n != 1 {
		t.Errorf("source started %d times, want 1", n)
	}
	if n := env.lastSource().KeyframesForced(); n != 3 {
		t.Errorf("keyframes forced %d, want 3 (one per viewer, none coalesced away)", n)
	}
	if got := len(env.mgr.ViewerList()); got != 3 {
		t.Fatalf("ViewerList has %d entries, want 3", got)
	}
	for _, rec := range env.mgr.ViewerList() {
		if rec.Rights != desktop.RightsViewOnly || rec.ID == "" || rec.JoinedAt.IsZero() {
			t.Errorf("viewer record %+v, want an ID, view-only rights and a join time", rec)
		}
	}
}
