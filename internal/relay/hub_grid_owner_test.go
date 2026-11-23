package relay

import (
	"context"
	"io"
	"log"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/coder/websocket"

	"github.com/scottkw/portty/internal/testutil"
)

// INPT-06 grid owner tests: the viewer that claimed last sets the grid, a
// plain report only stores the viewer's size, and the first local report on a
// never-sized grid bootstraps it.

// ownerHub is a hub with no grid yet; calls returns the resizeFn calls so far.
func ownerHub(t *testing.T) (*Hub, func() [][2]int) {
	t.Helper()
	r, w := io.Pipe()
	t.Cleanup(func() { _ = w.Close(); _ = r.Close() })
	var mu sync.Mutex
	var got [][2]int
	hub := NewHub("owner-test-session", r, w, DefaultScrollbackBytes, func(c, rows int) error {
		mu.Lock()
		defer mu.Unlock()
		got = append(got, [2]int{c, rows})
		return nil
	})
	return hub, func() [][2]int {
		mu.Lock()
		defer mu.Unlock()
		return slices.Clone(got)
	}
}

// gridSub subscribes a viewer of the given origin.
func gridSub(hub *Hub, origin string) *Subscriber {
	sub := &Subscriber{Msgs: make(chan []byte, 256), CloseSlow: func() {}, Origin: origin}
	hub.Subscribe(sub)
	return sub
}

// gridFrames empties sub's queue and returns the grid frames it held.
func gridFrames(sub *Subscriber) [][2]int {
	var out [][2]int
	for {
		select {
		case f := <-sub.Msgs:
			if f[0] == MsgResize && len(f) == 5 {
				out = append(out, [2]int{int(f[1])<<8 | int(f[2]), int(f[3])<<8 | int(f[4])})
			}
		default:
			return out
		}
	}
}

func hubGrid(hub *Hub) [2]int {
	hub.mu.Lock()
	defer hub.mu.Unlock()
	return [2]int{hub.ptyCols, hub.ptyRows}
}

func mustSize(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

// ownedAt100x30 is a hub whose local viewer a bootstrapped 100x30.
func ownedAt100x30(t *testing.T) (*Hub, func() [][2]int, *Subscriber) {
	t.Helper()
	hub, calls := ownerHub(t)
	a := gridSub(hub, "local")
	mustSize(t, hub.ResizeClient(a, 100, 30))
	if g := hubGrid(hub); g != [2]int{100, 30} || len(calls()) != 1 {
		t.Fatalf("bootstrap: grid %v resizeFn %v; want 100x30 once", g, calls())
	}
	gridFrames(a)
	return hub, calls, a
}

func TestHub_GridBootstrapFirstLocalReport(t *testing.T) {
	hub, calls := ownerHub(t)
	web := gridSub(hub, "web")
	mustSize(t, hub.ResizeClient(web, 120, 40))
	if g := hubGrid(hub); g != [2]int{0, 0} || len(calls()) != 0 {
		t.Fatalf("a web report sized a never-sized grid: %v, resizeFn %v", g, calls())
	}
	local := gridSub(hub, "local")
	mustSize(t, hub.ResizeClient(local, 90, 25))
	if g := hubGrid(hub); g != [2]int{90, 25} || !slices.Equal(calls(), [][2]int{{90, 25}}) {
		t.Fatalf("first local report: grid %v resizeFn %v; want 90x25 once", g, calls())
	}
	if got := gridFrames(web); !slices.Equal(got, [][2]int{{90, 25}}) {
		t.Fatalf("web viewer grid frames %v, want one 90x25", got)
	}
}

func TestHub_GridReportFromNonOwnerIgnored(t *testing.T) {
	hub, calls, a := ownedAt100x30(t)
	b := gridSub(hub, "local")
	mustSize(t, hub.ResizeClient(b, 80, 24))
	if g := hubGrid(hub); g != [2]int{100, 30} || len(calls()) != 1 {
		t.Fatalf("a non-owner's report moved the grid: %v, resizeFn %v", g, calls())
	}
	if f := append(gridFrames(a), gridFrames(b)...); len(f) != 0 {
		t.Fatalf("a non-owner's report broadcast %v", f)
	}
	hub.mu.Lock()
	stored := [2]int{b.Cols, b.Rows}
	hub.mu.Unlock()
	if stored != [2]int{80, 24} {
		t.Fatalf("report not stored: %v", stored)
	}
}

func TestHub_GridClaimTakesOwnership(t *testing.T) {
	buf := &diagBuf{}
	prevOut, prevFlags := log.Writer(), log.Flags()
	log.SetOutput(buf)
	log.SetFlags(0)
	t.Cleanup(func() { log.SetOutput(prevOut); log.SetFlags(prevFlags) })

	hub, calls, a := ownedAt100x30(t)
	b := gridSub(hub, "local")
	mustSize(t, hub.ClaimGrid(b, 80, 24))
	if g := hubGrid(hub); g != [2]int{80, 24} {
		t.Fatalf("claim: grid %v, want 80x24", g)
	}
	for name, s := range map[string]*Subscriber{"a": a, "b": b} {
		if got := gridFrames(s); !slices.Equal(got, [][2]int{{80, 24}}) {
			t.Fatalf("viewer %s grid frames %v, want one 80x24", name, got)
		}
	}
	var claimLine string
	for _, l := range strings.Split(buf.String(), "\n") {
		if strings.Contains(l, "viewer-diag: grid ") && strings.Contains(l, "-> 80x24") {
			claimLine = l
		}
	}
	if !strings.HasSuffix(claimLine, "why=claim") {
		t.Fatalf("grid diag line %q does not end with why=claim:\n%s", claimLine, buf.String())
	}

	mustSize(t, hub.ResizeClient(a, 100, 30))
	if g := hubGrid(hub); g != [2]int{80, 24} || len(calls()) != 2 {
		t.Fatalf("the old owner's report moved the grid: %v, resizeFn %v", g, calls())
	}
	mustSize(t, hub.ClaimGrid(a, 100, 30))
	if g := hubGrid(hub); g != [2]int{100, 30} || !slices.Equal(calls(), [][2]int{{100, 30}, {80, 24}, {100, 30}}) {
		t.Fatalf("taking it back: grid %v resizeFn %v", g, calls())
	}
}

func TestHub_GridOwnerReportStoredOnly(t *testing.T) {
	hub, calls, a := ownedAt100x30(t)
	mustSize(t, hub.ResizeClient(a, 120, 40))
	if g := hubGrid(hub); g != [2]int{100, 30} || len(calls()) != 1 || len(gridFrames(a)) != 0 {
		t.Fatalf("the owner's report moved the grid: %v, resizeFn %v", g, calls())
	}
	hub.mu.Lock()
	stored := [2]int{a.Cols, a.Rows}
	hub.mu.Unlock()
	if stored != [2]int{120, 40} {
		t.Fatalf("report not stored: %v", stored)
	}
	mustSize(t, hub.ClaimGrid(a, 120, 40))
	if g := hubGrid(hub); g != [2]int{120, 40} || len(calls()) != 2 {
		t.Fatalf("the owner's claim: grid %v resizeFn %v; want 120x40, one more call", g, calls())
	}
}

func TestHub_GridFrozenReportNotAppliedOnThaw(t *testing.T) {
	var mu sync.Mutex
	var calls [][2]int
	hub, _ := pipeHub(t, func(c, r int) error {
		mu.Lock()
		defer mu.Unlock()
		calls = append(calls, [2]int{c, r})
		return nil
	})
	a := gridSub(hub, "local")
	mustSize(t, hub.ResizeClient(a, 100, 30))
	if err := hub.Freeze(); err != nil {
		t.Fatalf("Freeze: %v", err)
	}
	mustSize(t, hub.ResizeClient(a, 120, 40))
	hub.Thaw()
	mu.Lock()
	defer mu.Unlock()
	if g := hubGrid(hub); g != [2]int{100, 30} || len(calls) != 1 {
		t.Fatalf("a report during a freeze was applied at Thaw: grid %v resizeFn %v", g, calls)
	}
}

func TestHub_GridOwnerLeavesFreezes(t *testing.T) {
	hub, calls, a := ownedAt100x30(t)
	b := gridSub(hub, "local")
	hub.Unsubscribe(a)
	if g := hubGrid(hub); g != [2]int{100, 30} {
		t.Fatalf("the owner leaving moved the grid: %v", g)
	}
	mustSize(t, hub.ResizeClient(b, 80, 24))
	if g := hubGrid(hub); g != [2]int{100, 30} || len(calls()) != 1 {
		t.Fatalf("a report after the owner left moved the grid: %v, resizeFn %v", g, calls())
	}
	mustSize(t, hub.ClaimGrid(b, 80, 24))
	if g := hubGrid(hub); g != [2]int{80, 24} || len(calls()) != 2 {
		t.Fatalf("a claim after the owner left: grid %v resizeFn %v", g, calls())
	}
}

func TestHub_GridWebClaim(t *testing.T) {
	hub, calls, a := ownedAt100x30(t)
	web := gridSub(hub, "web")
	mustSize(t, hub.ClaimGrid(web, 132, 43))
	if g := hubGrid(hub); g != [2]int{132, 43} || len(calls()) != 2 {
		t.Fatalf("web claim: grid %v resizeFn %v; want 132x43", g, calls())
	}
	if got := gridFrames(a); !slices.Equal(got, [][2]int{{132, 43}}) {
		t.Fatalf("local viewer grid frames %v, want one 132x43", got)
	}
}

func TestHub_GridReadOnlyIgnored(t *testing.T) {
	hub, calls := ownerHub(t)
	ro := &Subscriber{Msgs: make(chan []byte, 256), CloseSlow: func() {}, Origin: "local", ReadOnly: true}
	hub.Subscribe(ro)
	mustSize(t, hub.ResizeClient(ro, 90, 25)) // would bootstrap if it counted
	mustSize(t, hub.ClaimGrid(ro, 90, 25))
	hub.mu.Lock()
	stored := [2]int{ro.Cols, ro.Rows}
	hub.mu.Unlock()
	if g := hubGrid(hub); g != [2]int{0, 0} || len(calls()) != 0 || stored != [2]int{0, 0} {
		t.Fatalf("read-only viewer: grid %v resizeFn %v stored %v; want nothing", g, calls(), stored)
	}
}

func TestHub_GridClamp(t *testing.T) {
	hub, calls, a := ownedAt100x30(t)
	mustSize(t, hub.ClaimGrid(a, 5000, 5000))
	if g := hubGrid(hub); g != [2]int{500, 200} {
		t.Fatalf("claim 5000x5000: grid %v, want 500x200", g)
	}
	mustSize(t, hub.ClaimGrid(a, 3, 1))
	if g := hubGrid(hub); g != [2]int{20, 5} {
		t.Fatalf("claim 3x1: grid %v, want 20x5", g)
	}
	mustSize(t, hub.ClaimGrid(a, 0, 0))
	mustSize(t, hub.ClaimGrid(a, -4, 30))
	if g := hubGrid(hub); g != [2]int{20, 5} || len(calls()) != 3 {
		t.Fatalf("zero or negative claims: grid %v resizeFn %v; want 20x5, no more calls", g, calls())
	}
}

func TestHub_GridFrozenClaimAppliedOnThaw(t *testing.T) {
	var mu sync.Mutex
	var calls [][2]int
	hub, _ := pipeHub(t, func(c, r int) error {
		mu.Lock()
		defer mu.Unlock()
		calls = append(calls, [2]int{c, r})
		return nil
	})
	a := gridSub(hub, "local")
	mustSize(t, hub.ResizeClient(a, 100, 30))
	gridFrames(a)
	if err := hub.Freeze(); err != nil {
		t.Fatalf("Freeze: %v", err)
	}
	b := gridSub(hub, "web")
	mustSize(t, hub.ClaimGrid(b, 120, 40))
	mu.Lock()
	n := len(calls)
	mu.Unlock()
	if n != 1 || len(gridFrames(a)) != 0 || hubGrid(hub) != [2]int{100, 30} {
		t.Fatalf("a frozen claim reached the PTY: resizeFn %d calls, grid %v", n, hubGrid(hub))
	}
	hub.Thaw()
	mu.Lock()
	defer mu.Unlock()
	if !slices.Equal(calls, [][2]int{{100, 30}, {120, 40}}) || hubGrid(hub) != [2]int{120, 40} {
		t.Fatalf("after Thaw: resizeFn %v grid %v; want one more call 120x40", calls, hubGrid(hub))
	}
}

// TestRelayPump_ClaimTakesGrid drives claims through the real local relay
// pump: B's claim takes the grid from A, A's claim takes it back, and a
// 4-byte frame (an older viewer's report) changes nothing.
func TestRelayPump_ClaimTakesGrid(t *testing.T) {
	srv, manager, _, _, sessionID := setupTestServer(t)
	hub, ok := manager.Get(sessionID)
	if !ok {
		t.Fatal("hub not found")
	}
	a := dialWS(t, srv.URL, sessionID)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	mustWrite := func(c *websocket.Conn, frame []byte) {
		t.Helper()
		if err := c.Write(ctx, websocket.MessageBinary, frame); err != nil {
			t.Fatalf("write %v: %v", frame, err)
		}
	}
	mustWrite(a, []byte{MsgResize2, 0, 100, 0, 30}) // A's first fit bootstraps
	testutil.WaitFor(t, 2*time.Second, func() bool { return hubGrid(hub) == [2]int{100, 30} }, "A's first report never sized the grid")
	b := dialWS(t, srv.URL, sessionID)
	waitTwoSubscribers(t, manager, sessionID)

	// nextGrid reads c until a grid frame arrives.
	nextGrid := func(c *websocket.Conn, want [2]int) {
		t.Helper()
		for {
			typ, p := readFrame(t, c)
			if typ == MsgResize && len(p) == 4 {
				got := [2]int{int(p[0])<<8 | int(p[1]), int(p[2])<<8 | int(p[3])}
				if got == want {
					return
				}
			}
		}
	}

	mustWrite(b, []byte{MsgResize2, 0, 80, 0, 24, ResizeClaim})
	testutil.WaitFor(t, 2*time.Second, func() bool { return hubGrid(hub) == [2]int{80, 24} }, "B's claim never took the grid")
	nextGrid(a, [2]int{80, 24})
	nextGrid(b, [2]int{80, 24})

	mustWrite(a, []byte{MsgResize2, 0, 100, 0, 30, ResizeClaim})
	testutil.WaitFor(t, 2*time.Second, func() bool { return hubGrid(hub) == [2]int{100, 30} }, "A's claim never took the grid back")
	nextGrid(b, [2]int{100, 30})

	mustWrite(b, []byte{MsgResize2, 0, 80, 0, 24})
	// A ping after the report: its pong means the pump handled the report.
	mustWrite(b, []byte{MsgPing})
	for {
		typ, _ := readFrame(t, b)
		if typ == MsgPong {
			break
		}
	}
	if g := hubGrid(hub); g != [2]int{100, 30} {
		t.Fatalf("a 4-byte report moved the grid to %v", g)
	}
}

// captureDiag sends the log to a buffer for the rest of the test.
func captureDiag(t *testing.T) *diagBuf {
	t.Helper()
	buf := &diagBuf{}
	prevOut, prevFlags := log.Writer(), log.Flags()
	log.SetOutput(buf)
	log.SetFlags(0)
	t.Cleanup(func() { log.SetOutput(prevOut); log.SetFlags(prevFlags) })
	return buf
}

// gridLine is the last grid diag line that moved the grid to `to` (CxR).
func gridLine(buf *diagBuf, to string) string {
	var line string
	for _, l := range strings.Split(buf.String(), "\n") {
		if strings.Contains(l, "viewer-diag: grid ") && strings.Contains(l, "-> "+to+" ") {
			line = l
		}
	}
	return line
}

// typingReturnsUnderGridMu reports whether NoteTyping(sub) returns while the
// test holds gridMu, which only the lock-free owner fast path can do.
func typingReturnsUnderGridMu(t *testing.T, hub *Hub, sub *Subscriber) bool {
	t.Helper()
	hub.gridMu.Lock()
	done := make(chan struct{})
	go func() { _ = hub.NoteTyping(sub); close(done) }()
	select {
	case <-done:
		hub.gridMu.Unlock()
		return true
	case <-time.After(200 * time.Millisecond):
		hub.gridMu.Unlock()
		<-done
		return false
	}
}

func hubOwner(hub *Hub) *Subscriber {
	hub.mu.Lock()
	defer hub.mu.Unlock()
	return hub.owner
}

func TestHub_GridTypingClaims(t *testing.T) {
	buf := captureDiag(t)
	hub, calls, a := ownedAt100x30(t)
	b := gridSub(hub, "local")
	mustSize(t, hub.ResizeClient(b, 80, 24))
	mustSize(t, hub.NoteTyping(b))
	if g := hubGrid(hub); g != [2]int{80, 24} || len(calls()) != 2 {
		t.Fatalf("B typing: grid %v resizeFn %v; want 80x24", g, calls())
	}
	for name, s := range map[string]*Subscriber{"a": a, "b": b} {
		if got := gridFrames(s); !slices.Equal(got, [][2]int{{80, 24}}) {
			t.Fatalf("viewer %s grid frames %v, want one 80x24", name, got)
		}
	}
	if l := gridLine(buf, "80x24"); !strings.HasSuffix(l, "why=input") {
		t.Fatalf("grid diag line %q does not end with why=input:\n%s", l, buf.String())
	}
	mustSize(t, hub.NoteTyping(a))
	if g := hubGrid(hub); g != [2]int{100, 30} || len(calls()) != 3 || hubOwner(hub) != a {
		t.Fatalf("A typing: grid %v resizeFn %v; want 100x30 back, A the owner", g, calls())
	}
}

func TestHub_GridTypingOwnerFastPath(t *testing.T) {
	hub, calls, a := ownedAt100x30(t)
	if !typingReturnsUnderGridMu(t, hub, a) {
		t.Fatal("the owner's typing took gridMu with nothing to change")
	}
	if g := hubGrid(hub); g != [2]int{100, 30} || len(calls()) != 1 || len(gridFrames(a)) != 0 {
		t.Fatalf("the owner's typing changed something: grid %v resizeFn %v", g, calls())
	}
}

func TestHub_GridTypingOwnerNewerReport(t *testing.T) {
	buf := captureDiag(t)
	hub, calls, a := ownedAt100x30(t)
	mustSize(t, hub.ResizeClient(a, 90, 30))
	if g := hubGrid(hub); g != [2]int{100, 30} || len(calls()) != 1 {
		t.Fatalf("the owner's report moved the grid: %v, resizeFn %v", g, calls())
	}
	mustSize(t, hub.NoteTyping(a))
	if g := hubGrid(hub); g != [2]int{90, 30} || len(calls()) != 2 {
		t.Fatalf("the owner's typing after a newer report: grid %v resizeFn %v; want 90x30 once", g, calls())
	}
	if l := gridLine(buf, "90x30"); !strings.HasSuffix(l, "why=input") {
		t.Fatalf("grid diag line %q does not end with why=input", l)
	}
	mustSize(t, hub.NoteTyping(a))
	if len(calls()) != 2 || !typingReturnsUnderGridMu(t, hub, a) {
		t.Fatalf("a second keystroke was not a no-op on the fast path: resizeFn %v", calls())
	}
	// Reporting the grid's own size again leaves nothing to apply.
	mustSize(t, hub.ResizeClient(a, 90, 30))
	if !typingReturnsUnderGridMu(t, hub, a) {
		t.Fatal("a report equal to the grid left the owner on the slow path")
	}
}

func TestHub_GridTypingNeedsReport(t *testing.T) {
	hub, calls, a := ownedAt100x30(t)
	b := gridSub(hub, "web") // never reported a size: an older viewer
	mustSize(t, hub.NoteTyping(b))
	if g := hubGrid(hub); g != [2]int{100, 30} || len(calls()) != 1 || hubOwner(hub) != a {
		t.Fatalf("a viewer with no reported size took the grid: %v, resizeFn %v", g, calls())
	}
}

func TestHub_GridTypingReadOnly(t *testing.T) {
	hub, calls, a := ownedAt100x30(t)
	ro := &Subscriber{Msgs: make(chan []byte, 256), CloseSlow: func() {}, Origin: "web", ReadOnly: true, Cols: 80, Rows: 24}
	hub.Subscribe(ro)
	mustSize(t, hub.NoteTyping(ro))
	if g := hubGrid(hub); g != [2]int{100, 30} || len(calls()) != 1 || hubOwner(hub) != a {
		t.Fatalf("a read-only viewer's typing took the grid: %v, resizeFn %v", g, calls())
	}
}

func TestHub_GridOwnerLeaveClearsFast(t *testing.T) {
	hub, calls, a := ownedAt100x30(t)
	if !typingReturnsUnderGridMu(t, hub, a) {
		t.Fatal("precondition: the owner's typing should take the fast path")
	}
	hub.Unsubscribe(a)
	if typingReturnsUnderGridMu(t, hub, a) {
		t.Fatal("a departed owner's typing still took the lock-free fast path")
	}
	if hubOwner(hub) != nil || hubGrid(hub) != [2]int{100, 30} || len(calls()) != 1 {
		t.Fatalf("a departed viewer's typing took the grid: owner %p grid %v", hubOwner(hub), hubGrid(hub))
	}
}

// TestHub_GridRevokedOrGoneViewerCannotClaim (IN-09): a web viewer being
// kicked by a revoke, and a viewer that already left, can neither claim nor
// take the grid by typing; the owner and grid stay.
func TestHub_GridRevokedOrGoneViewerCannotClaim(t *testing.T) {
	hub, calls, a := ownedAt100x30(t)
	web := gridSub(hub, "web")
	mustSize(t, hub.ResizeClient(web, 80, 24)) // a known size, so NoteTyping could apply it
	gone := gridSub(hub, "local")
	hub.Unsubscribe(gone)

	hub.RevokeWebViewers(CloseReasonShareStopped)
	mustSize(t, hub.ClaimGrid(web, 90, 25))
	mustSize(t, hub.NoteTyping(web))
	mustSize(t, hub.ClaimGrid(gone, 70, 20))

	if hubOwner(hub) != a || hubGrid(hub) != [2]int{100, 30} || len(calls()) != 1 {
		t.Fatalf("a revoked or departed viewer took the grid: owner %p (want %p) grid %v resizeFn %v",
			hubOwner(hub), a, hubGrid(hub), calls())
	}
}

// Pins ROADMAP Phase 15 SC1 "if several viewers are attached, the reduced
// height actually wins" with the sizes `portty attach` sends under its bar
// (rows-1), under the reading that the claiming or typing viewer's reduced
// height wins (15-CONTEXT [Attach bar], INPT-06). A passive report never
// overrides another owner; that passive-watch case is TESTING.md M-153.
func TestHub_GridAttachBarRows(t *testing.T) {
	hub, calls := ownerHub(t)
	gui := gridSub(hub, "local")
	cli := gridSub(hub, "local")
	grid := func(step string, want [2]int) {
		t.Helper()
		if g := hubGrid(hub); g != want {
			t.Fatalf("%s: grid %v, want %v (resizeFn %v)", step, g, want, calls())
		}
	}

	mustSize(t, hub.ClaimGrid(gui, 120, 40))
	grid("GUI claim", [2]int{120, 40})
	mustSize(t, hub.ResizeClient(cli, 80, 23))
	grid("CLI report", [2]int{120, 40})
	mustSize(t, hub.ClaimGrid(cli, 80, 23))
	grid("CLI claim", [2]int{80, 23})
	mustSize(t, hub.ClaimGrid(gui, 120, 40))
	grid("GUI claim back", [2]int{120, 40})
	mustSize(t, hub.NoteTyping(cli))
	grid("CLI typing", [2]int{80, 23})
}
