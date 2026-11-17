package daemon

// Outbound HTTPS to tailnet peers. POST /peers/connect (peer_connect.go) and the
// remote terminal WS proxy (remote_ws_proxy.go) use this client. Every share
// server runs on a Tailscale (Let's Encrypt) certificate, so the client verifies
// it against the peer's MagicDNS name and dials the peer's Tailscale IP from
// tailscaled's own peer list, never a system-DNS answer (CR-02).

import (
	"context"
	"crypto/tls"
	"errors"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/scottkw/portty/internal/tailnet"
)

// peerHTTPClient dials addr whatever host the URL names, and verifies the
// server certificate against fqdn. An empty addr fails every dial.
func peerHTTPClient(fqdn, addr string) *http.Client {
	d := &net.Dialer{Timeout: 10 * time.Second}
	return &http.Client{
		Timeout: 10 * time.Second,
		Transport: &http.Transport{
			DialContext: func(ctx context.Context, network, _ string) (net.Conn, error) {
				if addr == "" {
					return nil, errors.New("peer has no Tailscale IP")
				}
				return d.DialContext(ctx, network, addr)
			},
			TLSClientConfig: &tls.Config{ServerName: fqdn, MinVersion: tls.VersionTLS12},
		},
	}
}

// peerClient returns the verified client for one peer: it dials
// ip:DefaultProbePort. Tests replace it with SetPeerClientForTest, so they
// never dial a real tailnet address.
func (a *API) peerClient(fqdn, ip string) *http.Client {
	if a.peerClientForTest != nil {
		return a.peerClientForTest
	}
	addr := ""
	if ip != "" {
		addr = net.JoinHostPort(ip, strconv.Itoa(tailnet.DefaultProbePort))
	}
	return peerHTTPClient(fqdn, addr)
}

// peerIP is the first Tailscale IP tailscaled reports for p, or "".
func peerIP(p tailnet.Peer) string {
	if len(p.TailscaleIPs) == 0 {
		return ""
	}
	return p.TailscaleIPs[0]
}

// redactCapTokenFromError removes the cap-token substring from an error's
// rendered message so we never leak token bytes into a client-visible error
// body. The cap token is the only secret in this code path; everything else
// (URL, sessionID) is already known to both peers.
func redactCapTokenFromError(err error, capToken string) string {
	if err == nil {
		return ""
	}
	msg := err.Error()
	if capToken == "" {
		return msg
	}
	return strings.ReplaceAll(msg, capToken, "<redacted>")
}
