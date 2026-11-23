//go:build playwrightfixture

// Package main is a Playwright e2e test fixture for Phase 93 web parity specs.
//
// This binary boots an in-process Portty web server with:
//   - Self-signed TLS on 127.0.0.1 (random port)
//   - One pre-seeded session ("playwright-test-session") backed by an io.Pipe
//     PTY stub (no real shell — the page only needs to attach for the addon
//     load assertions; PTY data is irrelevant).
//   - A capability token signed with a fixed test key.
//   - SetPluginSettingsProvider wired to an atomic.Value holding the latest
//     plugin settings JSON, so /api/plugin-config returns mutable state.
//   - A separate plain-HTTP admin listener on 127.0.0.1 (random port) exposing
//     POST /__test__/plugin-config for the SSE hot-swap spec to flip server-
//     side settings + trigger BroadcastPluginConfig().
//   - POST /__test__/whois {"as":"same|other|tagged|shared-in|not-found|
//     status-fault|whois-fault"} switches the identity the WhoIs fake answers
//     with (MOB-05); 400 on an unknown name. The default is the same user. The
//     fixture never asks the host's live tailscaled.
//   - GET /__test__/input returns {"hex":"<lowercase hex>"} of every byte the
//     hub has written to the PTY input stub since the last reset; POST
//     /__test__/input/reset empties it (204). The fixture drains the stub, so
//     key-bar e2e tests read what a page sent to the shell.
//   - GET /__test__/grid returns {"cols":N,"rows":N,"resizes":N}: the last
//     grid the hub asked the PTY for, and how many times. GET
//     /__test__/info-hits returns {"count":N}: info requests answered for the
//     session (past the capability check).
//   - POST /__test__/share-off ends the share the way the daemon does (grants
//     cleared, session disabled, web viewers closed "share stopped"; 204).
//     POST /__test__/share-on re-enables it with a new grant and returns
//     {"cap":"<token>"} (read,write). POST /__test__/end closes the PTY output
//     so the hub ends and live sockets close "session ended" (204).
//     POST /__test__/preview-pdf sends a one-page PDF preview to every viewer
//     ({"viewers":N}; 409 while one is streaming).
//   - PLAYWRIGHT_FIXTURE_CA_PEM=<path> writes the fixture's CA certificate
//     there (0644), for the iOS Simulator's trust store. The key never leaves
//     memory.
//
// Build-tagged 'playwrightfixture' so it never compiles into release builds
// (T-93-iPad-UAT-EVASION mitigation: the /__test__ surface is fixture-only).
//
// Usage (from frontend/playwright.config.ts webServer):
//
//	go run -tags=playwrightfixture ./cmd/playwright-fixture
//
// On startup the binary writes one line to stdout in KEY=VALUE form (then
// continues running until SIGTERM):
//
//	BASE_URL=https://127.0.0.1:PORT
//	CAP=<token>
//	ADMIN_URL=http://127.0.0.1:ADMIN_PORT
//	CA_PEM=<path>      (only when PLAYWRIGHT_FIXTURE_CA_PEM is set)
//	READY=1
//
// The Playwright config's webServer.url field is used to wait for readiness.
package main

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"log"
	"math/big"
	"net"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/scottkw/portty/internal/capability"
	"github.com/scottkw/portty/internal/relay"
	"github.com/scottkw/portty/internal/webserver"
	"github.com/scottkw/portty/internal/webserver/webservertest"
)

const (
	sessionID   = "playwright-test-session"
	sessionName = "Playwright Test Session"
	sessionHost = "fixture-host"
)

// fixedTestKey is a 32-byte deterministic HMAC signing key for the fixture.
// Tokens minted with this key are valid only against this fixture's webserver
// (because the webserver's SetSigningKey is called with the same value).
// Never used in production — gated by the playwrightfixture build tag.
var fixedTestKey = func() []byte {
	k := make([]byte, 32)
	for i := range k {
		k[i] = byte(i + 1)
	}
	return k
}()

func main() {
	log.SetFlags(log.LstdFlags | log.Lmicroseconds)
	log.SetPrefix("[playwright-fixture] ")

	// 1. Self-signed TLS for 127.0.0.1.
	tlsCfg, caPEM, err := selfSignedTLS()
	if err != nil {
		log.Fatalf("selfSignedTLS: %v", err)
	}

	// 2. Hub manager + one pre-seeded session backed by io.Pipe stub.
	manager := relay.NewHubManager()
	ptyOutR, ptyOutW := io.Pipe()
	inputCaptureR, inputCaptureW := io.Pipe()
	// Drain every byte the hub writes to its PTY input stub into a buffer the
	// admin /__test__/input routes read. Ends at EOF when shutdown closes
	// inputCaptureW.
	var (
		inputMu  sync.Mutex
		inputBuf bytes.Buffer
	)
	go func() {
		chunk := make([]byte, 4096)
		for {
			n, err := inputCaptureR.Read(chunk)
			if n > 0 {
				inputMu.Lock()
				inputBuf.Write(chunk[:n])
				inputMu.Unlock()
			}
			if err != nil {
				return
			}
		}
	}()
	// The hub's resize hook: the last grid it asked the PTY for, and how often.
	var (
		gridMu                          sync.Mutex
		gridCols, gridRows, gridResizes int
	)
	hub := manager.Create(sessionID, ptyOutR, inputCaptureW, func(cols, rows int) error {
		gridMu.Lock()
		gridCols, gridRows = cols, rows
		gridResizes++
		gridMu.Unlock()
		return nil
	})

	// 3. WebServer in tailscale-mode with TLSConfig override (mirrors
	//    testServerWithHub but without *testing.T).
	cfg := webserver.Config{
		BindIP:    "127.0.0.1",
		Port:      0,
		FQDN:      "127.0.0.1",
		TLSConfig: tlsCfg,
	}
	ws, err := webserver.NewWebServer(cfg, manager)
	if err != nil {
		log.Fatalf("NewWebServer: %v", err)
	}
	// MOB-05: every session route is same-account gated. Answer as the same
	// user by default; the fixture is not a test binary, so without a fake it
	// would ask the host's live tailscaled.
	who := webservertest.SameUser()
	ws.SetWhoIsClientForTest(who)
	ws.SetSigningKey(fixedTestKey)
	// Phase 120-06 Task 3 — wire the embedded React bundle so /app/ serves
	// the SPA under playwright tests. staticAppFixture() returns nil under
	// `-tags playwrightfixture` alone (assets_stub.go) and the embedded
	// bundle under `-tags playwrightfixture,wailsassets` (assets_prod.go).
	// global-setup.ts always builds with both tags so the bundle is present
	// in normal e2e runs; scenario 12 keeps a 503-tolerant smoke for the
	// stub case.
	ws.SetStaticAppFS(staticAppFixture())
	ws.EnableSession(sessionID)
	var infoHits atomic.Int64
	ws.SetSessionResolver(func(id string) (string, string, string, bool) {
		if id == sessionID {
			infoHits.Add(1) // runs only after the capability check passes
			return sessionName, "running", sessionHost, true
		}
		return "", "", "", false
	})

	// 4. Plugin settings: atomic.Value holding pre-marshaled JSON. Default
	//    is everything-on so the vendor-parity spec sees all addons load.
	var pluginJSON atomic.Value
	pluginJSON.Store(mustMarshal(defaultPluginSettings()))
	ws.SetPluginSettingsProvider(func() []byte {
		v := pluginJSON.Load()
		if v == nil {
			return nil
		}
		return v.([]byte)
	})

	// 5. Start the WebServer's HTTPS listener.
	if err := ws.Start(); err != nil {
		log.Fatalf("ws.Start: %v", err)
	}

	// 6. Mint the capability for the test session, matching what the daemon
	//    mints for a share: one full-control cap, "read,write".
	ownerClaims := capability.Claims{
		SID:     sessionID,
		Perms:   "read,write",
		IAT:     time.Now().Unix(),
		GrantID: "grant-playwright-fixture-owner",
		V:       1,
	}
	token, err := capability.Sign(ownerClaims, fixedTestKey)
	if err != nil {
		log.Fatalf("capability.Sign owner: %v", err)
	}
	ws.AddGrant(sessionID, ownerClaims.GrantID)

	// 7. Admin HTTP server (plain, separate port) for /__test__/plugin-config.
	adminLn, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		log.Fatalf("admin listen: %v", err)
	}
	adminMux := http.NewServeMux()
	adminMux.HandleFunc("/__test__/plugin-config", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		var settings map[string]bool
		if err := json.NewDecoder(r.Body).Decode(&settings); err != nil {
			http.Error(w, "decode body: "+err.Error(), http.StatusBadRequest)
			return
		}
		// Re-marshal to ensure canonical key set is present and well-formed.
		data, err := json.Marshal(settings)
		if err != nil {
			http.Error(w, "marshal: "+err.Error(), http.StatusInternalServerError)
			return
		}
		pluginJSON.Store(data)
		// Trigger SSE broadcast so any subscribed client receives the new frame.
		ws.BroadcastPluginConfig(r.Context())
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(data)
	})
	adminMux.HandleFunc("/__test__/whois", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		var body struct {
			As string `json:"as"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(w, "decode body: "+err.Error(), http.StatusBadRequest)
			return
		}
		id, err := webservertest.ParseIdentity(body.As)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		who.Set(id)
		w.WriteHeader(http.StatusOK)
	})
	adminMux.HandleFunc("/__test__/input", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		inputMu.Lock()
		h := hex.EncodeToString(inputBuf.Bytes())
		inputMu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{"hex": h})
	})
	adminMux.HandleFunc("/__test__/input/reset", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		inputMu.Lock()
		inputBuf.Reset()
		inputMu.Unlock()
		w.WriteHeader(http.StatusNoContent)
	})
	adminMux.HandleFunc("/__test__/grid", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		gridMu.Lock()
		g := map[string]int{"cols": gridCols, "rows": gridRows, "resizes": gridResizes}
		gridMu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(g)
	})
	adminMux.HandleFunc("/__test__/info-hits", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]int64{"count": infoHits.Load()})
	})
	// share-off: the daemon's order (grants, session, then the viewers).
	adminMux.HandleFunc("/__test__/share-off", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		ws.ClearGrants(sessionID)
		ws.DisableSession(sessionID)
		hub.RevokeWebViewers(relay.CloseReasonShareStopped)
		w.WriteHeader(http.StatusNoContent)
	})
	var shareOns atomic.Int64
	adminMux.HandleFunc("/__test__/share-on", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		claims := capability.Claims{
			SID:     sessionID,
			Perms:   "read,write",
			IAT:     time.Now().Unix(),
			GrantID: fmt.Sprintf("grant-playwright-fixture-%d", shareOns.Add(1)),
			V:       1,
		}
		tok, err := capability.Sign(claims, fixedTestKey)
		if err != nil {
			http.Error(w, "sign: "+err.Error(), http.StatusInternalServerError)
			return
		}
		ws.EnableSession(sessionID)
		ws.AddGrant(sessionID, claims.GrantID)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{"cap": tok})
	})
	// end: EOF on the PTY output ends the hub; the write pump closes every
	// live socket with the "session ended" reason on hub.Done.
	adminMux.HandleFunc("/__test__/end", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		_ = ptyOutW.Close()
		w.WriteHeader(http.StatusNoContent)
	})
	adminMux.HandleFunc("/__test__/preview-pdf", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		send, err := hub.SendPreview(relay.Preview{
			Name: "fixture.pdf",
			MIME: "application/pdf",
			Key:  "0123456789abcdef0123456789abcdef",
			Data: []byte(fixturePDF),
		})
		switch {
		case errors.Is(err, relay.ErrPreviewBusy):
			http.Error(w, err.Error(), http.StatusConflict)
		case err != nil:
			http.Error(w, err.Error(), http.StatusInternalServerError)
		default:
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]int{"viewers": send.Viewers})
		}
	})
	adminMux.HandleFunc("/__test__/ready", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
	adminSrv := &http.Server{
		Handler:           adminMux,
		ReadHeaderTimeout: 5 * time.Second,
	}
	go func() {
		if err := adminSrv.Serve(adminLn); err != nil && err != http.ErrServerClosed {
			log.Printf("admin Serve: %v", err)
		}
	}()

	// 8. Print the URLs/cap and READY signal so playwright.config.ts can
	//    parse stdout and pass the values via env vars to specs.
	baseURL := "https://" + ws.Addr()
	adminURL := "http://" + adminLn.Addr().String()
	fmt.Printf("BASE_URL=%s\n", baseURL)
	fmt.Printf("CAP=%s\n", token)
	fmt.Printf("ADMIN_URL=%s\n", adminURL)
	if p := os.Getenv("PLAYWRIGHT_FIXTURE_CA_PEM"); p != "" {
		if err := os.WriteFile(p, caPEM, 0o644); err != nil {
			log.Fatalf("write CA PEM: %v", err)
		}
		fmt.Printf("CA_PEM=%s\n", p)
	}
	fmt.Println("READY=1")

	// 9. Wait for SIGTERM/SIGINT, then clean up.
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGTERM, syscall.SIGINT)
	<-stop
	log.Println("shutdown signal received")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	_ = adminSrv.Shutdown(shutdownCtx)
	_ = ws.Stop()
	manager.Shutdown()
	_ = ptyOutW.Close()
	_ = inputCaptureW.Close()
}

func defaultPluginSettings() map[string]bool {
	return map[string]bool{
		"webgl":     true,
		"unicode11": true,
		"clipboard": true,
		"search":    true,
		"webLinks":  true,
		"image":     true,
		"serialize": true,
		"progress":  false,
	}
}

func mustMarshal(v any) []byte {
	data, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return data
}

// selfSignedTLS generates an in-memory self-signed CA + leaf for 127.0.0.1.
// Mirrors the helper used by `testServerWithHub` in the non-test package.
func selfSignedTLS() (*tls.Config, []byte, error) {
	caKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, nil, err
	}
	caTmpl := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{Organization: []string{"Playwright Fixture CA"}},
		NotBefore:             time.Now().Add(-time.Minute),
		NotAfter:              time.Now().Add(2 * time.Hour),
		IsCA:                  true,
		BasicConstraintsValid: true,
		KeyUsage:              x509.KeyUsageCertSign,
	}
	caDER, err := x509.CreateCertificate(rand.Reader, caTmpl, caTmpl, &caKey.PublicKey, caKey)
	if err != nil {
		return nil, nil, err
	}
	caPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: caDER})
	caCert, err := x509.ParseCertificate(caDER)
	if err != nil {
		return nil, nil, err
	}
	leafKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, nil, err
	}
	leafTmpl := &x509.Certificate{
		SerialNumber: big.NewInt(2),
		Subject:      pkix.Name{CommonName: "127.0.0.1"},
		IPAddresses:  []net.IP{net.ParseIP("127.0.0.1")},
		NotBefore:    time.Now().Add(-time.Minute),
		NotAfter:     time.Now().Add(2 * time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	}
	leafDER, err := x509.CreateCertificate(rand.Reader, leafTmpl, caCert, &leafKey.PublicKey, caKey)
	if err != nil {
		return nil, nil, err
	}
	leafCertPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: leafDER})
	leafKeyDER, err := x509.MarshalECPrivateKey(leafKey)
	if err != nil {
		return nil, nil, err
	}
	leafKeyPEM := pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: leafKeyDER})
	tlsCert, err := tls.X509KeyPair(leafCertPEM, leafKeyPEM)
	if err != nil {
		return nil, nil, err
	}
	return &tls.Config{
		Certificates: []tls.Certificate{tlsCert},
		MinVersion:   tls.VersionTLS12,
	}, caPEM, nil
}

// fixturePDF is a one-page blank PDF, 329 bytes, with a correct xref table.
const fixturePDF = "%PDF-1.4\n1 0 obj\n<< /Type /Catalog /Pages 2 0 R >>\nendobj\n2 0 obj\n<< /Type /Pages /Kids [3 0 R] /Count 1 >>\nendobj\n3 0 obj\n<< /Type /Page /Parent 2 0 R /MediaBox [0 0 200 200] >>\nendobj\nxref\n0 4\n0000000000 65535 f \n0000000009 00000 n \n0000000058 00000 n \n0000000115 00000 n \ntrailer\n<< /Size 4 /Root 1 0 R >>\nstartxref\n186\n%%EOF\n"
