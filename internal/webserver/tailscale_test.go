package webserver

import (
	"context"
	"errors"
	"fmt"
	"net/netip"
	"testing"

	"github.com/scottkw/portty/internal/tailnet"
	"tailscale.com/ipn/ipnstate"
)

func statusOK(s *ipnstate.Status) statusFunc {
	return func(ctx context.Context) (*ipnstate.Status, error) { return s, nil }
}

// TestLiveTailnetGuard_Webserver pins WR-07: in a test binary CheckHealth and
// the default WhoIs client never ask the user's tailscaled.
func TestLiveTailnetGuard_Webserver(t *testing.T) {
	if tailnet.LiveAllowed() {
		t.Skip("PORTTY_LIVE_TAILNET_TESTS=1: live tailnet access deliberately enabled")
	}
	if h := CheckHealth(context.Background()); h != (TailscaleHealth{}) {
		t.Errorf("CheckHealth = %+v; want zero value under the guard", h)
	}
	ws, err := NewWebServer(Config{BindIP: "127.0.0.1"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ws.whois.StatusWithoutPeers(context.Background()); !errors.Is(err, tailnet.ErrLiveTailnetInTest) {
		t.Errorf("default whois StatusWithoutPeers err = %v; want ErrLiveTailnetInTest", err)
	}
}

// TestCheckHealth_LocalAPIUnreachable: a LocalAPI error reads as not connected (fail closed, T-01-40).
func TestCheckHealth_LocalAPIUnreachable(t *testing.T) {
	prefsCalled := false
	h := checkHealth(context.Background(), func(ctx context.Context) (*ipnstate.Status, error) {
		return nil, fmt.Errorf("dial unix: connection refused")
	}, func(ctx context.Context) (bool, error) {
		prefsCalled = true
		return true, nil
	})
	if h != (TailscaleHealth{}) {
		t.Errorf("expected zero value for unreachable LocalAPI, got %+v", h)
	}
	if prefsCalled {
		t.Error("prefs probe must not run when the LocalAPI is unreachable")
	}
}

func TestCheckHealth_BackendState(t *testing.T) {
	tests := []struct {
		state         string
		wantConnected bool
	}{
		{state: "Stopped", wantConnected: false},
		{state: "NeedsLogin", wantConnected: false},
		{state: "NeedsMachineAuth", wantConnected: false},
		{state: "Starting", wantConnected: false},
		{state: "Running", wantConnected: true},
	}

	for _, tc := range tests {
		t.Run(tc.state, func(t *testing.T) {
			h := checkHealth(context.Background(), statusOK(&ipnstate.Status{
				BackendState: tc.state,
				TailscaleIPs: []netip.Addr{netip.MustParseAddr("100.64.0.1")},
				CertDomains:  []string{"host.ts.net"},
			}), nil)
			if h.Connected != tc.wantConnected {
				t.Errorf("state=%s: expected Connected=%v, got %v", tc.state, tc.wantConnected, h.Connected)
			}
			if !tc.wantConnected && h != (TailscaleHealth{}) {
				t.Errorf("state=%s: expected zero value when not connected, got %+v", tc.state, h)
			}
		})
	}
}

func TestCheckHealth_CertDomains(t *testing.T) {
	tests := []struct {
		name         string
		domains      []string
		wantHasCerts bool
		wantDomain   string
	}{
		{name: "no cert domains", domains: nil, wantHasCerts: false, wantDomain: ""},
		{name: "has cert domain", domains: []string{"host.ts.net"}, wantHasCerts: true, wantDomain: "host.ts.net"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			h := checkHealth(context.Background(), statusOK(&ipnstate.Status{
				BackendState: "Running",
				CertDomains:  tc.domains,
			}), nil)
			if h.HasCerts != tc.wantHasCerts {
				t.Errorf("expected HasCerts=%v, got %v", tc.wantHasCerts, h.HasCerts)
			}
			if h.Domain != tc.wantDomain {
				t.Errorf("expected Domain=%q, got %q", tc.wantDomain, h.Domain)
			}
		})
	}
}

func TestCheckHealth_FullyHealthy(t *testing.T) {
	h := checkHealth(context.Background(), statusOK(&ipnstate.Status{
		BackendState: "Running",
		TailscaleIPs: []netip.Addr{netip.MustParseAddr("100.64.0.1")},
		CertDomains:  []string{"myhost.tail46d69a.ts.net"},
	}), nil)

	want := TailscaleHealth{
		Connected: true,
		HasCerts:  true,
		IP:        "100.64.0.1",
		Domain:    "myhost.tail46d69a.ts.net",
	}
	if h != want {
		t.Errorf("expected %+v, got %+v", want, h)
	}
}

func TestCheckHealth_AcceptDNS(t *testing.T) {
	tests := []struct {
		name          string
		corpDNS       bool
		prefsErr      error
		wantAcceptDNS bool
	}{
		{name: "CorpDNS=false", corpDNS: false, wantAcceptDNS: false},
		{name: "CorpDNS=true", corpDNS: true, wantAcceptDNS: true},
		{name: "prefs error", corpDNS: true, prefsErr: fmt.Errorf("localapi prefs unavailable"), wantAcceptDNS: false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			h := checkHealth(context.Background(), statusOK(&ipnstate.Status{BackendState: "Running"}),
				func(ctx context.Context) (bool, error) { return tc.corpDNS, tc.prefsErr })
			if h.AcceptDNS != tc.wantAcceptDNS {
				t.Errorf("AcceptDNS = %v; want %v", h.AcceptDNS, tc.wantAcceptDNS)
			}
		})
	}
}
