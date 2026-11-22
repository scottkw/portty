package daemon

// POST /peers/connect: the one path the CLI and the GUI use to reach a session
// shared from another Mac on the same Tailscale account. It resolves the host from
// tailscaled's own peer list, builds the base URL from that peer's DNS name (never
// a URL the peer supplied), dials the peer's Tailscale IP with the certificate
// verified against that name (peer_client.go), reads the shared sessions and the share's capability
// through the host's WhoIs-gated peer routes, and stores the capability in
// RemoteCapStore so the relay WS proxy can attach. No code or token is pasted.

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"net/url"
	"strings"
	"syscall"
	"time"

	"github.com/scottkw/portty/internal/tailnet"
)

func defaultPeerBaseURL(p tailnet.Peer) string {
	return fmt.Sprintf("https://%s:%d", strings.TrimSuffix(p.DNSName, "."), tailnet.DefaultProbePort)
}

// SetPeerSeamsForTest replaces peer discovery, the probed-peer list and the base
// URL builder. Test-only; nil arguments keep the current seam.
func (a *API) SetPeerSeamsForTest(discover func(context.Context) ([]tailnet.Peer, error), probe func(context.Context) []tailnet.Peer, baseURL func(tailnet.Peer) string) {
	if discover != nil {
		a.discoverPeers = discover
	}
	if probe != nil {
		a.probePeers = probe
	}
	if baseURL != nil {
		a.peerBaseURL = baseURL
	}
}

// peerShortName is the MagicDNS short name: the first label of the DNS name,
// falling back to the HostName.
// magicDNSLabel is a MagicDNS name's short label (the first one), the host name
// every surface shows. An IP literal (the VM build's share domain) stays whole.
func magicDNSLabel(name string) string {
	name = strings.TrimSuffix(name, ".")
	if net.ParseIP(name) != nil {
		return name
	}
	label, _, _ := strings.Cut(name, ".")
	return label
}

func peerShortName(p tailnet.Peer) string {
	if label := magicDNSLabel(p.DNSName); label != "" {
		return label
	}
	return p.Hostname
}

// matchPeer finds host among online peers: the full MagicDNS name first (what
// the GUI sends, since HostNames can repeat: WR-05), then the MagicDNS label,
// then HostName, all case-insensitive.
func matchPeer(peers []tailnet.Peer, host string) (tailnet.Peer, bool) {
	if fqdn := strings.TrimSuffix(host, "."); strings.Contains(fqdn, ".") {
		for _, p := range peers {
			if strings.EqualFold(strings.TrimSuffix(p.DNSName, "."), fqdn) {
				return p, true
			}
		}
	}
	for _, p := range peers {
		if label, _, _ := strings.Cut(strings.TrimSuffix(p.DNSName, "."), "."); label != "" && strings.EqualFold(label, host) {
			return p, true
		}
	}
	for _, p := range peers {
		if p.Hostname != "" && strings.EqualFold(p.Hostname, host) {
			return p, true
		}
	}
	return tailnet.Peer{}, false
}

func choices(ss []tailnet.ShareableSessionMeta) []PeerSessionChoice {
	out := make([]PeerSessionChoice, 0, len(ss))
	for _, s := range ss {
		out = append(out, PeerSessionChoice{ID: s.ID, Name: s.Name})
	}
	return out
}

// resolvePeerSession picks the shared session the caller asked for. It returns
// the session or an error body with its HTTP status.
func resolvePeerSession(shared []tailnet.ShareableSessionMeta, want string) (tailnet.ShareableSessionMeta, int, *PeerConnectErrorResponse) {
	if want == "" {
		if len(shared) == 1 {
			return shared[0], 0, nil
		}
		return tailnet.ShareableSessionMeta{}, http.StatusConflict, &PeerConnectErrorResponse{Error: PeerErrMultiple, Sessions: choices(shared)}
	}
	var named []tailnet.ShareableSessionMeta
	for _, s := range shared {
		if s.Name == want {
			named = append(named, s)
		}
	}
	if len(named) == 1 {
		return named[0], 0, nil
	}
	if len(named) > 1 {
		return tailnet.ShareableSessionMeta{}, http.StatusConflict, &PeerConnectErrorResponse{Error: PeerErrAmbiguous, Sessions: choices(named)}
	}
	var prefixed []tailnet.ShareableSessionMeta
	for _, s := range shared {
		if s.ID == want {
			return s, 0, nil
		}
		if strings.HasPrefix(s.ID, want) {
			prefixed = append(prefixed, s)
		}
	}
	if len(prefixed) == 1 {
		return prefixed[0], 0, nil
	}
	return tailnet.ShareableSessionMeta{}, http.StatusNotFound, &PeerConnectErrorResponse{Error: PeerErrNoSuchSession, Sessions: choices(shared)}
}

func (a *API) handlePeerConnect(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 8192)
	var req PeerConnectRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || strings.TrimSpace(req.Host) == "" {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 20*time.Second)
	defer cancel()

	peers, err := a.discoverPeers(ctx)
	if err != nil {
		writeJSON(w, http.StatusServiceUnavailable, PeerConnectErrorResponse{Error: PeerErrTailscaleOff})
		return
	}
	peer, ok := matchPeer(peers, strings.TrimSpace(req.Host))
	if !ok {
		hosts := []string{}
		for _, p := range a.probePeers(ctx) {
			hosts = append(hosts, peerShortName(p))
		}
		writeJSON(w, http.StatusNotFound, PeerConnectErrorResponse{Error: PeerErrUnknownHost, Hosts: hosts})
		return
	}
	host := peerShortName(peer)
	baseURL := a.peerBaseURL(peer)
	ip := peerIP(peer)
	client := a.peerClient(strings.TrimSuffix(peer.DNSName, "."), ip)
	defer client.CloseIdleConnections() // the client is per-request; don't leave its keep-alive open on both Macs

	shared, status, ferr := tailnet.FetchPeerSessionsMetaWithClient(ctx, baseURL, client)
	switch {
	case status == tailnet.MetaForbidden:
		writeJSON(w, http.StatusForbidden, PeerConnectErrorResponse{Error: PeerErrForbidden, Detail: host})
		return
	case status == tailnet.MetaOK && len(shared) == 0, errors.Is(ferr, syscall.ECONNREFUSED):
		// The share server only runs while something is shared, so a refused
		// port means "not sharing" (RESEARCH Pitfall 5).
		writeJSON(w, http.StatusConflict, PeerConnectErrorResponse{Error: PeerErrNotSharing, Detail: host})
		return
	case status != tailnet.MetaOK:
		log.Printf("peers/connect: %s unreachable: %v", host, ferr)
		writeJSON(w, http.StatusBadGateway, PeerConnectErrorResponse{Error: PeerErrUnreachable, Detail: host})
		return
	}

	sess, code, perr := resolvePeerSession(shared, strings.TrimSpace(req.Session))
	if perr != nil {
		perr.Detail = host
		writeJSON(w, code, *perr)
		return
	}

	capToken, err := tailnet.FetchPeerCap(ctx, baseURL, sess.ID, client)
	switch {
	case errors.Is(err, tailnet.ErrPeerForbidden):
		writeJSON(w, http.StatusForbidden, PeerConnectErrorResponse{Error: PeerErrForbidden, Detail: host})
		return
	case errors.Is(err, tailnet.ErrPeerNotShared):
		writeJSON(w, http.StatusNotFound, PeerConnectErrorResponse{Error: PeerErrNoSuchSession, Detail: host, Sessions: choices(shared)})
		return
	case err != nil:
		log.Printf("peers/connect: cap from %s: %s", host, redactCapTokenFromError(err, capToken))
		writeJSON(w, http.StatusBadGateway, PeerConnectErrorResponse{Error: PeerErrUnreachable, Detail: host})
		return
	}
	// A different capability for a session we already hold means a re-share or a
	// re-mint: end its loopback forwards and approvals before the new one lands.
	if prevBase, _, prevCap, held := a.remoteCaps.Get(sess.ID); held && prevCap != capToken {
		if pu, perr := url.Parse(prevBase); perr == nil {
			a.portForwards.RevokeSession(pu.Hostname(), sess.ID)
		}
	}
	if err := a.remoteCaps.Put(sess.ID, baseURL, ip, capToken); err != nil {
		log.Printf("peers/connect: store cap for %s: %s", host, redactCapTokenFromError(err, capToken))
		writeJSON(w, http.StatusBadGateway, PeerConnectErrorResponse{Error: PeerErrUnreachable, Detail: host})
		return
	}
	writeJSON(w, http.StatusOK, PeerConnectResponse{
		SessionID: sess.ID,
		Name:      sess.Name,
		Host:      host,
		URL:       baseURL + "/sessions/" + url.PathEscape(sess.ID) + "?cap=" + url.QueryEscape(capToken),
	})
}
