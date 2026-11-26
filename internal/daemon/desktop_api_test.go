package daemon

import (
	"context"
	"crypto/tls"
	"encoding/json"
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
	"github.com/scottkw/portty/internal/screencap"
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
