package desktop_test

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/json"
	"io"
	"math/big"
	"net"
	"net/http"
	"net/netip"
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

	"github.com/scottkw/portty/internal/capability"
	"github.com/scottkw/portty/internal/desktop"
	"github.com/scottkw/portty/internal/relay"
	"github.com/scottkw/portty/internal/screencap"
	"github.com/scottkw/portty/internal/webserver"
	"github.com/scottkw/portty/internal/webserver/webservertest"
)

const indexHTML = "<!doctype html><div id=root></div>"

// testBackend is the daemon's DesktopBackend in miniature: a fixed grant that
// can be switched off, delegating viewers to the Manager and counting how many
// sockets ever reached it.
type testBackend struct {
	mgr   *desktop.Manager
	grant string
	off   atomic.Bool
	calls atomic.Int32
}

func (b *testBackend) DesktopGrant() (string, bool) { return b.grant, !b.off.Load() }

func (b *testBackend) ServeDesktopViewer(ctx context.Context, c *websocket.Conn) {
	b.calls.Add(1)
	b.mgr.ServeDesktopViewer(ctx, c)
}

// tracerEnv is a loopback share server with a desktop Manager behind it.
type tracerEnv struct {
	ws      *webserver.WebServer
	mgr     *desktop.Manager
	backend *testBackend
	key     []byte
	base    string // https://127.0.0.1:<port>
	client  *http.Client

	mu      sync.Mutex
	sources []*screencap.SyntheticSource
}

func (e *tracerEnv) lastSource() *screencap.SyntheticSource {
	e.mu.Lock()
	defer e.mu.Unlock()
	if len(e.sources) == 0 {
		return nil
	}
	return e.sources[len(e.sources)-1]
}

func (e *tracerEnv) token(t *testing.T, grant string) string {
	t.Helper()
	tok, err := capability.SignDesktop(capability.DesktopClaims{
		Kind: capability.DesktopTokenKind, Mode: capability.DesktopModeAlways,
		GrantID: grant, IAT: time.Now().Unix(), V: 1,
	}, e.key)
	if err != nil {
		t.Fatal(err)
	}
	return tok
}

func (e *tracerEnv) wsURL(tok string) string {
	return "wss" + strings.TrimPrefix(e.base, "https") + "/desktop/ws?cap=" + tok
}

func (e *tracerEnv) dial(ctx context.Context, tok string) (*websocket.Conn, *http.Response, error) {
	return websocket.Dial(ctx, e.wsURL(tok), &websocket.DialOptions{
		HTTPClient: e.client,
		HTTPHeader: http.Header{"Origin": {e.base}},
	})
}

func selfSignedTLS(t *testing.T) *tls.Config {
	t.Helper()
	k, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "127.0.0.1"},
		IPAddresses:  []net.IP{net.ParseIP("127.0.0.1")},
		NotBefore:    time.Now().Add(-time.Minute),
		NotAfter:     time.Now().Add(time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &k.PublicKey, k)
	if err != nil {
		t.Fatal(err)
	}
	return &tls.Config{
		Certificates: []tls.Certificate{{Certificate: [][]byte{der}, PrivateKey: k}},
		MinVersion:   tls.VersionTLS12,
	}
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

// newTracerServer starts a loopback share server (self-signed cert, fake
// same-user WhoIs, a one-file app bundle) and a Manager on a synthetic source.
func newTracerServer(t *testing.T) *tracerEnv {
	t.Helper()
	desktop.AllowLoopbackForTest()
	env := &tracerEnv{key: make([]byte, 32)}
	if _, err := rand.Read(env.key); err != nil {
		t.Fatal(err)
	}

	ws, err := webserver.NewWebServer(webserver.Config{
		BindIP: "127.0.0.1", Port: 0, FQDN: "127.0.0.1", TLSConfig: selfSignedTLS(t),
	}, relay.NewHubManager())
	if err != nil {
		t.Fatal(err)
	}
	ws.SetWhoIsClientForTest(webservertest.SameUser())
	ws.SetSigningKey(env.key)
	ws.SetStaticAppFS(fstest.MapFS{"index.html": {Data: []byte(indexHTML)}})

	mgr, err := desktop.NewManager(desktop.Deps{
		Bind:      netip.MustParseAddr("127.0.0.1"),
		MediaPort: freeUDPPort(t),
		NewSource: func() screencap.Source {
			s := screencap.NewSyntheticSource(1280, 800)
			env.mu.Lock()
			env.sources = append(env.sources, s)
			env.mu.Unlock()
			return s
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := mgr.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = mgr.Close() })

	env.backend = &testBackend{mgr: mgr, grant: "g1"}
	ws.SetDesktopBackend(env.backend)
	if err := ws.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = ws.Stop() })

	env.ws, env.mgr, env.base = ws, mgr, ws.BaseURL()
	env.client = &http.Client{
		Transport: &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: true}}, //nolint:gosec // loopback test, self-signed
		// A redirect is an answer, not something to follow.
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}
	return env
}

// readType reads signalling messages until one of the wanted type arrives.
func readType(ctx context.Context, t *testing.T, c *websocket.Conn, typ string) map[string]any {
	t.Helper()
	for {
		_, b, err := c.Read(ctx)
		if err != nil {
			t.Fatalf("waiting for %q message: %v", typ, err)
		}
		var m map[string]any
		if err := json.Unmarshal(b, &m); err != nil {
			t.Fatalf("non-JSON server frame: %v", err)
		}
		if m["type"] == typ {
			return m
		}
	}
}

func writeJSON(ctx context.Context, t *testing.T, c *websocket.Conn, v any) {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	if err := c.Write(ctx, websocket.MessageText, b); err != nil {
		t.Fatal(err)
	}
}

// nalTypes lists the NAL unit types in an Annex-B buffer.
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

func hasAll(types []int, want ...int) bool {
	for _, w := range want {
		found := false
		for _, g := range types {
			if g == w {
				found = true
			}
		}
		if !found {
			return false
		}
	}
	return true
}

// loopbackClient is a pion viewer: a recvonly H.264 transceiver that
// depacketizes the first received access units into Annex-B.
type loopbackClient struct {
	pc  *webrtc.PeerConnection
	aus chan []byte
}

func newLoopbackClient(t *testing.T) *loopbackClient {
	t.Helper()
	se := webrtc.SettingEngine{}
	se.SetIncludeLoopbackCandidate(true)
	se.SetNetworkTypes([]webrtc.NetworkType{webrtc.NetworkTypeUDP4})
	se.SetICEMulticastDNSMode(ice.MulticastDNSModeDisabled)
	me := &webrtc.MediaEngine{}
	if err := me.RegisterDefaultCodecs(); err != nil {
		t.Fatal(err)
	}
	api := webrtc.NewAPI(webrtc.WithSettingEngine(se), webrtc.WithMediaEngine(me))
	pc, err := api.NewPeerConnection(webrtc.Configuration{})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = pc.Close() })
	if _, err := pc.AddTransceiverFromKind(webrtc.RTPCodecTypeVideo, webrtc.RTPTransceiverInit{Direction: webrtc.RTPTransceiverDirectionRecvonly}); err != nil {
		t.Fatal(err)
	}
	lc := &loopbackClient{pc: pc, aus: make(chan []byte, 64)}
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
				case lc.aus <- au:
				default:
				}
				au = nil
			}
		}
	})
	return lc
}

// offer returns the complete non-trickle offer SDP.
func (lc *loopbackClient) offer(t *testing.T) string {
	t.Helper()
	off, err := lc.pc.CreateOffer(nil)
	if err != nil {
		t.Fatal(err)
	}
	gathered := webrtc.GatheringCompletePromise(lc.pc)
	if err := lc.pc.SetLocalDescription(off); err != nil {
		t.Fatal(err)
	}
	<-gathered
	return lc.pc.LocalDescription().SDP
}

func (lc *loopbackClient) accept(t *testing.T, sdp string) {
	t.Helper()
	if err := lc.pc.SetRemoteDescription(webrtc.SessionDescription{Type: webrtc.SDPTypeAnswer, SDP: sdp}); err != nil {
		t.Fatal(err)
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

// TestDesktopTracer_LoopbackStream is the phase tracer: a desktop token opens
// the page, the viewer's WebSocket carries hello and a non-trickle offer, pion
// answers with ICE-lite, and RTP with SPS, PPS and an IDR slice arrives over
// loopback from the synthetic source. Closing the viewer stops capture.
func TestDesktopTracer_LoopbackStream(t *testing.T) {
	env := newTracerServer(t)
	tok := env.token(t, "g1")
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// The page: SPA index, no-store, no-referrer.
	resp, err := env.client.Get(env.base + "/desktop/?cap=" + tok)
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("page status %d, want 200", resp.StatusCode)
	}
	if got := resp.Header.Get("Cache-Control"); got != "no-store" {
		t.Errorf("Cache-Control %q, want no-store", got)
	}
	if got := resp.Header.Get("Referrer-Policy"); got != "no-referrer" {
		t.Errorf("Referrer-Policy %q, want no-referrer", got)
	}
	if string(body) != indexHTML {
		t.Errorf("page body %q, want the SPA index", body)
	}

	// Signalling: hello, then a non-trickle offer.
	conn, _, err := env.dial(ctx, tok)
	if err != nil {
		t.Fatalf("dial /desktop/ws: %v", err)
	}
	defer func() { _ = conn.CloseNow() }()
	writeJSON(ctx, t, conn, map[string]any{"type": "hello", "v": 1, "caps": map[string]int{"video": 1}})
	hello := readType(ctx, t, conn, "hello")
	if id, _ := hello["viewer"].(string); len(id) != 16 {
		t.Errorf("viewer id %q, want 16 hex chars", id)
	}

	lc := newLoopbackClient(t)
	writeJSON(ctx, t, conn, map[string]any{"type": "offer", "sdp": lc.offer(t)})
	answer := readType(ctx, t, conn, "answer")
	lc.accept(t, answer["sdp"].(string))

	// The first access unit carries SPS (7), PPS (8) and an IDR slice (5).
	select {
	case au := <-lc.aus:
		if types := nalTypes(au); !hasAll(types, 7, 8, 5) {
			t.Fatalf("first access unit has NAL types %v, no SPS+PPS+IDR (7, 8, 5)", types)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("no access unit within 5s")
	}

	st := readType(ctx, t, conn, "state")
	if st["state"] != "streaming" || st["viewers"] != float64(1) {
		t.Errorf("state message %v, want streaming with 1 viewer", st)
	}
	if env.mgr.Viewers() != 1 {
		t.Errorf("Viewers() = %d, want 1", env.mgr.Viewers())
	}

	// Leaving ends the viewer and stops capture.
	src := env.lastSource()
	_ = conn.Close(websocket.StatusNormalClosure, "")
	_ = lc.pc.Close()
	eventually(t, 2*time.Second, "viewers to reach 0", func() bool { return env.mgr.Viewers() == 0 })
	eventually(t, 2*time.Second, "the source to stop", func() bool {
		select {
		case _, ok := <-src.Frames():
			return !ok
		default:
			return false
		}
	})
}
