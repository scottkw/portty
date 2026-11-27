package daemon

// Phase 21-06 — POST /peers/desktop, the link route and the loopback desktop
// proxy, against a real office share server (the Phase 19 desktop test env with
// its synthetic source). Everything binds loopback; nothing touches tailscaled.

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
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
	viewer, _, _ := testDaemon(t)
	office := tailnet.Peer{Hostname: "office", DNSName: officeFQDN + "."}
	viewer.SetPeerClientForTest(peerTestClient())
	viewer.SetPeerSeamsForTest(
		func(context.Context) ([]tailnet.Peer, error) { return []tailnet.Peer{office}, nil },
		func(context.Context) []tailnet.Peer { return []tailnet.Peer{office} },
		func(tailnet.Peer) string { return baseURL },
	)
	return viewer
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
