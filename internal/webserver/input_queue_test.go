package webserver

import (
	"bytes"
	"context"
	"crypto/sha256"
	"io"
	"log"
	"regexp"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/scottkw/portty/internal/relay"
	"github.com/scottkw/portty/internal/testutil"
)

const (
	pasteFrames    = 256
	pasteFrameSize = relay.MaxInputPayload // 32767: the client's split size
)

// pastePayload is 8 MiB of printable bytes (nothing the OSC absorber strips)
// that differ per frame, so a reorder or a dropped frame changes the hash.
func pastePayload() []byte {
	p := make([]byte, pasteFrames*pasteFrameSize)
	for i := range p {
		p[i] = byte('!' + (i/pasteFrameSize*7+i)%90)
	}
	return p
}

// joinPasteViewer joins a share viewer on a hub whose PTY input is a pipe the
// test controls, with a 50 ms / 100 ms heartbeat.
func joinPasteViewer(t *testing.T, sid string) (*websocket.Conn, *relay.Hub, *io.PipeReader) {
	t.Helper()
	ws, client, _, input := testServerWithHub(t, sid)
	ws.SetSigningKey(capTestKey)
	token := issueCapFor(t, ws, sid, "read,write")
	ws.heartbeatEvery, ws.heartbeatTimeout = 50*time.Millisecond, 100*time.Millisecond
	hub, _ := ws.manager.Get(sid)
	conn := dialResume(t, ws, client, sid, token, "")
	if typ, _ := readWSFrame(t, conn); typ != relay.MsgResize {
		t.Fatalf("first frame type 0x%02x, want grid frame", typ)
	}
	readReplay(t, conn)
	return conn, hub, input
}

// sendPaste writes the payload as MsgInput frames, the way relayClient splits
// a paste. It returns when every write returned (errors end it early).
func sendPaste(conn *websocket.Conn, payload []byte) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	for off := 0; off < len(payload); off += pasteFrameSize {
		frame := append([]byte{relay.MsgInput}, payload[off:off+pasteFrameSize]...)
		if err := conn.Write(ctx, websocket.MessageBinary, frame); err != nil {
			return
		}
	}
}

// pipeSink reads the PTY pipe into a buffer from the moment it is started.
type pipeSink struct {
	mu sync.Mutex
	b  []byte
}

func (s *pipeSink) drain(r io.Reader) {
	buf := make([]byte, 64<<10)
	for {
		n, err := r.Read(buf)
		s.mu.Lock()
		s.b = append(s.b, buf[:n]...)
		s.mu.Unlock()
		if err != nil {
			return
		}
	}
}

func (s *pipeSink) bytes() []byte {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]byte(nil), s.b...)
}

// TestShare_PasteBacklogKeepsViewer (INPT-01, SC1): a viewer that keeps
// reading pastes 8 MiB into a PTY that is not reading. The read pump queues the
// frames and keeps reading, so pings get answered and the heartbeat never drops
// the viewer; once the PTY drains, every byte arrives in order.
func TestShare_PasteBacklogKeepsViewer(t *testing.T) {
	conn, hub, input := joinPasteViewer(t, "paste-backlog")
	closed := make(chan struct{})
	go func() { // a live viewer: coder/websocket answers pings inside Read
		defer close(closed)
		for {
			if _, _, err := conn.Read(context.Background()); err != nil {
				return
			}
		}
	}()
	payload := pastePayload()
	go sendPaste(conn, payload)

	// Hold the PTY for 4x the 100 ms heartbeat timeout. A pump stuck on the PTY
	// stays subscribed, so the socket closing is the drop to watch for.
	for end := time.Now().Add(400 * time.Millisecond); time.Now().Before(end); time.Sleep(10 * time.Millisecond) {
		select {
		case <-closed:
			t.Fatal("viewer dropped while the paste is queued (heartbeat timeout)")
		default:
		}
		if n := hub.RemoteViewerCount(); n != 1 {
			t.Fatalf("viewer count %d while the paste is queued, want 1", n)
		}
	}

	got := make([]byte, len(payload))
	done := make(chan error, 1)
	go func() { _, err := io.ReadFull(input, got); done <- err }()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("read PTY: %v", err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("the paste never fully reached the PTY")
	}
	if sha256.Sum256(got) != sha256.Sum256(payload) {
		t.Fatal("PTY bytes differ from the paste (reordered or lost)")
	}
	if n := hub.RemoteViewerCount(); n != 1 {
		t.Fatalf("viewer count %d after the paste, want 1", n)
	}
}

// TestShare_DeadViewerReapedBacklogDelivered (INPT-01, SC1): a viewer that
// pastes and then goes silent (pings unanswered, as a dead network looks) is
// still reaped by the heartbeat while its backlog is queued. The backlog is not
// dropped: once the PTY drains it arrives whole for every frame the server
// read, in order, and the input goes idle.
func TestShare_DeadViewerReapedBacklogDelivered(t *testing.T) {
	diag := &syncBuf{}
	prev := log.Writer()
	log.SetOutput(diag)
	t.Cleanup(func() { log.SetOutput(prev) })

	conn, hub, input := joinPasteViewer(t, "paste-dead")
	payload := pastePayload()
	go sendPaste(conn, payload) // no read loop: pings go unanswered

	testutil.WaitFor(t, 3*time.Second, func() bool { return hub.RemoteViewerCount() == 0 },
		"silent viewer was never reaped")
	testutil.WaitFor(t, 2*time.Second, func() bool {
		return bytes.Contains([]byte(diag.String()), []byte("viewer-diag: heartbeat timeout"))
	}, "no heartbeat timeout logged:\n%s", diag.String())
	if idle, _ := hub.WaitInputIdle(0); idle {
		t.Fatal("input idle at the reap, want the backlog still queued behind the blocked PTY")
	}

	// The pump logs its frame count when it exits.
	totalRE := regexp.MustCompile(`viewer-diag: input total session=\S+ conn=\d+ frames=(\d+)`)
	var frames int
	testutil.WaitFor(t, 2*time.Second, func() bool {
		m := totalRE.FindStringSubmatch(diag.String())
		if m == nil {
			return false
		}
		frames, _ = strconv.Atoi(m[1])
		return true
	}, "the read pump never logged its input total")
	if frames == 0 {
		t.Fatal("the server read no frames before the reap")
	}

	sink := &pipeSink{}
	go sink.drain(input)
	testutil.WaitFor(t, 10*time.Second, func() bool {
		idle, _ := hub.WaitInputIdle(0)
		return idle && len(sink.bytes()) >= frames*pasteFrameSize
	}, "backlog never drained")
	got := sink.bytes()
	if len(got) != frames*pasteFrameSize {
		t.Fatalf("PTY got %d bytes, want %d (every frame the server read)", len(got), frames*pasteFrameSize)
	}
	if !bytes.Equal(got, payload[:len(got)]) {
		t.Fatal("PTY bytes are not an in-order prefix of the paste")
	}
}
