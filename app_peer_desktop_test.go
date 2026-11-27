package main

import (
	"net"
	"net/http"
	"strings"
	"testing"

	"github.com/scottkw/portty/internal/daemon"
)

// 21-07: the peer-desktop bindings, tested against a fake daemon on a unix
// socket in a temp dir. Nothing here opens a browser, System Settings or a
// tailnet connection.

// fakePeerDesktopDaemon serves the given routes plus GET /relay-port (4242) and
// returns an App wired to it. routes maps "METHOD /path" to a handler.
func fakePeerDesktopDaemon(t *testing.T, routes map[string]http.HandlerFunc) *App {
	t.Helper()
	sock := respawnHome(t)
	ln, err := net.Listen("unix", sock)
	if err != nil {
		t.Fatal(err)
	}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /relay-port", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"port":4242}`))
	})
	for pattern, h := range routes {
		mux.HandleFunc(pattern, h)
	}
	srv := &http.Server{Handler: mux}
	go func() { _ = srv.Serve(ln) }()
	t.Cleanup(func() { _ = srv.Close() })
	return &App{client: daemon.NewDaemonClient(sock)}
}

func replyJSON(status int, body string) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}
}

func TestConnectPeerDesktop_LoopbackURLHasNoToken(t *testing.T) {
	a := fakePeerDesktopDaemon(t, map[string]http.HandlerFunc{
		"POST /peers/desktop": replyJSON(200, `{"host":"office-air.example.ts.net","name":"office-air"}`),
	})
	got := a.ConnectPeerDesktop("OFFICE-AIR.example.ts.net.")
	want := PeerDesktopResult{
		Host: "office-air.example.ts.net",
		URL:  "ws://127.0.0.1:4242/api/relay/remote-desktop/office-air.example.ts.net/ws",
	}
	if got != want {
		t.Errorf("ConnectPeerDesktop = %+v, want %+v", got, want)
	}
	if strings.Contains(got.URL, "cap") || strings.Contains(got.URL, "?") {
		t.Errorf("url %q must carry no token", got.URL)
	}
}

func TestConnectPeerDesktop_DesktopOffKeepsURL(t *testing.T) {
	a := fakePeerDesktopDaemon(t, map[string]http.HandlerFunc{
		"POST /peers/desktop": replyJSON(409, `{"error":"desktop_off","detail":"office-air"}`),
	})
	got := a.ConnectPeerDesktop("Office-Air.example.ts.net")
	if got.ErrorCode != "desktop_off" {
		t.Errorf("errorCode = %q, want desktop_off", got.ErrorCode)
	}
	if got.URL != "ws://127.0.0.1:4242/api/relay/remote-desktop/office-air.example.ts.net/ws" {
		t.Errorf("url = %q, want it filled even on a daemon error", got.URL)
	}
}

func TestConnectPeerDesktop_Unreachable(t *testing.T) {
	t.Run("nil client", func(t *testing.T) {
		got := (&App{}).ConnectPeerDesktop("office-air")
		if got.ErrorCode != daemon.PeerErrUnreachable || got.URL != "" {
			t.Errorf("nil client = %+v, want unreachable and an empty url", got)
		}
	})
	t.Run("daemon not running", func(t *testing.T) {
		a := &App{client: daemon.NewDaemonClient(respawnHome(t))}
		got := a.ConnectPeerDesktop("office-air")
		if got.ErrorCode != daemon.PeerErrUnreachable || got.URL != "" {
			t.Errorf("dead daemon = %+v, want unreachable and an empty url", got)
		}
	})
}
