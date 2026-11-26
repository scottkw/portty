//go:build e2e

package webserver_test

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"math/big"
	"net"
	"net/http"
	"net/netip"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/chromedp/cdproto/page"
	"github.com/chromedp/chromedp"
	"github.com/coder/websocket"

	"github.com/scottkw/portty/internal/capability"
	"github.com/scottkw/portty/internal/desktop"
	"github.com/scottkw/portty/internal/relay"
	"github.com/scottkw/portty/internal/screencap"
	"github.com/scottkw/portty/internal/webserver"
	"github.com/scottkw/portty/internal/webserver/webservertest"
)

// e2eBackend is the daemon's DesktopBackend in miniature: one fixed grant,
// viewers handed to the Manager.
type e2eBackend struct{ mgr *desktop.Manager }

func (b e2eBackend) DesktopGrant() (string, bool) { return "g1", true }
func (b e2eBackend) ServeDesktopViewer(ctx context.Context, c *websocket.Conn, admit func() bool) {
	b.mgr.ServeDesktopViewer(ctx, c, admit)
}

func e2eSelfSignedTLS(t *testing.T) *tls.Config {
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

func e2eFreeUDPPort(t *testing.T) int {
	t.Helper()
	c, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1)})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = c.Close() }()
	return c.LocalAddr().(*net.UDPAddr).Port
}

// TestDesktopCSP_E2E loads the built /desktop/ page in headless Chrome under the
// real share-server CSP header, against a synthetic loopback source. It proves
// the page boots DesktopView, opens the signalling socket, negotiates a
// PeerConnection and receives RTP bytes, with no securitypolicyviolation.
// Decode is not asserted: the synthetic fixture is not decodable, real frames
// are proven in the VMs (19-12).
func TestDesktopCSP_E2E(t *testing.T) {
	dist, err := filepath.Abs("../../frontend/dist")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dist, "index.html")); err != nil {
		t.Skip("frontend/dist/index.html is missing: run pnpm --dir frontend build first")
	}
	desktop.AllowLoopbackForTest()

	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		t.Fatal(err)
	}
	ws, err := webserver.NewWebServer(webserver.Config{
		BindIP: "127.0.0.1", Port: 0, FQDN: "127.0.0.1", TLSConfig: e2eSelfSignedTLS(t),
	}, relay.NewHubManager())
	if err != nil {
		t.Fatal(err)
	}
	ws.SetWhoIsClientForTest(webservertest.SameUser())
	ws.SetSigningKey(key)
	ws.SetStaticAppFS(os.DirFS(dist))

	mgr, err := desktop.NewManager(desktop.Deps{
		Bind:      netip.MustParseAddr("127.0.0.1"),
		MediaPort: e2eFreeUDPPort(t),
		NewSource: func() screencap.Source { return screencap.NewSyntheticSource(1280, 800) },
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := mgr.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = mgr.Close() })
	ws.SetDesktopBackend(e2eBackend{mgr: mgr})
	if err := ws.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = ws.Stop() })

	tok, err := capability.SignDesktop(capability.DesktopClaims{
		Kind: capability.DesktopTokenKind, Mode: capability.DesktopModeAlways,
		GrantID: "g1", IAT: time.Now().Unix(), V: 1,
	}, key)
	if err != nil {
		t.Fatal(err)
	}
	link := ws.BaseURL() + "/desktop/?cap=" + tok

	// The page's own CSP header must be the /app/ one, byte for byte.
	client := &http.Client{Transport: &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: true}}} //nolint:gosec // loopback, self-signed
	csp := func(u string) string {
		res, err := client.Get(u)
		if err != nil {
			t.Fatal(err)
		}
		defer func() { _ = res.Body.Close() }()
		return res.Header.Get("Content-Security-Policy")
	}
	got := csp(link)
	if got == "" || got != csp(ws.BaseURL()+"/app/") {
		t.Fatalf("/desktop/ CSP %q differs from /app/ CSP %q", got, csp(ws.BaseURL()+"/app/"))
	}

	opts := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.Flag("headless", true),
		chromedp.Flag("ignore-certificate-errors", true),
	)
	allocCtx, cancelAlloc := chromedp.NewExecAllocator(context.Background(), opts...)
	defer cancelAlloc()
	ctx, cancelCtx := chromedp.NewContext(allocCtx)
	defer cancelCtx()
	ctx, cancelTimeout := context.WithTimeout(ctx, 45*time.Second)
	defer cancelTimeout()

	initScript := `
      window.__cspViolations = [];
      document.addEventListener('securitypolicyviolation', (e) => {
        window.__cspViolations.push({ directive: e.violatedDirective, blockedURI: e.blockedURI });
      });
    `
	var violations []map[string]interface{}
	var bytesReceived float64
	err = chromedp.Run(ctx,
		chromedp.ActionFunc(func(ctx context.Context) error {
			_, err := page.AddScriptToEvaluateOnNewDocument(initScript).Do(ctx)
			return err
		}),
		chromedp.Navigate(link+"#debug"),
		chromedp.WaitVisible(".desktop-view", chromedp.ByQuery),
		chromedp.Poll(`(window.__desktopStats && window.__desktopStats.bytesReceived) || 0`, &bytesReceived,
			chromedp.WithPollingTimeout(30*time.Second), chromedp.WithPollingInterval(250*time.Millisecond)),
		chromedp.Evaluate("window.__cspViolations", &violations),
	)
	if err != nil {
		if strings.Contains(err.Error(), "executable file not found") || strings.Contains(err.Error(), "chrome not found") {
			t.Skipf("chromedp: Chrome unavailable (%v)", err)
		}
		t.Fatalf("chromedp.Run: %v", err)
	}
	if bytesReceived <= 0 {
		t.Errorf("bytesReceived = %v, want > 0", bytesReceived)
	}
	if len(violations) > 0 {
		t.Errorf("CSP violations on /desktop/: %+v", violations)
	}
}
