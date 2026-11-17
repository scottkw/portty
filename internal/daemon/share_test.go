package daemon

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/scottkw/portty/internal/relay"
	"github.com/scottkw/portty/internal/webserver"
)

func healthyTailnet(context.Context) webserver.TailscaleHealth {
	return webserver.TailscaleHealth{Connected: true, HasCerts: true, IP: "127.0.0.1", Domain: "127.0.0.1"}
}

// loopbackShareServer builds a share server on a random loopback port with a
// self-signed certificate, standing in for the tailnet server on 7444.
func loopbackShareServer(_ webserver.TailscaleHealth, m *relay.HubManager) (*webserver.WebServer, error) {
	cfg, err := webserver.GenerateSelfSignedCert("127.0.0.1")
	if err != nil {
		return nil, err
	}
	return webserver.NewWebServer(webserver.Config{BindIP: "127.0.0.1", Port: 0, FQDN: "127.0.0.1", TLSConfig: cfg}, m)
}

func noWarm(context.Context, string) error { return nil }

// shareTestDaemon is a test daemon with a bootstrapped key and share seams
// pointing at a loopback share server.
func shareTestDaemon(t *testing.T) (*API, *DaemonClient) {
	t.Helper()
	api, client, _ := testDaemon(t)
	if err := api.BootstrapCapabilityState(); err != nil {
		t.Fatalf("BootstrapCapabilityState: %v", err)
	}
	api.SetShareSeamsForTest(healthyTailnet, loopbackShareServer, noWarm)
	return api, client
}

func newShareSession(t *testing.T, client *DaemonClient) string {
	t.Helper()
	id, err := client.CreateSession("share-test", "", 80, 24)
	if err != nil {
		t.Fatalf("CreateSession: %v", err)
	}
	return id
}

func mustShare(t *testing.T, client *DaemonClient, id string) ShareResponse {
	t.Helper()
	resp, err := client.SetSessionShared(id, true)
	if err != nil {
		t.Fatalf("SetSessionShared(true): %v", err)
	}
	if !resp.Shared || !strings.Contains(resp.URL, "?cap=") {
		t.Fatalf("share-on response = %+v; want shared with a ?cap= link", resp)
	}
	return resp
}

// putShareRaw drives PUT /sessions/{id}/share on the mux to observe status codes.
func putShareRaw(t *testing.T, api *API, id string, shared bool) (int, ShareErrorResponse) {
	t.Helper()
	b, _ := json.Marshal(ShareRequest{Shared: shared})
	rec := httptest.NewRecorder()
	api.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodPut, "/sessions/"+id+"/share", bytes.NewReader(b)))
	var e ShareErrorResponse
	_ = json.Unmarshal(rec.Body.Bytes(), &e)
	return rec.Code, e
}

func insecureNoRedirectClient() *http.Client {
	return &http.Client{
		Timeout:       5 * time.Second,
		Transport:     &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: true}}, //nolint:gosec // self-signed loopback test server
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}
}

func linkParts(t *testing.T, link string) (*url.URL, string) {
	t.Helper()
	u, err := url.Parse(link)
	if err != nil {
		t.Fatalf("parse link %q: %v", link, err)
	}
	return u, u.Query().Get("cap")
}

// linkStatus GETs a share link without following redirects.
func linkStatus(t *testing.T, link string) (int, string) {
	t.Helper()
	resp, err := insecureNoRedirectClient().Get(link)
	if err != nil {
		t.Fatalf("GET %s: %v", link, err)
	}
	resp.Body.Close()
	return resp.StatusCode, resp.Header.Get("Location")
}

func TestShare_OnReturnsWorkingLink(t *testing.T) {
	api, client := shareTestDaemon(t)
	id := newShareSession(t, client)
	resp := mustShare(t, client, id)
	u, token := linkParts(t, resp.URL)
	if u.Scheme != "https" || u.Path != "/sessions/"+id || token == "" {
		t.Fatalf("link = %q; want https://<host>/sessions/%s?cap=<token>", resp.URL, id)
	}

	if claims := extractClaimsFromURL(t, resp.URL, api.CurrentSigningKey()); claims.Perms != "read,write" || claims.SID != id {
		t.Fatalf("link claims = %+v; want SID %s with perms exactly read,write (SHAR-02)", claims, id)
	}

	code, loc := linkStatus(t, resp.URL)
	if code != http.StatusFound || !strings.HasPrefix(loc, "/app/?session=") {
		t.Fatalf("GET link = %d Location %q; want 302 to /app/?session=", code, loc)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	hdr := http.Header{}
	hdr.Set("Origin", "https://"+u.Host)
	wsURL := "wss://" + u.Host + "/sessions/" + id + "/ws?cap=" + url.QueryEscape(token)
	conn, _, err := websocket.Dial(ctx, wsURL, &websocket.DialOptions{HTTPClient: insecureNoRedirectClient(), HTTPHeader: hdr})
	if err != nil {
		t.Fatalf("websocket.Dial: %v", err)
	}
	defer conn.CloseNow()
	if err := conn.Write(ctx, websocket.MessageBinary, relay.MakeInputFrame([]byte("hello\n"))); err != nil {
		t.Fatalf("write input: %v", err)
	}
	var out []byte
	for !bytes.Contains(out, []byte("hello")) {
		_, m, err := conn.Read(ctx)
		if err != nil {
			t.Fatalf("read (output so far %q): %v", out, err)
		}
		if len(m) > 0 && m[0] == relay.MsgOutput {
			out = append(out, m[1:]...)
		}
	}
}

func TestShare_Idempotent(t *testing.T) {
	_, client := shareTestDaemon(t)
	id := newShareSession(t, client)
	first := mustShare(t, client, id)
	second := mustShare(t, client, id)
	if second.URL != first.URL {
		t.Fatalf("second share-on link %q differs from first %q", second.URL, first.URL)
	}
	got, err := client.GetSessionShare(id)
	if err != nil || !got.Shared || got.URL != first.URL {
		t.Fatalf("GetSessionShare = %+v, %v; want shared with %q", got, err, first.URL)
	}
}

func TestShare_TailscaleOff(t *testing.T) {
	api, client := shareTestDaemon(t)
	api.SetShareSeamsForTest(func(context.Context) webserver.TailscaleHealth { return webserver.TailscaleHealth{} }, loopbackShareServer, noWarm)
	id := newShareSession(t, client)
	code, e := putShareRaw(t, api, id, true)
	if code != http.StatusServiceUnavailable || e.Error != ShareErrTailscaleOff {
		t.Fatalf("share-on = %d %+v; want 503 %s", code, e, ShareErrTailscaleOff)
	}
	_, err := client.SetSessionShared(id, true)
	var se *ShareError
	if !errors.As(err, &se) || se.Code != ShareErrTailscaleOff {
		t.Fatalf("client error = %v; want *ShareError %s", err, ShareErrTailscaleOff)
	}
}

func TestShare_HTTPSOff(t *testing.T) {
	api, client := shareTestDaemon(t)
	built := 0
	api.SetShareSeamsForTest(
		func(context.Context) webserver.TailscaleHealth {
			return webserver.TailscaleHealth{Connected: true, IP: "127.0.0.1"}
		},
		func(h webserver.TailscaleHealth, m *relay.HubManager) (*webserver.WebServer, error) {
			built++
			return loopbackShareServer(h, m)
		},
		noWarm)
	id := newShareSession(t, client)
	code, e := putShareRaw(t, api, id, true)
	if code != http.StatusConflict || e.Error != ShareErrHTTPSOff {
		t.Fatalf("share-on = %d %+v; want 409 %s", code, e, ShareErrHTTPSOff)
	}
	api.mu.RLock()
	running := api.webServer != nil
	api.mu.RUnlock()
	if built != 0 || running {
		t.Fatalf("a share server was built (%d) or is running (%v) with HTTPS off", built, running)
	}
}

func TestShare_UnknownSession(t *testing.T) {
	api, _ := shareTestDaemon(t)
	code, e := putShareRaw(t, api, "no-such-session", true)
	if code != http.StatusNotFound || e.Error != ShareErrNotFound {
		t.Fatalf("share-on = %d %+v; want 404 %s", code, e, ShareErrNotFound)
	}
}

func webEnabled(t *testing.T, client *DaemonClient, id string) bool {
	t.Helper()
	sessions, err := client.ListSessions()
	if err != nil {
		t.Fatalf("ListSessions: %v", err)
	}
	for _, s := range sessions {
		if s.ID == id {
			return s.WebEnabled
		}
	}
	t.Fatalf("session %s not listed", id)
	return false
}

func TestShare_OffStopsServerAndNewLinkDiffers(t *testing.T) {
	_, client := shareTestDaemon(t)
	id := newShareSession(t, client)
	first := mustShare(t, client, id)
	if !webEnabled(t, client, id) {
		t.Fatal("webEnabled false while shared")
	}
	oldURL, oldToken := linkParts(t, first.URL)

	off, err := client.SetSessionShared(id, false)
	if err != nil || off.Shared {
		t.Fatalf("share-off = %+v, %v", off, err)
	}
	if webEnabled(t, client, id) {
		t.Fatal("webEnabled true after share-off")
	}
	assertNotListening(t, oldURL.Host)
	if got, _ := client.GetSessionShare(id); got.Shared {
		t.Fatalf("GetSessionShare after off = %+v", got)
	}

	second := mustShare(t, client, id)
	newURL, newToken := linkParts(t, second.URL)
	if newToken == oldToken {
		t.Fatal("share-on after off reused the old token")
	}
	stale := "https://" + newURL.Host + newURL.Path + "?cap=" + url.QueryEscape(oldToken)
	if code, _ := linkStatus(t, stale); code == http.StatusFound {
		t.Fatal("the old link's token still opens the session after re-share")
	}
	// Turning off a session that is not shared is a no-op success.
	if _, err := client.SetSessionShared(id, false); err != nil {
		t.Fatal(err)
	}
	if off, err := client.SetSessionShared(id, false); err != nil || off.Shared {
		t.Fatalf("second share-off = %+v, %v", off, err)
	}
}

func TestShare_FirstShareWritesCTAck(t *testing.T) {
	api, client := shareTestDaemon(t)
	ack := filepath.Join(api.engine.configDir, "ct_disclosed")
	first := mustShare(t, client, newShareSession(t, client))
	if !first.FirstShare {
		t.Fatal("first share-on: firstShare false")
	}
	if b, err := os.ReadFile(ack); err != nil || string(b) != "1" {
		t.Fatalf("ct_disclosed = %q, %v; want \"1\"", b, err)
	}
	if fi, _ := os.Stat(ack); fi.Mode().Perm() != 0o600 {
		t.Fatalf("ct_disclosed mode = %v; want 0600", fi.Mode().Perm())
	}
	if second := mustShare(t, client, newShareSession(t, client)); second.FirstShare {
		t.Fatal("second session's share-on: firstShare true")
	}
}

func shareServerRunning(api *API) bool {
	api.mu.RLock()
	defer api.mu.RUnlock()
	return api.webServer != nil
}

func assertNotListening(t *testing.T, host string) {
	t.Helper()
	if c, err := net.DialTimeout("tcp", host, time.Second); err == nil {
		c.Close()
		t.Fatalf("share server still accepting on %s", host)
	}
}

func TestShare_OffDisconnectsWebViewersOnly(t *testing.T) {
	api, client := shareTestDaemon(t)
	id := newShareSession(t, client)
	mustShare(t, client, id)
	hub, ok := api.engine.Manager().Get(id)
	if !ok {
		t.Fatalf("hub for session %s not found", id)
	}
	webClosed := make(chan struct{}, 1)
	localClosed := make(chan struct{}, 1)
	sig := func(ch chan struct{}) func() {
		return func() {
			select {
			case ch <- struct{}{}:
			default:
			}
		}
	}
	hub.Subscribe(&relay.Subscriber{Msgs: make(chan []byte, 256), CloseSlow: sig(webClosed), Origin: "web"})
	hub.Subscribe(&relay.Subscriber{Msgs: make(chan []byte, 256), CloseSlow: sig(localClosed), Origin: "local"})

	if _, err := client.SetSessionShared(id, false); err != nil {
		t.Fatal(err)
	}
	select {
	case <-webClosed:
	case <-time.After(2 * time.Second):
		t.Fatal("web viewer was not disconnected on share-off (M-49: silent dead terminal)")
	}
	select {
	case <-localClosed:
		t.Fatal("share-off closed the owner's local subscriber")
	case <-time.After(150 * time.Millisecond):
	}
}

func TestShare_KillDropsRecordAndStopsServer(t *testing.T) {
	api, client := shareTestDaemon(t)
	id := newShareSession(t, client)
	u, _ := linkParts(t, mustShare(t, client, id).URL)
	if err := client.KillSession(id); err != nil {
		t.Fatalf("KillSession: %v", err)
	}
	if got, _ := client.GetSessionShare(id); got.Shared {
		t.Fatalf("GetSessionShare after kill = %+v", got)
	}
	if shareServerRunning(api) {
		t.Fatal("share server still set after the last shared session was killed")
	}
	assertNotListening(t, u.Host)
}

func TestShare_ExitCleanupDropsRecord(t *testing.T) {
	api, client := shareTestDaemon(t)
	exiting := newShareSession(t, client)
	other := newShareSession(t, client)
	mustShare(t, client, exiting)
	u, _ := linkParts(t, mustShare(t, client, other).URL)

	api.runSessionExitCleanupForTest(exiting)
	if got, _ := client.GetSessionShare(exiting); got.Shared {
		t.Fatalf("exited session still shared: %+v", got)
	}
	if !shareServerRunning(api) {
		t.Fatal("share server stopped while another session is still shared")
	}

	api.runSessionExitCleanupForTest(other)
	if shareServerRunning(api) {
		t.Fatal("share server still set after the last shared session exited")
	}
	assertNotListening(t, u.Host)
}

func TestShare_KeyRegenRemintsLinks(t *testing.T) {
	_, client := shareTestDaemon(t)
	id := newShareSession(t, client)
	old := mustShare(t, client, id)
	if err := client.RegenerateSigningKey(); err != nil {
		t.Fatalf("RegenerateSigningKey: %v", err)
	}
	if code, _ := linkStatus(t, old.URL); code == http.StatusFound {
		t.Fatal("the link from before key regeneration still opens the session")
	}
	cur, err := client.GetSessionShare(id)
	if err != nil || !cur.Shared {
		t.Fatalf("GetSessionShare after regen = %+v, %v; want still shared", cur, err)
	}
	if cur.URL == old.URL {
		t.Fatal("key regeneration kept the old link")
	}
	if code, loc := linkStatus(t, cur.URL); code != http.StatusFound || !strings.HasPrefix(loc, "/app/?session=") {
		t.Fatalf("re-minted link = %d %q; want 302 to /app/", code, loc)
	}
}

func TestShare_NoServerAtBoot(t *testing.T) {
	api, client := shareTestDaemon(t)
	id := newShareSession(t, client)
	if shareServerRunning(api) {
		t.Fatal("a share server is running before anything was shared")
	}
	if got, err := client.GetSessionShare(id); err != nil || got.Shared {
		t.Fatalf("GetSessionShare = %+v, %v; want not shared", got, err)
	}
}
