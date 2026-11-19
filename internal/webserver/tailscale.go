package webserver

import (
	"context"

	"github.com/scottkw/portty/internal/tailnet"
	"tailscale.com/client/local"
	"tailscale.com/ipn/ipnstate"
)

// TailscaleHealth is the result of a connection check against the local tailscaled
// LocalAPI. Serialised to JSON and returned to the Wails frontend via GetTailscaleStatus().
// The zero value means "not connected".
type TailscaleHealth struct {
	Connected bool   `json:"connected"` // LocalAPI answered and BackendState == "Running"
	HasCerts  bool   `json:"hasCerts"`  // len(CertDomains) > 0
	IP        string `json:"ip"`        // first TailscaleIP as string, empty if not connected
	Domain    string `json:"domain"`    // first CertDomain e.g. hostname.ts.net, empty if none
	// AcceptDNS is true when the local Tailscale node has accept-dns enabled (the default).
	// false when not connected or prefs are unavailable (safe zero value). (DNS-03)
	AcceptDNS bool `json:"acceptDns"`
}

// statusFunc is the injectable status function type for testability.
type statusFunc func(ctx context.Context) (*ipnstate.Status, error)

// prefsFunc is the injectable prefs-probe function type for testability.
// It returns the CorpDNS (accept-dns) boolean and an error.
type prefsFunc func(ctx context.Context) (bool, error)

// checkHealth is the internal testable health check. It fails closed: any LocalAPI
// error, or any backend state other than Running, reads as not connected (T-01-40).
func checkHealth(ctx context.Context, fn statusFunc, pf prefsFunc) TailscaleHealth {
	var h TailscaleHealth
	status, err := fn(ctx)
	if err != nil {
		return h
	}
	h.Connected = status.BackendState == "Running"
	if !h.Connected {
		return h
	}
	h.HasCerts = len(status.CertDomains) > 0
	if len(status.TailscaleIPs) > 0 {
		h.IP = status.TailscaleIPs[0].String()
	}
	if len(status.CertDomains) > 0 {
		h.Domain = status.CertDomains[0]
	}
	// A prefs failure leaves AcceptDNS false (unknown / assume disabled for warning).
	if pf != nil {
		if corpDNS, prefsErr := pf(ctx); prefsErr == nil {
			h.AcceptDNS = corpDNS
		}
	}
	return h
}

// realPrefsFunc returns a prefsFunc that calls lc.GetPrefs and extracts CorpDNS.
func realPrefsFunc(lc *local.Client) prefsFunc {
	return func(ctx context.Context) (bool, error) {
		prefs, err := lc.GetPrefs(ctx)
		if err != nil {
			return false, err
		}
		return prefs.CorpDNS, nil
	}
}

// CheckHealth queries tailscaled via local.Client and returns TailscaleHealth.
// ctx should carry a short timeout (3-5 seconds) to prevent UI hangs.
func CheckHealth(ctx context.Context) TailscaleHealth {
	if !tailnet.LiveAllowed() {
		return TailscaleHealth{} // not connected (WR-07)
	}
	var lc local.Client
	return checkHealth(ctx, lc.StatusWithoutPeers, realPrefsFunc(&lc))
}
