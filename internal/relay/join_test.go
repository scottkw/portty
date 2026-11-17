package relay

import (
	"bytes"
	"fmt"
	"io"
	"math/rand"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/scottkw/portty/internal/testutil"
)

// fullReplay Joins a throwaway subscriber with no resume, unsubscribes it and
// returns the concatenated replay chunk payloads: what a fresh viewer replays.
func fullReplay(t *testing.T, hub *Hub) []byte {
	t.Helper()
	r := join(t, hub, 0, false)
	if r.Mode != ReplayFull {
		t.Fatalf("Join without resume: mode %q, want %q", r.Mode, ReplayFull)
	}
	return payloads(r)
}

// join Joins a throwaway subscriber and unsubscribes it again.
func join(t *testing.T, hub *Hub, resume uint64, hasResume bool) Replay {
	t.Helper()
	sub := &Subscriber{Msgs: make(chan []byte, 256), CloseSlow: func() {}}
	r := hub.Join(sub, resume, hasResume)
	hub.Unsubscribe(sub)
	return r
}

func payloads(r Replay) []byte {
	var out []byte
	for _, c := range r.Chunks {
		out = append(out, c[1:]...)
	}
	return out
}

// runJoinHub starts a hub whose Run drains a pipe; the returned writer is the PTY.
func runJoinHub(t *testing.T, scrollbackBytes int) (*Hub, *io.PipeWriter) {
	t.Helper()
	r, w := io.Pipe()
	hub := NewHub("join-test", r, w, scrollbackBytes, nil)
	done := make(chan struct{})
	go func() { hub.Run(); close(done) }()
	t.Cleanup(func() { _ = w.Close(); <-done })
	return hub, w
}

// emit writes one PTY chunk (one frame: io.Pipe hands each Write to one Read)
// and waits until the hub has recorded it.
func emit(t *testing.T, hub *Hub, w io.Writer, s string) {
	t.Helper()
	want := hub.scrollback.End() + uint64(1+len(s))
	if _, err := io.WriteString(w, s); err != nil {
		t.Fatalf("pty write: %v", err)
	}
	testutil.WaitFor(t, 2*time.Second, func() bool { return hub.scrollback.End() >= want },
		"hub never recorded %q", s)
}

func TestJoin_DeltaWhenResumeInRing(t *testing.T) {
	hub, w := runJoinHub(t, DefaultScrollbackBytes)
	emit(t, hub, w, "already-seen\r\n")
	off := join(t, hub, 0, false).End
	emit(t, hub, w, "missed-1\r\n")
	emit(t, hub, w, "missed-2\r\n")

	r := join(t, hub, off, true)
	if r.Mode != ReplayDelta {
		t.Fatalf("mode %q, want delta", r.Mode)
	}
	want := append(MakeOutputFrame([]byte("missed-1\r\n")), MakeOutputFrame([]byte("missed-2\r\n"))...)
	if got := payloads(r); !bytes.Equal(got, want) {
		t.Fatalf("delta payload %q, want %q", got, want)
	}
	if r.End != hub.scrollback.End() {
		t.Fatalf("End %d, want ring end %d", r.End, hub.scrollback.End())
	}
}

func TestJoin_DeltaAtEndIsEmpty(t *testing.T) {
	hub, w := runJoinHub(t, DefaultScrollbackBytes)
	emit(t, hub, w, "hello\r\n")
	off := join(t, hub, 0, false).End
	r := join(t, hub, off, true)
	if r.Mode != ReplayDelta || len(r.Chunks) != 0 || r.End != off {
		t.Fatalf("got mode=%q chunks=%d end=%d, want delta with no chunks at %d", r.Mode, len(r.Chunks), r.End, off)
	}
}

func TestJoin_FullWhenResumeOutOfRange(t *testing.T) {
	t.Run("no_resume", func(t *testing.T) {
		hub, w := runJoinHub(t, DefaultScrollbackBytes)
		emit(t, hub, w, "hello\r\n")
		if r := join(t, hub, 1, false); r.Mode != ReplayFull {
			t.Fatalf("mode %q, want full", r.Mode)
		}
	})
	t.Run("older_than_ring", func(t *testing.T) {
		hub, w := runJoinHub(t, 64)
		for i := 0; i < 10; i++ {
			emit(t, hub, w, "0123456789abcdef\r\n")
		}
		if !hub.scrollback.Truncated() {
			t.Fatal("fixture did not wrap the ring")
		}
		if r := join(t, hub, 0, true); r.Mode != ReplayFull {
			t.Fatalf("mode %q, want full", r.Mode)
		}
	})
	t.Run("past_end", func(t *testing.T) {
		hub, w := runJoinHub(t, DefaultScrollbackBytes)
		emit(t, hub, w, "hello\r\n")
		if r := join(t, hub, hub.scrollback.End()+1, true); r.Mode != ReplayFull {
			t.Fatalf("mode %q, want full", r.Mode)
		}
	})
	t.Run("new_session", func(t *testing.T) {
		hub, _ := runJoinHub(t, DefaultScrollbackBytes)
		if r := join(t, hub, 0, false); r.Mode != ReplayFull || len(r.Chunks) != 0 || r.End != 0 {
			t.Fatalf("got mode=%q chunks=%d end=%d, want full with no chunks at 0", r.Mode, len(r.Chunks), r.End)
		}
	})
}

// TestJoin_DeltaStripsTerminalQueries: a delta is query-stripped like the full
// replay, so a rejoining terminal never answers stale queries into the PTY
// (T-05-02).
func TestJoin_DeltaStripsTerminalQueries(t *testing.T) {
	hub, w := runJoinHub(t, DefaultScrollbackBytes)
	emit(t, hub, w, "before\r\n")
	off := join(t, hub, 0, false).End
	emit(t, hub, w, "a\x1b[6nb\x1b[cc")
	r := join(t, hub, off, true)
	if got, want := payloads(r), MakeOutputFrame([]byte("abc")); r.Mode != ReplayDelta || !bytes.Equal(got, want) {
		t.Fatalf("mode %q payload %q, want delta %q", r.Mode, got, want)
	}
}

func TestJoin_ChunksAtMostReplayChunk(t *testing.T) {
	hub, w := runJoinHub(t, DefaultScrollbackBytes)
	line := strings.Repeat("y", 4095) + "\n"
	for i := 0; i < 25; i++ { // about 100 KiB
		emit(t, hub, w, line)
	}
	r := join(t, hub, 0, false)
	if len(r.Chunks) < 2 {
		t.Fatalf("%d chunks, want several", len(r.Chunks))
	}
	for i, c := range r.Chunks {
		if c[0] != MsgOutput || len(c)-1 > ReplayChunk {
			t.Fatalf("chunk %d: type 0x%02x payload %d bytes, want MsgOutput <= %d", i, c[0], len(c)-1, ReplayChunk)
		}
	}
	if got := payloads(r); !bytes.Equal(got, hub.ScrollbackSnapshot()) {
		t.Fatalf("chunks concatenate to %d bytes, want the %d-byte ring", len(got), len(hub.ScrollbackSnapshot()))
	}
}

// TestJoin_ExactlyOnceUnderConcurrentOutput: viewers that join while output
// streams see every line exactly once across replay and live frames (SHAR-07).
func TestJoin_ExactlyOnceUnderConcurrentOutput(t *testing.T) {
	const lines = 4000 // 13 frame bytes each: stays inside the 256 KiB ring
	hub, w := runJoinHub(t, DefaultScrollbackBytes)

	go func() {
		for i := 0; i < lines; i++ {
			if _, err := fmt.Fprintf(w, "LINE-%06d\n", i); err != nil {
				return
			}
		}
		_, _ = io.WriteString(w, "STOP\n")
	}()

	var wg sync.WaitGroup
	for v := 0; v < 20; v++ {
		wg.Add(1)
		threshold := uint64(rand.Intn(lines*13*9/10))
		go func(v int) {
			defer wg.Done()
			for hub.scrollback.End() < threshold {
				time.Sleep(time.Millisecond)
			}
			sub := &Subscriber{Msgs: make(chan []byte, lines+16)}
			sub.CloseSlow = func() { t.Errorf("viewer %d kicked as slow", v) }
			r := hub.Join(sub, 0, false)
			defer hub.Unsubscribe(sub)
			stream := payloads(r)
			for !bytes.Contains(stream, []byte("STOP\n")) {
				select {
				case f := <-sub.Msgs:
					if f[0] == MsgOutput {
						stream = append(stream, f[1:]...)
					}
				case <-time.After(20 * time.Second):
					t.Errorf("viewer %d: no STOP", v)
					return
				}
			}
			checkConsecutive(t, v, stream)
		}(v)
	}
	wg.Wait()
}

// checkConsecutive asserts the LINE numbers in stream (0x01 frame headers
// removed) run strictly consecutively up to STOP.
func checkConsecutive(t *testing.T, v int, stream []byte) {
	t.Helper()
	text := string(bytes.ReplaceAll(stream, []byte{MsgOutput}, nil))
	prev := -1
	for _, l := range strings.Split(text, "\n") {
		if l == "STOP" {
			return
		}
		if l == "" {
			continue
		}
		n, err := strconv.Atoi(strings.TrimPrefix(l, "LINE-"))
		if err != nil {
			t.Errorf("viewer %d: bad line %q", v, l)
			return
		}
		if prev >= 0 && n != prev+1 {
			t.Errorf("viewer %d: line %d after %d (duplicate or gap)", v, n, prev)
			return
		}
		prev = n
	}
	t.Errorf("viewer %d: STOP not found", v)
}

func TestParseResume(t *testing.T) {
	for _, tc := range []struct {
		in   string
		want uint64
		ok   bool
	}{
		{"", 0, false},
		{"abc", 0, false},
		{"-1", 0, false},
		{"18446744073709551616", 0, false},
		{"12", 12, true},
	} {
		got, ok := ParseResume(tc.in)
		if ok != tc.ok || (ok && got != tc.want) {
			t.Errorf("ParseResume(%q) = %d, %t; want %d, %t", tc.in, got, ok, tc.want, tc.ok)
		}
	}
}
