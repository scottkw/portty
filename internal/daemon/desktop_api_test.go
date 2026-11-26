package daemon

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"testing/fstest"
	"time"

	"github.com/coder/websocket"
	"github.com/pion/ice/v4"
	"github.com/pion/rtp/codecs"
	"github.com/pion/webrtc/v4"

	"github.com/scottkw/portty/internal/desktop"
	"github.com/scottkw/portty/internal/relay"
	"github.com/scottkw/portty/internal/screencap"
	"github.com/scottkw/portty/internal/webserver"
	"github.com/scottkw/portty/internal/webserver/webservertest"
)

const desktopTestIndex = "<!doctype html><div id=root></div>"

// loopbackAddrs stands in for the interface list: the "tailnet address" of
// these tests is 127.0.0.1, which is on a local interface (not userspace).
func loopbackAddrs() ([]net.Addr, error) {
	return []net.Addr{&net.IPNet{IP: net.IPv4(127, 0, 0, 1), Mask: net.CIDRMask(8, 32)}}, nil
}

func freeUDPPort(t *testing.T) int {
	t.Helper()
	c, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1)})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = c.Close() }()
	return c.LocalAddr().(*net.UDPAddr).Port
}

// useTestAppBundle gives the share server a one-file SPA bundle for /desktop/.
func useTestAppBundle(t *testing.T) {
	t.Helper()
	old := getStaticAppFS()
	SetStaticAppFS(fstest.MapFS{"index.html": {Data: []byte(desktopTestIndex)}})
	t.Cleanup(func() { SetStaticAppFS(old) })
}

// desktopEnv is a test daemon with the desktop seams on loopback and a
// synthetic capture source.
type desktopEnv struct {
	api    *API
	client *DaemonClient
	media  int

	mu      sync.Mutex
	sources []*screencap.SyntheticSource
}

func (e *desktopEnv) newSource() screencap.Source {
	s := screencap.NewSyntheticSource(1280, 800)
	e.mu.Lock()
	e.sources = append(e.sources, s)
	e.mu.Unlock()
	return s
}

func newDesktopEnv(t *testing.T) *desktopEnv {
	t.Helper()
	desktop.AllowLoopbackForTest()
	useTestAppBundle(t)
	api, client := shareTestDaemon(t)
	e := &desktopEnv{api: api, client: client, media: freeUDPPort(t)}
	api.SetDesktopSeamsForTest(loopbackAddrs, e.newSource, e.media)
	return e
}

func (e *desktopEnv) configPath() string {
	return filepath.Join(e.api.engine.configDir, desktop.ConfigFile)
}

func (e *desktopEnv) mustOn(t *testing.T) DesktopResponse {
	t.Helper()
	resp, err := e.client.SetDesktopAlwaysAvailable(true)
	if err != nil {
		t.Fatalf("SetDesktopAlwaysAvailable(true): %v", err)
	}
	if !resp.AlwaysAvailable || !strings.Contains(resp.URL, "/desktop/?cap=") {
		t.Fatalf("desktop-on response = %+v; want always-available with a /desktop/?cap= link", resp)
	}
	return resp
}

// desktopViewer is a pion viewer attached through /desktop/ws.
type desktopViewer struct {
	conn  *websocket.Conn
	pc    *webrtc.PeerConnection
	aus   chan []byte
	done  chan struct{} // closed when the signalling socket ends
	code  atomic.Int32  // the close status, -1 when it was not a close frame
	state atomic.Value  // last state message, map[string]any
}

func nalTypes(b []byte) []int {
	var out []int
	for i := 0; i+3 < len(b); i++ {
		if b[i] == 0 && b[i+1] == 0 && b[i+2] == 1 {
			out = append(out, int(b[i+3]&0x1f))
			i += 3
		}
	}
	return out
}

func containsNAL(types []int, want int) bool {
	for _, g := range types {
		if g == want {
			return true
		}
	}
	return false
}

func desktopWSURL(link string) (wsURL, origin string, err error) {
	u, err := url.Parse(link)
	if err != nil {
		return "", "", err
	}
	return "wss://" + u.Host + "/desktop/ws?cap=" + url.QueryEscape(u.Query().Get("cap")), "https://" + u.Host, nil
}

func insecureClient() *http.Client {
	return &http.Client{
		Timeout:   10 * time.Second,
		Transport: &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: true}}, //nolint:gosec // self-signed loopback test server
	}
}

// dialDesktop opens /desktop/ws with a browser-shaped Origin.
func dialDesktop(ctx context.Context, link string) (*websocket.Conn, *http.Response, error) {
	wsURL, origin, err := desktopWSURL(link)
	if err != nil {
		return nil, nil, err
	}
	return websocket.Dial(ctx, wsURL, &websocket.DialOptions{
		HTTPClient: insecureClient(),
		HTTPHeader: http.Header{"Origin": {origin}},
	})
}

func readWSType(ctx context.Context, t *testing.T, c *websocket.Conn, typ string) map[string]any {
	t.Helper()
	for {
		_, b, err := c.Read(ctx)
		if err != nil {
			t.Fatalf("waiting for %q message: %v", typ, err)
		}
		var m map[string]any
		if err := json.Unmarshal(b, &m); err != nil {
			t.Fatalf("non-JSON frame: %v", err)
		}
		if m["type"] == typ {
			return m
		}
	}
}

func writeWS(ctx context.Context, t *testing.T, c *websocket.Conn, v any) {
	t.Helper()
	b, _ := json.Marshal(v)
	if err := c.Write(ctx, websocket.MessageText, b); err != nil {
		t.Fatal(err)
	}
}

func newPionViewer(t *testing.T) (*webrtc.PeerConnection, chan []byte) {
	t.Helper()
	se := webrtc.SettingEngine{}
	se.SetIncludeLoopbackCandidate(true)
	se.SetIPFilter(func(ip net.IP) bool { return ip.IsLoopback() })
	se.SetNetworkTypes([]webrtc.NetworkType{webrtc.NetworkTypeUDP4})
	se.SetICEMulticastDNSMode(ice.MulticastDNSModeDisabled)
	me := &webrtc.MediaEngine{}
	if err := me.RegisterDefaultCodecs(); err != nil {
		t.Fatal(err)
	}
	pc, err := webrtc.NewAPI(webrtc.WithSettingEngine(se), webrtc.WithMediaEngine(me)).NewPeerConnection(webrtc.Configuration{})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = pc.Close() })
	if _, err := pc.AddTransceiverFromKind(webrtc.RTPCodecTypeVideo, webrtc.RTPTransceiverInit{Direction: webrtc.RTPTransceiverDirectionRecvonly}); err != nil {
		t.Fatal(err)
	}
	aus := make(chan []byte, 256)
	pc.OnTrack(func(track *webrtc.TrackRemote, _ *webrtc.RTPReceiver) {
		var h264 codecs.H264Packet
		var au []byte
		for {
			pkt, _, err := track.ReadRTP()
			if err != nil {
				return
			}
			out, err := h264.Unmarshal(pkt.Payload)
			if err != nil {
				continue
			}
			au = append(au, out...)
			if pkt.Marker {
				select {
				case aus <- au:
				default:
				}
				au = nil
			}
		}
	})
	return pc, aus
}

// connectViewer runs hello, offer and answer for one viewer and keeps reading
// the signalling socket so a close code is observed.
func connectViewer(t *testing.T, link string) *desktopViewer {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	conn, _, err := dialDesktop(ctx, link)
	if err != nil {
		t.Fatalf("dial /desktop/ws: %v", err)
	}
	writeWS(ctx, t, conn, map[string]any{"type": "hello", "v": 1, "caps": map[string]int{"video": 1}})
	readWSType(ctx, t, conn, "hello")

	pc, aus := newPionViewer(t)
	off, err := pc.CreateOffer(nil)
	if err != nil {
		t.Fatal(err)
	}
	gathered := webrtc.GatheringCompletePromise(pc)
	if err := pc.SetLocalDescription(off); err != nil {
		t.Fatal(err)
	}
	<-gathered
	writeWS(ctx, t, conn, map[string]any{"type": "offer", "sdp": pc.LocalDescription().SDP})
	answer := readWSType(ctx, t, conn, "answer")
	if err := pc.SetRemoteDescription(webrtc.SessionDescription{Type: webrtc.SDPTypeAnswer, SDP: answer["sdp"].(string)}); err != nil {
		t.Fatal(err)
	}

	v := &desktopViewer{conn: conn, pc: pc, aus: aus, done: make(chan struct{})}
	v.code.Store(-1)
	go func() {
		defer close(v.done)
		for {
			_, b, err := conn.Read(context.Background())
			if err != nil {
				if cs := websocket.CloseStatus(err); cs != -1 {
					v.code.Store(int32(cs))
				}
				return
			}
			var m map[string]any
			if json.Unmarshal(b, &m) == nil && m["type"] == "state" {
				v.state.Store(m)
			}
		}
	}()
	t.Cleanup(func() { _ = conn.CloseNow() })
	return v
}

// waitIDR waits for an access unit carrying an IDR slice.
func (v *desktopViewer) waitIDR(t *testing.T, within time.Duration) {
	t.Helper()
	deadline := time.After(within)
	for {
		select {
		case au := <-v.aus:
			if containsNAL(nalTypes(au), 5) {
				return
			}
		case <-deadline:
			t.Fatalf("no keyframe within %v", within)
		}
	}
}

func eventually(t *testing.T, d time.Duration, what string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(d)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for %s", what)
}

func desktopPageStatus(t *testing.T, link string) (int, string) {
	t.Helper()
	req, err := http.NewRequest(http.MethodGet, link, nil)
	if err != nil {
		t.Fatal(err)
	}
	c := insecureClient()
	c.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	resp, err := c.Do(req)
	if err != nil {
		t.Fatalf("GET %s: %v", link, err)
	}
	defer func() { _ = resp.Body.Close() }()
	body, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, resp.Header.Get("X-Portty-Desktop") + "|" + string(body)
}

func (e *desktopEnv) status(t *testing.T) DesktopStatusResponse {
	t.Helper()
	st, err := e.client.DesktopStatus()
	if err != nil {
		t.Fatalf("DesktopStatus: %v", err)
	}
	return st
}

// TestDesktopAPI_OnLinkStreamTracer is Task 1's tracer: PUT /desktop on writes
// desktop.json, starts the share server and the Manager, returns a link, and the
// link opens the page and streams a keyframe from the synthetic source.
func TestDesktopAPI_OnLinkStreamTracer(t *testing.T) {
	e := newDesktopEnv(t)
	resp := e.mustOn(t)
	if !resp.FirstShare {
		t.Errorf("FirstShare = false on the first share of any kind, want true")
	}

	fi, err := os.Stat(e.configPath())
	if err != nil {
		t.Fatalf("desktop.json not written: %v", err)
	}
	if fi.Mode().Perm() != 0o600 {
		t.Errorf("desktop.json mode %o, want 600", fi.Mode().Perm())
	}
	cfg, err := desktop.LoadConfig(e.api.engine.configDir)
	if err != nil || !cfg.AlwaysAvailable || len(cfg.GrantID) != 32 || cfg.IAT == 0 || cfg.SchemaVersion != 1 {
		t.Fatalf("desktop.json = %+v, %v; want schema 1, alwaysAvailable, a 128-bit hex grant and iat", cfg, err)
	}
	if shareServerRunning(e.api) == false {
		t.Fatal("share server is not running after desktop on")
	}
	if got, err := e.client.DesktopLink(); err != nil || got != resp.URL {
		t.Errorf("GET /desktop/link = %q, %v; want the same URL as the PUT answer %q", got, err, resp.URL)
	}

	if code, body := desktopPageStatus(t, resp.URL); code != http.StatusOK || !strings.HasSuffix(body, desktopTestIndex) {
		t.Fatalf("GET link = %d %q; want 200 and the SPA index", code, body)
	}

	v := connectViewer(t, resp.URL)
	v.waitIDR(t, 10*time.Second)
	eventually(t, 3*time.Second, "status to say streaming with 1 viewer", func() bool {
		st := e.status(t)
		return st.AlwaysAvailable && st.State == "streaming" && st.Viewers == 1
	})
	if st := e.status(t); st.Tailscale != "ok" {
		t.Errorf("status tailscale = %q, want ok", st.Tailscale)
	}
}

// --- Task 2 helpers --------------------------------------------------------

func freeTCPPort(t *testing.T) int {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = l.Close() }()
	return l.Addr().(*net.TCPAddr).Port
}

func (e *desktopEnv) lastSource() *screencap.SyntheticSource {
	e.mu.Lock()
	defer e.mu.Unlock()
	if len(e.sources) == 0 {
		return nil
	}
	return e.sources[len(e.sources)-1]
}

// waitClosed waits for the signalling socket to end and returns its close code.
func (v *desktopViewer) waitClosed(t *testing.T, within time.Duration) int {
	t.Helper()
	select {
	case <-v.done:
		return int(v.code.Load())
	case <-time.After(within):
		t.Fatalf("viewer socket still open after %v", within)
		return 0
	}
}

// fixedPortShareServer is loopbackShareServer on a chosen port, so two daemons
// built one after the other serve the same host:port (what a restart does with
// the real fixed port 7444).
func fixedPortShareServer(port int, built *atomic.Int32) func(webserver.TailscaleHealth, *relay.HubManager) (*webserver.WebServer, error) {
	return func(_ webserver.TailscaleHealth, m *relay.HubManager) (*webserver.WebServer, error) {
		built.Add(1)
		cfg, err := webserver.GenerateSelfSignedCert("127.0.0.1")
		if err != nil {
			return nil, err
		}
		ws, err := webserver.NewWebServer(webserver.Config{BindIP: "127.0.0.1", Port: port, FQDN: "127.0.0.1", TLSConfig: cfg}, m)
		if err != nil {
			return nil, err
		}
		ws.SetWhoIsClientForTest(webservertest.SameUser())
		return ws, nil
	}
}

// manualDesktopAPI builds a daemon on an existing config dir with its seams set
// BEFORE Start, as a restarted daemon has them, and starts it.
func manualDesktopAPI(t *testing.T, dir string, health func(context.Context) webserver.TailscaleHealth, newServer func(webserver.TailscaleHealth, *relay.HubManager) (*webserver.WebServer, error), media int, poll time.Duration) (*API, *DaemonClient, *desktopEnv) {
	t.Helper()
	return manualDesktopAPIAddrs(t, dir, health, newServer, loopbackAddrs, media, poll)
}

func manualDesktopAPIAddrs(t *testing.T, dir string, health func(context.Context) webserver.TailscaleHealth, newServer func(webserver.TailscaleHealth, *relay.HubManager) (*webserver.WebServer, error), addrs func() ([]net.Addr, error), media int, poll time.Duration) (*API, *DaemonClient, *desktopEnv) {
	t.Helper()
	desktop.AllowLoopbackForTest()
	useTestAppBundle(t)
	t.Setenv("HOME", t.TempDir())
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	engine := NewSessionEngine()
	engine.configDir = dir
	engine.SetSpawnCommandForTest("/bin/cat", nil)
	api := NewAPI(engine)
	if err := api.BootstrapCapabilityState(); err != nil {
		t.Fatal(err)
	}
	env := &desktopEnv{api: api, media: media}
	api.SetShareSeamsForTest(health, newServer, noWarm)
	api.SetDesktopSeamsForTest(addrs, env.newSource, media)
	if poll > 0 {
		api.desktopPollEvery = poll
	}
	sock := shortSocketPath(t, "m.sock")
	if err := api.Start(sock); err != nil {
		t.Fatalf("api.Start: %v", err)
	}
	t.Cleanup(func() { _ = api.Stop() })
	t.Cleanup(func() { killAllSessions(engine) })
	env.client = NewDaemonClient(sock)
	time.Sleep(10 * time.Millisecond)
	return api, env.client, env
}

func desktopUp(api *API) bool {
	api.mu.RLock()
	defer api.mu.RUnlock()
	return api.webServer != nil && api.desktopMgr != nil
}

func mustSaveConfig(t *testing.T, dir string, c desktop.Config) {
	t.Helper()
	if err := desktop.SaveConfig(dir, c); err != nil {
		t.Fatal(err)
	}
}

// --- Task 2 tests ----------------------------------------------------------

func TestDesktopAPI_Refusals(t *testing.T) {
	cases := []struct {
		name   string
		code   string
		health func(context.Context) webserver.TailscaleHealth
		addrs  func() ([]net.Addr, error)
	}{
		{"tailscale off", ShareErrTailscaleOff, func(context.Context) webserver.TailscaleHealth { return webserver.TailscaleHealth{} }, loopbackAddrs},
		{"https off", ShareErrHTTPSOff, func(context.Context) webserver.TailscaleHealth {
			return webserver.TailscaleHealth{Connected: true, IP: "127.0.0.1"}
		}, loopbackAddrs},
		{"userspace", ShareErrUserspace, healthyTailnet, func() ([]net.Addr, error) {
			return []net.Addr{&net.IPNet{IP: net.IPv4(192, 168, 1, 5), Mask: net.CIDRMask(24, 32)}}, nil
		}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			e := newDesktopEnv(t)
			e.api.SetShareSeamsForTest(c.health, loopbackShareServer, noWarm)
			e.api.SetDesktopSeamsForTest(c.addrs, e.newSource, e.media)
			_, err := e.client.SetDesktopAlwaysAvailable(true)
			var serr *ShareError
			if !errors.As(err, &serr) || serr.Code != c.code {
				t.Fatalf("PUT /desktop on = %v; want a ShareError with code %q", err, c.code)
			}
			if _, statErr := os.Stat(e.configPath()); !errors.Is(statErr, os.ErrNotExist) {
				t.Errorf("desktop.json exists after a refusal (stat err %v)", statErr)
			}
			if shareServerRunning(e.api) {
				t.Error("share server is running after a refusal")
			}
			if st := e.status(t); st.AlwaysAvailable || st.State != "off" {
				t.Errorf("status after a refusal = %+v; want off", st)
			}
		})
	}
}

func TestDesktopAPI_OffClosesViewers(t *testing.T) {
	e := newDesktopEnv(t)
	// A terminal share keeps the server up, so the old link can still be asked.
	id := newShareSession(t, e.client)
	termLink := mustShare(t, e.client, id)
	on := e.mustOn(t)

	v := connectViewer(t, on.URL)
	v.waitIDR(t, 10*time.Second)
	src := e.lastSource()

	if _, err := e.client.SetDesktopAlwaysAvailable(false); err != nil {
		t.Fatalf("PUT /desktop off: %v", err)
	}
	if code := v.waitClosed(t, 10*time.Second); code != 4410 {
		t.Errorf("viewer closed with %d, want 4410", code)
	}
	cfg, err := desktop.LoadConfig(e.api.engine.configDir)
	if err != nil || cfg.AlwaysAvailable || cfg.GrantID != "" {
		t.Errorf("desktop.json after off = %+v, %v; want alwaysAvailable false and no grantID", cfg, err)
	}
	if code, body := desktopPageStatus(t, on.URL); code != http.StatusGone || !strings.HasPrefix(body, "off|") {
		t.Errorf("old link after off = %d %q; want 410 with X-Portty-Desktop: off", code, body)
	}
	eventually(t, 3*time.Second, "the capture source to stop", func() bool {
		select {
		case _, ok := <-src.Frames():
			return !ok
		default:
			return false
		}
	})
	if !shareServerRunning(e.api) {
		t.Error("desktop off stopped the server a terminal share still needs")
	}
	if code, _ := linkStatus(t, termLink.URL); code != http.StatusFound {
		t.Errorf("terminal link after desktop off = %d, want 302", code)
	}
	if _, err := e.client.DesktopLink(); err == nil {
		t.Error("GET /desktop/link after off succeeded, want an error")
	}
	if st := e.status(t); st.AlwaysAvailable || st.State != "off" {
		t.Errorf("status after off = %+v; want off", st)
	}
}

func TestDesktopAPI_OffStopsServerWhenNothingElseNeedsIt(t *testing.T) {
	e := newDesktopEnv(t)
	e.mustOn(t)
	if _, err := e.client.SetDesktopAlwaysAvailable(false); err != nil {
		t.Fatal(err)
	}
	if shareServerRunning(e.api) {
		t.Error("server still running with the desktop off and no terminal share")
	}
	if _, err := e.client.SetDesktopAlwaysAvailable(false); err != nil {
		t.Errorf("a second off = %v; want it harmless", err)
	}
}

func TestDesktopAPI_RegenerateKey(t *testing.T) {
	e := newDesktopEnv(t)
	old := e.mustOn(t)
	v := connectViewer(t, old.URL)
	v.waitIDR(t, 10*time.Second)

	if err := e.client.RegenerateSigningKey(); err != nil {
		t.Fatalf("RegenerateSigningKey: %v", err)
	}
	if code := v.waitClosed(t, 10*time.Second); code != 4401 {
		t.Errorf("viewer closed with %d, want 4401", code)
	}
	if code, body := desktopPageStatus(t, old.URL); code != http.StatusUnauthorized || !strings.HasPrefix(body, "token|") {
		t.Errorf("old link after regeneration = %d %q; want 401 with X-Portty-Desktop: token", code, body)
	}
	fresh, err := e.client.DesktopLink()
	if err != nil || fresh == old.URL {
		t.Fatalf("Copy Link after regeneration = %q, %v; want a new URL", fresh, err)
	}
	if code, _ := desktopPageStatus(t, fresh); code != http.StatusOK {
		t.Errorf("new link = %d, want 200", code)
	}
}

func TestDesktopAPI_LinkStableAcrossRestart(t *testing.T) {
	e := newDesktopEnv(t)
	var built atomic.Int32
	port := freeTCPPort(t)
	e.api.SetShareSeamsForTest(healthyTailnet, fixedPortShareServer(port, &built), noWarm)
	a := e.mustOn(t)
	dir := e.api.engine.configDir
	if err := e.api.Stop(); err != nil {
		t.Fatal(err)
	}

	// Daemon B: same config dir (capability.key, desktop.json), same address.
	b, bClient, _ := manualDesktopAPI(t, dir, healthyTailnet, fixedPortShareServer(port, &built), e.media, 0)
	eventually(t, 10*time.Second, "B's poller to bring the desktop up", func() bool { return desktopUp(b) })
	got, err := bClient.DesktopLink()
	if err != nil {
		t.Fatal(err)
	}
	if got != a.URL {
		t.Fatalf("link after restart differs from the saved link:\n before %s\n after  %s", a.URL, got)
	}
	if code, _ := desktopPageStatus(t, a.URL); code != http.StatusOK {
		t.Errorf("saved link on the restarted daemon = %d, want 200", code)
	}
}

func TestDesktopAPI_PollerStartsAtDaemonStart(t *testing.T) {
	dir := t.TempDir()
	mustSaveConfig(t, dir, desktop.Config{AlwaysAvailable: true, GrantID: "0123456789abcdef0123456789abcdef", IAT: 1_700_000_000})
	var (
		connected atomic.Bool
		domain    atomic.Value
		built     atomic.Int32
	)
	domain.Store("127.0.0.1")
	health := func(context.Context) webserver.TailscaleHealth {
		if !connected.Load() {
			return webserver.TailscaleHealth{}
		}
		return webserver.TailscaleHealth{Connected: true, HasCerts: true, IP: "127.0.0.1", Domain: domain.Load().(string)}
	}
	port := freeTCPPort(t)
	api, client, _ := manualDesktopAPI(t, dir, health, fixedPortShareServer(port, &built), freeUDPPort(t), 20*time.Millisecond)

	// Tailscale is down: several ticks pass and nothing starts.
	time.Sleep(150 * time.Millisecond)
	if shareServerRunning(api) || desktopUp(api) || built.Load() != 0 {
		t.Fatal("something started while Tailscale was off")
	}
	if st, err := client.DesktopStatus(); err != nil || !st.AlwaysAvailable || st.Tailscale != "off" || st.State != "idle" {
		t.Fatalf("status while Tailscale is off = %+v, %v; want always-available, idle, tailscale off", st, err)
	}

	connected.Store(true)
	eventually(t, 5*time.Second, "the poller to start the server and the Manager", func() bool { return desktopUp(api) })
	link, err := client.DesktopLink()
	if err != nil {
		t.Fatal(err)
	}
	if code, _ := desktopPageStatus(t, link); code != http.StatusOK {
		t.Fatalf("saved link after the poller started it = %d, want 200", code)
	}

	// The tailnet name changes: both are rebuilt and the link still opens.
	api.mu.RLock()
	oldMgr := api.desktopMgr
	api.mu.RUnlock()
	domain.Store("renamed.example")
	eventually(t, 5*time.Second, "the poller to restart both", func() bool {
		api.mu.RLock()
		defer api.mu.RUnlock()
		return api.desktopMgr != nil && api.desktopMgr != oldMgr && api.webServer != nil
	})
	if built.Load() != 2 {
		t.Errorf("share server built %d times, want 2 (start, then the restart)", built.Load())
	}
	if code, _ := desktopPageStatus(t, link); code != http.StatusOK {
		t.Errorf("saved link after the restart = %d, want 200", code)
	}
}

func TestDesktopAPI_PollerIgnoresOffAndUserspace(t *testing.T) {
	var asked atomic.Int32
	health := func(ctx context.Context) webserver.TailscaleHealth {
		asked.Add(1)
		return healthyTailnet(ctx)
	}
	// Off: the poller never asks Tailscale anything.
	api, _, _ := manualDesktopAPI(t, t.TempDir(), health, loopbackShareServer, freeUDPPort(t), 10*time.Millisecond)
	time.Sleep(100 * time.Millisecond)
	if asked.Load() != 0 || shareServerRunning(api) {
		t.Fatalf("poller with the desktop off asked health %d times / started a server", asked.Load())
	}

	// On, but the address is on no interface: recorded, nothing starts.
	dir2 := t.TempDir()
	mustSaveConfig(t, dir2, desktop.Config{AlwaysAvailable: true, GrantID: "ab", IAT: 1})
	noAddrs := func() ([]net.Addr, error) { return nil, nil }
	api2, client2, _ := manualDesktopAPIAddrs(t, dir2, healthyTailnet, loopbackShareServer, noAddrs, freeUDPPort(t), 10*time.Millisecond)
	eventually(t, 3*time.Second, "userspace to be recorded", func() bool {
		st, err := client2.DesktopStatus()
		return err == nil && st.Tailscale == "userspace"
	})
	if shareServerRunning(api2) || desktopUp(api2) {
		t.Error("the poller started the desktop on a userspace Tailscale")
	}
}

func TestDesktopAPI_ViewerCapRefusedBeforeUpgrade(t *testing.T) {
	e := newDesktopEnv(t)
	on := e.mustOn(t)
	viewers := make([]*desktopViewer, 0, desktop.MaxViewers)
	for i := 0; i < desktop.MaxViewers; i++ {
		viewers = append(viewers, connectViewer(t, on.URL))
	}
	eventually(t, 5*time.Second, "the status to say 8 viewers", func() bool { return e.status(t).Viewers == desktop.MaxViewers })

	// A ninth, with a valid token and Origin, as a browser would send it.
	wsURL, origin, err := desktopWSURL(on.URL)
	if err != nil {
		t.Fatal(err)
	}
	req, err := http.NewRequest(http.MethodGet, strings.Replace(wsURL, "wss://", "https://", 1), nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Connection", "Upgrade")
	req.Header.Set("Upgrade", "websocket")
	req.Header.Set("Sec-WebSocket-Version", "13")
	req.Header.Set("Sec-WebSocket-Key", "dGhlIHNhbXBsZSBub25jZQ==")
	req.Header.Set("Origin", origin)
	resp, err := insecureClient().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusTooManyRequests || resp.Header.Get("Upgrade") != "" {
		t.Fatalf("ninth viewer = %d (Upgrade %q); want 429 with no upgrade", resp.StatusCode, resp.Header.Get("Upgrade"))
	}
	// The eight keep streaming and the count is unchanged.
	for i, v := range viewers {
		select {
		case <-v.aus:
		case <-time.After(5 * time.Second):
			t.Fatalf("viewer %d stopped receiving video", i)
		}
	}
	if st := e.status(t); st.Viewers != desktop.MaxViewers || st.State != "streaming" {
		t.Errorf("status = %+v; want streaming with 8 viewers", st)
	}
}

func TestDesktopAPI_StatusReportsLaunchd(t *testing.T) {
	e := newDesktopEnv(t)
	old := launchdManaged
	t.Cleanup(func() { launchdManaged = old })
	for _, managed := range []bool{true, false} {
		launchdManaged = managed
		st := e.status(t)
		if st.Launchd != managed {
			t.Errorf("launchd = %v with the managed flag %v", st.Launchd, managed)
		}
		if st.AlwaysAvailable || st.State != "off" {
			t.Errorf("status %+v: the launchd flag changed alwaysAvailable or state", st)
		}
	}
	e.mustOn(t)
	launchdManaged = false
	if st := e.status(t); st.Launchd || !st.AlwaysAvailable || st.State != "idle" {
		t.Errorf("status of a fallback-started daemon with the desktop on = %+v; want launchd false, on, idle", st)
	}
}

func TestDesktopAPI_BadConfigIsExplicit(t *testing.T) {
	for _, tc := range []struct {
		name string
		body string
		mode os.FileMode
	}{
		{"malformed JSON", `{"schemaVersion":1,`, 0o600},
		{"wrong mode", `{"schemaVersion":1,"alwaysAvailable":true,"grantID":"ab","iat":1}`, 0o644},
		{"unknown schema", `{"schemaVersion":7,"alwaysAvailable":true,"grantID":"ab","iat":1}`, 0o600},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			p := filepath.Join(dir, desktop.ConfigFile)
			if err := os.WriteFile(p, []byte(tc.body), tc.mode); err != nil {
				t.Fatal(err)
			}
			if err := os.Chmod(p, tc.mode); err != nil {
				t.Fatal(err)
			}
			api, client, _ := manualDesktopAPI(t, dir, healthyTailnet, loopbackShareServer, freeUDPPort(t), 10*time.Millisecond)

			st, err := client.DesktopStatus()
			if err != nil {
				t.Fatal(err)
			}
			if st.AlwaysAvailable || st.Reason == "" {
				t.Fatalf("status = %+v; want always-available off with a reason", st)
			}
			// Turning it on does not replace the file either.
			if _, err := client.SetDesktopAlwaysAvailable(true); err == nil {
				t.Error("PUT on with an unusable desktop.json succeeded")
			}
			time.Sleep(60 * time.Millisecond) // several poller ticks
			if shareServerRunning(api) || desktopUp(api) {
				t.Error("something started with an unusable desktop.json")
			}
			got, err := os.ReadFile(p)
			if err != nil || string(got) != tc.body {
				t.Errorf("desktop.json was rewritten: %q, %v", got, err)
			}
			if fi, _ := os.Stat(p); fi.Mode().Perm() != tc.mode {
				t.Errorf("desktop.json mode changed to %o", fi.Mode().Perm())
			}
		})
	}
}

// 19-13 finding: a userspace refusal recorded at a turn-on attempt must not
// stick while the desktop stays off; the next poll rechecks it, so a fixed
// Tailscale unblocks the switch without a daemon restart. Nothing starts.
func TestDesktopAPI_UserspaceRefusalClearsWhileOff(t *testing.T) {
	e := newDesktopEnv(t)
	e.api.SetShareSeamsForTest(healthyTailnet, loopbackShareServer, noWarm)
	e.api.SetDesktopSeamsForTest(func() ([]net.Addr, error) {
		return []net.Addr{&net.IPNet{IP: net.IPv4(192, 168, 1, 5), Mask: net.CIDRMask(24, 32)}}, nil
	}, e.newSource, e.media)
	if _, err := e.client.SetDesktopAlwaysAvailable(true); err == nil {
		t.Fatal("PUT /desktop on succeeded on a userspace Tailscale")
	}
	if st := e.status(t); st.Tailscale != "userspace" {
		t.Fatalf("Tailscale = %q after the refusal; want userspace", st.Tailscale)
	}

	e.api.SetDesktopSeamsForTest(loopbackAddrs, e.newSource, e.media) // the user fixed Tailscale
	e.api.desktopTick(context.Background())
	st := e.status(t)
	if st.Tailscale != "ok" {
		t.Errorf("Tailscale = %q after a poll with the address local; want ok", st.Tailscale)
	}
	if st.AlwaysAvailable || st.State != "off" || shareServerRunning(e.api) {
		t.Errorf("the recheck turned something on: %+v, server running %v", st, shareServerRunning(e.api))
	}
}
