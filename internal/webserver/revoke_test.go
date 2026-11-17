package webserver

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"io"
	"log"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/scottkw/portty/internal/relay"
)

// readUntilClose drains data frames until the connection closes and returns
// the close error.
func readUntilClose(t *testing.T, conn *websocket.Conn) websocket.CloseError {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	for {
		_, _, err := conn.Read(ctx)
		if err == nil {
			continue
		}
		var ce websocket.CloseError
		if !errors.As(err, &ce) {
			t.Fatalf("conn.Read = %v (%T), want a CloseError", err, err)
		}
		return ce
	}
}

func assertShareStopped(t *testing.T, ce websocket.CloseError) {
	t.Helper()
	if ce.Code != websocket.StatusNormalClosure || ce.Reason != relay.CloseReasonShareStopped {
		t.Fatalf("close = %d %q, want %d %q", ce.Code, ce.Reason, websocket.StatusNormalClosure, relay.CloseReasonShareStopped)
	}
}

// revokeShare does what the daemon's share-off does, in its order.
func revokeShare(t *testing.T, ws *WebServer, sessionID string) {
	t.Helper()
	ws.ClearGrants(sessionID)
	ws.DisableSession(sessionID)
	hub, ok := ws.manager.Get(sessionID)
	if !ok {
		t.Fatalf("hub %s not found", sessionID)
	}
	hub.RevokeWebViewers(relay.CloseReasonShareStopped)
}

func dialShared(t *testing.T, sessionID string) (*WebServer, *websocket.Conn, *io.PipeReader) {
	t.Helper()
	ws, client, _, input := testServerWithHub(t, sessionID)
	ws.SetSigningKey(capTestKey)
	token := issueCapFor(t, ws, sessionID, "read,write")
	hdr := http.Header{}
	hdr.Set("Origin", ws.BaseURL())
	conn := dialWebServerWS(t, client, ws.BaseURL(), "/sessions/"+sessionID+"/ws?cap="+token, hdr)
	// Wait until the server has subscribed the viewer.
	hub, _ := ws.manager.Get(sessionID)
	deadline := time.Now().Add(2 * time.Second)
	for hub.RemoteViewerCount() == 0 {
		if time.Now().After(deadline) {
			t.Fatal("viewer never subscribed")
		}
		time.Sleep(5 * time.Millisecond)
	}
	return ws, conn, input
}

func TestRevoke_ClosesWithShareStopped(t *testing.T) {
	ws, conn, _ := dialShared(t, "revoke-close")
	revokeShare(t, ws, "revoke-close")
	assertShareStopped(t, readUntilClose(t, conn))
}

func TestRevoke_InputAfterRevokeNeverReachesPTY(t *testing.T) {
	ws, conn, input := dialShared(t, "revoke-input")
	revokeShare(t, ws, "revoke-input")
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	// Writes after the kick fail or are dropped; none may reach the PTY.
	_ = conn.Write(ctx, websocket.MessageBinary, relay.MakeInputFrame([]byte("after-revoke\n")))

	got := make(chan []byte, 1)
	go func() {
		buf := make([]byte, 256)
		n, _ := input.Read(buf)
		got <- buf[:n]
	}()
	select {
	case b := <-got:
		if len(b) > 0 {
			t.Fatalf("input after revoke reached the PTY: %q", b)
		}
	case <-time.After(300 * time.Millisecond):
	}
}

func TestRevoke_SubscribeRaceClosed(t *testing.T) {
	const id = "revoke-race"
	ws, client, _, _ := testServerWithHub(t, id)
	ws.SetSigningKey(capTestKey)
	token := issueCapFor(t, ws, id, "read,write")
	// The revoke lands after the capability check and before Subscribe, so
	// RevokeWebViewers cannot see this viewer; the post-subscribe re-check must.
	ws.beforeSubscribeHookForTest = func() { revokeShare(t, ws, id) }
	hdr := http.Header{}
	hdr.Set("Origin", ws.BaseURL())
	conn := dialWebServerWS(t, client, ws.BaseURL(), "/sessions/"+id+"/ws?cap="+token, hdr)
	assertShareStopped(t, readUntilClose(t, conn))
}

func openPluginStream(t *testing.T, sessionID string) (*WebServer, *bufio.Reader) {
	t.Helper()
	ws, client := testServer(t)
	ws.SetSigningKey(capTestKey)
	ws.EnableSession(sessionID)
	ws.SetPluginSettingsProvider(func() []byte { return []byte(`{"webgl":true}`) })
	tok := issueCapFor(t, ws, sessionID, "read,write")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	t.Cleanup(cancel)
	req, _ := http.NewRequestWithContext(ctx, "GET", ws.BaseURL()+"/api/plugin-config/stream?cap="+tok, nil)
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("client.Do: %v", err)
	}
	t.Cleanup(func() { resp.Body.Close() })
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("stream status = %d", resp.StatusCode)
	}
	rdr := bufio.NewReader(resp.Body)
	for { // consume the initial frame
		line, err := rdr.ReadString('\n')
		if err != nil {
			t.Fatalf("initial frame: %v", err)
		}
		if line == "\n" {
			break
		}
	}
	return ws, rdr
}

// readToEOF returns what the stream sends before it ends, failing if it is
// still open after d.
func readToEOF(t *testing.T, rdr *bufio.Reader, d time.Duration) string {
	t.Helper()
	done := make(chan string, 1)
	go func() {
		b, _ := io.ReadAll(rdr)
		done <- string(b)
	}()
	select {
	case s := <-done:
		return s
	case <-time.After(d):
		t.Fatalf("stream still open %v after revoke", d)
	}
	return ""
}

func TestRevoke_PluginStreamEndsWithoutBroadcast(t *testing.T) {
	ws, rdr := openPluginStream(t, "revoke-sse")
	ws.ClearGrants("revoke-sse")
	if rest := readToEOF(t, rdr, time.Second); rest != "" {
		t.Fatalf("stream sent %q after revoke", rest)
	}
}

func TestRevoke_PluginStreamDropsFrameAfterRevoke(t *testing.T) {
	ws, rdr := openPluginStream(t, "revoke-sse-frame")
	// Clear the grant without firing the signal path's effect first: remove
	// the grant directly, then broadcast, so only the per-send re-check stands.
	ws.mu.Lock()
	delete(ws.grants, "revoke-sse-frame")
	ws.mu.Unlock()
	ws.BroadcastPluginConfig(context.Background())
	if rest := readToEOF(t, rdr, time.Second); strings.Contains(rest, "plugin-config") {
		t.Fatalf("a frame was delivered after revoke: %q", rest)
	}
}

type syncBuf struct {
	mu sync.Mutex
	b  bytes.Buffer
}

func (s *syncBuf) Write(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.b.Write(p)
}

func (s *syncBuf) String() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.b.String()
}

// CR-01: a link holder flooding resize frames gets one "web resize ignored"
// line per connection, with the total in the input total line.
func TestResizeFlood_LogsOncePerConnection(t *testing.T) {
	diag := &syncBuf{}
	prev := log.Writer()
	log.SetOutput(diag)
	t.Cleanup(func() { log.SetOutput(prev) })

	_, conn, _ := dialShared(t, "resize-flood")
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	const n = 50
	for i := 0; i < n; i++ {
		if err := conn.Write(ctx, websocket.MessageBinary, []byte{relay.MsgResize2, 0, 80, 0, 24}); err != nil { // the CLI's client resize frame
			t.Fatalf("write resize %d: %v", i, err)
		}
	}
	conn.Close(websocket.StatusNormalClosure, "")

	deadline := time.Now().Add(2 * time.Second)
	for !strings.Contains(diag.String(), "viewer-diag: input total") {
		if time.Now().After(deadline) {
			t.Fatalf("no input total line:\n%s", diag.String())
		}
		time.Sleep(5 * time.Millisecond)
	}
	out := diag.String()
	if c := strings.Count(out, "web resize ignored"); c != 1 {
		t.Errorf("resize ignored lines = %d; want 1:\n%s", c, out)
	}
	if !strings.Contains(out, "frames=0 bytes=0 resizes=50") {
		t.Errorf("input total line lacks resizes=%d:\n%s", n, out)
	}
}
