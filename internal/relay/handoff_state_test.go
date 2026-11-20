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
