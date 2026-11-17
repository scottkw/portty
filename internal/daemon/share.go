package daemon

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"time"

	"github.com/scottkw/portty/internal/capability"
	"github.com/scottkw/portty/internal/relay"
	"github.com/scottkw/portty/internal/tailnet"
	"github.com/scottkw/portty/internal/webserver"
	"tailscale.com/client/local"
)

// shareRecord is one shared session: the grant registered on the share server
// and the full-control capability token the link carries. In memory only; the
// daemon outlives app restarts, and a daemon restart ends every session.
type shareRecord struct {
	grantID, token string
}

// Production seams, copied into each API by NewAPI. vmpreview builds replace
// them in init() (share_vmpreview.go); tests use SetShareSeamsForTest.
var (
	defaultShareHealth = webserver.CheckHealth

	defaultNewShareServer = func(h webserver.TailscaleHealth, m *relay.HubManager) (*webserver.WebServer, error) {
		return webserver.NewWebServer(webserver.Config{BindIP: h.IP, Port: tailnet.DefaultProbePort, FQDN: h.Domain}, m)
	}

	// defaultWarmCert fetches the tailnet HTTPS certificate before the server
	// starts, so the first browser request does not wait on issuance.
	defaultWarmCert = func(ctx context.Context, domain string) error {
		var lc local.Client
		_, _, err := lc.CertPair(ctx, domain)
		return err
	}
)

// SetShareSeamsForTest replaces the Tailscale health check, the share server
// factory and the certificate warm-up. Test-only.
func (a *API) SetShareSeamsForTest(
	health func(context.Context) webserver.TailscaleHealth,
	newServer func(webserver.TailscaleHealth, *relay.HubManager) (*webserver.WebServer, error),
	warmCert func(context.Context, string) error,
) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.shareHealth, a.newShareServer, a.warmCert = health, newServer, warmCert
}

func shareStatus(code string) int {
	switch code {
	case ShareErrNotFound:
		return http.StatusNotFound
	case ShareErrTailscaleOff:
		return http.StatusServiceUnavailable
	case ShareErrHTTPSOff, ShareErrPortInUse:
		return http.StatusConflict
	default:
		return http.StatusInternalServerError
	}
}

func shareURL(ws *webserver.WebServer, id, token string) string {
	return ws.BaseURL() + "/sessions/" + id + "?cap=" + url.QueryEscape(token)
}

// handleGetShare reports whether a session is shared and, if so, its link.
func (a *API) handleGetShare(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	a.mu.RLock()
	rec, ok := a.shares[id]
	ws := a.webServer
	a.mu.RUnlock()
	if !ok || ws == nil {
		writeJSON(w, http.StatusOK, ShareResponse{Shared: false})
		return
	}
	writeJSON(w, http.StatusOK, ShareResponse{Shared: true, URL: shareURL(ws, id, rec.token)})
}

// handlePutShare turns sharing on or off for one session. Every entry point
// (GUI, CLI, sidebar menu) flips this same state.
func (a *API) handlePutShare(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var req ShareRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 8<<10)).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}
	if !req.Shared {
		a.mu.Lock()
		a.unshareLocked(id)
		a.mu.Unlock()
		writeJSON(w, http.StatusOK, ShareResponse{Shared: false})
		return
	}
	resp, serr := a.shareOn(r.Context(), id)
	if serr != nil {
		writeJSON(w, shareStatus(serr.Code), ShareErrorResponse{Error: serr.Code, Detail: serr.Detail})
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

func (a *API) sessionShareable(id string) bool {
	for _, s := range a.engine.ListSessions() {
		if s.ID == id {
			return s.State != "stopped"
		}
	}
	return false
}

// shareOn starts the share server if none is running and mints the session's
// record. a.mu is never held across the health check, the warm-up or Start.
func (a *API) shareOn(ctx context.Context, id string) (ShareResponse, *ShareError) {
	if !a.sessionShareable(id) {
		return ShareResponse{}, &ShareError{ShareErrNotFound, "no running session " + id}
	}
	a.mu.RLock()
	health, newServer, warmCert := a.shareHealth, a.newShareServer, a.warmCert
	a.mu.RUnlock()

	hctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	h := health(hctx)
	cancel()
	if !h.Connected || h.IP == "" {
		return ShareResponse{}, &ShareError{ShareErrTailscaleOff, "Tailscale isn't connected"}
	}
	if !h.HasCerts || h.Domain == "" {
		return ShareResponse{}, &ShareError{ShareErrHTTPSOff, "HTTPS certificates are off for your tailnet"}
	}

	a.mu.RLock()
	ws := a.webServer
	a.mu.RUnlock()
	if ws == nil {
		if serr := a.startShareServer(ctx, h, newServer, warmCert); serr != nil {
			return ShareResponse{}, serr
		}
	}

	a.mu.Lock()
	defer a.mu.Unlock()
	ws = a.webServer
	if ws == nil { // the last share was turned off while we were starting
		return ShareResponse{}, &ShareError{ShareErrStartFailed, "the share server stopped while starting"}
	}
	if rec, ok := a.shares[id]; ok {
		return ShareResponse{Shared: true, URL: shareURL(ws, id, rec.token)}, nil
	}
	rec, err := a.mintShareRecord(id)
	if err != nil {
		return ShareResponse{}, &ShareError{ShareErrStartFailed, err.Error()}
	}
	ws.EnableSession(id)
	ws.AddGrant(id, rec.grantID)
	a.shares[id] = rec

	// The first share publishes the tailnet name in Certificate Transparency
	// logs; the GUI reads this acknowledgement with HasCTDisclosure.
	ack := filepath.Join(a.engine.configDir, "ct_disclosed")
	_, statErr := os.Stat(ack)
	first := errors.Is(statErr, os.ErrNotExist)
	if first {
		if err := os.WriteFile(ack, []byte("1"), 0o600); err != nil {
			fmt.Fprintf(os.Stderr, "daemon: record CT acknowledgement: %v\n", err)
		}
	}
	return ShareResponse{Shared: true, URL: shareURL(ws, id, rec.token), FirstShare: first}, nil
}

// startShareServer builds, wires and starts a share server, then installs it
// unless another request installed one first.
func (a *API) startShareServer(
	ctx context.Context,
	h webserver.TailscaleHealth,
	newServer func(webserver.TailscaleHealth, *relay.HubManager) (*webserver.WebServer, error),
	warmCert func(context.Context, string) error,
) *ShareError {
	wctx, cancel := context.WithTimeout(ctx, 20*time.Second)
	err := warmCert(wctx, h.Domain)
	cancel()
	if err != nil {
		return &ShareError{ShareErrStartFailed, "couldn't get an HTTPS certificate: " + err.Error()}
	}
	ws, err := newServer(h, a.engine.Manager())
	if err != nil {
		return &ShareError{ShareErrStartFailed, err.Error()}
	}
	ws.SetSessionResolver(func(sessionID string) (name, status, hostname string, ok bool) {
		for _, s := range a.engine.ListSessions() {
			if s.ID == sessionID {
				return s.Name, s.Status, s.Hostname, true
			}
		}
		return "", "", "", false
	})
	// Phase 93 PLUG-04: plugin settings as pre-marshaled JSON (func() []byte
	// avoids the daemon→webserver→daemon import cycle).
	ws.SetPluginSettingsProvider(a.guestPluginConfigJSON)
	// Same-account peers receive the link's own token (one token per share).
	ws.SetShareTokenProvider(func(sessionID string) (string, bool) {
		a.mu.RLock()
		defer a.mu.RUnlock()
		rec, ok := a.shares[sessionID]
		return rec.token, ok
	})
	// Signing key BEFORE Start so requireCapability never sees nil (Pitfall 3).
	ws.SetSigningKey(a.CurrentSigningKey())
	ws.SetStaticAppFS(getStaticAppFS())
	if err := ws.Start(); err != nil {
		if errors.Is(err, webserver.ErrPortInUse) {
			return &ShareError{ShareErrPortInUse, fmt.Sprintf("port %d is in use", tailnet.DefaultProbePort)}
		}
		return &ShareError{ShareErrStartFailed, err.Error()}
	}

	a.mu.Lock()
	if a.webServer != nil { // lost a race to another share-on
		a.mu.Unlock()
		_ = ws.Stop()
		return nil
	}
	a.webServer = ws
	// Phase 93 PLUG-04 push channel: SSE subscribers get a frame on every
	// plugin-settings change. One share server at a time, so one listener.
	a.engine.SetPluginSettingsListener(func() {
		ws.BroadcastPluginConfig(context.Background())
	})
	for sid, rec := range a.shares {
		ws.EnableSession(sid)
		ws.AddGrant(sid, rec.grantID)
	}
	a.mu.Unlock()
	return nil
}

// mintShareRecord signs one full-control capability for the session with the
// current key. A share always grants full control (SHAR-02).
func (a *API) mintShareRecord(id string) (shareRecord, error) {
	key := a.CurrentSigningKey()
	if key == nil {
		return shareRecord{}, errors.New("capability: signing key not bootstrapped")
	}
	var gid [16]byte
	if _, err := rand.Read(gid[:]); err != nil {
		return shareRecord{}, err
	}
	claims := capability.Claims{SID: id, Perms: "read,write", IAT: time.Now().Unix(), GrantID: hex.EncodeToString(gid[:]), V: 1}
	tok, err := capability.Sign(claims, key)
	if err != nil {
		return shareRecord{}, err
	}
	return shareRecord{grantID: claims.GrantID, token: tok}, nil
}

// unshareLocked drops the session's record, revokes its grants, closes its
// web viewers with relay.CloseReasonShareStopped and stops the share server
// when nothing is shared. Shared by share-off, session exit and kill. Caller
// holds a.mu.
// Order matters: grants clear before the kick so a viewer subscribing
// mid-revoke fails its post-subscribe re-check, and the kick precedes Stop
// because Stop closes only the listener (hijacked WebSockets survive it).
func (a *API) unshareLocked(id string) {
	delete(a.shares, id)
	ws := a.webServer
	if ws != nil {
		ws.ClearGrants(id)
		ws.DisableSession(id)
	}
	if hub, ok := a.engine.Manager().Get(id); ok {
		hub.RevokeWebViewers(relay.CloseReasonShareStopped)
	}
	if ws != nil && len(a.shares) == 0 {
		_ = ws.Stop()
		a.engine.SetPluginSettingsListener(nil)
		a.webServer = nil
	}
}

// remintSharesLocked gives every share a new capability signed with the
// current key: the old grant is cleared and viewers on the old link are
// closed with relay.CloseReasonShareStopped. A share that cannot be re-minted
// is turned off. Caller holds a.mu.
func (a *API) remintSharesLocked(ws *webserver.WebServer) {
	for id := range a.shares {
		ws.ClearGrants(id)
		if hub, ok := a.engine.Manager().Get(id); ok {
			hub.RevokeWebViewers(relay.CloseReasonShareStopped)
		}
		rec, err := a.mintShareRecord(id)
		if err != nil {
			fmt.Fprintf(os.Stderr, "daemon: re-mint share %s: %v\n", id, err)
			a.unshareLocked(id)
			continue
		}
		ws.AddGrant(id, rec.grantID)
		a.shares[id] = rec
	}
}
