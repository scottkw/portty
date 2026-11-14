package daemon

// Remote session caps on the daemon socket.
//
// After a successful join-code exchange the desktop GUI deposits the remote
// peer's (sessionID, baseURL, capToken) here so it can open the remote session
// (and reuse the held cap) without re-prompting for a join code:
//
//   POST /api/remote-files/caps                          — deposit the cap
//   GET  /api/remote-files/caps/{sessionID}/open-url     — cap-bearing open URL
//
// The "remote-files" name is historical (the store was built for the removed
// remote file browser); the wire paths are kept so existing clients still work.
// The outbound HTTPS client below is used by the remote terminal WS proxy
// (remote_ws_proxy.go) and accepts the tailnet peer's self-signed cert.

import (
	"crypto/tls"
	"encoding/json"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// handleRemoteSessionOpenURL serves GET /api/remote-files/caps/{sessionID}/open-url.
// It reads the stored (baseURL, capToken) from RemoteCapStore for the given sessionID
// and returns a cap-bearing open URL of the form baseURL+/sessions/{id}?cap=TOKEN.
//
// This is a read-only path — no cap is minted or modified. The cap enters the
// returned URL by the same design as the existing handleModalExchange open path,
// so no new exposure surface is introduced (T-146-05-01).
//
// Responses:
//
//	200 + {"url": "https://peer.../sessions/{id}?cap=TOKEN"}  — cap found
//	400 + plain text                                          — missing sessionID path value
//	404 + {"error": "no cap registered for session"}          — no stored cap
//	500 + plain text                                          — store not initialised
func (a *API) handleRemoteSessionOpenURL(w http.ResponseWriter, r *http.Request) {
	sessionID := r.PathValue("sessionID")
	if sessionID == "" {
		http.Error(w, "missing sessionID", http.StatusBadRequest)
		return
	}
	if a.remoteCaps == nil {
		http.Error(w, "remote cap store not initialised", http.StatusInternalServerError)
		return
	}
	baseURL, capToken, ok := a.remoteCaps.Get(sessionID)
	if !ok {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "no cap registered for session"})
		return
	}
	// Build the cap-bearing open URL. TrimRight removes any trailing slash so the
	// path segment /sessions/ is not doubled. PathEscape handles session IDs that
	// contain special characters (though in practice they are UUIDs). The cap token
	// is URL-query-encoded to survive special characters.
	openURL := strings.TrimRight(baseURL, "/") + "/sessions/" + url.PathEscape(sessionID) + "?cap=" + url.QueryEscape(capToken)
	writeJSON(w, http.StatusOK, map[string]string{"url": openURL})
}

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

// handleRegisterRemoteCap accepts the (sessionID, baseURL, capToken) tuple
// from the desktop GUI / TUI after a successful join-code exchange and stores
// it in the RemoteCapStore. handleRemoteSessionOpenURL and the remote terminal
// WS proxy look up the cap from this store.
//
// Request body: JSON {sessionId, baseUrl, capToken}.
// Responses:
//
//	200 + {"ok": true}                            on success
//	400 + plain text                              on malformed body or empty fields
//
// 200 (not 204) is used so the response carries a small JSON envelope the
// frontend can assert on.
func (a *API) handleRegisterRemoteCap(w http.ResponseWriter, r *http.Request) {
	// Cap the body to avoid a malicious local process pinning memory with
	// a giant JSON blob. 8 KiB matches handleSetPluginSettings.
	r.Body = http.MaxBytesReader(w, r.Body, 8192)

	var req struct {
		SessionID string `json:"sessionId"`
		BaseURL   string `json:"baseUrl"`
		CapToken  string `json:"capToken"`
	}
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if a.remoteCaps == nil {
		http.Error(w, "remote cap store not initialised", http.StatusInternalServerError)
		return
	}
	if err := a.remoteCaps.Put(req.SessionID, req.BaseURL, req.CapToken); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
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
