package daemon

// Phase 13 tracer: two daemons in one process, the viewing daemon's forward
// talking to the host's real share server (webserver port routes, real
// capability and port token), which proxies to a dev server on loopback.
// Nothing reaches the tailnet: the share server is a loopback server with a
// self-signed certificate and a same-user WhoIs fake.

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/scottkw/portty/internal/relay"
	"github.com/scottkw/portty/internal/webserver"
)

func TestPortForwardE2E_HostDevServer(t *testing.T) {
	n := freeLoopbackPort(t) // the port the session prints and the browser opens

	// The dev server, on a port of its own. The host's dialer maps n to it
	// because both Macs share this machine's loopback.
	var mu sync.Mutex
	var seenHost, seenURI string
	var seenHdr http.Header
	dev := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		seenHost, seenURI, seenHdr = r.Host, r.RequestURI, r.Header.Clone()
		mu.Unlock()
		io.WriteString(w, "dev-server-body")
	}))
	defer dev.Close()
	devAddr := dev.Listener.Addr().String()

	// Host Mac: a share server whose proxy dials the dev server for port n.
	host, hostClient := shareTestDaemon(t)
	host.SetShareSeamsForTest(healthyTailnet, func(h webserver.TailscaleHealth, m *relay.HubManager) (*webserver.WebServer, error) {
		ws, err := loopbackShareServer(h, m)
		if err != nil {
			return nil, err
		}
		ws.SetPortDialerForTest(func(ctx context.Context, port int) (net.Conn, error) {
			if port != n {
				return nil, fmt.Errorf("unexpected port %d", port)
			}
			return (&net.Dialer{}).DialContext(ctx, "tcp", devAddr)
		})
		return ws, nil
	}, noWarm)
	host.engine.SetSpawnCommandForTest("/bin/sh", []string{"-c", fmt.Sprintf("echo 'Serving on http://localhost:%d/'; exec /bin/cat", n)})
	id := newShareSession(t, hostClient)
	link := mustShare(t, hostClient, id)
	linkURL, linkToken := linkParts(t, link.URL)

	hub, ok := host.engine.Manager().Get(id)
	if !ok {
		t.Fatal("no hub for the shared session")
	}
	deadline := time.Now().Add(5 * time.Second)
	for !hub.PortPrinted(n) {
		if time.Now().After(deadline) {
			t.Fatalf("the session never printed port %d", n)
		}
		time.Sleep(20 * time.Millisecond)
	}

	// Viewing Mac: holds the link's capability, dials the host's share server.
	viewAPI, _, viewSocket := testDaemon(t)
	viewAPI.SetPeerClientForTest(insecureNoRedirectClient())
	if err := viewAPI.remoteCaps.Put(id, "https://"+linkURL.Host, "", linkToken); err != nil {
		t.Fatal(err)
	}
	view := &portEnv{api: viewAPI, socket: viewSocket, client: unixClient(viewSocket)}

	code, got, errCode := view.open(t, id, localhostURL(n, "/"), true)
	if code != http.StatusOK || got.Status != PortStatusOpen || got.Port != n {
		t.Fatalf("open = %d %+v %q", code, got, errCode)
	}

	resp, err := http.Get("http://127.0.0.1:" + strconv.Itoa(n) + "/hello?q=1")
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK || string(body) != "dev-server-body" {
		t.Fatalf("GET through the forward = %d %q", resp.StatusCode, body)
	}
	mu.Lock()
	defer mu.Unlock()
	if seenHost != "localhost:"+strconv.Itoa(n) {
		t.Errorf("dev server saw Host %q, want localhost:%d", seenHost, n)
	}
	if seenURI != "/hello?q=1" {
		t.Errorf("dev server saw RequestURI %q, want /hello?q=1", seenURI)
	}
	for k := range seenHdr {
		if strings.HasPrefix(k, "X-Portty") || strings.HasPrefix(k, "X-Forwarded") || k == "Forwarded" {
			t.Errorf("dev server saw header %s", k)
		}
	}

	// A port the session never printed is refused at the grant.
	other := freeLoopbackPort(t)
	code, _, errCode = view.open(t, id, localhostURL(other, "/"), true)
	if code != http.StatusForbidden || errCode != PortErrRefused {
		t.Errorf("unprinted port = %d %q, want 403 %q", code, errCode, PortErrRefused)
	}
	if listening(other) {
		t.Errorf("a forward listens on %d after a refused grant", other)
	}
}
