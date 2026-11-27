package daemon

// Phase 21-06 — POST /peers/desktop, the link route and the loopback desktop
// proxy, against a real office share server (the Phase 19 desktop test env with
// its synthetic source). Everything binds loopback; nothing touches tailscaled.

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/scottkw/portty/internal/tailnet"
)

const officeFQDN = "office-air.example"

// peerTestClient trusts any certificate and has no Timeout: proxied WebSockets
// are long-lived.
func peerTestClient() *http.Client {
	return &http.Client{Transport: &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: true}}} //nolint:gosec // self-signed loopback share server
}

// newViewerFor is a viewing daemon whose peer seams point at baseURL as the
// office Mac.
func newViewerFor(t *testing.T, baseURL string) *API {
	t.Helper()
	viewer, _ := newViewerClientFor(t, baseURL)
	return viewer
}

// newViewerClientFor is newViewerFor plus a DaemonClient on the daemon socket.
func newViewerClientFor(t *testing.T, baseURL string) (*API, *DaemonClient) {
	t.Helper()
	viewer, client, _ := testDaemon(t)
	office := tailnet.Peer{Hostname: "office", DNSName: officeFQDN + "."}
	viewer.SetPeerClientForTest(peerTestClient())
	viewer.SetPeerSeamsForTest(
		func(context.Context) ([]tailnet.Peer, error) { return []tailnet.Peer{office}, nil },
		func(context.Context) []tailnet.Peer { return []tailnet.Peer{office} },
		func(tailnet.Peer) string { return baseURL },
	)
	return viewer, client
}

// postPeerDesktop drives POST /peers/desktop on the viewer's mux.
func postPeerDesktop(t *testing.T, a *API, host string) (int, string) {
	t.Helper()
	b, _ := json.Marshal(PeerDesktopRequest{Host: host})
	rec := httptest.NewRecorder()
	a.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/peers/desktop", bytes.NewReader(b)))
	return rec.Code, rec.Body.String()
}

// dialDesktopProxy opens the viewer's loopback desktop proxy for host with a web
// view Origin.
func dialDesktopProxy(ctx context.Context, relayURL, host string) (*websocket.Conn, *http.Response, error) {
	return websocket.Dial(ctx, "ws"+strings.TrimPrefix(relayURL, "http")+"/api/relay/remote-desktop/"+host+"/ws", &websocket.DialOptions{
		HTTPHeader: http.Header{"Origin": {"wails://wails"}},
	})
}

func TestPeerDesktop_Tracer(t *testing.T) {
	host := newDesktopEnv(t)
	link := host.mustOn(t)
	lu, err := url.Parse(link.URL)
	if err != nil {
		t.Fatal(err)
	}
	token := lu.Query().Get("cap")
	viewer := newViewerFor(t, "https://"+lu.Host)

	code, body := postPeerDesktop(t, viewer, officeFQDN)
	if code != http.StatusOK {
		t.Fatalf("POST /peers/desktop = %d %s", code, body)
	}
	if strings.Contains(body, token) {
		t.Fatalf("response body carries the token: %s", body)
	}
	var resp PeerDesktopResponse
	if err := json.Unmarshal([]byte(body), &resp); err != nil || resp.Host != officeFQDN || resp.Name != "office-air" {
		t.Fatalf("response = %+v (%v); want host %q name office-air", resp, err, officeFQDN)
	}

	relaySrv := httptest.NewServer(viewer.wrapRelayWithRemoteFiles(http.NotFoundHandler()))
	t.Cleanup(relaySrv.Close)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	conn, _, err := dialDesktopProxy(ctx, relaySrv.URL, officeFQDN)
	if err != nil {
		t.Fatalf("dial desktop proxy: %v", err)
	}
	defer func() { _ = conn.CloseNow() }()
	writeWS(ctx, t, conn, map[string]any{"type": "hello", "v": 1, "caps": map[string]int{"video": 1}})
	readWSType(ctx, t, conn, "hello")
}

// capRoute is a viewer plus a fake peer whose cap route answers status (0 = 200
// with a token).
func capRoute(t *testing.T, status int, token string) (*API, *fakeDesktopPeer) {
	t.Helper()
	peer := newFakeDesktopPeer(t, echoWS)
	peer.freshCap, peer.capStatus = token, status
	return newViewerFor(t, peer.srv.URL), peer
}

func decodeConnectErr(t *testing.T, body string) PeerConnectErrorResponse {
	t.Helper()
	var e PeerConnectErrorResponse
	if err := json.Unmarshal([]byte(body), &e); err != nil {
		t.Fatalf("error body %q: %v", body, err)
	}
	return e
}

func TestPeerDesktop_CapRouteFailures(t *testing.T) {
	for _, tc := range []struct {
		name       string
		capStatus  int
		wantStatus int
		wantCode   string
	}{
		{"forbidden", http.StatusForbidden, http.StatusForbidden, PeerErrForbidden},
		{"desktop off", http.StatusNotFound, http.StatusConflict, PeerErrDesktopOff},
		{"server error", http.StatusInternalServerError, http.StatusBadGateway, PeerErrUnreachable},
	} {
		t.Run(tc.name, func(t *testing.T) {
			a, _ := capRoute(t, tc.capStatus, "tok")
			if err := a.desktopCaps.Put(officeFQDN, "https://127.0.0.1:1", "", "previous"); err != nil {
				t.Fatal(err)
			}
			code, body := postPeerDesktop(t, a, officeFQDN)
			e := decodeConnectErr(t, body)
			if code != tc.wantStatus || e.Error != tc.wantCode {
				t.Fatalf("POST = %d %s; want %d %s", code, body, tc.wantStatus, tc.wantCode)
			}
			if tc.wantCode == PeerErrForbidden && e.Detail != "office-air" {
				t.Fatalf("detail = %q; want the host", e.Detail)
			}
			_, _, held, ok := a.desktopCaps.Get(officeFQDN)
			if tc.wantCode == PeerErrDesktopOff && ok {
				t.Fatalf("token %q survived a desktop_off answer", held)
			}
			if tc.wantCode != PeerErrDesktopOff && held != "previous" {
				t.Fatalf("a %s answer changed the held token to %q", tc.wantCode, held)
			}
		})
	}
}

// A refused share port is the peer daemon restarting or just waking, not "off":
// only a 404 is terminal, so this stays retryable and the held token is kept.
func TestPeerDesktop_RefusedSharePortIsRetryable(t *testing.T) {
	peer := newFakeDesktopPeer(t, echoWS)
	a := newViewerFor(t, peer.srv.URL)
	peer.srv.Close()
	if err := a.desktopCaps.Put(officeFQDN, peer.srv.URL, "", "previous"); err != nil {
		t.Fatal(err)
	}
	code, body := postPeerDesktop(t, a, officeFQDN)
	if e := decodeConnectErr(t, body); code != http.StatusBadGateway || e.Error != PeerErrUnreachable {
		t.Fatalf("POST = %d %s; want 502 unreachable", code, body)
	}
	if _, _, held, ok := a.desktopCaps.Get(officeFQDN); !ok || held != "previous" {
		t.Fatal("a refused share port dropped the held token")
	}
}

func TestPeerDesktop_HostResolutionFailures(t *testing.T) {
	a, _ := capRoute(t, 0, "tok")
	if code, body := postPeerDesktop(t, a, "other-air"); code != http.StatusNotFound {
		t.Fatalf("unknown host = %d %s; want 404", code, body)
	} else if e := decodeConnectErr(t, body); e.Error != PeerErrUnknownHost || len(e.Hosts) != 1 || e.Hosts[0] != "office-air" {
		t.Fatalf("unknown host body = %+v; want unknown_host with the probed short names", e)
	}
	if code, _ := postPeerDesktop(t, a, "  "); code != http.StatusBadRequest {
		t.Fatalf("empty host = %d; want 400", code)
	}
	a.SetPeerSeamsForTest(func(context.Context) ([]tailnet.Peer, error) { return nil, context.DeadlineExceeded }, nil, nil)
	if code, body := postPeerDesktop(t, a, officeFQDN); code != http.StatusServiceUnavailable || decodeConnectErr(t, body).Error != PeerErrTailscaleOff {
		t.Fatalf("discover error = %d %s; want 503 tailscale_off", code, body)
	}
}

// AR-21-01: connect re-resolves the host against the (own-account-filtered)
// discovery list on the daemon side. A host the UI names that discovery does not
// list, such as a node shared in from another account, is refused before any
// cap is fetched or stored.
func TestPeerDesktop_RefusesHostMissingFromDiscovery(t *testing.T) {
	a, peer := capRoute(t, 0, "tok")
	const sharedIn = "sharedin-air.example"
	for _, host := range []string{sharedIn, "sharedin-air"} {
		if code, body := postPeerDesktop(t, a, host); code != http.StatusNotFound || decodeConnectErr(t, body).Error != PeerErrUnknownHost {
			t.Fatalf("POST %q = %d %s; want 404 unknown_host", host, code, body)
		}
	}
	peer.mu.Lock()
	posts := peer.capPosts
	peer.mu.Unlock()
	if posts != 0 {
		t.Fatalf("cap route hit %d times for a host discovery does not list", posts)
	}
	if _, _, _, ok := a.desktopCaps.Get(sharedIn); ok {
		t.Fatal("a cap was stored for a host discovery does not list")
	}
}

func TestPeerDesktop_FreezeGateExempt(t *testing.T) {
	a, _ := capRoute(t, 0, "tok")
	a.frozen.Store(true)
	do := func(path, body string) int {
		rec := httptest.NewRecorder()
		a.handler.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, path, strings.NewReader(body)))
		return rec.Code
	}
	if code := do("/peers/desktop", `{"host":"`+officeFQDN+`"}`); code != http.StatusOK {
		t.Fatalf("POST /peers/desktop while frozen = %d; want it served (200)", code)
	}
	if code := do("/sessions", `{}`); code != http.StatusServiceUnavailable {
		t.Fatalf("POST /sessions while frozen = %d; want 503", code)
	}
}

func TestPeerDesktop_LinkRoute(t *testing.T) {
	const token = "a+b/c=d e" // characters the query encoding must escape
	a, peer := capRoute(t, 0, token)

	get := func(h http.Handler, host string) (int, string) {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/peers/desktop/link?host="+url.QueryEscape(host), nil))
		return rec.Code, rec.Body.String()
	}
	if code, _ := get(a.Handler(), officeFQDN); code != http.StatusNotFound {
		t.Fatalf("link before connect = %d; want 404", code)
	}
	if code, body := postPeerDesktop(t, a, officeFQDN); code != http.StatusOK {
		t.Fatalf("connect = %d %s", code, body)
	}
	posts := peer.capPostCount()
	code, body := get(a.Handler(), "OFFICE-AIR.Example")
	var got PeerDesktopLinkResponse
	if err := json.Unmarshal([]byte(body), &got); err != nil || code != http.StatusOK {
		t.Fatalf("link = %d %s (%v)", code, body, err)
	}
	if want := peer.srv.URL + "/desktop/?cap=" + url.QueryEscape(token); got.URL != want {
		t.Fatalf("link url = %q; want %q", got.URL, want)
	}
	if peer.capPostCount() != posts {
		t.Fatal("the link route made a network call")
	}
	// Never on the relay mux: a browser page can reach that port.
	if code, body := get(a.wrapRelayWithRemoteFiles(http.NotFoundHandler()), officeFQDN); code != http.StatusNotFound || strings.Contains(body, "cap=") {
		t.Fatalf("relay mux answered the link route: %d %s", code, body)
	}
}

func TestPeerDesktop_SecondConnectReplacesToken(t *testing.T) {
	a, peer := capRoute(t, 0, "tok-one")
	if code, body := postPeerDesktop(t, a, officeFQDN); code != http.StatusOK {
		t.Fatalf("first connect = %d %s", code, body)
	}
	peer.mu.Lock()
	peer.freshCap = "tok-two"
	peer.mu.Unlock()
	if code, body := postPeerDesktop(t, a, "Office-Air.EXAMPLE"); code != http.StatusOK {
		t.Fatalf("second connect = %d %s", code, body)
	}
	for _, host := range []string{officeFQDN, "OFFICE-AIR.example.", "Office-Air.Example"} {
		if _, _, tok, ok := a.desktopCaps.Get(host); !ok || tok != "tok-two" {
			t.Fatalf("Get(%q) = %q, %v; want the replaced token tok-two", host, tok, ok)
		}
	}
	if n := len(a.desktopCaps.entries); n != 1 {
		t.Fatalf("store holds %d entries; case variants must share one", n)
	}
}

func TestPeerDesktop_ClientMethods(t *testing.T) {
	peer := newFakeDesktopPeer(t, echoWS)
	peer.freshCap = "tok-client"
	_, client := newViewerClientFor(t, peer.srv.URL)

	resp, err := client.ConnectPeerDesktop(officeFQDN)
	if err != nil || resp.Host != officeFQDN || resp.Name != "office-air" {
		t.Fatalf("ConnectPeerDesktop = %+v, %v", resp, err)
	}
	link, err := client.PeerDesktopLink(officeFQDN)
	if want := peer.srv.URL + "/desktop/?cap=tok-client"; err != nil || link != want {
		t.Fatalf("PeerDesktopLink = %q, %v; want %q", link, err, want)
	}

	peer.mu.Lock()
	peer.capStatus = http.StatusNotFound
	peer.mu.Unlock()
	var pce *PeerConnectError
	if _, err := client.ConnectPeerDesktop(officeFQDN); !errors.As(err, &pce) || pce.Code != PeerErrDesktopOff {
		t.Fatalf("ConnectPeerDesktop on a 409 = %v; want *PeerConnectError desktop_off", err)
	}
	if _, err := client.PeerDesktopLink(officeFQDN); err == nil {
		t.Fatal("PeerDesktopLink after desktop_off succeeded; the token must be gone")
	}
}
