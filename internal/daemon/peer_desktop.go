package daemon

// POST /peers/desktop and GET /peers/desktop/link (Phase 21-06): the viewing
// daemon's side of a peer's always-available desktop. Like POST /peers/connect
// it resolves the host from tailscaled's own peer list, builds the base URL from
// that peer's DNS name (never a URL the peer supplied) and fetches the standing
// desktop token through the verified peer client. The token goes into the
// in-memory DesktopCapStore only; the answer carries no token. The web view then
// opens the loopback proxy (remote_desktop_ws_proxy.go), which dials the peer
// with the stored token.

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"net/url"
	"strings"
	"syscall"
	"time"

	"github.com/scottkw/portty/internal/tailnet"
)

// PeerErrDesktopOff is the PeerConnectError code for a peer whose desktop is
// off: its cap route answered 404 or its share port refused the connection.
const PeerErrDesktopOff = "desktop_off"

// PeerDesktopRequest is the body of POST /peers/desktop.
type PeerDesktopRequest struct {
	Host string `json:"host"`
}

// PeerDesktopResponse is the answer: the peer's full MagicDNS name (the key the
// proxy and link routes take) and its short name. It never carries the token.
type PeerDesktopResponse struct {
	Host string `json:"host"`
	Name string `json:"name"`
}

// PeerDesktopLinkResponse is the answer of GET /peers/desktop/link.
type PeerDesktopLinkResponse struct {
	URL string `json:"url"`
}

// peerDesktopFail is a failed cap fetch: the HTTP status and token-free body the
// route answers with. It is also the error refreshPeerDesktopCap returns.
type peerDesktopFail struct {
	status int
	body   PeerConnectErrorResponse
}

func (f *peerDesktopFail) Error() string { return f.body.Error }

// connectPeerDesktop matches host against tailscaled's peer list, fetches the
// peer's desktop token through the verified client and stores it under the
// peer's full DNS name. It returns the peer, or a *peerDesktopFail.
func (a *API) connectPeerDesktop(ctx context.Context, host string) (tailnet.Peer, *peerDesktopFail) {
	peers, err := a.discoverPeers(ctx)
	if err != nil {
		return tailnet.Peer{}, &peerDesktopFail{http.StatusServiceUnavailable, PeerConnectErrorResponse{Error: PeerErrTailscaleOff}}
	}
	peer, ok := matchPeer(peers, host)
	if !ok {
		hosts := []string{}
		for _, p := range a.probePeers(ctx) {
			hosts = append(hosts, peerShortName(p))
		}
		return tailnet.Peer{}, &peerDesktopFail{http.StatusNotFound, PeerConnectErrorResponse{Error: PeerErrUnknownHost, Hosts: hosts}}
	}
	short := peerShortName(peer)
	fqdn := strings.TrimSuffix(peer.DNSName, ".")
	baseURL := a.peerBaseURL(peer)
	ip := peerIP(peer)
	client := a.peerClient(fqdn, ip)
	defer client.CloseIdleConnections() // per-request client: don't leave its keep-alive open on both Macs

	token, err := tailnet.FetchPeerDesktopCap(ctx, baseURL, client)
	switch {
	case errors.Is(err, tailnet.ErrPeerForbidden):
		return peer, &peerDesktopFail{http.StatusForbidden, PeerConnectErrorResponse{Error: PeerErrForbidden, Detail: short}}
	case errors.Is(err, tailnet.ErrPeerDesktopOff), errors.Is(err, syscall.ECONNREFUSED):
		// 404 is the always-available-off answer; a refused share port means the
		// share server is not running at all. Either way the held token is stale.
		a.desktopCaps.Delete(fqdn)
		return peer, &peerDesktopFail{http.StatusConflict, PeerConnectErrorResponse{Error: PeerErrDesktopOff, Detail: short}}
	case err != nil:
		log.Printf("peers/desktop: cap from %s: %s", short, redactCapTokenFromError(err, token))
		return peer, &peerDesktopFail{http.StatusBadGateway, PeerConnectErrorResponse{Error: PeerErrUnreachable, Detail: short}}
	}
	if err := a.desktopCaps.Put(fqdn, baseURL, ip, token); err != nil {
		log.Printf("peers/desktop: store cap for %s: %s", short, redactCapTokenFromError(err, token))
		return peer, &peerDesktopFail{http.StatusBadGateway, PeerConnectErrorResponse{Error: PeerErrUnreachable, Detail: short}}
	}
	return peer, nil
}

// refreshPeerDesktopCap fetches a fresh token for host and stores it. The error
// is a *peerDesktopFail and never carries the token.
func (a *API) refreshPeerDesktopCap(ctx context.Context, host string) error {
	if _, fail := a.connectPeerDesktop(ctx, host); fail != nil {
		return fail
	}
	return nil
}

func (a *API) handlePeerDesktop(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 8192)
	var req PeerDesktopRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || strings.TrimSpace(req.Host) == "" {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 20*time.Second)
	defer cancel()
	peer, fail := a.connectPeerDesktop(ctx, strings.TrimSpace(req.Host))
	if fail != nil {
		writeJSON(w, fail.status, fail.body)
		return
	}
	writeJSON(w, http.StatusOK, PeerDesktopResponse{Host: strings.TrimSuffix(peer.DNSName, "."), Name: peerShortName(peer)})
}

// handlePeerDesktopLink answers the browser link of a held peer desktop. It
// makes no network call and is registered on the daemon socket only: the relay
// port is reachable by browser pages and the answer carries the token.
func (a *API) handlePeerDesktopLink(w http.ResponseWriter, r *http.Request) {
	baseURL, _, token, ok := a.desktopCaps.Get(r.URL.Query().Get("host"))
	if !ok {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "no desktop token held for host"})
		return
	}
	writeJSON(w, http.StatusOK, PeerDesktopLinkResponse{URL: strings.TrimRight(baseURL, "/") + "/desktop/?cap=" + url.QueryEscape(token)})
}
