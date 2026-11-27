package daemon

// Phase 21-06 — Peer desktop token store.
//
// DesktopCapStore holds the standing desktop token of each peer Mac this daemon
// has connected to (POST /peers/desktop), keyed by the peer's full MagicDNS name.
// It is the sibling of RemoteCapStore (keyed by session id): the desktop proxy
// (remote_desktop_ws_proxy.go) reads it so the web view never sees the token.
//
// Security invariant: the store lives in process memory only and nothing in this
// file writes a token to disk. A daemon restart wipes it and the next
// POST /peers/desktop repopulates it. desktop_caps_test.go pins this with a
// source-grep guard, as remote_caps_test.go does for RemoteCapStore.

import (
	"errors"
	"fmt"
	"net/url"
	"strings"
	"sync"
)

type desktopCapEntry struct {
	baseURL string
	peerIP  string
	token   string
}

// DesktopCapStore is a thread-safe in-memory map of peer host → desktop token.
// The zero value is NOT usable: call NewDesktopCapStore.
type DesktopCapStore struct {
	mu      sync.RWMutex
	entries map[string]desktopCapEntry
}

// NewDesktopCapStore constructs an empty store.
func NewDesktopCapStore() *DesktopCapStore {
	return &DesktopCapStore{entries: make(map[string]desktopCapEntry)}
}

// desktopCapKey normalises a host name: lower case, no trailing dot.
func desktopCapKey(host string) string {
	return strings.ToLower(strings.TrimSuffix(strings.TrimSpace(host), "."))
}

// Put stores (baseURL, peerIP, token) for host, replacing any prior entry.
// baseURL must be an https URL with a host so the token never travels in
// cleartext; peerIP may be empty (tests; the verified peer client then fails
// every dial).
func (s *DesktopCapStore) Put(host, baseURL, peerIP, token string) error {
	key := desktopCapKey(host)
	if key == "" {
		return errors.New("desktop caps: host required")
	}
	if token == "" {
		return errors.New("desktop caps: token required")
	}
	u, err := url.Parse(baseURL)
	if err != nil {
		return fmt.Errorf("desktop caps: baseURL parse error: %w", err)
	}
	if u.Scheme != "https" {
		return fmt.Errorf("desktop caps: baseURL must use https scheme, got %q", u.Scheme)
	}
	if u.Host == "" {
		return errors.New("desktop caps: baseURL must have a non-empty host")
	}
	s.mu.Lock()
	s.entries[key] = desktopCapEntry{baseURL: baseURL, peerIP: peerIP, token: token}
	s.mu.Unlock()
	return nil
}

// Get returns the stored tuple for host, or ok=false.
func (s *DesktopCapStore) Get(host string) (baseURL, peerIP, token string, ok bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	e, ok := s.entries[desktopCapKey(host)]
	if !ok {
		return "", "", "", false
	}
	return e.baseURL, e.peerIP, e.token, true
}

// Delete removes host's entry. No-op when unknown.
func (s *DesktopCapStore) Delete(host string) {
	s.mu.Lock()
	delete(s.entries, desktopCapKey(host))
	s.mu.Unlock()
}
