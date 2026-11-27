package main

import (
	"net"
	"net/http"
	"strings"
	"testing"

	"github.com/scottkw/portty/internal/daemon"
	"github.com/scottkw/portty/internal/tailnet"
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

func TestRemotePeersFromGroups(t *testing.T) {
	got := remotePeersFromGroups([]tailnet.PeerSessionMetaGroup{
		{Hostname: "office-air", DNSName: "office-air.example.ts.net", Reachable: true, Desktop: true},
		{Hostname: "home-air", DNSName: "home-air.example.ts.net", Reachable: true,
			Sessions: []tailnet.ShareableSessionMeta{{ID: "s1", Name: "main", Status: "running", URL: "https://x/s1"}}},
	})
	if len(got) != 2 {
		t.Fatalf("got %d peers, want 2", len(got))
	}
	if !got[0].Desktop || got[0].Hostname != "office-air" || got[0].DNSName != "office-air.example.ts.net" || !got[0].Reachable {
		t.Errorf("peer 0 = %+v, want desktop true with its names and reachable", got[0])
	}
	if got[0].Sessions == nil || len(got[0].Sessions) != 0 {
		t.Errorf("peer 0 sessions = %#v, want an empty non-nil slice", got[0].Sessions)
	}
	if got[1].Desktop || len(got[1].Sessions) != 1 || got[1].Sessions[0] != (RemoteSession{ID: "s1", Name: "main", Status: "running", URL: "https://x/s1"}) {
		t.Errorf("peer 1 = %+v, want desktop false and its one session", got[1])
	}
}

// captureOpened swaps the openURLFunc seam on a and records what it is given.
func captureOpened(a *App) *[]string {
	opened := &[]string{}
	a.openURLFunc = func(u string) { *opened = append(*opened, u) }
	return opened
}

func TestOpenPeerDesktopInBrowser(t *testing.T) {
	const link = "https://office-air.example.ts.net/desktop/?cap=SECRET"
	t.Run("held link opens once", func(t *testing.T) {
		a := fakePeerDesktopDaemon(t, map[string]http.HandlerFunc{
			"GET /peers/desktop/link": replyJSON(200, `{"url":"`+link+`"}`),
		})
		opened := captureOpened(a)
		if got := a.OpenPeerDesktopInBrowser("office-air"); got != "" {
			t.Errorf("returned %q, want an empty code", got)
		}
		if len(*opened) != 1 || (*opened)[0] != link {
			t.Errorf("opened %v, want the link once", *opened)
		}
	})
	t.Run("404 connects once and retries the link once", func(t *testing.T) {
		var links, connects int
		a := fakePeerDesktopDaemon(t, map[string]http.HandlerFunc{
			"GET /peers/desktop/link": func(w http.ResponseWriter, r *http.Request) {
				links++
				if links == 1 {
					replyJSON(404, `{"error":"no desktop token held for host"}`)(w, r)
					return
				}
				replyJSON(200, `{"url":"`+link+`"}`)(w, r)
			},
			"POST /peers/desktop": func(w http.ResponseWriter, r *http.Request) {
				connects++
				replyJSON(200, `{"host":"office-air.example.ts.net","name":"office-air"}`)(w, r)
			},
		})
		opened := captureOpened(a)
		if got := a.OpenPeerDesktopInBrowser("office-air"); got != "" {
			t.Errorf("returned %q, want an empty code", got)
		}
		if links != 2 || connects != 1 || len(*opened) != 1 || (*opened)[0] != link {
			t.Errorf("links=%d connects=%d opened=%v, want 2, 1 and the link once", links, connects, *opened)
		}
	})
	t.Run("failed connect returns its code and opens nothing", func(t *testing.T) {
		a := fakePeerDesktopDaemon(t, map[string]http.HandlerFunc{
			"GET /peers/desktop/link": replyJSON(404, `{}`),
			"POST /peers/desktop":     replyJSON(403, `{"error":"forbidden"}`),
		})
		opened := captureOpened(a)
		if got := a.OpenPeerDesktopInBrowser("office-air"); got != "forbidden" {
			t.Errorf("returned %q, want forbidden", got)
		}
		if len(*opened) != 0 {
			t.Errorf("opened %v, want nothing", *opened)
		}
	})
	t.Run("nil client is unreachable", func(t *testing.T) {
		a := &App{}
		opened := captureOpened(a)
		if got := a.OpenPeerDesktopInBrowser("office-air"); got != daemon.PeerErrUnreachable || len(*opened) != 0 {
			t.Errorf("returned %q, opened %v; want unreachable and nothing", got, *opened)
		}
	})
}

func TestOpenLocalNetworkSettings(t *testing.T) {
	var opened []string
	old := openSystemURL
	openSystemURL = func(u string) error { opened = append(opened, u); return nil }
	t.Cleanup(func() { openSystemURL = old })
	if err := (&App{}).OpenLocalNetworkSettings(); err != nil || len(opened) != 1 || opened[0] != paneLocalNetwork {
		t.Errorf("OpenLocalNetworkSettings = %v, opened %v; want %q once", err, opened, paneLocalNetwork)
	}
}
