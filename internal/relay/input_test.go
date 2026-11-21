package relay

import (
	"bytes"
	"errors"
	"io"
	"testing"
	"time"

	"github.com/scottkw/portty/internal/testutil"
)

// inputHub is a hub whose PTY writer is an io.Pipe: the writer blocks until
// the test reads, so not reading is a PTY that is not draining its input.
func inputHub(t *testing.T) (*Hub, *io.PipeReader) {
	t.Helper()
	r, w := io.Pipe()
	h := NewHub("input-test", bytes.NewReader(nil), w, 1024, nil)
	t.Cleanup(func() {
		h.Shutdown()
		_ = r.Close() // frees a writer goroutine still blocked in the pipe
	})
	return h, r
}

// returnsWithin reports whether f returns within d, leaving it running if not.
func returnsWithin(d time.Duration, f func() error) (<-chan error, bool) {
	ch := make(chan error, 1)
	go func() { ch <- f() }()
	select {
	case err := <-ch:
		ch <- err
		return ch, true
	case <-time.After(d):
		return ch, false
	}
}

// fillToQuota writes InputMaxFrames one-byte frames on c (the first is in
// flight in the blocked writer, the rest queued; both count).
func fillToQuota(t *testing.T, c *InputConn) {
	t.Helper()
	for i := 0; i < InputMaxFrames; i++ {
		if err := c.Write([]byte{'q'}); err != nil {
			t.Fatalf("write %d: %v", i, err)
		}
	}
}

// TestHubInput_FIFOAcrossConnections: frames from two connections reach the
// PTY in exact arrival order (T-10-07: a dropped viewer's backlog cannot
// interleave with a reconnect's input).
func TestHubInput_FIFOAcrossConnections(t *testing.T) {
	h, r := inputHub(t)
	a, b := h.OpenInput(nil), h.OpenInput(nil)
	var want []byte
	for i := 0; i < 200; i++ {
		fa := []byte{'A', byte('a' + i%26)}
		fb := []byte{'B', byte('0' + i%10)}
		if err := a.Write(fa); err != nil {
			t.Fatal(err)
		}
		if err := b.Write(fb); err != nil {
			t.Fatal(err)
		}
		want = append(want, fa...)
		want = append(want, fb...)
	}
	got := make([]byte, len(want))
	if _, err := io.ReadFull(r, got); err != nil {
		t.Fatalf("read pipe: %v", err)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("PTY got %q, want %q", got, want)
	}
}

// TestHubInput_QuotaBlocksOnlyThatConn: a connection at its frame quota blocks
// on its next Write; another connection on the same hub does not (T-10-05).
func TestHubInput_QuotaBlocksOnlyThatConn(t *testing.T) {
	h, r := inputHub(t)
	a, b := h.OpenInput(nil), h.OpenInput(nil)
	fillToQuota(t, a)

	blocked, returned := returnsWithin(100*time.Millisecond, func() error { return a.Write([]byte{'x'}) })
	if returned {
		t.Fatalf("Write at quota returned (%v), want it to block", <-blocked)
	}
	if _, ok := returnsWithin(time.Second, func() error { return b.Write([]byte{'y'}) }); !ok {
		t.Fatal("another connection's Write blocked behind a full connection")
	}

	// Draining the PTY frees a slot: the blocked Write returns.
	go func() { _, _ = io.Copy(io.Discard, r) }()
	select {
	case err := <-blocked:
		if err != nil {
			t.Fatalf("Write after drain: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Write still blocked after the PTY drained")
	}
}

// fillHub fills h to InputHubMaxBytes: two connections at their frame quota
// of full-size frames, topped up exactly by a third.
func fillHub(t *testing.T, h *Hub) {
	t.Helper()
	frame := bytes.Repeat([]byte{'p'}, MaxInputPayload) // read-only, so shared
	for _, c := range []*InputConn{h.OpenInput(nil), h.OpenInput(nil)} {
		for i := 0; i < InputMaxFrames; i++ {
			if err := c.Write(frame); err != nil {
				t.Fatal(err)
			}
		}
	}
	h.inMu.Lock()
	rest := InputHubMaxBytes - h.inBytes
	h.inMu.Unlock()
	if err := h.OpenInput(nil).Write(bytes.Repeat([]byte{'p'}, rest)); err != nil {
		t.Fatal(err)
	}
}

// TestHubInput_HubCapBlocks (WR-01): with the hub full, a connection gets in
// only its InputConnHubAllowance (two full-size frames), then blocks far
// below its own quota until the PTY drains.
func TestHubInput_HubCapBlocks(t *testing.T) {
	h, r := inputHub(t)
	fillHub(t, h)
	frame := bytes.Repeat([]byte{'p'}, MaxInputPayload)
	c := h.OpenInput(nil)
	for i := 0; i < 2; i++ {
		if _, ok := returnsWithin(time.Second, func() error { return c.Write(frame) }); !ok {
			t.Fatalf("frame %d within the allowance blocked on the hub cap", i)
		}
	}
	blocked, returned := returnsWithin(100*time.Millisecond, func() error { return c.Write(frame) })
	if returned {
		t.Fatalf("Write over the hub cap returned (%v), want it to block", <-blocked)
	}
	go func() { _, _ = io.Copy(io.Discard, r) }()
	select {
	case err := <-blocked:
		if err != nil {
			t.Fatalf("Write after drain: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Write still blocked after the PTY drained")
	}
}

// TestHubInput_HubCapAllowsTyping (WR-05): another connection's backlog
// holds the hub at its cap; a viewer that is only typing keeps queueing
// keystrokes without blocking, so its pump keeps reading and answering pings.
func TestHubInput_HubCapAllowsTyping(t *testing.T) {
	h, _ := inputHub(t)
	fillHub(t, h)
	c := h.OpenInput(nil)
	if _, ok := returnsWithin(2*time.Second, func() error {
		for i := 0; i < 100; i++ {
			if err := c.Write([]byte{'k'}); err != nil {
				return err
			}
		}
		return nil
	}); !ok {
		t.Fatal("small writes blocked behind another connection's backlog")
	}
}

// gateWriter blocks every Write until release is closed, then fails.
type gateWriter struct{ release chan struct{} }

func (g *gateWriter) Write(p []byte) (int, error) {
	<-g.release
	return 0, errors.New("pty gone")
}

// TestHubInput_WriterDeathUnblocks: a failing PTY write, and separately a hub
// shutdown, wake a pump blocked at quota with ErrInputClosed, and every later
// Write gets it too (T-10-06).
func TestHubInput_WriterDeathUnblocks(t *testing.T) {
	t.Run("write error", func(t *testing.T) {
		g := &gateWriter{release: make(chan struct{})}
		h := NewHub("input-err", bytes.NewReader(nil), g, 1024, nil)
		t.Cleanup(h.Shutdown)
		c := h.OpenInput(nil)
		fillToQuota(t, c)
		blocked, returned := returnsWithin(50*time.Millisecond, func() error { return c.Write([]byte{'x'}) })
		if returned {
			t.Fatalf("Write at quota returned early: %v", <-blocked)
		}
		close(g.release)
		select {
		case err := <-blocked:
			if !errors.Is(err, ErrInputClosed) {
				t.Fatalf("blocked Write = %v, want ErrInputClosed", err)
			}
		case <-time.After(2 * time.Second):
			t.Fatal("blocked Write never woke after the writer failed")
		}
		if err := h.OpenInput(nil).Write([]byte{'z'}); !errors.Is(err, ErrInputClosed) {
			t.Fatalf("later Write = %v, want ErrInputClosed", err)
		}
	})
	t.Run("hub shutdown", func(t *testing.T) {
		h, _ := inputHub(t)
		c := h.OpenInput(nil)
		fillToQuota(t, c)
		blocked, returned := returnsWithin(50*time.Millisecond, func() error { return c.Write([]byte{'x'}) })
		if returned {
			t.Fatalf("Write at quota returned early: %v", <-blocked)
		}
		h.Shutdown()
		select {
		case err := <-blocked:
			if !errors.Is(err, ErrInputClosed) {
				t.Fatalf("blocked Write = %v, want ErrInputClosed", err)
			}
		case <-time.After(2 * time.Second):
			t.Fatal("blocked Write never woke after Shutdown")
		}
		if err := c.Write([]byte{'z'}); !errors.Is(err, ErrInputClosed) {
			t.Fatalf("later Write = %v, want ErrInputClosed", err)
		}
	})
}

// TestHubInput_CloseKeepsBacklog: a closed connection's queued frames still
// reach the PTY (nothing already read is lost), and WaitInputIdle reports the
// backlog until it drains.
func TestHubInput_CloseKeepsBacklog(t *testing.T) {
	h, r := inputHub(t)
	c := h.OpenInput(nil)
	want := []byte("first|second|third")
	for _, f := range bytes.SplitAfter(want, []byte("|")) {
		if err := c.Write(f); err != nil {
			t.Fatal(err)
		}
	}
	c.Close()

	if idle, queued := h.WaitInputIdle(50 * time.Millisecond); idle || queued != len(want) {
		t.Fatalf("WaitInputIdle with a blocked PTY = (%t, %d), want (false, %d)", idle, queued, len(want))
	}
	got := make([]byte, len(want))
	if _, err := io.ReadFull(r, got); err != nil {
		t.Fatalf("read pipe: %v", err)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("PTY got %q, want %q", got, want)
	}
	testutil.WaitFor(t, 2*time.Second, func() bool {
		idle, queued := h.WaitInputIdle(0)
		return idle && queued == 0
	}, "input never went idle after the backlog drained")
}

// TestHubInput_RevokeDropsBacklog (CR-01): Stop Sharing drops a web viewer's
// queued input, so after the revoke only the frame already in the PTY write
// lands; frames its pump queues later are dropped too. A local viewer's
// queued input on the same hub is still written, in order.
func TestHubInput_RevokeDropsBacklog(t *testing.T) {
	h, r := inputHub(t)
	web := &Subscriber{Msgs: make(chan []byte, 8), Origin: "web", Kick: func(string) {}}
	local := &Subscriber{Msgs: make(chan []byte, 8), Origin: "local"}
	h.Subscribe(web)
	h.Subscribe(local)
	wc, lc := h.OpenInput(web), h.OpenInput(local)

	if err := wc.Write([]byte("first|")); err != nil {
		t.Fatal(err)
	}
	testutil.WaitFor(t, 2*time.Second, func() bool { // the writer popped it: in flight
		h.inMu.Lock()
		defer h.inMu.Unlock()
		return h.inHead == len(h.inq)
	}, "the writer never took the first frame")
	for _, w := range []struct {
		c *InputConn
		b string
	}{{wc, "w2|"}, {lc, "local|"}, {wc, "w3|"}} {
		if err := w.c.Write([]byte(w.b)); err != nil {
			t.Fatal(err)
		}
	}

	h.RevokeWebViewers(CloseReasonShareStopped)
	if err := wc.Write([]byte("after|")); err != nil {
		t.Fatalf("Write after revoke = %v, want nil (dropped)", err)
	}

	want := []byte("first|local|")
	got := make([]byte, len(want))
	if _, err := io.ReadFull(r, got); err != nil {
		t.Fatalf("read pipe: %v", err)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("PTY got %q, want %q", got, want)
	}
	testutil.WaitFor(t, 2*time.Second, func() bool {
		idle, queued := h.WaitInputIdle(0)
		return idle && queued == 0
	}, "revoked input still queued")
}

// waitInFlight waits until the writer has popped every queued frame (the
// last one is then in the blocked PTY write).
func waitInFlight(t *testing.T, h *Hub) {
	t.Helper()
	testutil.WaitFor(t, 2*time.Second, func() bool {
		h.inMu.Lock()
		defer h.inMu.Unlock()
		return h.inHead == len(h.inq)
	}, "the writer never took the queued frame")
}

// TestHubInput_RevokeDropsDepartedBacklog (CR-02): a web viewer that closed
// its tab (pump closed, unsubscribed) with input queued is no longer a
// revoke target, but Stop Sharing still drops its backlog. A local viewer's
// input is still written, and a web viewer that joins after the revoke (a
// new share) types normally.
func TestHubInput_RevokeDropsDepartedBacklog(t *testing.T) {
	h, r := inputHub(t)
	web := &Subscriber{Msgs: make(chan []byte, 8), Origin: "web", Kick: func(string) {}}
	local := &Subscriber{Msgs: make(chan []byte, 8), Origin: "local"}
	h.Subscribe(web)
	h.Subscribe(local)
	wc, lc := h.OpenInput(web), h.OpenInput(local)
	if err := wc.Write([]byte("first|")); err != nil {
		t.Fatal(err)
	}
	waitInFlight(t, h)
	for _, w := range []struct {
		c *InputConn
		b string
	}{{wc, "w2|"}, {lc, "local|"}, {wc, "w3|"}} {
		if err := w.c.Write([]byte(w.b)); err != nil {
			t.Fatal(err)
		}
	}
	wc.Close()
	h.Unsubscribe(web)

	h.RevokeWebViewers(CloseReasonShareStopped)

	next := &Subscriber{Msgs: make(chan []byte, 8), Origin: "web", Kick: func(string) {}}
	h.Subscribe(next)
	if err := h.OpenInput(next).Write([]byte("new|")); err != nil {
		t.Fatal(err)
	}
	want := []byte("first|local|new|")
	got := make([]byte, len(want))
	if _, err := io.ReadFull(r, got); err != nil {
		t.Fatalf("read pipe: %v", err)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("PTY got %q, want %q", got, want)
	}
}

// TestHubInput_RevokeDropsBlockedPumpAfterUnsubscribe (CR-02): the write pump
// ended and the handler unsubscribed while the read pump is still blocked in
// Write at quota. The revoke wakes it with the frame dropped, and its queued
// frames go too: after the in-flight frame the PTY gets only local input.
func TestHubInput_RevokeDropsBlockedPumpAfterUnsubscribe(t *testing.T) {
	h, r := inputHub(t)
	web := &Subscriber{Msgs: make(chan []byte, 8), Origin: "web", Kick: func(string) {}}
	h.Subscribe(web)
	wc := h.OpenInput(web)
	fillToQuota(t, wc)
	blocked, returned := returnsWithin(100*time.Millisecond, func() error { return wc.Write([]byte{'x'}) })
	if returned {
		t.Fatalf("Write at quota returned (%v), want it to block", <-blocked)
	}
	h.Unsubscribe(web)

	h.RevokeWebViewers(CloseReasonShareStopped)
	select {
	case err := <-blocked:
		if err != nil {
			t.Fatalf("blocked Write after revoke = %v, want nil (dropped)", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("blocked Write never woke on the revoke")
	}
	if err := h.OpenInput(nil).Write([]byte("L")); err != nil {
		t.Fatal(err)
	}
	want := []byte("qL") // the in-flight frame, then the local one
	got := make([]byte, len(want))
	if _, err := io.ReadFull(r, got); err != nil {
		t.Fatalf("read pipe: %v", err)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("PTY got %q, want %q", got, want)
	}
}
