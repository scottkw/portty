package daemon_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/scottkw/portty/internal/daemon"
	"github.com/scottkw/portty/internal/tailnet"
)

// fakePeerHost serves the host side of the same-account peer routes (04-03):
// GET /api/sessions/meta and POST /api/peer/sessions/{id}/cap.
type fakePeerHost struct {
	shared       []tailnet.ShareableSessionMeta
	metaStatus   int // 0 = 200
	capStatus    int // 0 = 200
	capRequested string
}

func (f *fakePeerHost) handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/sessions/meta", func(w http.ResponseWriter, r *http.Request) {
		if f.metaStatus != 0 {
			w.Header().Set(tailnet.MarkerHeader, tailnet.MarkerValue)
			http.Error(w, "forbidden", f.metaStatus)
			return
		}
		_ = json.NewEncoder(w).Encode(f.shared)
	})
	mux.HandleFunc("POST /api/peer/sessions/{id}/cap", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Content-Type") != "application/json" {
			http.Error(w, "unsupported", http.StatusUnsupportedMediaType)
			return
		}
		f.capRequested = r.PathValue("id")
		if f.capStatus != 0 {
			http.Error(w, "no", f.capStatus)
			return
		}
		_, _ = w.Write([]byte(`{"cap":"CAP-` + r.PathValue("id") + `"}`))
	})
	return mux
}

var officeAir = tailnet.Peer{Hostname: "Kens-MacBook-Air", DNSName: "office-air.tail1234.ts.net.", Online: true}

type connectEnv struct {
	host   *fakePeerHost
	peer   *httptest.Server
	socket *httptest.Server
	api    *daemon.API
}

func newConnectEnv(t *testing.T, shared ...tailnet.ShareableSessionMeta) *connectEnv {
	t.Helper()
	host := &fakePeerHost{shared: shared}
	peer := httptest.NewTLSServer(host.handler())
	t.Cleanup(peer.Close)
	engine := daemon.NewSessionEngine()
	engine.ConfigDirForTest(t.TempDir())
	api := daemon.NewAPI(engine)
	api.SetPeerClientForTest(peer.Client())
	api.SetPeerSeamsForTest(
		func(context.Context) ([]tailnet.Peer, error) { return []tailnet.Peer{officeAir}, nil },
		func(context.Context) []tailnet.Peer {
			return []tailnet.Peer{officeAir, {Hostname: "home", DNSName: "home-air.tail1234.ts.net."}}
		},
		func(tailnet.Peer) string { return peer.URL },
	)
	socket := httptest.NewServer(api.Handler())
	t.Cleanup(socket.Close)
	return &connectEnv{host: host, peer: peer, socket: socket, api: api}
}

func (e *connectEnv) connect(t *testing.T, host, session string) (int, daemon.PeerConnectResponse, daemon.PeerConnectErrorResponse) {
	t.Helper()
	b, _ := json.Marshal(daemon.PeerConnectRequest{Host: host, Session: session})
	resp, err := http.Post(e.socket.URL+"/peers/connect", "application/json", bytes.NewReader(b))
	if err != nil {
		t.Fatalf("POST /peers/connect: %v", err)
	}
	defer resp.Body.Close()
	var ok daemon.PeerConnectResponse
	var bad daemon.PeerConnectErrorResponse
	if resp.StatusCode == http.StatusOK {
		_ = json.NewDecoder(resp.Body).Decode(&ok)
	} else {
		_ = json.NewDecoder(resp.Body).Decode(&bad)
	}
	return resp.StatusCode, ok, bad
}

var (
	buildSess = tailnet.ShareableSessionMeta{ID: "aaaa1111-0000", Name: "build", Status: "running"}
	logsSess  = tailnet.ShareableSessionMeta{ID: "bbbb2222-0000", Name: "logs", Status: "running"}
)

func TestPeerConnect_HostMatching(t *testing.T) {
	for _, host := range []string{"office-air", "Kens-MacBook-Air", "OFFICE-AIR", "kens-macbook-air"} {
		t.Run(host, func(t *testing.T) {
			e := newConnectEnv(t, buildSess)
			code, ok, bad := e.connect(t, host, "build")
			if code != http.StatusOK {
				t.Fatalf("status %d (%+v), want 200", code, bad)
			}
			if ok.Host != "office-air" || ok.SessionID != buildSess.ID || ok.Name != "build" {
				t.Fatalf("response = %+v", ok)
			}
		})
	}
}

func TestPeerConnect_StoresCap(t *testing.T) {
	e := newConnectEnv(t, buildSess, logsSess)
	code, ok, _ := e.connect(t, "office-air", "build")
	if code != http.StatusOK {
		t.Fatalf("status %d", code)
	}
	want := e.peer.URL + "/sessions/" + buildSess.ID + "?cap=CAP-" + buildSess.ID
	if ok.URL != want {
		t.Fatalf("url = %q, want %q", ok.URL, want)
	}
	// The relay proxy can now attach: no "no cap registered" 404.
	relay := httptest.NewServer(e.api.RelayHandler())
	defer relay.Close()
	resp, err := http.Get(relay.URL + "/api/relay/remote/" + buildSess.ID + "/ws")
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		t.Fatal("cap was not stored in RemoteCapStore")
	}
}

func TestPeerConnect_UniqueIDPrefix(t *testing.T) {
	e := newConnectEnv(t, buildSess, logsSess)
	code, ok, _ := e.connect(t, "office-air", "bbbb")
	if code != http.StatusOK || ok.SessionID != logsSess.ID {
		t.Fatalf("status %d, response %+v", code, ok)
	}
}

func TestPeerConnect_AmbiguousName(t *testing.T) {
	dup := tailnet.ShareableSessionMeta{ID: "cccc3333-0000", Name: "build"}
	e := newConnectEnv(t, buildSess, dup)
	code, _, bad := e.connect(t, "office-air", "build")
	if code != http.StatusConflict || bad.Error != daemon.PeerErrAmbiguous || len(bad.Sessions) != 2 {
		t.Fatalf("status %d, body %+v", code, bad)
	}
}

func TestPeerConnect_NoSuchSession(t *testing.T) {
	e := newConnectEnv(t, buildSess, logsSess)
	code, _, bad := e.connect(t, "office-air", "deploy")
	if code != http.StatusNotFound || bad.Error != daemon.PeerErrNoSuchSession || len(bad.Sessions) != 2 || bad.Detail != "office-air" {
		t.Fatalf("status %d, body %+v", code, bad)
	}
}

func TestPeerConnect_EmptySession(t *testing.T) {
	e := newConnectEnv(t, buildSess)
	if code, ok, _ := e.connect(t, "office-air", ""); code != http.StatusOK || ok.SessionID != buildSess.ID {
		t.Fatalf("one shared: status %d, %+v", code, ok)
	}
	e = newConnectEnv(t, buildSess, logsSess)
	code, _, bad := e.connect(t, "office-air", "")
	if code != http.StatusConflict || bad.Error != daemon.PeerErrMultiple || len(bad.Sessions) != 2 {
		t.Fatalf("several shared: status %d, body %+v", code, bad)
	}
}

func TestPeerConnect_NotSharing(t *testing.T) {
	e := newConnectEnv(t) // 200 []
	if code, _, bad := e.connect(t, "office-air", ""); code != http.StatusConflict || bad.Error != daemon.PeerErrNotSharing {
		t.Fatalf("empty list: status %d, body %+v", code, bad)
	}
	e = newConnectEnv(t, buildSess)
	e.peer.Close() // refused port: the share server is not running
	if code, _, bad := e.connect(t, "office-air", "build"); code != http.StatusConflict || bad.Error != daemon.PeerErrNotSharing {
		t.Fatalf("refused: status %d, body %+v", code, bad)
	}
}

func TestPeerConnect_Forbidden(t *testing.T) {
	e := newConnectEnv(t, buildSess)
	e.host.metaStatus = http.StatusForbidden
	if code, _, bad := e.connect(t, "office-air", "build"); code != http.StatusForbidden || bad.Error != daemon.PeerErrForbidden {
		t.Fatalf("meta 403: status %d, body %+v", code, bad)
	}
	e = newConnectEnv(t, buildSess)
	e.host.capStatus = http.StatusForbidden
	if code, _, bad := e.connect(t, "office-air", "build"); code != http.StatusForbidden || bad.Error != daemon.PeerErrForbidden {
		t.Fatalf("cap 403: status %d, body %+v", code, bad)
	}
}

func TestPeerConnect_TailscaleOff(t *testing.T) {
	e := newConnectEnv(t, buildSess)
	e.api.SetPeerSeamsForTest(func(context.Context) ([]tailnet.Peer, error) { return nil, errors.New("tailscaled not running") }, nil, nil)
	if code, _, bad := e.connect(t, "office-air", "build"); code != http.StatusServiceUnavailable || bad.Error != daemon.PeerErrTailscaleOff {
		t.Fatalf("status %d, body %+v", code, bad)
	}
}

func TestPeerConnect_UnknownHostListsProbed(t *testing.T) {
	e := newConnectEnv(t, buildSess)
	code, _, bad := e.connect(t, "garage-mini", "build")
	if code != http.StatusNotFound || bad.Error != daemon.PeerErrUnknownHost || strings.Join(bad.Hosts, ",") != "office-air,home-air" {
		t.Fatalf("status %d, body %+v", code, bad)
	}
}

func TestPeerConnect_ClientDecodesError(t *testing.T) {
	e := newConnectEnv(t, buildSess, logsSess)
	sock := fmt.Sprintf("/tmp/ahpc%d_%d.sock", os.Getpid(), time.Now().UnixNano()%100000)
	t.Cleanup(func() { os.Remove(sock) })
	if err := e.api.Start(sock); err != nil {
		t.Fatalf("api.Start: %v", err)
	}
	t.Cleanup(func() { e.api.Stop() })
	c := daemon.NewDaemonClient(sock)
	_, err := c.ConnectPeer("office-air", "")
	var pe *daemon.PeerConnectError
	if !errors.As(err, &pe) || pe.Code != daemon.PeerErrMultiple || len(pe.Sessions) != 2 {
		t.Fatalf("err = %#v", err)
	}
	got, err := c.ConnectPeer("office-air", "logs")
	if err != nil || got.SessionID != logsSess.ID || got.Host != "office-air" {
		t.Fatalf("ConnectPeer = %+v, %v", got, err)
	}
}
