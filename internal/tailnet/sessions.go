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
	"syscall"
	"time"

	"golang.org/x/sync/errgroup"
)

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
	DNSName   string                 `json:"dnsName"` // MagicDNS FQDN, no trailing dot: the unambiguous connect key
	Reachable bool                   `json:"reachable"`
	Sessions  []ShareableSessionMeta `json:"sessions"`
	// Desktop is true when the peer's meta answer carried DesktopHeader: DesktopAlways,
	// i.e. its always-available desktop is on.
	Desktop bool `json:"desktop"`
}

// DesktopHeader is the response header on GET /api/sessions/meta that says the
// peer's always-available desktop is on. Only the exact value DesktopAlways means
// so: a later value is ignored, never misread. The /desktop/ routes use the same
// header name for their refusal reason (values token and off); the two never share
// a response.
const (
	DesktopHeader = "X-Portty-Desktop"
	DesktopAlways = "always"
)

// PeerMeta is one peer's GET /api/sessions/meta answer: the session list, how the
// peer answered, and whether it announced an always-available desktop.
type PeerMeta struct {
	Sessions []ShareableSessionMeta
	Status   MetaStatus
	Desktop  bool
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
	// MetaRefused: the peer answered the connection with a reset, so nothing
	// listens on its share port. A Portty host stops its share server when its
	// last share ends, so the Mac is reachable with nothing shared. A timeout or
	// no route is not this.
	MetaRefused
)

// Errors returned by FetchPeerCap.
var (
	ErrPeerForbidden = errors.New("peer refused access")
	ErrPeerNotShared = errors.New("session is not shared")
	// ErrPeerDesktopOff: the peer answered 404 to the desktop cap request, so its
	// always-available desktop is off (or its signing key does not exist yet).
	ErrPeerDesktopOff = errors.New("peer desktop is off")
)

// doFetchSessionsMeta performs the HTTP GET and JSON decode for /api/sessions/meta.
// If host is non-empty, it is set as the request Host header (used for IP-fallback
// TLS validation). On MetaOK the slice is non-nil (possibly empty) and Desktop is
// true only when DesktopHeader equals DesktopAlways. err carries the transport or
// decode error for MetaUnreachable and the transport error for MetaRefused (the
// daemon's peer connect route reads ECONNREFUSED from it).
func doFetchSessionsMeta(ctx context.Context, url string, client *http.Client, host string) (PeerMeta, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return PeerMeta{Status: MetaUnreachable}, err
	}
	if host != "" {
		req.Host = host
	}
	resp, err := client.Do(req)
	if err != nil {
		if errors.Is(err, syscall.ECONNREFUSED) {
			return PeerMeta{Status: MetaRefused}, err
		}
		return PeerMeta{Status: MetaUnreachable}, err
	}
	defer resp.Body.Close()
	// The forbidden branch comes first: a different-account Mac never yields Desktop.
	if resp.StatusCode == http.StatusForbidden && resp.Header.Get(MarkerHeader) == MarkerValue {
		return PeerMeta{Status: MetaForbidden}, nil
	}
	if resp.StatusCode != http.StatusOK {
		return PeerMeta{Status: MetaUnreachable}, fmt.Errorf("meta: unexpected status %d", resp.StatusCode)
	}
	items := []ShareableSessionMeta{}
	if err := json.NewDecoder(resp.Body).Decode(&items); err != nil {
		return PeerMeta{Status: MetaUnreachable}, err
	}
	if items == nil { // body was JSON null
		items = []ShareableSessionMeta{}
	}
	return PeerMeta{
		Sessions: items,
		Status:   MetaOK,
		Desktop:  strings.TrimSpace(resp.Header.Get(DesktopHeader)) == DesktopAlways,
	}, nil
}

// FetchPeerSessionsMeta is FetchPeerMeta without the desktop flag.
func FetchPeerSessionsMeta(ctx context.Context, peer Peer) ([]ShareableSessionMeta, MetaStatus) {
	m := FetchPeerMeta(ctx, peer)
	return m.Sessions, m.Status
}

// FetchPeerMeta fetches /api/sessions/meta from a single peer over HTTPS.
// Uses TLS 1.2 minimum and a 5-second timeout.
//
// Tries the DNS name first, then (only when unreachable) falls back to the first
// TailscaleIP with ServerName set to the FQDN for certificate validation
// (Pitfall 6: fresh client per IP-fallback call).
func FetchPeerMeta(ctx context.Context, peer Peer) PeerMeta {
	if !LiveAllowed() {
		return PeerMeta{Status: MetaUnreachable}
	}
	fqdn := strings.TrimSuffix(peer.DNSName, ".")
	client := &http.Client{
		Timeout: 5 * time.Second,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{MinVersion: tls.VersionTLS12},
		},
	}
	url := fmt.Sprintf("https://%s:%d/api/sessions/meta", fqdn, DefaultProbePort)
	meta, _ := doFetchSessionsMeta(ctx, url, client, "")
	// Only MetaUnreachable falls back to the IP: a refusal by the DNS name is the
	// host's own answer (nothing shared), so retrying by IP would not change it.
	if meta.Status == MetaUnreachable && len(peer.TailscaleIPs) > 0 {
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
		meta, _ = doFetchSessionsMeta(ctx, ipURL, ipClient, hostHeader)
	}
	if meta.Status != MetaOK {
		return PeerMeta{Status: meta.Status}
	}
	// Enrich URL if empty (defensive — the webserver sets it, but guard for stubs).
	for i := range meta.Sessions {
		if meta.Sessions[i].URL == "" {
			meta.Sessions[i].URL = fmt.Sprintf("https://%s:%d/sessions/%s", fqdn, DefaultProbePort, meta.Sessions[i].ID)
		}
	}
	return meta
}

// FetchPeerSessionsMetaWithClient fetches baseURL + /api/sessions/meta with the
// given client. The daemon's peer connect route and tests use it. err is set for
// MetaUnreachable and MetaRefused; the daemon's peer connect route reads
// ECONNREFUSED from it to tell a refused port apart.
func FetchPeerSessionsMetaWithClient(ctx context.Context, baseURL string, client *http.Client) ([]ShareableSessionMeta, MetaStatus, error) {
	m, err := FetchPeerMetaWithClient(ctx, baseURL, client)
	return m.Sessions, m.Status, err
}

// FetchPeerMetaWithClient is FetchPeerSessionsMetaWithClient with the desktop flag.
func FetchPeerMetaWithClient(ctx context.Context, baseURL string, client *http.Client) (PeerMeta, error) {
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

// FetchPeerDesktopCap asks a same-account peer for its standing always-available
// desktop token via POST /api/peer/desktop/cap, a twin of FetchPeerCap. Returns
// ErrPeerForbidden on 403 and ErrPeerDesktopOff on 404: always-available is off on
// that Mac (turning it off kills the token) or it has no signing key yet. The
// token is never part of an error.
func FetchPeerDesktopCap(ctx context.Context, baseURL string, client *http.Client) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, baseURL+"/api/peer/desktop/cap", strings.NewReader("{}"))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("peer desktop cap: %w", err)
	}
	defer resp.Body.Close()
	switch resp.StatusCode {
	case http.StatusOK:
	case http.StatusForbidden:
		return "", ErrPeerForbidden
	case http.StatusNotFound:
		return "", ErrPeerDesktopOff
	default:
		return "", fmt.Errorf("peer desktop cap: unexpected status %d", resp.StatusCode)
	}
	var body struct {
		Cap string `json:"cap"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 64<<10)).Decode(&body); err != nil {
		return "", fmt.Errorf("peer desktop cap: decode response: %w", err)
	}
	if body.Cap == "" {
		return "", errors.New("peer desktop cap: empty capability")
	}
	return body.Cap, nil
}

// FetchAllPeerSessionsMeta fetches shareable-session metadata from all given peers
// concurrently (max 5 goroutines). Unreachable peers stay in the result
// (Reachable=false, Sessions=[]) and so do reachable peers with zero sessions
// (RB-01/RB-04). A peer that refuses the connection (MetaRefused) is reachable with
// zero sessions: its share server is stopped because nothing is shared. Peers that answer MetaForbidden (different Tailscale account) are
// dropped entirely. Results are sorted by Hostname for determinism.
func FetchAllPeerSessionsMeta(ctx context.Context, peers []Peer) []PeerSessionMetaGroup {
	return fetchAllPeerMeta(ctx, peers, FetchPeerMeta)
}

// fetchAllPeerSessionsMeta is the two-value fetch seam: fetchAllPeerMeta with
// Desktop always false.
func fetchAllPeerSessionsMeta(ctx context.Context, peers []Peer, fetch func(context.Context, Peer) ([]ShareableSessionMeta, MetaStatus)) []PeerSessionMetaGroup {
	return fetchAllPeerMeta(ctx, peers, func(ctx context.Context, p Peer) PeerMeta {
		sessions, status := fetch(ctx, p)
		return PeerMeta{Sessions: sessions, Status: status}
	})
}

func fetchAllPeerMeta(ctx context.Context, peers []Peer, fetch func(context.Context, Peer) PeerMeta) []PeerSessionMetaGroup {
	var mu sync.Mutex
	groups := make([]PeerSessionMetaGroup, 0, len(peers))

	g, gctx := errgroup.WithContext(ctx)
	g.SetLimit(5)
	for _, p := range peers {
		p := p
		g.Go(func() error {
			m := fetch(gctx, p)
			sessions, status := m.Sessions, m.Status
			if status == MetaForbidden {
				return nil
			}
			if sessions == nil {
				sessions = []ShareableSessionMeta{}
			}
			mu.Lock()
			groups = append(groups, PeerSessionMetaGroup{
				Hostname:  p.Hostname,
				DNSName:   strings.TrimSuffix(p.DNSName, "."),
				Reachable: status == MetaOK || status == MetaRefused,
				Sessions:  sessions,
				Desktop:   m.Desktop,
			})
			mu.Unlock()
			return nil
		})
	}
	_ = g.Wait()
	sort.Slice(groups, func(i, j int) bool { return groups[i].Hostname < groups[j].Hostname })
	return groups
}
