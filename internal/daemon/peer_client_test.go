package daemon

import (
	"crypto/tls"
	"crypto/x509"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestPeerHTTPClient_VerifiesCertAndDialsIP (CR-02): the peer client dials the
// given address, never the URL host, and refuses a certificate it can't verify
// for the peer's MagicDNS name.
func TestPeerHTTPClient_VerifiesCertAndDialsIP(t *testing.T) {
	var gotHost string
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotHost = r.Host
	}))
	defer srv.Close()
	addr := srv.Listener.Addr().String()

	// Untrusted (self-signed) certificate: refused, handler never runs.
	_, err := peerHTTPClient("office-air.tail1.ts.net", addr).Get("https://office-air.invalid/api/sessions/meta")
	var cve *tls.CertificateVerificationError
	if !errors.As(err, &cve) {
		t.Fatalf("self-signed peer: err = %v; want a certificate verification error", err)
	}
	if gotHost != "" {
		t.Fatal("request reached a peer whose certificate failed verification")
	}

	// Trusted certificate for the name: the unresolvable URL host still reaches
	// the server, so the dial went to addr and not through DNS.
	c := peerHTTPClient("example.com", addr) // httptest's cert covers example.com
	pool := x509.NewCertPool()
	pool.AddCert(srv.Certificate())
	c.Transport.(*http.Transport).TLSClientConfig.RootCAs = pool
	resp, err := c.Get("https://office-air.invalid/api/sessions/meta")
	if err != nil {
		t.Fatalf("trusted peer: %v", err)
	}
	resp.Body.Close()
	if gotHost != "office-air.invalid" {
		t.Fatalf("Host = %q; want the URL host", gotHost)
	}

	// No Tailscale IP: fail, never fall back to DNS.
	if _, err := peerHTTPClient("example.com", "").Get("https://example.com/"); err == nil || !strings.Contains(err.Error(), "no Tailscale IP") {
		t.Fatalf("empty addr: err = %v", err)
	}
}
