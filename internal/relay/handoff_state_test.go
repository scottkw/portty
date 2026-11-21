package relay

// Phase 9 (UPGR-01, UPGR-02): a hub's state moves to another daemon. Freeze
// stops the read loop without telling anyone the session ended; Export turns
// the frozen hub into plain data; NewHubFromState rebuilds a staged hub from
// it, whose deltas, renders and continuations equal a reference hub's.

import (
	"bytes"
	"encoding/gob"
	"errors"
	"fmt"
	"io"
	"os"
	"slices"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/scottkw/portty/internal/testutil"
)

// pipeHub is a hub on an os.Pipe (pollable, so it takes read deadlines),
// started. Closing w ends the session.
func pipeHub(t *testing.T, resizeFn func(cols, rows int) error) (*Hub, *os.File) {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	hub := NewHub("handoff", r, io.Discard, DefaultScrollbackBytes, resizeFn)
	go hub.Run()
	t.Cleanup(func() { _ = w.Close(); _ = r.Close() })
	return hub, w
}

// ringPayload is the ring without its 0x01 frame headers.
func ringPayload(h *Hub) []byte {
	return bytes.ReplaceAll(h.scrollback.Snapshot(), []byte{MsgOutput}, nil)
}

func isClosed(c <-chan struct{}) bool {
	select {
	case <-c:
		return true
	default:
		return false
	}
}

// TestHub_FreezeThaw (T-R1): a freeze stops reading without closing Done, the
// shell keeps writing, and the thawed hub reads every byte once.
func TestHub_FreezeThaw(t *testing.T) {
	hub, w := pipeHub(t, nil)
	var n atomic.Int64
	stop := make(chan struct{})
	writerDone := make(chan struct{})
	go func() {
		defer close(writerDone)
		for {
			select {
			case <-stop:
				return
			case <-time.After(10 * time.Millisecond):
			}
			if _, err := fmt.Fprintf(w, "tick %d\n", n.Load()); err != nil {
				return
			}
			n.Add(1)
		}
	}()
	time.Sleep(100 * time.Millisecond)

	start := time.Now()
	if err := hub.Freeze(); err != nil {
		t.Fatalf("Freeze: %v", err)
	}
	if d := time.Since(start); d > 200*time.Millisecond {
		t.Fatalf("Freeze took %v, want under 200ms", d)
	}
	if isClosed(hub.Done()) {
		t.Fatal("Freeze closed Done: viewers would be told the session ended")
	}
	frozenEnd := hub.scrollback.End()
	time.Sleep(300 * time.Millisecond)
	if got := hub.scrollback.End(); got != frozenEnd {
		t.Fatalf("frozen hub read on: end %d -> %d", frozenEnd, got)
	}
	hub.Thaw()
	time.Sleep(100 * time.Millisecond)
	close(stop)
	<-writerDone
	last := n.Load() - 1
	want := fmt.Sprintf("tick %d\n", last)
	testutil.WaitFor(t, 2*time.Second, func() bool { return bytes.HasSuffix(ringPayload(hub), []byte(want)) },
		"thawed hub never read %q", want)

	lines := strings.Split(strings.TrimSuffix(string(ringPayload(hub)), "\n"), "\n")
	if int64(len(lines)) != last+1 {
		t.Fatalf("ring holds %d ticks, want %d", len(lines), last+1)
	}
	for i, l := range lines {
		if l != "tick "+strconv.Itoa(i) {
			t.Fatalf("tick %d is %q: lost or repeated across the freeze", i, l)
		}
	}
	if isClosed(hub.Done()) {
		t.Fatal("Done closed after Thaw")
	}
}

// noDeadlineReader has no SetReadDeadline at all.
type noDeadlineReader struct{ io.Reader }

// blockingMaster takes deadlines the way a blocking PTY master does: never.
type blockingMaster struct{ *os.File }

func (blockingMaster) SetReadDeadline(time.Time) error { return os.ErrNoDeadline }

// TestHub_FreezeWithoutDeadline: Freeze fails at once (it never hangs) when
// the reader cannot take a deadline, and the hub keeps reading (Pitfall 6).
func TestHub_FreezeWithoutDeadline(t *testing.T) {
	for name, wrap := range map[string]func(*os.File) io.Reader{
		"no SetReadDeadline": func(f *os.File) io.Reader { return noDeadlineReader{f} },
		"os.ErrNoDeadline":   func(f *os.File) io.Reader { return blockingMaster{f} },
	} {
		t.Run(name, func(t *testing.T) {
			r, w, err := os.Pipe()
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = w.Close(); _ = r.Close() })
			hub := NewHub("nodeadline", wrap(r), io.Discard, DefaultScrollbackBytes, nil)
			go hub.Run()
			start := time.Now()
			if err := hub.Freeze(); err == nil {
				t.Fatal("Freeze succeeded on a reader without deadlines")
			}
			if d := time.Since(start); d > 50*time.Millisecond {
				t.Fatalf("Freeze took %v to fail, want under 50ms", d)
			}
			if _, err := io.WriteString(w, "still-read"); err != nil {
				t.Fatal(err)
			}
			testutil.WaitFor(t, 2*time.Second, func() bool { return bytes.Contains(ringPayload(hub), []byte("still-read")) },
				"hub stopped reading after a failed freeze")
		})
	}
}

// exitOnDeadline is a shell that exits as the freeze is called: the pending
// Read returns EOF, not the deadline.
type exitOnDeadline struct{ exited chan struct{} }

func (e *exitOnDeadline) Read([]byte) (int, error) { <-e.exited; return 0, io.EOF }
func (e *exitOnDeadline) SetReadDeadline(t time.Time) error {
	if !t.IsZero() {
		close(e.exited)
	}
	return nil
}

// TestHub_FreezeWhileEnding: a shell that exits during the freeze ends the
// hub (Done closed) and Freeze reports ErrHubEnded at once, not after 2 s.
func TestHub_FreezeWhileEnding(t *testing.T) {
	t.Run("exits as the freeze starts", func(t *testing.T) {
		hub := NewHub("ending", &exitOnDeadline{exited: make(chan struct{})}, io.Discard, DefaultScrollbackBytes, nil)
		go hub.Run()
		start := time.Now()
		if err := hub.Freeze(); !errors.Is(err, ErrHubEnded) {
			t.Fatalf("Freeze: %v, want ErrHubEnded", err)
		}
		if d := time.Since(start); d > 100*time.Millisecond {
			t.Fatalf("Freeze took %v, want under 100ms", d)
		}
		if !isClosed(hub.Done()) {
			t.Fatal("Done open after the shell exited")
		}
	})
	t.Run("already exited", func(t *testing.T) {
		hub, w := pipeHub(t, nil)
		_ = w.Close()
		testutil.WaitFor(t, 2*time.Second, func() bool { return isClosed(hub.Done()) }, "hub never ended")
		start := time.Now()
		if err := hub.Freeze(); !errors.Is(err, ErrHubEnded) {
			t.Fatalf("Freeze: %v, want ErrHubEnded", err)
		}
		if d := time.Since(start); d > 100*time.Millisecond {
			t.Fatalf("Freeze took %v, want under 100ms", d)
		}
	})
}

// TestHub_FrozenResizeAppliedOnThaw: a frozen hub records a local viewer's
// size but leaves the PTY alone; Thaw applies the arbiter's grid once.
func TestHub_FrozenResizeAppliedOnThaw(t *testing.T) {
	var mu sync.Mutex
	var calls [][2]int
	hub, _ := pipeHub(t, func(c, r int) error {
		mu.Lock()
		defer mu.Unlock()
		calls = append(calls, [2]int{c, r})
		return nil
	})
	if err := hub.Freeze(); err != nil {
		t.Fatalf("Freeze: %v", err)
	}
	sub := &Subscriber{Msgs: make(chan []byte, 256), CloseSlow: func() {}, Origin: "local"}
	hub.Subscribe(sub)
	if err := hub.ResizeClient(sub, 100, 30); err != nil {
		t.Fatal(err)
	}
	mu.Lock()
	n := len(calls)
	mu.Unlock()
	if n != 0 {
		t.Fatalf("frozen resize reached the PTY: %v", calls)
	}
	if len(sub.Msgs) != 0 {
		t.Fatal("frozen resize was broadcast")
	}
	hub.Thaw()
	mu.Lock()
	defer mu.Unlock()
	if len(calls) != 1 || calls[0] != [2]int{100, 30} {
		t.Fatalf("resizeFn calls after Thaw: %v, want one 100x30", calls)
	}
}

// importedGridHub imports and starts a hub whose grid is 85x29 after a genuine
// resize, so geomEnd is set and below the end, as in V-02. calls returns
// the resizeFn calls so far.
func importedGridHub(t *testing.T) (hub *Hub, calls func() [][2]int) {
	t.Helper()
	r0, w0, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = w0.Close(); _ = r0.Close() })
	orig := NewHub("imported", r0, io.Discard, DefaultScrollbackBytes, nil)
	resizeHub(t, orig, 80, 24) // the spawn grid: not genuine
	feedHubs(plainShellStream(0, 4<<10, 80), 1<<10, orig)
	resizeHub(t, orig, 85, 29) // genuine: geomEnd > 0
	feedHubs(plainShellStream(1000, 2<<10, 85), 1<<10, orig)
	freezeIdle(t, orig)
	st, err := orig.Export()
	if err != nil {
		t.Fatalf("Export: %v", err)
	}
	if st.GeomEnd == 0 || st.GeomEnd >= st.Ring.End || st.PtyCols != 85 || st.PtyRows != 29 {
		t.Fatalf("fixture: geomEnd %d end %d grid %dx%d", st.GeomEnd, st.Ring.End, st.PtyCols, st.PtyRows)
	}
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = w.Close(); _ = r.Close() })
	var mu sync.Mutex
	var got [][2]int
	hub, err = NewHubFromState(gobRoundTrip(t, st), r, io.Discard, func(c, rows int) error {
		mu.Lock()
		defer mu.Unlock()
		got = append(got, [2]int{c, rows})
		return nil
	})
	if err != nil {
		t.Fatalf("NewHubFromState: %v", err)
	}
	hub.Start()
	t.Cleanup(hub.Shutdown)
	return hub, func() [][2]int {
		mu.Lock()
		defer mu.Unlock()
		return slices.Clone(got)
	}
}

// joinLocal joins a local viewer at resume and reports its size, the way a
// reconnecting GUI tab or `portty attach` does. It returns the replay mode
// and whether a grid frame reached the viewer.
func joinLocal(t *testing.T, hub *Hub, resume uint64, cols, rows int) (*Subscriber, string, bool) {
	t.Helper()
	sub := &Subscriber{Msgs: make(chan []byte, 256), CloseSlow: func() {}, Origin: "local"}
	r := hub.Join(sub, resume, true)
	for len(hub.TakePending(sub)) > 0 {
	}
	if err := hub.ResizeClient(sub, cols, rows); err != nil {
		t.Fatal(err)
	}
	return sub, r.Mode, drainResize(sub)
}

// drainResize empties sub's queue and reports whether it held a grid frame.
func drainResize(sub *Subscriber) bool {
	seen := false
	for {
		select {
		case f := <-sub.Msgs:
			seen = seen || f[0] == MsgResize
		default:
			return seen
		}
	}
}

// TestHub_HandoffRejoinKeepsGrid (V-02): after a handoff the local viewers
// rejoin one by one. The 100x30 attach first, then the 85x29 GUI tab, leave
// the imported 85x29 grid alone, with no grid frame and no geomEnd bump, so
// both resumes are deltas: joins and reports never move a known grid
// (INPT-06). A claim afterwards is a real resize and applies once.
func TestHub_HandoffRejoinKeepsGrid(t *testing.T) {
	hub, calls := importedGridHub(t)
	hub.emuMu.Lock()
	geomEnd := hub.geomEnd
	hub.emuMu.Unlock()
	end := hub.scrollback.End()

	a, mode, grid := joinLocal(t, hub, end, 100, 30)
	if mode != ReplayDelta || grid {
		t.Fatalf("attach: mode %s, grid frame %v; want a delta and no grid frame", mode, grid)
	}
	b, mode, grid := joinLocal(t, hub, end, 85, 29)
	if mode != ReplayDelta || grid {
		t.Fatalf("GUI tab: mode %s, grid frame %v; want a delta and no grid frame", mode, grid)
	}
	if drainResize(a) {
		t.Fatal("the GUI tab's join sent the attach a grid frame")
	}
	hub.emuMu.Lock()
	gotGeom := hub.geomEnd
	hub.emuMu.Unlock()
	if gotGeom != geomEnd || hub.Cols() != 85 || hub.Rows() != 29 || len(calls()) != 0 {
		t.Fatalf("grid %dx%d geomEnd %d (was %d) resizeFn %v; want 85x29 untouched", hub.Cols(), hub.Rows(), gotGeom, geomEnd, calls())
	}

	if err := hub.ClaimGrid(a, 100, 30); err != nil {
		t.Fatal(err)
	}
	if hub.Cols() != 100 || hub.Rows() != 30 || !slices.Equal(calls(), [][2]int{{100, 30}}) {
		t.Fatalf("after the attach's claim: %dx%d, resizeFn %v; want 100x30 once", hub.Cols(), hub.Rows(), calls())
	}
	for name, s := range map[string]*Subscriber{"attach": a, "GUI tab": b} {
		if got := gridFrames(s); !slices.Equal(got, [][2]int{{100, 30}}) {
			t.Fatalf("%s grid frames %v, want one 100x30", name, got)
		}
	}
}

// TestHub_HandoffFrozenClaimAppliedOnThaw: a claim on an imported hub frozen
// for the next handoff touches neither the PTY nor the viewers; Thaw (a
// rollback) applies it once.
func TestHub_HandoffFrozenClaimAppliedOnThaw(t *testing.T) {
	hub, calls := importedGridHub(t)
	a, _, _ := joinLocal(t, hub, hub.scrollback.End(), 100, 30)
	if err := hub.Freeze(); err != nil {
		t.Fatalf("Freeze: %v", err)
	}
	if err := hub.ClaimGrid(a, 100, 30); err != nil {
		t.Fatal(err)
	}
	if hub.Cols() != 85 || len(calls()) != 0 || drainResize(a) {
		t.Fatalf("a claim resized a frozen hub: %dx%d, resizeFn %v", hub.Cols(), hub.Rows(), calls())
	}
	hub.Thaw()
	if hub.Cols() != 100 || hub.Rows() != 30 || len(calls()) != 1 || !drainResize(a) {
		t.Fatalf("after Thaw: %dx%d, resizeFn %v; want 100x30 once, broadcast", hub.Cols(), hub.Rows(), calls())
	}
}

// feedHubs records s on every hub as reads of at most chunk bytes, the way
// Run does (recordFrameLocked under emuMu).
func feedHubs(s string, chunk int, hubs ...*Hub) {
	for len(s) > 0 {
		n := min(len(s), chunk)
		for _, h := range hubs {
			h.emuMu.Lock()
			h.recordFrameLocked(MakeOutputFrame([]byte(s[:n])), []byte(s[:n]))
			h.emuMu.Unlock()
		}
		s = s[n:]
	}
}

// freezeIdle starts Run on hub's (empty) pipe and freezes it: the state was
// recorded directly, so Run reads nothing before the freeze.
func freezeIdle(t *testing.T, hub *Hub) {
	t.Helper()
	go hub.Run()
	if err := hub.Freeze(); err != nil {
		t.Fatalf("Freeze: %v", err)
	}
}

// gobRoundTrip encodes and decodes st as the handoff socket does.
func gobRoundTrip(t *testing.T, st HubState) HubState {
	t.Helper()
	var buf bytes.Buffer
	if err := gob.NewEncoder(&buf).Encode(st); err != nil {
		t.Fatalf("gob encode: %v", err)
	}
	var out HubState
	if err := gob.NewDecoder(&buf).Decode(&out); err != nil {
		t.Fatalf("gob decode: %v", err)
	}
	return out
}

// plainShellStream is about size bytes of shell output: numbered lines,
// prompts, colours and lines longer than cols (soft wraps).
func plainShellStream(from, size, cols int) string {
	var b strings.Builder
	for i := from; b.Len() < size; i++ {
		switch i % 7 {
		case 0:
			fmt.Fprintf(&b, "\x1b[32m$\x1b[m ls -la %d\r\n", i)
		case 3:
			fmt.Fprintf(&b, "LINE-%06d %s\r\n", i, strings.Repeat("w", cols+cols/3))
		default:
			fmt.Fprintf(&b, "LINE-%06d \x1b[1;3%dm%s\x1b[m\r\n", i, i%8, strings.Repeat("x", 16+i%40))
		}
	}
	return b.String()
}

// importedHub exports the frozen hub, gob round-trips the state and imports
// it onto a fresh pipe, staged (not started). It returns the hub and the
// pipe's writer.
func importedHub(t *testing.T, orig *Hub) (*Hub, *os.File, HubState) {
	t.Helper()
	st, err := orig.Export()
	if err != nil {
		t.Fatalf("Export: %v", err)
	}
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = w.Close(); _ = r.Close() })
	got, err := NewHubFromState(gobRoundTrip(t, st), r, io.Discard, nil)
	if err != nil {
		t.Fatalf("NewHubFromState: %v", err)
	}
	return got, w, st
}

// requireSameDeltas checks the ring and offsets, which decide every delta,
// then Joins both hubs at a spread of resume offsets across the window and
// at its edges.
func requireSameDeltas(t *testing.T, ref, got *Hub) {
	t.Helper()
	if !bytes.Equal(ref.scrollback.Snapshot(), got.scrollback.Snapshot()) ||
		ref.scrollback.End() != got.scrollback.End() || ref.scrollback.Truncated() != got.scrollback.Truncated() {
		t.Fatalf("ring differs: end %d/%d truncated %v/%v", ref.scrollback.End(), got.scrollback.End(),
			ref.scrollback.Truncated(), got.scrollback.Truncated())
	}
	ref.emuMu.Lock()
	rg := ref.geomEnd
	ref.emuMu.Unlock()
	got.emuMu.Lock()
	gg := got.geomEnd
	got.emuMu.Unlock()
	if rg != gg {
		t.Fatalf("geomEnd %d, want %d", gg, rg)
	}
	end := ref.scrollback.End()
	start := end - uint64(len(ref.scrollback.Snapshot()))
	offs := []uint64{0, start - 1, start, start + 1, rg, rg + 1, end - 1, end, end + 1}
	for x := start; x < end; x += 4099 {
		offs = append(offs, x)
	}
	for _, x := range offs {
		a, b := join(t, ref, x, true), join(t, got, x, true)
		if a.Mode != b.Mode || a.End != b.End || !bytes.Equal(bytes.Join(a.Chunks, nil), bytes.Join(b.Chunks, nil)) {
			t.Fatalf("resume %d: imported %s end %d (%d bytes), reference %s end %d (%d bytes)",
				x, b.Mode, b.End, b.Bytes(), a.Mode, a.End, a.Bytes())
		}
	}
}

// requireSameRender checks the late joiner's rendered replay: grid, cursor
// and the last 200 history rows, in a vt guest.
func requireSameRender(t *testing.T, ref, got *Hub, cols, rows int) {
	t.Helper()
	rg, _ := joinGuest(t, ref, cols, rows)
	gg, _ := joinGuest(t, got, cols, rows)
	requireSameGrid(t, "joined screen", screenRows(rg), screenRows(gg))
	requireSameCursor(t, rg, gg)
	rs, gs := scrollbackRows(rg), scrollbackRows(gg)
	if len(rs) != len(gs) {
		t.Fatalf("joined history: %d rows, want %d", len(gs), len(rs))
	}
	requireSameGrid(t, "joined history", rs[max(0, len(rs)-200):], gs[max(0, len(gs)-200):])
}

// requireSameEmu checks RenderSnapshot and the live emulator's cursor.
func requireSameEmu(t *testing.T, what string, ref, got *Hub) {
	t.Helper()
	if a, b := ref.RenderSnapshot(), got.RenderSnapshot(); !bytes.Equal(a, b) {
		requireSameGrid(t, what+": RenderSnapshot", strings.Split(string(a), "\r\n"), strings.Split(string(b), "\r\n"))
		t.Fatalf("%s: RenderSnapshot differs", what)
	}
	ref.emuMu.Lock()
	rp := ref.liveEmu.CursorPosition()
	ref.emuMu.Unlock()
	got.emuMu.Lock()
	gp := got.liveEmu.CursorPosition()
	got.emuMu.Unlock()
	if rp != gp {
		t.Fatalf("%s: cursor %v, want %v", what, gp, rp)
	}
}

// TestHandoffState_RoundTripPlain (T-R2): a shell session that wrapped the
// 256 KiB ring and had one genuine resize moves through gob into a staged
// hub on a new pipe. Every resume offset gets the reference's delta, a late
// joiner renders the same, and later output lands the same.
func TestHandoffState_RoundTripPlain(t *testing.T) {
	const cols, rows = 80, 24
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = w.Close(); _ = r.Close() })
	orig := NewHub("plain", r, io.Discard, DefaultScrollbackBytes, nil)
	ref := NewHub("plain", nil, nil, DefaultScrollbackBytes, nil)
	for _, h := range []*Hub{orig, ref} {
		resizeHub(t, h, 100, rows) // the spawn grid: not genuine
	}
	feedHubs(plainShellStream(0, 120<<10, 100), 16<<10, orig, ref)
	for _, h := range []*Hub{orig, ref} {
		resizeHub(t, h, cols, rows) // genuine: geomEnd > 0
	}
	feedHubs(plainShellStream(5000, 200<<10, cols)+"$ par", 7<<10+13, orig, ref)
	if !ref.scrollback.Truncated() || ref.geomEnd == 0 || ref.history.Len() < 200 {
		t.Fatalf("fixture invalid: truncated %v geomEnd %d history %d", ref.scrollback.Truncated(), ref.geomEnd, ref.history.Len())
	}

	freezeIdle(t, orig)
	got, gw, st := importedHub(t, orig)
	if st.Ring.End != ref.scrollback.End() || got.scrollback.End() != ref.scrollback.End() {
		t.Fatalf("ring end: state %d, imported %d, want %d (the seed must not reach the ring)",
			st.Ring.End, got.scrollback.End(), ref.scrollback.End())
	}
	if got.Cols() != cols || got.Rows() != rows {
		t.Fatalf("grid %dx%d, want %dx%d", got.Cols(), got.Rows(), cols, rows)
	}

	requireSameDeltas(t, ref, got)
	requireSameRender(t, ref, got, cols, rows)
	requireSameEmu(t, "after import", ref, got)

	got.Start()
	cont := "tial\r\n" + plainShellStream(9000, 4<<10, cols)
	before := got.scrollback.End()
	if _, err := io.WriteString(gw, cont); err != nil {
		t.Fatal(err)
	}
	feedHubs(cont, 1<<10, ref)
	testutil.WaitFor(t, 2*time.Second, func() bool {
		d, _, _ := got.scrollback.SnapshotFrom(before)
		return len(bytes.ReplaceAll(d, []byte{MsgOutput}, nil)) == len(cont)
	}, "imported hub never read the continuation")
	requireSameEmu(t, "after the continuation", ref, got)
	requireSameRender(t, ref, got, cols, rows)
}

// validState is a small exported state that imports cleanly.
func validState(t *testing.T) HubState {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = w.Close(); _ = r.Close() })
	hub := NewHub("valid", r, io.Discard, DefaultScrollbackBytes, nil)
	resizeHub(t, hub, 40, 10)
	feedHubs(plainShellStream(0, 4<<10, 40), 1<<10, hub)
	freezeIdle(t, hub)
	st, err := hub.Export()
	if err != nil {
		t.Fatalf("Export: %v", err)
	}
	if _, err := NewHubFromState(st, nil, nil, nil); err != nil {
		t.Fatalf("fixture invalid: %v", err)
	}
	return st
}

// TestHandoffState_ImportValidates (T-09-11): every invalid field is an
// error and no hub.
func TestHandoffState_ImportValidates(t *testing.T) {
	cases := map[string]func(*HubState){
		"ring longer than its max":  func(s *HubState) { s.Ring.Max = len(s.Ring.Buf) - 1 },
		"ring max over the default": func(s *HubState) { s.Ring.Max = DefaultScrollbackBytes + 1 },
		"end below the ring length": func(s *HubState) { s.Ring.End = uint64(len(s.Ring.Buf)) - 1 },
		"geomEnd past the end":      func(s *HubState) { s.GeomEnd = s.Ring.End + 1 },
		"history over the line cap": func(s *HubState) {
			s.History.Lines = make([]string, HistoryMaxLines+1)
			s.History.Wrapped = make([]bool, HistoryMaxLines+1)
		},
		"history over the byte cap": func(s *HubState) {
			s.History.Lines = []string{strings.Repeat("x", HistoryMaxBytes+1)}
			s.History.Wrapped = []bool{false}
		},
		"wrapped length mismatch":      func(s *HubState) { s.History.Wrapped = append(s.History.Wrapped, true) },
		"zero columns":                 func(s *HubState) { s.EmuCols = 0 },
		"5000 columns":                 func(s *HubState) { s.EmuCols = 5000 },
		"5000 pty columns":             func(s *HubState) { s.PtyCols = 5000 },
		"main region outside the rows": func(s *HubState) { s.MainRegion = [2]int{2, s.EmuRows + 1} },
		"main region inverted":         func(s *HubState) { s.MainRegion = [2]int{5, 3} },
		"too many wrap keys":           func(s *HubState) { s.Wraps.Ring = make([]uint64, wrapSetMax+1) },
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			st := validState(t)
			mutate(&st)
			hub, err := NewHubFromState(st, nil, nil, nil)
			if err == nil || hub != nil {
				t.Fatalf("imported an invalid state: hub %v err %v", hub, err)
			}
		})
	}
}

// TestHandoffState_CwdRoundTrip (PATH-01): the cwd log and the history row
// counter cross an export, a gob round trip and an import, and every line
// resolves to the directory it did before.
func TestHandoffState_CwdRoundTrip(t *testing.T) {
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = w.Close(); _ = r.Close() })
	orig := NewHub("cwd", r, io.Discard, DefaultScrollbackBytes, nil)
	resizeHub(t, orig, 80, 24)
	feedHubs(osc7("/a")+"MARK-A\r\n"+osc7("/b%20c")+"MARK-B\r\n"+fillerLines(60)+
		osc7("/d")+"MARK-D\r\n"+osc7("/e")+"MARK-E\r\n", 1<<10, orig)
	if orig.history.Len() == 0 {
		t.Fatal("fixture: nothing in history")
	}

	freezeIdle(t, orig)
	got, _, st := importedHub(t, orig)
	if len(st.Cwds) == 0 || st.HistoryPushed != orig.history.Pushed() {
		t.Fatalf("state: %d records, counter %d (hub %d)", len(st.Cwds), st.HistoryPushed, orig.history.Pushed())
	}
	if got.history.Pushed() != orig.history.Pushed() || !slices.Equal(got.cwds.recs, orig.cwds.recs) {
		t.Fatalf("imported counter %d log %v, want %d %v", got.history.Pushed(), got.cwds.recs, orig.history.Pushed(), orig.cwds.recs)
	}
	for m, want := range map[string]string{"MARK-A": "/a", "MARK-B": "/b c", "MARK-D": "/d", "MARK-E": "/e"} {
		wantCwd(t, orig, m, want)
		wantCwd(t, got, m, want)
	}
}

// TestHandoffState_CwdOldShape: a state from a daemon before PATH-01 has no
// Cwds and no HistoryPushed; it imports with an empty log and the counter at
// the number of history lines.
func TestHandoffState_CwdOldShape(t *testing.T) {
	st := validState(t)
	if len(st.History.Lines) == 0 {
		t.Fatal("fixture: no history lines")
	}
	st.Cwds, st.HistoryPushed = nil, 0
	got, err := NewHubFromState(gobRoundTrip(t, st), nil, nil, nil)
	if err != nil {
		t.Fatalf("old-shape state refused: %v", err)
	}
	if len(got.cwds.recs) != 0 || got.history.Pushed() != uint64(len(st.History.Lines)) {
		t.Fatalf("log %v, counter %d; want empty and %d", got.cwds.recs, got.history.Pushed(), len(st.History.Lines))
	}
}

// TestHandoffState_CwdValidates (T-12-13): malformed cwd state is refused, so
// the old daemon keeps the session.
func TestHandoffState_CwdValidates(t *testing.T) {
	many := func(n, size int) []CwdRecord {
		recs := make([]CwdRecord, n)
		for i := range recs {
			d := fmt.Sprintf("/d%04d", i)
			recs[i] = CwdRecord{Row: uint64(i + 1), Dir: d + strings.Repeat("x", size-len(d))}
		}
		return recs
	}
	cases := map[string]func(*HubState){
		"relative directory":     func(s *HubState) { s.Cwds = []CwdRecord{{1, "rel/dir"}} },
		"empty directory":        func(s *HubState) { s.Cwds = []CwdRecord{{1, ""}} },
		"unclean directory":      func(s *HubState) { s.Cwds = []CwdRecord{{1, "/a/../b"}} },
		"trailing slash":         func(s *HubState) { s.Cwds = []CwdRecord{{1, "/a/"}} },
		"newline in directory":   func(s *HubState) { s.Cwds = []CwdRecord{{1, "/a\nb"}} },
		"directory over 1024":    func(s *HubState) { s.Cwds = many(1, cwdMaxPathBytes+1) },
		"equal rows":             func(s *HubState) { s.Cwds = []CwdRecord{{5, "/a"}, {5, "/b"}} },
		"decreasing rows":        func(s *HubState) { s.Cwds = []CwdRecord{{6, "/a"}, {5, "/b"}} },
		"row past the grid":      func(s *HubState) { s.Cwds = []CwdRecord{{s.HistoryPushed + maxHandoffGrid + 1, "/a"}} },
		"1025 records":           func(s *HubState) { s.Cwds = many(cwdLogMaxRecords+1, 8) },
		"over 64 KiB":            func(s *HubState) { s.Cwds = many(70, 1000) },
		"counter below the line": func(s *HubState) { s.HistoryPushed = uint64(len(s.History.Lines)) - 1 },
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			st := validState(t)
			if len(st.History.Lines) < 2 {
				t.Fatal("fixture: too few history lines")
			}
			mutate(&st)
			hub, err := NewHubFromState(st, nil, nil, nil)
			if err == nil || hub != nil {
				t.Fatalf("imported an invalid state: hub %v err %v", hub, err)
			}
		})
	}

	st := validState(t) // the limits themselves are accepted
	st.Cwds = append(many(cwdLogMaxRecords-1, 8), CwdRecord{Row: st.HistoryPushed + maxHandoffGrid, Dir: "/last"})
	if _, err := NewHubFromState(st, nil, nil, nil); err != nil {
		t.Fatalf("a log at its limits was refused: %v", err)
	}
}

// TestHandoffState_StagedReadsNothing: an imported hub reads no byte until
// Start, so the old daemon still owns the PTY until COMMIT.
func TestHandoffState_StagedReadsNothing(t *testing.T) {
	st := validState(t)
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = w.Close(); _ = r.Close() })
	hub, err := NewHubFromState(st, r, io.Discard, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := io.WriteString(w, "pending"); err != nil {
		t.Fatal(err)
	}
	time.Sleep(100 * time.Millisecond)
	if got := hub.scrollback.End(); got != st.Ring.End {
		t.Fatalf("staged hub read: end %d, want %d", got, st.Ring.End)
	}
	hub.Start()
	hub.Start() // idempotent: one reader only
	testutil.WaitFor(t, 2*time.Second, func() bool { return bytes.HasSuffix(ringPayload(hub), []byte("pending")) },
		"started hub never read the pending bytes")
}

// TestHubManager_AddStaged: a staged hub joins the manager without starting;
// an existing id is refused.
func TestHubManager_AddStaged(t *testing.T) {
	m := NewHubManager()
	st := validState(t)
	hub, err := NewHubFromState(st, nil, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := m.AddStaged(st.SessionID, hub); err != nil {
		t.Fatalf("AddStaged: %v", err)
	}
	if got, ok := m.Get(st.SessionID); !ok || got != hub {
		t.Fatal("staged hub not in the manager")
	}
	if err := m.AddStaged(st.SessionID, hub); err == nil {
		t.Fatal("AddStaged accepted a duplicate id")
	}
}

// splitReads cuts s into reads of n bytes, at byte positions.
func splitReads(s string, n int) []string {
	var out []string
	for len(s) > 0 {
		k := min(len(s), n)
		out = append(out, s[:k])
		s = s[k:]
	}
	return out
}

// recordReads records each read on every hub as one PTY read.
func recordReads(reads []string, hubs ...*Hub) {
	for _, s := range reads {
		feedHubs(s, len(s), hubs...)
	}
}

// vimLike is a full-screen editor's session in the alt screen at cols x
// rows: full repaint, a styled status line, a scroll region, a saved cursor
// and a pen left set, the last read cut mid-CSI.
func vimLike(cols, rows int) []string {
	var b strings.Builder
	b.WriteString("$ vim notes.txt\r\n\x1b[?1049h\x1b[?1h\x1b=\x1b[H\x1b[2J")
	for y := 1; y < rows; y++ {
		if y < 6 {
			fmt.Fprintf(&b, "\x1b[%d;1Hline %d of the \x1b[1mfile\x1b[m", y, y)
		} else {
			fmt.Fprintf(&b, "\x1b[%d;1H\x1b[34m~\x1b[m", y)
		}
	}
	fmt.Fprintf(&b, "\x1b[%d;1H\x1b[7m%-*s\x1b[m", rows, cols, "notes.txt [+]")
	fmt.Fprintf(&b, "\x1b[1;%dr\x1b[3;7H\x1b7\x1b[5;12H\x1b[1;33m", rows-1)
	return append(splitReads(b.String(), 301), "\x1b[3")
}

// handoffCase is one corpus session: reads before the export and after it.
type handoffCase struct {
	name       string
	cols, rows int
	ring       int
	pre, cont  []string
}

func handoffCorpus() []handoffCase {
	claude := claudeStream(200<<10, fidCols)
	next := strings.Count(claude, "\x1b[?2026h")
	shell := plainShellStream(0, 30<<10, 80)
	vim := append(splitReads(shell, 4<<10), vimLike(80, 24)...)
	link := "\x1b]8;id=7;https://example.invalid/a\x1b\\"
	return []handoffCase{
		{"wrapped Claude session, live redraw", fidCols, fidRows, 64 << 10,
			splitReads(claude, 16<<10+7), []string{claudeLikeRedraw(next, fidCols), claudeLikeTurn(next+1, fidCols)}},
		{"soft-wrapped lines, export in pending wrap", 40, 10, DefaultScrollbackBytes,
			splitReads(plainShellStream(0, 20<<10, 40)+strings.Repeat("y", 80), 997), []string{"z\r\nmore\r\n", "$ "}},
		{"SO/G1 line drawing and an open OSC 8 link", 40, 10, DefaultScrollbackBytes,
			[]string{"$ x\r\n\x1b)0\x0eqqq\x1b(0lqk\r\n", link + "linked"}, []string{"text\x0fplain\x1b]8;;\x1b\\ end\r\nqqq\x1b(B"}},
		{"scroll region with origin mode", 40, 10, DefaultScrollbackBytes,
			[]string{shell[:2000], "\x1b[3;7r\x1b[?6h\x1b[1;1Hab\x1b7"}, []string{"\r\nA\r\nB\r\nC\r\nD\r\nE\r\nF\x1b8Z", "\x1b[r\x1b[?6l\x1b[10;1Hdone"}},
		{"vim-like in the alt screen (1049h)", 80, 24, DefaultScrollbackBytes,
			vim, []string{"1mX\x1b[m", "\x1b[23;1H\n\nscrolled", "\x1b8Y\x1b[2;1H\x1b[Kline 2 edited"}},
		{"vim-like, then the app exits after the import", 80, 24, DefaultScrollbackBytes,
			vim, []string{"1mX\x1b[m", "\x1b[r\x1b[?1049l\x1b[?1l\x1b>", "$ ls\r\nnotes.txt\r\n$ "}},
		{"alt screen in pending wrap", 40, 10, DefaultScrollbackBytes,
			[]string{"$ top\r\n", "\x1b[?1049h\x1b[3;1H" + strings.Repeat("p", 40)}, []string{"Q", "\x1b[?1049l$ "}},
		{"alt screen in pending wrap, saved cursor elsewhere", 40, 10, DefaultScrollbackBytes,
			[]string{"$ top\r\n", "\x1b[?1049h\x1b[2;5H\x1b7\x1b[3;1H" + strings.Repeat("p", 40)}, []string{"Q", "\x1b8R\x1b[?1049l$ "}},
		{"alt screen in pending wrap at the saved cursor", 40, 10, DefaultScrollbackBytes,
			[]string{"$ top\r\n", "\x1b[?1049h\x1b[2;5H\x1b7\x1b[4;1H" + strings.Repeat("p", 40) + "\x1b8"}, []string{"Q", "\x1b[?1049l$ "}},
		{"1047 app keeps the shell's saved cursor", 40, 10, DefaultScrollbackBytes,
			[]string{"ab\x1b7\r\ncd", "\x1b[?1047hALT\x1b[5;5H"}, []string{"x", "\x1b[?1047l\x1b8Z"}},
		{"link open on the main screen, none in the alt screen", 40, 10, DefaultScrollbackBytes,
			[]string{"\x1b]8;;https://example.invalid/m\x1b\\$ ", "\x1b[?1049h\x1b]8;;\x1b\\alt"}, []string{"more", "\x1b[?1049lmain-linked"}},
		{"line drawing G0 behind the alt screen", 40, 10, DefaultScrollbackBytes,
			[]string{"\x1b(0lqk\r\n", "\x1b[?1049h\x1b(Bqx q\x1b(0"}, []string{"qq", "\x1b[?1049lqq"}},
		{"region in the alt screen", 40, 10, DefaultScrollbackBytes,
			[]string{"$ less\r\n", "\x1b[?1049h\x1b[2;6r\x1b[6;1Hbottom"}, []string{"\nA\nB\nC", "\x1b[?1049l$ "}},
	}
}

// runHandoffCase records pre on the original and a reference hub, moves the
// original through Export, gob and NewHubFromState, then records cont on the
// imported hub and the reference, checking deltas, renders and the emulator.
func runHandoffCase(t *testing.T, c handoffCase) {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = w.Close(); _ = r.Close() })
	orig := NewHub("corpus", r, io.Discard, c.ring, nil)
	ref := NewHub("corpus", nil, nil, c.ring, nil)
	resizeHub(t, orig, c.cols, c.rows)
	resizeHub(t, ref, c.cols, c.rows)
	recordReads(c.pre, orig, ref)
	freezeIdle(t, orig)
	got, _, _ := importedHub(t, orig)

	requireSameDeltas(t, ref, got)
	requireSameRender(t, ref, got, c.cols, c.rows)
	requireSameEmu(t, "after import", ref, got)
	for i, s := range c.cont {
		recordReads([]string{s}, ref, got)
		requireSameEmu(t, fmt.Sprintf("after continuation read %d", i), ref, got)
	}
	requireSameRender(t, ref, got, c.cols, c.rows)
}

// TestHandoffState_RoundTripCorpus (T-R2): sessions in a shell and inside a
// full-screen program, exported mid-stream, render, place the cursor, replay
// history and deltas like a reference hub fed everything, also after the
// app's exit (1049l) fires the alt-exit rebuild on the imported hub.
func TestHandoffState_RoundTripCorpus(t *testing.T) {
	for _, c := range handoffCorpus() {
		t.Run(c.name, func(t *testing.T) { runHandoffCase(t, c) })
	}
}

// TestHandoffState_ExportReseedsSelf (Pitfall 7): Export's probes and its
// ?1049l touch the old emulator, so Export rebuilds it from its own export:
// the old hub then equals the imported one, and a rollback (Thaw) resumes on
// it.
func TestHandoffState_ExportReseedsSelf(t *testing.T) {
	for _, c := range handoffCorpus() {
		t.Run(c.name, func(t *testing.T) {
			r, w, err := os.Pipe()
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = w.Close(); _ = r.Close() })
			orig := NewHub("reseed", r, io.Discard, c.ring, nil)
			resizeHub(t, orig, c.cols, c.rows)
			recordReads(c.pre, orig)
			freezeIdle(t, orig)
			got, _, _ := importedHub(t, orig)
			requireSameEmu(t, "old hub after Export", got, orig)
			if orig.Digest() != got.Digest() {
				t.Fatal("old hub's digest differs from the imported hub's")
			}

			orig.Thaw()
			before := orig.scrollback.End()
			if _, err := io.WriteString(w, "rolled-back"); err != nil {
				t.Fatal(err)
			}
			testutil.WaitFor(t, 2*time.Second, func() bool { return orig.scrollback.End() > before },
				"thawed hub never read after Export")
		})
	}
}

// TestHandoffState_DigestDetectsDifference: a state with one ring byte
// changed still imports, but its hub's digest differs.
func TestHandoffState_DigestDetectsDifference(t *testing.T) {
	st := validState(t)
	a, err := NewHubFromState(st, nil, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	bad := st
	bad.Ring.Buf = bytes.Clone(st.Ring.Buf)
	bad.Ring.Buf[len(bad.Ring.Buf)/2] ^= 0x20
	b, err := NewHubFromState(bad, nil, nil, nil)
	if err != nil {
		t.Fatalf("changed ring byte: %v", err)
	}
	if a.Digest() == b.Digest() {
		t.Fatal("a changed ring byte kept the same digest")
	}
	again, err := NewHubFromState(st, nil, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if a.Digest() != again.Digest() {
		t.Fatal("the same state gave two digests")
	}
}

// truecolorRow is cols cells, each with its own truecolor fg and bg.
func truecolorRow(cols, seed int) string {
	var b strings.Builder
	for x := 0; x < cols; x++ {
		v := (x*7 + seed*13) % 256
		fmt.Fprintf(&b, "\x1b[38;2;%d;%d;%dm\x1b[48;2;%d;%d;%dm%c", v, 255-v, (v*3)%256, (v*5)%256, v, 255-v, 'a'+rune(x%26))
	}
	return b.String()
}

// TestHandoffState_FitsSessionFrameCap (WR-02, the review's measured case):
// a 400x120 hub with truecolor output, a full styled history and a
// full-screen app exports to a gob that fits HandoffStateMaxBytes with
// margin, so one heavy session does not abort the whole handoff.
func TestHandoffState_FitsSessionFrameCap(t *testing.T) {
	const cols, rows = 400, 120
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = w.Close(); _ = r.Close() })
	h := NewHub("cap", r, io.Discard, DefaultScrollbackBytes, nil)
	resizeHub(t, h, cols, rows)
	var s strings.Builder
	for i := 0; i < 600; i++ { // scrolls into history
		s.WriteString(truecolorRow(cols, i) + "\x1b[m\r\n")
	}
	s.WriteString("\x1b[?1049h\x1b[H")
	for i := 0; i < rows; i++ {
		fmt.Fprintf(&s, "\x1b[%d;1H%s", i+1, truecolorRow(cols, i+1000))
	}
	feedHubs(s.String(), 16<<10, h)
	freezeIdle(t, h)
	st, err := h.Export()
	if err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	if err := gob.NewEncoder(&buf).Encode(st); err != nil {
		t.Fatal(err)
	}
	hb := 0
	for _, l := range st.History.Lines {
		hb += len(l)
	}
	t.Logf("ring=%d history=%d (%d lines) seed=%d gob=%d cap=%d", len(st.Ring.Buf), hb, len(st.History.Lines), len(st.Seed), buf.Len(), HandoffStateMaxBytes)
	if hb < HistoryMaxBytes*3/4 || !st.HasEmu || !strings.Contains(string(st.Seed), "\x1b[?1047h") {
		t.Fatalf("setup: history %d bytes, seed %d bytes; want a near-full history inside a full-screen app", hb, len(st.Seed))
	}
	if 2*buf.Len() > HandoffStateMaxBytes {
		t.Errorf("gob of %d bytes, want at most half of HandoffStateMaxBytes (%d)", buf.Len(), HandoffStateMaxBytes)
	}
}
