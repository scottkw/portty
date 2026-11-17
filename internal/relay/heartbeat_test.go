package relay

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"
)

// heartbeatPair runs Heartbeat on a server conn that has a reading goroutine
// and returns the client conn, Heartbeat's result channel and its cancel.
func heartbeatPair(t *testing.T, every, timeout time.Duration) (*websocket.Conn, <-chan bool, chan struct{}, context.CancelFunc) {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	result := make(chan bool, 1)
	readEnded := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, err := websocket.Accept(w, r, nil)
		if err != nil {
			return
		}
		defer c.CloseNow()
		go func() {
			defer close(readEnded)
			for {
				if _, _, err := c.Read(context.Background()); err != nil {
					return
				}
			}
		}()
		result <- Heartbeat(ctx, c, every, timeout)
	}))
	t.Cleanup(srv.Close)
	client, _, err := websocket.Dial(context.Background(), "ws"+strings.TrimPrefix(srv.URL, "http"), nil)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	t.Cleanup(func() { client.CloseNow() })
	return client, result, readEnded, cancel
}

// TestHeartbeat_ClosesSilentPeer: a client that never reads never answers a
// ping, so Heartbeat closes the server conn and reports it.
func TestHeartbeat_ClosesSilentPeer(t *testing.T) {
	_, result, readEnded, _ := heartbeatPair(t, 20*time.Millisecond, 50*time.Millisecond)
	select {
	case closed := <-result:
		if !closed {
			t.Fatal("Heartbeat returned false for a silent peer")
		}
	case <-time.After(time.Second):
		t.Fatal("Heartbeat did not close a silent peer within 1s")
	}
	select {
	case <-readEnded: // the server conn is closed, so its reader ended
	case <-time.After(time.Second):
		t.Fatal("server conn still open after Heartbeat returned")
	}
}

// TestHeartbeat_KeepsLivePeer: a reading client answers every ping; Heartbeat
// keeps running and returns false once its context ends.
func TestHeartbeat_KeepsLivePeer(t *testing.T) {
	client, result, _, cancel := heartbeatPair(t, 20*time.Millisecond, 50*time.Millisecond)
	go func() {
		for {
			if _, _, err := client.Read(context.Background()); err != nil {
				return
			}
		}
	}()
	time.Sleep(200 * time.Millisecond) // 10 intervals
	select {
	case <-result:
		t.Fatal("Heartbeat ended while the peer was answering pings")
	default:
	}
	cancel()
	select {
	case closed := <-result:
		if closed {
			t.Fatal("Heartbeat returned true on ctx cancel")
		}
	case <-time.After(time.Second):
		t.Fatal("Heartbeat did not return after ctx cancel")
	}
}
