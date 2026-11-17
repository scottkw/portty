package tailnet

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	neturl "net/url"
	"sort"
	"strings"
	"sync"
	"time"

	"golang.org/x/sync/errgroup"
)

// PeerSession describes a single session on a remote peer.
type PeerSession struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Status   string `json:"status"`
	Hostname string `json:"hostname"` // peer hostname (for display)
	FQDN     string `json:"fqdn"`     // peer FQDN (for URL construction)
	URL      string `json:"url"`      // pre-built session URL
}

// PeerSessionGroup groups sessions by peer hostname.
type PeerSessionGroup struct {
	Hostname string        `json:"hostname"`
	Sessions []PeerSession `json:"sessions"`
}

// FetchPeerSessions fetches /api/sessions from a single peer over HTTPS.
// Uses TLS 1.2 minimum and a 5-second timeout. Returns empty slice on any error.
// If DNS-based fetch fails and the peer has TailscaleIPs, retries using the
// first IP with TLS ServerName set to the FQDN for certificate validation.
func FetchPeerSessions(ctx context.Context, peer Peer) []PeerSession {
	fqdn := strings.TrimSuffix(peer.DNSName, ".")
	client := &http.Client{
		Timeout: 5 * time.Second,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{MinVersion: tls.VersionTLS12},
		},
	}
	url := fmt.Sprintf("https://%s:%d/api/sessions", fqdn, DefaultProbePort)
	sessions := doFetchSessions(ctx, url, client, "")
	// DNS or connection failure — try Tailscale IP fallback.
	if len(sessions) == 0 && len(peer.TailscaleIPs) > 0 {
		ipClient := &http.Client{
			Timeout: 5 * time.Second,
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{
					ServerName: fqdn,
					MinVersion: tls.VersionTLS12,
				},
			},
		}
		ipURL := fmt.Sprintf("https://%s:%d/api/sessions", peer.TailscaleIPs[0], DefaultProbePort)
		host := fmt.Sprintf("%s:%d", fqdn, DefaultProbePort)
		sessions = doFetchSessions(ctx, ipURL, ipClient, host)
	}
	// Enrich each session with peer context and URL.
	for i := range sessions {
		sessions[i].Hostname = peer.Hostname
		sessions[i].FQDN = fqdn
		if sessions[i].URL == "" {
			sessions[i].URL = fmt.Sprintf("https://%s:%d/sessions/%s", fqdn, DefaultProbePort, sessions[i].ID)
		}
	}
	return sessions
}

// FetchPeerSessionsWithClient fetches sessions using an injected HTTP client.
// Used in tests to bypass TLS with httptest servers.
func FetchPeerSessionsWithClient(ctx context.Context, baseURL string, client *http.Client) []PeerSession {
	url := baseURL + "/api/sessions"
	return doFetchSessions(ctx, url, client, "")
}

// FetchAllPeerSessions discovers and fetches sessions from all given peers
// concurrently (max 5 goroutines). Returns groups sorted by hostname.
func FetchAllPeerSessions(ctx context.Context, peers []Peer) []PeerSessionGroup {
	var mu sync.Mutex
	groupMap := make(map[string][]PeerSession)

	g, gctx := errgroup.WithContext(ctx)
	g.SetLimit(5)
	for _, p := range peers {
		p := p
		g.Go(func() error {
			sessions := FetchPeerSessions(gctx, p)
			if len(sessions) == 0 {
				return nil
			}
			mu.Lock()
			groupMap[p.Hostname] = append(groupMap[p.Hostname], sessions...)
			mu.Unlock()
			return nil
		})
	}
	_ = g.Wait()

	groups := make([]PeerSessionGroup, 0, len(groupMap))
	for hostname, sess := range groupMap {
		groups = append(groups, PeerSessionGroup{Hostname: hostname, Sessions: sess})
	}
	sort.Slice(groups, func(i, j int) bool { return groups[i].Hostname < groups[j].Hostname })
	return groups
}

// ShareableSessionMeta contains the minimum metadata needed to list and pick a
// remote session. Returned by GET /api/sessions/meta (open endpoint, no cap
// required). Contains ONLY non-sensitive metadata — never cap tokens, grants, or
// session content (RB-03 no-enumeration contract).
// Credentials are delivered out of band by the owner; this struct carries no creds.
type ShareableSessionMeta struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Status string `json:"status"`
	URL    string `json:"url"`
}

// PeerSessionMetaGroup groups shareable-session metadata by peer hostname.
// Reachable discriminates between an unreachable peer (Reachable=false, Sessions=[])
// and a reachable peer with zero shareable sessions (Reachable=true, Sessions=[]).
// This nil-vs-empty distinction is the RB-04 fix — peers are never silently dropped.
type PeerSessionMetaGroup struct {
	Hostname  string                 `json:"hostname"`
	Reachable bool                   `json:"reachable"`
	Sessions  []ShareableSessionMeta `json:"sessions"`
}

// MetaStatus says how a peer answered GET /api/sessions/meta.
type MetaStatus int

const (
	// MetaUnreachable: transport error, timeout, or an unexpected status.
	MetaUnreachable MetaStatus = iota
	// MetaOK: 200 with a decoded (possibly empty) list.
	MetaOK
	// MetaForbidden: 403 carrying X-Portty: 1, i.e. the peer runs Portty but is
	// signed in to a different Tailscale account. Callers hide these peers.
	MetaForbidden
)

// Errors returned by FetchPeerCap.
var (
	ErrPeerForbidden = errors.New("peer refused access")
	ErrPeerNotShared = errors.New("session is not shared")
)

// doFetchSessionsMeta performs the HTTP GET and JSON decode for /api/sessions/meta.
// If host is non-empty, it is set as the request Host header (used for IP-fallback
// TLS validation). On MetaOK the slice is non-nil (possibly empty). err carries the
// transport or decode error when the status is MetaUnreachable.
func doFetchSessionsMeta(ctx context.Context, url string, client *http.Client, host string) ([]ShareableSessionMeta, MetaStatus, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, MetaUnreachable, err
	}
	if host != "" {
		req.Host = host
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, MetaUnreachable, err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusForbidden && resp.Header.Get(MarkerHeader) == MarkerValue {
		return nil, MetaForbidden, nil
	}
	if resp.StatusCode != http.StatusOK {
		return nil, MetaUnreachable, fmt.Errorf("meta: unexpected status %d", resp.StatusCode)
	}
	items := []ShareableSessionMeta{}
	if err := json.NewDecoder(resp.Body).Decode(&items); err != nil {
		return nil, MetaUnreachable, err
	}
	if items == nil { // body was JSON null
		items = []ShareableSessionMeta{}
	}
	return items, MetaOK, nil
}

// FetchPeerSessionsMeta fetches /api/sessions/meta from a single peer over HTTPS.
// Uses TLS 1.2 minimum and a 5-second timeout.
//
// Tries the DNS name first, then (only when unreachable) falls back to the first
// TailscaleIP with ServerName set to the FQDN for certificate validation
// (Pitfall 6: fresh client per IP-fallback call).
func FetchPeerSessionsMeta(ctx context.Context, peer Peer) ([]ShareableSessionMeta, MetaStatus) {
	fqdn := strings.TrimSuffix(peer.DNSName, ".")
	client := &http.Client{
		Timeout: 5 * time.Second,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{MinVersion: tls.VersionTLS12},
		},
	}
	url := fmt.Sprintf("https://%s:%d/api/sessions/meta", fqdn, DefaultProbePort)
	sessions, status, _ := doFetchSessionsMeta(ctx, url, client, "")
	if status == MetaUnreachable && len(peer.TailscaleIPs) > 0 {
		// The TailscaleIP may include a port (e.g. in test environments); parse it.
		ipHost, ipPort, err := net.SplitHostPort(peer.TailscaleIPs[0])
		var ipURL string
		var hostHeader string
		if err != nil {
			// No embedded port — use the IP as-is with DefaultProbePort.
			ipHost = peer.TailscaleIPs[0]
			ipURL = fmt.Sprintf("https://%s:%d/api/sessions/meta", ipHost, DefaultProbePort)
			hostHeader = fmt.Sprintf("%s:%d", fqdn, DefaultProbePort)
		} else {
			// Embedded port (e.g. test environment: "127.0.0.1:PORT").
			ipURL = fmt.Sprintf("https://%s:%s/api/sessions/meta", ipHost, ipPort)
			hostHeader = fmt.Sprintf("%s:%s", fqdn, ipPort)
		}
		ipClient := &http.Client{
			Timeout: 5 * time.Second,
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{ServerName: fqdn, MinVersion: tls.VersionTLS12},
			},
		}
		sessions, status, _ = doFetchSessionsMeta(ctx, ipURL, ipClient, hostHeader)
	}
	if status != MetaOK {
		return nil, status
	}
	// Enrich URL if empty (defensive — the webserver sets it, but guard for stubs).
	for i := range sessions {
		if sessions[i].URL == "" {
			sessions[i].URL = fmt.Sprintf("https://%s:%d/sessions/%s", fqdn, DefaultProbePort, sessions[i].ID)
		}
	}
	return sessions, MetaOK
}

// FetchPeerSessionsMetaWithClient fetches baseURL + /api/sessions/meta with the
// given client. The daemon's peer connect route and tests use it. err is set only
// when the status is MetaUnreachable, so callers can tell a refused port apart.
func FetchPeerSessionsMetaWithClient(ctx context.Context, baseURL string, client *http.Client) ([]ShareableSessionMeta, MetaStatus, error) {
	return doFetchSessionsMeta(ctx, baseURL+"/api/sessions/meta", client, "")
}

// FetchPeerCap asks a same-account peer for a shared session's capability via
// POST /api/peer/sessions/{id}/cap. Returns ErrPeerForbidden on 403 and
// ErrPeerNotShared on 404. The token is never part of an error.
func FetchPeerCap(ctx context.Context, baseURL, sessionID string, client *http.Client) (string, error) {
	u := baseURL + "/api/peer/sessions/" + neturl.PathEscape(sessionID) + "/cap"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, u, strings.NewReader("{}"))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("peer cap: %w", err)
	}
	defer resp.Body.Close()
	switch resp.StatusCode {
	case http.StatusOK:
	case http.StatusForbidden:
		return "", ErrPeerForbidden
	case http.StatusNotFound:
		return "", ErrPeerNotShared
	default:
		return "", fmt.Errorf("peer cap: unexpected status %d", resp.StatusCode)
	}
	var body struct {
		Cap string `json:"cap"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 64<<10)).Decode(&body); err != nil {
		return "", fmt.Errorf("peer cap: decode response: %w", err)
	}
	if body.Cap == "" {
		return "", errors.New("peer cap: empty capability")
	}
	return body.Cap, nil
}

// FetchAllPeerSessionsMeta fetches shareable-session metadata from all given peers
// concurrently (max 5 goroutines). Unreachable peers stay in the result
// (Reachable=false, Sessions=[]) and so do reachable peers with zero sessions
// (RB-01/RB-04). Peers that answer MetaForbidden (different Tailscale account) are
// dropped entirely. Results are sorted by Hostname for determinism.
func FetchAllPeerSessionsMeta(ctx context.Context, peers []Peer) []PeerSessionMetaGroup {
	return fetchAllPeerSessionsMeta(ctx, peers, FetchPeerSessionsMeta)
}

func fetchAllPeerSessionsMeta(ctx context.Context, peers []Peer, fetch func(context.Context, Peer) ([]ShareableSessionMeta, MetaStatus)) []PeerSessionMetaGroup {
	var mu sync.Mutex
	groups := make([]PeerSessionMetaGroup, 0, len(peers))

	g, gctx := errgroup.WithContext(ctx)
	g.SetLimit(5)
	for _, p := range peers {
		p := p
		g.Go(func() error {
			sessions, status := fetch(gctx, p)
			if status == MetaForbidden {
				return nil
			}
			if sessions == nil {
				sessions = []ShareableSessionMeta{}
			}
			mu.Lock()
			groups = append(groups, PeerSessionMetaGroup{
				Hostname:  p.Hostname,
				Reachable: status == MetaOK,
				Sessions:  sessions,
			})
			mu.Unlock()
			return nil
		})
	}
	_ = g.Wait()
	sort.Slice(groups, func(i, j int) bool { return groups[i].Hostname < groups[j].Hostname })
	return groups
}

// doFetchSessions performs the HTTP request and JSON decode. If host is non-empty,
// it is set as the request Host header (used for IP-fallback TLS validation).
func doFetchSessions(ctx context.Context, url string, client *http.Client, host string) []PeerSession {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil
	}
	if host != "" {
		req.Host = host
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil
	}
	var items []struct {
		ID       string `json:"id"`
		Name     string `json:"name"`
		Status   string `json:"status"`
		Hostname string `json:"hostname"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&items); err != nil {
		return nil
	}
	sessions := make([]PeerSession, 0, len(items))
	for _, item := range items {
		sessions = append(sessions, PeerSession{
			ID:     item.ID,
			Name:   item.Name,
			Status: item.Status,
		})
	}
	return sessions
}
