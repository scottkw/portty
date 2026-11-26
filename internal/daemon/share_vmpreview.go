//go:build vmpreview

// VM-only fake for the native GUI check in the Tart VM, which has no
// Tailscale. Never in a release build: the vmpreview tag must not appear in
// .github, build.sh or scripts (tests/check-brand.sh enforces this in CI).
// Shares are served on 127.0.0.1:7444 with a self-signed certificate.
//
// The MOB-05 same-account gate (Phase 14) asks tailscaled who the caller is, and a loopback
// caller is never a tailnet peer, so every loopback link was refused with 403 before the
// capability check. vmLoopbackWhoIs answers that a 127.0.0.1 caller is this Mac's own user and
// anything else is not found; the link-token check after the gate runs unchanged (18-07 R1).

package daemon

import (
	"context"
	"net"

	"github.com/scottkw/portty/internal/relay"
	"github.com/scottkw/portty/internal/tailnet"
	"github.com/scottkw/portty/internal/webserver"
	"tailscale.com/client/local"
	"tailscale.com/client/tailscale/apitype"
	"tailscale.com/ipn/ipnstate"
	"tailscale.com/tailcfg"
)

func init() {
	defaultShareHealth = func(context.Context) webserver.TailscaleHealth {
		return webserver.TailscaleHealth{Connected: true, HasCerts: true, IP: "127.0.0.1", Domain: "127.0.0.1"}
	}
	defaultNewShareServer = func(_ webserver.TailscaleHealth, m *relay.HubManager) (*webserver.WebServer, error) {
		cfg, err := webserver.GenerateSelfSignedCert("127.0.0.1")
		if err != nil {
			return nil, err
		}
		ws, err := webserver.NewWebServer(webserver.Config{BindIP: "127.0.0.1", Port: tailnet.DefaultProbePort, FQDN: "127.0.0.1", TLSConfig: cfg}, m)
		if err != nil {
			return nil, err
		}
		ws.SetWhoIsClientForTest(vmLoopbackWhoIs{})
		return ws, nil
	}
	defaultWarmCert = func(context.Context, string) error { return nil }
}

const vmLoopbackUser tailcfg.UserID = 1

type vmLoopbackWhoIs struct{}

func (vmLoopbackWhoIs) StatusWithoutPeers(context.Context) (*ipnstate.Status, error) {
	return &ipnstate.Status{Self: &ipnstate.PeerStatus{UserID: vmLoopbackUser}}, nil
}

func (vmLoopbackWhoIs) WhoIs(_ context.Context, remoteAddr string) (*apitype.WhoIsResponse, error) {
	if host, _, err := net.SplitHostPort(remoteAddr); err == nil && host == "127.0.0.1" {
		return &apitype.WhoIsResponse{Node: &tailcfg.Node{User: vmLoopbackUser}}, nil
	}
	return nil, local.ErrPeerNotFound
}
