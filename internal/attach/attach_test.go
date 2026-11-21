package attach

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"syscall"
	"testing"
	"time"

	"github.com/coder/websocket"
)

func TestLockedWriter(t *testing.T) {
	var buf bytes.Buffer
	lw := NewLockedWriter(&buf)
	n, err := lw.Write([]byte("hello"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if n != 5 {
		t.Errorf("expected 5 bytes written, got %d", n)
	}
	if buf.String() != "hello" {
		t.Errorf("expected 'hello', got %q", buf.String())
	}
}

func TestMakeClientResizeFrame(t *testing.T) {
	frame := MakeClientResizeFrame(80, 24)
	if len(frame) != 5 {
		t.Fatalf("expected 5 bytes, got %d", len(frame))
	}
	// MsgResize2 = 0x11
	if frame[0] != 0x11 {
		t.Errorf("expected first byte 0x11 (MsgResize2), got 0x%02x", frame[0])
	}
	// 80 = 0x0050, 24 = 0x0018
	cols := uint16(frame[1])<<8 | uint16(frame[2])
	rows := uint16(frame[3])<<8 | uint16(frame[4])
	if cols != 80 {
		t.Errorf("expected cols=80, got %d", cols)
	}
	if rows != 24 {
		t.Errorf("expected rows=24, got %d", rows)
	}
}

// INPT-06: a claim is the report plus relay.ResizeClaim in byte 5.
func TestMakeClientClaimFrame(t *testing.T) {
	if got, want := MakeClientClaimFrame(80, 24), []byte{0x11, 0, 80, 0, 24, 0x01}; !bytes.Equal(got, want) {
		t.Fatalf("MakeClientClaimFrame(80, 24) = %v, want %v", got, want)
	}
}

// INPT-06: a SIGWINCH (the attach user resized the terminal) sends a claim
// with the new size; a size lookup error sends nothing; ctx cancel stops it.
func TestWatchResize_SendsClaim(t *testing.T) {
	frames := make(chan []byte, 8)
	readCtx, stopRead := context.WithCancel(context.Background())
	t.Cleanup(stopRead)
	f := newFakeRelay(t, func(f *fakeRelay, i int, c *websocket.Conn) {
		for {
			_, msg, err := c.Read(readCtx)
			if err != nil {
				return
			}
			frames <- msg
		}
	}, nil)
	conn := f.dial(t)

	type size struct {
		cols, rows int
		err        error
	}
	sizes := make(chan size, 4)
	winch := make(chan os.Signal, 1)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan struct{})
	go func() {
		watchResize(ctx, conn, winch, func() (int, int, error) { s := <-sizes; return s.cols, s.rows, s.err })
		close(done)
	}()

	next := func() []byte {
		t.Helper()
		select {
		case m := <-frames:
			return m
		case <-time.After(loopWait):
			t.Fatal("no frame from the resize watcher")
			return nil
		}
	}
	sizes <- size{cols: 90, rows: 30}
	winch <- syscall.SIGWINCH
	if got, want := next(), MakeClientClaimFrame(90, 30); !bytes.Equal(got, want) || len(got) != 6 {
		t.Fatalf("frame after SIGWINCH = %v, want the claim %v", got, want)
	}

	sizes <- size{err: errors.New("not a terminal")}
	winch <- syscall.SIGWINCH
	sizes <- size{cols: 70, rows: 20}
	winch <- syscall.SIGWINCH
	if got, want := next(), MakeClientClaimFrame(70, 20); !bytes.Equal(got, want) {
		t.Fatalf("frame after a size error and a good SIGWINCH = %v, want only the claim %v", got, want)
	}

	cancel()
	select {
	case <-done:
	case <-time.After(loopWait):
		t.Fatal("the resize watcher did not stop on ctx cancel")
	}
}

// Local attach: a paste over 32 KiB goes out in frames under the relay's
// 32768-byte read limit and arrives whole and in order (HARD-02).
func TestStdinPump_LargePasteArrivesWhole(t *testing.T) {
	f := newFakeRelay(t, func(f *fakeRelay, i int, c *websocket.Conn) {
		f.pump(i, c, nil) //nolint:errcheck
	}, nil)
	paste := pasteBytes(100_000)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = AttachSession(ctx, f.dial(t), bytes.NewReader(paste), io.Discard, 0x1C, nil, nil)
	waitFor(t, "the whole paste", func() bool { return len(f.input(0)) >= len(paste) })
	if got := f.input(0); got != string(paste) {
		t.Fatalf("relay got %d bytes; want the %d-byte paste in order", len(got), len(paste))
	}
}
