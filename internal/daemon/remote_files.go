package daemon

// Outbound HTTPS to tailnet peers. POST /peers/connect (peer_connect.go) and the
// remote terminal WS proxy (remote_ws_proxy.go) use this client; it accepts the
// tailnet peer's self-signed cert. The "remote-files" name is historical (the
// store was built for the removed remote file browser).

import (
	"crypto/tls"
	"net/http"
	"strings"
	"time"
)

// newRemoteFilesHTTPClient constructs the outbound HTTPS client used by the
// remote terminal WS proxy. Mirrors internal/tailnet/sessions.go's shape:
// TLS 1.2 minimum, 10-second timeout. InsecureSkipVerify is acceptable here
// because the peer is a tailnet host with a self-signed cert (Tailscale itself
// guarantees the transport-layer authenticity).
func newRemoteFilesHTTPClient() *http.Client {
	return &http.Client{
		Timeout: 10 * time.Second,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				MinVersion:         tls.VersionTLS12,
				InsecureSkipVerify: true, //nolint:gosec // tailnet self-signed cert; matches 122-03 client
			},
		},
	}
}

// remoteFilesClient returns the HTTP client the WS proxy uses for outbound
// requests. Tests override this by setting a.remoteFilesClientForTest before
// issuing requests; production code falls back to newRemoteFilesHTTPClient().
func (a *API) remoteFilesClient() *http.Client {
	if a.remoteFilesClientForTest != nil {
		return a.remoteFilesClientForTest
	}
	return newRemoteFilesHTTPClient()
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
